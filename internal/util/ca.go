package util

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

// loadCACert loads the CA certificate from the given config.
// CAData is populated from the kubeconfig in dev setups
// while CAFile is populated when running in a cluster.
func LoadCACert(cfg *rest.Config, log *zap.SugaredLogger) ([]byte, error) {
	if len(cfg.CAData) > 0 {
		return cfg.CAData, nil
	}

	if cfg.CAFile != "" {
		caCrtData, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			log.Infof("unable to read CA certificate from %s: %v", cfg.CAFile, err)
		} else if len(caCrtData) == 0 {
			log.Infof("CA certificate file %s is empty", cfg.CAFile)
		} else {
			return caCrtData, nil
		}
	}

	return nil, fmt.Errorf("failed to load CA certificate: no CA certificate data found in config")
}
