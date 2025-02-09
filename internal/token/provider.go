package token

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type TokenInfo struct {
	Token     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// RefreshTime returns the time when the token should be refreshed based on an 80% lifetime.
// If the token lifetime is invalid or the prepared time is in the past, it returns the expiration time.
func (t *TokenInfo) RefreshTime() time.Time {
	ttlDuration := t.ExpiresAt.Sub(t.IssuedAt)
	if ttlDuration <= 0 {
		// Fallback to expiration time if times are invalid.
		return t.ExpiresAt
	}
	return t.IssuedAt.Add(time.Duration(float64(ttlDuration) * 0.8))
}

// parseToken decodes the JWT token and extracts the "iat" and "exp" claims.
// Note: ParseUnverified is used here since we assume the token comes from a trusted source.
func parseToken(tokenStr string) (tokenInfo *TokenInfo, err error) {
	parser := jwt.NewParser()
	// Parse the token without validating the signature.
	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to cast claims to MapClaims")
	}

	expVal, ok := claims["exp"].(float64)
	if !ok {
		return nil, fmt.Errorf(`"exp" claim missing or invalid`)
	}
	iatVal, ok := claims["iat"].(float64)
	if !ok {
		return nil, fmt.Errorf(`"iat" claim missing or invalid`)
	}

	exp := time.Unix(int64(expVal), 0)
	iat := time.Unix(int64(iatVal), 0)

	return &TokenInfo{
		Token:     tokenStr,
		IssuedAt:  iat,
		ExpiresAt: exp,
	}, nil
}

// EnsureToken checks whether the current token (if available via secretName) is still valid by reading its "iat" and "exp" claims.
// It calculates the token's TTL and determines a refresh time at 80% of its lifetime.
// If the token is not yet due for refresh, it returns the token from the secret.
// Otherwise, it requests a new token using the Kubernetes API.
func EnsureToken(
	ctx context.Context,
	kubeClient *kubernetes.Clientset,
	existingToken string,
	expirationSeconds int64,
	serviceAccountName string,
	namespace string,
) (*TokenInfo, error) {

	if existingToken != "" {
		tokenInfo, err := parseToken(existingToken)
		if err == nil && time.Now().Before(tokenInfo.RefreshTime()) {
			return tokenInfo, nil
		} else if err != nil {
			return nil, err
		}
	}

	// Request a new token.
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}

	tokenResp, err := kubeClient.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, serviceAccountName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to request token for service account %s/%s: %w", namespace, serviceAccountName, err)
	}

	tokenData := tokenResp.Status.Token
	tokenInfo, err := parseToken(tokenData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token claims: %w", err)
	}

	return tokenInfo, nil
}
