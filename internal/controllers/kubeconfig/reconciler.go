package kubeconfig

import (
	"context"

	"github.com/reddit/achilles-sdk/pkg/fsm"
	"github.com/reddit/achilles-sdk/pkg/fsm/types"
	"github.com/reddit/achilles-sdk/pkg/io"
	"github.com/reddit/achilles-sdk/pkg/logging"
	"github.com/reddit/achilles-sdk/pkg/meta"
	"github.com/reddit/achilles-sdk/pkg/sets"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/klaudworks/kubeconfig-operator/api/klaud.works/v1alpha1"
	"github.com/klaudworks/kubeconfig-operator/internal/controlplane"
	"github.com/klaudworks/kubeconfig-operator/internal/serviceaccount"
	"github.com/klaudworks/kubeconfig-operator/internal/token"
	"github.com/klaudworks/kubeconfig-operator/internal/util"
)

// These kubebuilder markers[0] define the access (RBAC) requirements for the
// controller. They are used to produced appropriate Roles (manifests) that can
// be applied to the cluster. You should add a marker for resource/verb
// combination.
//
// [0]: https://book.kubebuilder.io/reference/markers/rbac.html

// +kubebuilder:rbac:groups=klaud.works,resources=kubeconfigs;kubeconfigs/status,verbs=*
// +kubebuilder:rbac:groups="",resources=secrets,verbs=*
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=*
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=*
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=*

const (
	controllerName = "Kubeconfig"
)

type state = types.State[*v1alpha1.Kubeconfig]

type reconciler struct {
	c          *io.ClientApplicator
	scheme     *runtime.Scheme
	log        *zap.SugaredLogger
	kubeClient *kubernetes.Clientset
	caCrtData  []byte
}

func (r *reconciler) provisionServiceAccount() *state {
	return &state{
		Name:      "provision-service-account",
		Condition: conditionServiceAccountProvisioned,
		Transition: func(
			ctx context.Context,
			kubeconfig *v1alpha1.Kubeconfig,
			out *types.OutputSet,
		) (*state, types.Result) {
			builder := serviceaccount.NewBuilder(kubeconfig)

			outputs := builder.Build()
			for _, o := range outputs {
				var applyOpts []io.ApplyOption

				// Avoid owner refs for cluster-scoped or cross-namespace objects.
				switch o.(type) {
				case *rbacv1.ClusterRole:
					applyOpts = append(applyOpts, io.WithoutOwnerRefs())
				case *rbacv1.ClusterRoleBinding:
					applyOpts = append(applyOpts, io.WithoutOwnerRefs())
				default:
					if o.GetNamespace() != kubeconfig.GetNamespace() {
						applyOpts = append(applyOpts, io.WithoutOwnerRefs())
					}
				}

				out.Apply(o, applyOpts...)
			}

			kubeconfig.Status.ServiceAccountRef = ptr.To(builder.ServiceAccount().Name)
			return r.deleteStalePermissions(outputs), types.DoneResult()
		},
	}
}

func (r *reconciler) deleteStalePermissions(desiredObjs []client.Object) *state {
	return &state{
		Name:      "delete-stale-permissions",
		Condition: conditionStalePermissionsRemoved,
		Transition: func(
			ctx context.Context,
			kubeconfig *v1alpha1.Kubeconfig,
			out *types.OutputSet,
		) (*state, types.Result) {
			desired := sets.NewObjectSet(r.scheme, desiredObjs...)
			actual := sets.NewObjectSet(r.scheme)

			// get all existing managed resources
			for _, ref := range kubeconfig.Status.ResourceRefs {
				obj, err := meta.NewObjectForGVK(r.scheme, ref.GroupVersionKind())

				if err != nil {
					return nil, types.ErrorResultf("constructing new %T %s: %s", obj, client.ObjectKeyFromObject(obj), err)
				}
				obj.SetName(ref.Name)
				obj.SetNamespace(ref.Namespace)

				if err := r.c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {

					if errors.IsNotFound(err) {
						// warn for managed resource that wasn't explicitly deleted by the controller, but is deleted on the kube-apiserver
						r.log.Warnf("managed resource %T %s is not found", obj, client.ObjectKeyFromObject(obj))
						continue
					}
					return nil, types.ErrorResultf("getting managed object %T %s: %s", obj, client.ObjectKeyFromObject(obj), err)
				}

				// skip non-permission resources like the kubeconfig secret
				if obj.GetLabels()["kubeconfig-operator/type"] != "permission" {
					continue
				}

				actual.Insert(obj)
			}

			// delete stale permissions
			for _, staleObj := range actual.Difference(desired).List() {
				out.Delete(staleObj)
			}

			if kubeconfig.DeletionTimestamp != nil {
				return nil, types.DoneResult()
			}
			return r.provisionKubeconfig(), types.DoneResult()
		},
	}
}

