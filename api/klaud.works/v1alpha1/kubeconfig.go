package v1alpha1

import (
	"github.com/reddit/achilles-sdk-api/api"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&Kubeconfig{}, &KubeconfigList{})
}

const (
	TypeKubeconfigProvisioned     api.ConditionType = "KubeconfigProvisioned"
	TypeServiceAccountProvisioned api.ConditionType = "ServiceAccountProvisioned"
	TypeStalePermissionsRemoved   api.ConditionType = "StalePermissionsRemoved"
)

// Kubeconfig is the Schema for the Kubeconfig API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Issued",type="string",JSONPath=".status.serviceAccountTokenIssuedAt",description="Kubeconfig issued timestamp"
// +kubebuilder:printcolumn:name="Expires",type="string",JSONPath=".status.serviceAccountTokenExpiresAt",description="Kubeconfig expiration timestamp"
// +kubebuilder:printcolumn:name="Refreshes",type="string",JSONPath=".status.serviceAccountTokenRefreshesAt",description="Kubeconfig refresh timestamp"
type Kubeconfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeconfigSpec   `json:"spec,omitempty"`
	Status KubeconfigStatus `json:"status,omitempty"`
}

// KubeconfigList contains a list of Kubeconfig
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
type KubeconfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kubeconfig `json:"items"`
}

// KubeconfigSpec defines the desired state of Kubeconfig
type KubeconfigSpec struct {
	// Server is the Kubernetes API server URL.
	// Set this to the external URL of the cluster.
	// You can copy this from your admin kubeconfig.
	// Required
	Server string `json:"server"`

	// ClusterName is the name of the cluster in the created kubeconfig.
	// This is also used as the context name. You can change this to anything you want.
	// Optional
	// +kubebuilder:default="kubernetes"
	ClusterName string `json:"clusterName,omitempty"`

	// ExpirationTTL is the time to live for the service account token.
	// Specified in days e.g. "365d". Default is 365 days.
	// Optional
	// +kubebuilder:default="365d"
	ExpirationTTL string `json:"expirationTTL,omitempty"`

	// NamespacedPermissions defines a list of namespaced scoped permissions. Optional
	NamespacedPermissions []NamespacedPermissions `json:"namespacedPermissions,omitempty"`

	// ClusterPermissions defines cluster scoped permissions. Optional
	ClusterPermissions *ClusterPermissions `json:"clusterPermissions,omitempty"`
}

type NamespacedPermissions struct {
	// Namespace the role applies to. Required
	Namespace string `json:"namespace"`

	// Rules for the role. Required
	Rules []rbacv1.PolicyRule `json:"rules"`
}

type ClusterPermissions struct {
	// Rules for the role. Required
	Rules []rbacv1.PolicyRule `json:"rules"`
}

// KubeconfigStatus defines the observed state of Kubeconfig
type KubeconfigStatus struct {
	api.ConditionedStatus `json:",inline"`

	// ResourceRefs is a list of all resources managed by this object.
	ResourceRefs []api.TypedObjectRef `json:"resourceRefs,omitempty"`

	// KubeconfigSecretRef is a reference to the Secret containing the kubeconfig.
	KubeconfigSecretRef *string `json:"kubeconfigSecretRef,omitempty"`

	// ServiceAccountRef is a reference to the ServiceAccount that will be used to provision the kubeconfig.
	ServiceAccountRef *string `json:"serviceAccountRef,omitempty"`

	// ServiceAccountTokenExpiresAt specifies when the service account token will expire.
	ServiceAccountTokenExpiresAt *metav1.Time `json:"serviceAccountTokenExpiresAt,omitempty"`

	// ServiceAccountTokenRefreshesAt specifies when the service account token will be refreshed.
	ServiceAccountTokenRefreshesAt *metav1.Time `json:"serviceAccountTokenRefreshesAt,omitempty"`

	// ServiceAccountTokenIssuedAt specifies when the service account token was issued.
	ServiceAccountTokenIssuedAt *metav1.Time `json:"serviceAccountTokenIssuedAt,omitempty"`
}

func (c *Kubeconfig) GetConditions() []api.Condition {
	return c.Status.Conditions
}

func (c *Kubeconfig) SetConditions(cond ...api.Condition) {
	c.Status.SetConditions(cond...)
}

func (c *Kubeconfig) GetCondition(t api.ConditionType) api.Condition {
	return c.Status.GetCondition(t)
}

func (c *Kubeconfig) SetManagedResources(refs []api.TypedObjectRef) {
	c.Status.ResourceRefs = refs
}

func (c *Kubeconfig) GetManagedResources() []api.TypedObjectRef {
	return c.Status.ResourceRefs
}
