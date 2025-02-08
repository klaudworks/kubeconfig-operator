package scheme

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/klaudworks/kubeconfig-operator/api/klaud.works/v1alpha1"
)

var AddToSchemes = runtime.SchemeBuilder{}

func init() {
	AddToSchemes.Register(kscheme.AddToScheme)  // native kubernetes schemes
	AddToSchemes.Register(v1alpha1.AddToScheme) // internal schemes
}

func NewScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()

	if err := AddToSchemes.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("add resources to scheme: %w", err)
	}

	return s, nil
}

func MustNewScheme() *runtime.Scheme {
	s, err := NewScheme()
	if err != nil {
		panic(err)
	}
	return s
}