func (r *reconciler) provisionKubeconfig() *state {
	return &state{
		Name:      "provision-kubeconfig",
		Condition: conditionKubeconfigProvisioned,
		Transition: func(
			ctx context.Context,
			kubeconfig *v1alpha1.Kubeconfig,
			out *types.OutputSet,
		) (*state, types.Result) {

			namespace := kubeconfig.GetNamespace()

			if kubeconfig.Status.ServiceAccountRef == nil {
				return nil, types.ErrorResultf("missing service account reference in status")
			}
			saName := *kubeconfig.Status.ServiceAccountRef

			// Retrieve the ServiceAccount.
			sa := &corev1.ServiceAccount{}
			if err := r.c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: saName}, sa); err != nil {
				return nil, types.ErrorResultf("failed to get service account %s/%s: %v", namespace, saName, err)
			}

			// Parse ExpirationTTL (expects a value like "365d").
			expirationSeconds, err := util.ParseExpirationTTL(kubeconfig.Spec.ExpirationTTL)
			if err != nil {
				return nil, types.ErrorResultf("failed to parse expirationTTL: %v", err)
			}

			// Use the token package to ensure the token is valid or refreshed.
			tokenInfo, refreshed, err := token.EnsureToken(ctx, r.kubeClient, namespace, saName, expirationSeconds, kubeconfig.Status.ServiceAccountTokenExpiration)
			if err != nil {
				return nil, types.ErrorResultf("%v", err)
			}
			if !refreshed {
				// Existing token is still valid. No need to create a new kubeconfig.
				return nil, types.DoneResult()
			}

			if len(r.caCrtData) == 0 {
				return nil, types.ErrorResultf("cluster CA certificate data is not available")
			}

			server := kubeconfig.Spec.Server
			clusterName := kubeconfig.Spec.ClusterName

			// Build and serialize a new kubeconfig using the requested token.
			kc := &clientcmdapi.Config{
				CurrentContext: clusterName,
				Clusters: map[string]*clientcmdapi.Cluster{
					clusterName: {
						Server:                   server,
						CertificateAuthorityData: r.caCrtData,
					},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					saName: {
						Token: tokenInfo.Token,
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					clusterName: {
						Cluster:   clusterName,
						AuthInfo:  saName,
						Namespace: namespace,
					},
				},
			}

			kubeconfigBytes, err := clientcmd.Write(*kc)
			if err != nil {
				return nil, types.ErrorResultf("failed to serialize kubeconfig: %v", err)
			}

			// Create or update the secret containing the kubeconfig.
			secretName := kubeconfig.GetName()
			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(kubeconfig, v1alpha1.GroupVersion.WithKind("Kubeconfig")),
					},
				},
				Data: map[string][]byte{
					"kubeconfig": kubeconfigBytes,
				},
				Type: corev1.SecretTypeOpaque,
			}

			out.Apply(kubeconfigSecret)

			// Update the kubeconfig status with the new token details.
			kubeconfig.Status.KubeconfigSecretRef = ptr.To(secretName)
			kubeconfig.Status.ServiceAccountTokenExpiration = ptr.To(tokenInfo.ExpirationTimestamp)
			kubeconfig.Status.ServiceAccountTokenRefresh = ptr.To(tokenInfo.RefreshTimestamp)

			return nil, types.DoneResult()
		},
	}
}

func SetupController(
	ctx context.Context,
	cpCtx controlplane.Context,
	mgr ctrl.Manager,
	rl workqueue.RateLimiter,
	c *io.ClientApplicator,
) error {
	_, log, err := logging.ControllerCtx(ctx, controllerName)
	if err != nil {
		return err
	}

	cfg := mgr.GetConfig()
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	r := &reconciler{
		c:          c,
		scheme:     mgr.GetScheme(),
		log:        log,
		kubeClient: kubeClient,
		caCrtData:  cfg.CAData,
	}

	builder := fsm.NewBuilder(
		&v1alpha1.Kubeconfig{},
		r.provisionServiceAccount(),
		mgr.GetScheme(),
	).Manages(
		corev1.SchemeGroupVersion.WithKind("Secret"),
		corev1.SchemeGroupVersion.WithKind("ServiceAccount"),
		rbacv1.SchemeGroupVersion.WithKind("Role"),
		rbacv1.SchemeGroupVersion.WithKind("RoleBinding"),
		rbacv1.SchemeGroupVersion.WithKind("ClusterRole"),
		rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"),
	).WithFinalizerState(
		// NOTE: we can't rely on native Kubernetes GC to delete cluster scoped resources (ClusterRole, ClusterRoleBinding)
		// or cross-namespace resources (Roles, RoleBindings) so we need to handle this ourselves
		r.deleteStalePermissions(nil),
	)

	return builder.Build()(mgr, log, rl, cpCtx.Metrics)
}
