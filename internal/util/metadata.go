package util

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func AddLabel(obj metav1.Object, key, value string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	obj.SetLabels(labels)
}
