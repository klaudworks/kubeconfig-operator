package kubeconfig

import (
	"github.com/reddit/achilles-sdk-api/api"
	corev1 "k8s.io/api/core/v1"

	"github.com/klaudworks/kubeconfig-operator/api/klaud.works/v1alpha1"
)

var conditionServiceAccountProvisioned = api.Condition{
	Type:    v1alpha1.TypeServiceAccountProvisioned,
	Status:  corev1.ConditionTrue,
	Message: "Kubeconfig service account has been provisioned.",
}

var conditionStalePermissionsRemoved = api.Condition{
	Type:    v1alpha1.TypeStalePermissionsRemoved,
	Status:  corev1.ConditionTrue,
	Message: "Stale permissions have been removed",
}

var conditionKubeconfigProvisioned = api.Condition{
	Type:    v1alpha1.TypeKubeconfigProvisioned,
	Status:  corev1.ConditionTrue,
	Message: "Kubeconfig service account has been provisioned.",
}
