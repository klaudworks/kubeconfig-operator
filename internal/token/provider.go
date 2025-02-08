package token

import (
	"context"
	"fmt"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TokenInfo holds information about a provisioned ServiceAccount token.
type TokenInfo struct {
	Token               string
	ExpirationTimestamp metav1.Time
	RefreshTimestamp    metav1.Time
}

// EnsureToken checks whether the current token (if provided) is still valid
// based on a refresh threshold (20% of the full TTL). If the token is near
// expiration (or is absent), it requests a new token using the Kubernetes API.
// It returns a TokenInfo, a boolean (refreshed=true if a new token was obtained),
// or an error if the token request fails.
func EnsureToken(ctx context.Context, kubeClient *kubernetes.Clientset, namespace, saName string, expirationSeconds int64, currentExpiration *metav1.Time) (*TokenInfo, bool, error) {
	ttlDuration := time.Duration(expirationSeconds) * time.Second
	refreshThreshold := time.Duration(float64(ttlDuration) * 0.20)

	if currentExpiration != nil {
		remaining := time.Until(currentExpiration.Time)
		if remaining > refreshThreshold {
			// Token is still valid; no need to request a new one.
			return nil, false, nil
		}
	}

	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}
	tokenResp, err := kubeClient.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("failed to request token for service account %s/%s: %w", namespace, saName, err)
	}

	tokenData := tokenResp.Status.Token
	tokenExpiration := tokenResp.Status.ExpirationTimestamp
	refreshTime := metav1.NewTime(tokenExpiration.Time.Add(-refreshThreshold))

	return &TokenInfo{
		Token:               tokenData,
		ExpirationTimestamp: tokenExpiration,
		RefreshTimestamp:    refreshTime,
	}, true, nil
}
