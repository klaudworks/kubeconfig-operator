package serviceaccount

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/klaudworks/kubeconfig-operator/api/klaud.works/v1alpha1"
	"github.com/klaudworks/kubeconfig-operator/internal/util"
)

type builder struct {
	kubeconfig *v1alpha1.Kubeconfig
}

func NewBuilder(
	kubeconfig *v1alpha1.Kubeconfig,
) *builder {
	return &builder{
		kubeconfig: kubeconfig,
	}
}

func (b *builder) Build() []client.Object {
	resources := []client.Object{
		b.ServiceAccount(),
		b.secret(),
	}

	resources = append(resources, b.roleAndBindings()...)
	resources = append(resources, b.clusterRoleAndBinding()...)

	for _, resource := range resources {
		util.AddLabel(resource, "kubeconfig-operator/type", "permission")
	}

	return resources
}

func (b *builder) ServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.kubeconfig.GetName() + "-token",
			Namespace: b.kubeconfig.GetNamespace(),
		},
	}
}

func (b *builder) secret() *corev1.Secret {
	sa := b.ServiceAccount()

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.kubeconfig.Name,
			Namespace: b.kubeconfig.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.GetName(),
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
}

func (b *builder) roleAndBindings() []client.Object {
	var objs []client.Object

	for _, namespacedRole := range b.kubeconfig.Spec.NamespacedPermissions {
		role := b.role(b.kubeconfig, namespacedRole.Namespace, namespacedRole.Rules)
		objs = append(objs, role)

		roleRef := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		}

		objs = append(objs, b.roleBinding(roleRef, namespacedRole.Namespace))
	}

	return objs
}

func (b *builder) role(kubeconfig *v1alpha1.Kubeconfig, ns string, rules []rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfig.GetName(),
			Namespace: ns,
		},
		Rules: rules,
	}
}

func (b *builder) roleBinding(roleRef rbacv1.RoleRef, ns string) *rbacv1.RoleBinding {
	sa := b.ServiceAccount()
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.kubeconfig.GetName(),
			Namespace: ns,
		},
		RoleRef: roleRef,
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.GetName(),
				Namespace: sa.GetNamespace(),
			},
		},
	}
}

func (b *builder) clusterRoleAndBinding() []client.Object {
	var objs []client.Object
	if b.kubeconfig.Spec.ClusterPermissions == nil {
		return nil
	}

	clusterRole := b.clusterRole(b.kubeconfig.Spec.ClusterPermissions.Rules)
	objs = append(objs, clusterRole)

	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	objs = append(objs, b.clusterRoleBinding(roleRef))

	return objs
}

func (b *builder) clusterRole(rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			// NOTE: ClusterRoles are cluster-scoped objects so we qualify the name with the namespace to avoid colliding names
			Name: fmt.Sprintf("%s-%s", b.kubeconfig.GetName(), b.kubeconfig.GetNamespace()),
		},
		Rules: rules,
	}
}

func (b *builder) clusterRoleBinding(roleRef rbacv1.RoleRef) *rbacv1.ClusterRoleBinding {
	sa := b.ServiceAccount()
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			// NOTE: ClusterRoles are cluster-scoped objects so we qualify the name with the namespace to avoid colliding names
			Name: fmt.Sprintf("%s-%s", b.kubeconfig.GetName(), b.kubeconfig.GetNamespace()),
		},
		RoleRef: roleRef,
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.GetName(),
				Namespace: sa.GetNamespace(),
			},
		},
	}
}
