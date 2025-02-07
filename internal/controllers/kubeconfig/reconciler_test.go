package kubeconfig_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/klaudworks/kubeconfig-operator/api/klaud.works/v1alpha1"
)

var _ = Describe("KubeconfigReconciler", Ordered, func() {
	var (
		ctx        = context.Background()
		kubeconfig *v1alpha1.Kubeconfig
	)

	BeforeEach(func() {
		// Create a Kubeconfig object with two namespaced permissions and one cluster-wide permission.
		kubeconfig = &v1alpha1.Kubeconfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foobar",
				Namespace: "default",
			},
			Spec: v1alpha1.KubeconfigSpec{
				Server:      "https://kubernetes.example.com",
				ClusterName: "kubernetes",
				NamespacedPermissions: []v1alpha1.NamespacedPermissions{
					{
						Namespace: "default",
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"configmaps"},
								Verbs:     []string{"*"},
							},
						},
					},
					{
						Namespace: "kube-system",
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"configmaps"},
								Verbs:     []string{"get", "list", "watch"},
							},
						},
					},
				},
				ClusterPermissions: &v1alpha1.ClusterPermissions{
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
							Verbs:     []string{"get", "list", "watch"},
						},
					},
				},
			},
		}

		// Create the Kubeconfig resource
		Expect(c.Create(ctx, kubeconfig)).To(Succeed())
	})

	It("should reconcile Kubeconfig objects", func() {
		By("provisioning resources required for the kubeconfig")

		// 1. ServiceAccount Token Secret (created by the builder)
		Eventually(func(g Gomega) {
			expectedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name, // same as the created ServiceAccount name
					Namespace: kubeconfig.Namespace,
				},
			}
			actualSecret := &corev1.Secret{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(Succeed())
			g.Expect(actualSecret.Type).To(Equal(corev1.SecretTypeServiceAccountToken))
			g.Expect(actualSecret.Annotations).To(HaveKeyWithValue("kubernetes.io/service-account.name", kubeconfig.Name))
		}).Should(Succeed())

		// 2. ServiceAccount creation
		Eventually(func(g Gomega) {
			expectedSA := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: kubeconfig.Namespace,
				},
			}
			actualSA := &corev1.ServiceAccount{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSA), actualSA)).To(Succeed())
		}).Should(Succeed())

		// SIMULATE TOKEN INJECTION:
		// In a real environment, the service account token secret would be populated automatically.
		// For testing, patch the secret (named "foobar") with fake token data.
		By("simulating token injection on the service account token secret")
		Eventually(func(g Gomega) {
			saSecret := &corev1.Secret{}
			g.Expect(c.Get(ctx, client.ObjectKey{Namespace: kubeconfig.Namespace, Name: kubeconfig.Name}, saSecret)).To(Succeed())
			if saSecret.Data == nil {
				saSecret.Data = map[string][]byte{}
			}
			saSecret.Data["token"] = []byte("fake-token")
			saSecret.Data["ca.crt"] = []byte("fake-ca-crt")
			g.Expect(c.Update(ctx, saSecret)).To(Succeed())
		}).Should(Succeed())

		// 3. Role and RoleBinding for "default" namespace permission
		Eventually(func(g Gomega) {
			expectedRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: "default",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"configmaps"},
						Verbs:     []string{"*"},
					},
				},
			}
			expectedRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: "default",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "Role",
					Name:     kubeconfig.Name,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      kubeconfig.Name,
						Namespace: kubeconfig.Namespace,
					},
				},
			}

			actualRole := &rbacv1.Role{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedRole), actualRole)).To(Succeed())
			g.Expect(actualRole.Rules).To(Equal(expectedRole.Rules))

			actualRoleBinding := &rbacv1.RoleBinding{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedRoleBinding), actualRoleBinding)).To(Succeed())
			g.Expect(actualRoleBinding.RoleRef).To(Equal(expectedRoleBinding.RoleRef))
			g.Expect(actualRoleBinding.Subjects).To(Equal(expectedRoleBinding.Subjects))
		}).Should(Succeed())

		// 4. Role and RoleBinding for "kube-system" namespace permission
		Eventually(func(g Gomega) {
			expectedRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: "kube-system",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"configmaps"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			}
			expectedRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: "kube-system",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "Role",
					Name:     kubeconfig.Name,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      kubeconfig.Name,
						Namespace: kubeconfig.Namespace,
					},
				},
			}

			actualRole := &rbacv1.Role{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedRole), actualRole)).To(Succeed())
			g.Expect(actualRole.Rules).To(Equal(expectedRole.Rules))

			actualRoleBinding := &rbacv1.RoleBinding{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedRoleBinding), actualRoleBinding)).To(Succeed())
			g.Expect(actualRoleBinding.RoleRef).To(Equal(expectedRoleBinding.RoleRef))
			g.Expect(actualRoleBinding.Subjects).To(Equal(expectedRoleBinding.Subjects))
		}).Should(Succeed())

		// 5. ClusterRole and ClusterRoleBinding for cluster permissions
		Eventually(func(g Gomega) {
			clusterRoleName := fmt.Sprintf("%s-%s", kubeconfig.Name, kubeconfig.Namespace)
			expectedClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleName,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"namespaces"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			}
			expectedClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleName,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     clusterRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      kubeconfig.Name,
						Namespace: kubeconfig.Namespace,
					},
				},
			}

			actualClusterRole := &rbacv1.ClusterRole{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedClusterRole), actualClusterRole)).To(Succeed())
			g.Expect(actualClusterRole.Rules).To(Equal(expectedClusterRole.Rules))

			actualClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedClusterRoleBinding), actualClusterRoleBinding)).To(Succeed())
			g.Expect(actualClusterRoleBinding.RoleRef).To(Equal(expectedClusterRoleBinding.RoleRef))
			g.Expect(actualClusterRoleBinding.Subjects).To(Equal(expectedClusterRoleBinding.Subjects))
		}).Should(Succeed())

		By("updating status with references to secrets")
		// The service account secret ref should be set (to the same name as the Kubeconfig/ServiceAccount)
		Eventually(func(g Gomega) {
			actual := &v1alpha1.Kubeconfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: kubeconfig.Namespace,
				},
			}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(actual), actual)).To(Succeed())
			g.Expect(actual.Status.ServiceAccountSecretRef).To(Equal(ptr.To(kubeconfig.Name)))
		}).Should(Succeed())

		By("provisioning a kubeconfig secret for client usage")
		// Expect the reconciler to create a separate secret containing the actual kubeconfig data.
		Eventually(func(g Gomega) {
			kubeconfigSecretName := kubeconfig.Name + "-kubeconfig"
			expectedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfigSecretName,
					Namespace: kubeconfig.Namespace,
				},
			}
			actualSecret := &corev1.Secret{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(Succeed())
			g.Expect(actualSecret.Data).To(HaveKey("kubeconfig"))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			actual := &v1alpha1.Kubeconfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: kubeconfig.Namespace,
				},
			}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(actual), actual)).To(Succeed())
			expectedKubeconfigSecretRef := ptr.To(kubeconfig.Name + "-kubeconfig")
			g.Expect(actual.Status.KubeconfigSecretRef).To(Equal(expectedKubeconfigSecretRef))
		}).Should(Succeed())

		By("cleaning up stale permissions")
		// Remove the "kube-system" permission from the Kubeconfig and expect its Role/RoleBinding to be deleted.
		updatedKubeconfig := kubeconfig.DeepCopy()
		_, err := controllerutil.CreateOrPatch(ctx, c, updatedKubeconfig, func() error {
			updatedKubeconfig.Spec.NamespacedPermissions = []v1alpha1.NamespacedPermissions{
				{
					Namespace: "default",
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"configmaps"},
							Verbs:     []string{"*"},
						},
					},
				},
			}
			return nil
		})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			expectedDeletedRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: "kube-system",
				},
			}
			expectedDeletedRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfig.Name,
					Namespace: "kube-system",
				},
			}
			g.Expect(errors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(expectedDeletedRole), &rbacv1.Role{}))).To(BeTrue())
			g.Expect(errors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(expectedDeletedRoleBinding), &rbacv1.RoleBinding{}))).To(BeTrue())
		}).Should(Succeed())

		By("performing finalizer logic and cleaning up cluster-scoped resources")
		// Delete the Kubeconfig; the finalizer logic should ensure that cluster-scoped
		// objects (ClusterRole and ClusterRoleBinding) are cleaned up.
		Expect(c.Delete(ctx, kubeconfig)).To(Succeed())

		Eventually(func(g Gomega) {
			clusterRoleName := fmt.Sprintf("%s-%s", kubeconfig.Name, kubeconfig.Namespace)
			expectedDeletedClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleName,
				},
			}
			expectedDeletedClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleName,
				},
			}

			g.Expect(errors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(expectedDeletedClusterRole), &rbacv1.ClusterRole{}))).To(BeTrue())
			g.Expect(errors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(expectedDeletedClusterRoleBinding), &rbacv1.ClusterRoleBinding{}))).To(BeTrue())
		}).Should(Succeed())
	})
})
