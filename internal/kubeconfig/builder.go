package kubeconfig

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/klaudworks/kubeconfig-operator/api/klaud.works/v1alpha1"
)

type BuildConfig struct {
	Kubeconfig         *v1alpha1.Kubeconfig
	Namespace          string
	ServiceAccountName string
	Token              string
	CACrtData          []byte
}

func Build(config BuildConfig) (*corev1.Secret, error) {
	if config.Kubeconfig == nil {
		return nil, errors.New("BuildConfig.Kubeconfig is required")
	}
	if len(config.CACrtData) == 0 {
		return nil, errors.New("BuildConfig.CACrtData is required")
	}

	kubeconfigYaml, err := generateKubeconfigYaml(config)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Kubeconfig.GetName() + "-kubeconfig",
			Namespace: config.Kubeconfig.GetNamespace(),
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfigYaml,
			"token":      []byte(config.Token),
			"ca.crt":     config.CACrtData,
		},
		Type: corev1.SecretTypeOpaque,
	}

	return secret, nil
}

func generateKubeconfigYaml(config BuildConfig) ([]byte, error) {
	// Build the context name as serviceaccountname@clustername.
	contextName := fmt.Sprintf("%s@%s", config.ServiceAccountName, config.Kubeconfig.Spec.ClusterName)

	cfg := &clientcmdapi.Config{
		CurrentContext: contextName,
		Clusters: map[string]*clientcmdapi.Cluster{
			config.Kubeconfig.Spec.ClusterName: {
				Server:                   config.Kubeconfig.Spec.Server,
				CertificateAuthorityData: config.CACrtData,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			config.ServiceAccountName: {
				Token: config.Token,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:   config.Kubeconfig.Spec.ClusterName,
				AuthInfo:  config.ServiceAccountName,
				Namespace: config.Namespace,
			},
		},
	}

	return clientcmd.Write(*cfg)
}
