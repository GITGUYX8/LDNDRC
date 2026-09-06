package nodes

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Minter issues short-lived K3s join tokens.
type Minter interface {
	MintToken(ctx context.Context, nodeName, description string) (token, tokenID string, err error)
}

// BootstrapMinter creates Kubernetes bootstrap-token Secrets in kube-system
// via client-go — the in-cluster equivalent of `k3s token create --ttl`.
type BootstrapMinter struct {
	client kubernetes.Interface
	caPath string
}

// NewBootstrapMinter builds a minter using the default in-cluster CA path.
func NewBootstrapMinter(client kubernetes.Interface) *BootstrapMinter {
	return &BootstrapMinter{client: client, caPath: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"}
}

// MintToken creates a 15-minute bootstrap token and formats it in K3s secure
// format: K10<ca-hash>::<id>.<secret>.
func (m *BootstrapMinter) MintToken(ctx context.Context, nodeName, description string) (string, string, error) {
	tokenID, err := randToken(6)
	if err != nil {
		return "", "", err
	}
	secret, err := randToken(16)
	if err != nil {
		return "", "", err
	}
	exp := time.Now().UTC().Add(TokenTTL).Format(time.RFC3339)
	_, err = m.client.CoreV1().Secrets("kube-system").Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bootstrap-token-" + tokenID,
		},
		Type: "bootstrap.kubernetes.io/token",
		StringData: map[string]string{
			"token-id":                       tokenID,
			"token-secret":                   secret,
			"expiration":                     exp,
			"description":                    description,
			"auth-extra-groups":              "system:bootstrappers",
			"usage-bootstrap-authentication": "true",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("create bootstrap token: %w", err)
	}
	caHash, err := caHash(m.caPath)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf("K10%s::%s.%s", caHash, tokenID, secret), tokenID, nil
}

func caHash(path string) (string, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read cluster CA: %w", err)
	}
	sum := sha256.Sum256(pemBytes)
	return hex.EncodeToString(sum[:]), nil
}

func randToken(n int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(raw[i])%len(charset)]
	}
	return string(b), nil
}
