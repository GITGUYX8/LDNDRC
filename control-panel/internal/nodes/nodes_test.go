package nodes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func testMinter(t *testing.T) (*BootstrapMinter, *fake.Clientset) {
	t.Helper()
	caPath := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(caPath, []byte("fake-ca-pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := fake.NewSimpleClientset()
	return &BootstrapMinter{client: client, caPath: caPath}, client
}

func TestMintTokenShape(t *testing.T) {
	minter, client := testMinter(t)
	token, tokenID, err := minter.MintToken(context.Background(), "ldndrc-abc123", "ldndrc-abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "K10") || !strings.Contains(token, "::") {
		t.Fatalf("token not in K10 secure format: %s", token)
	}
	secret, err := client.CoreV1().Secrets("kube-system").Get(context.Background(), "bootstrap-token-"+tokenID, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// NOTE: the fake clientset stores StringData as-is (a real API server
	// would fold it into Data), so assert on StringData here.
	if secret.StringData["token-id"] != tokenID {
		t.Fatalf("secret token-id mismatch: %#v", secret.StringData)
	}
	if secret.StringData["auth-extra-groups"] != "system:bootstrappers" {
		t.Fatalf("secret groups mismatch: %#v", secret.StringData)
	}
}

func TestMintTokenFailsWithoutCA(t *testing.T) {
	minter := &BootstrapMinter{client: fake.NewSimpleClientset(), caPath: "/nonexistent/ca.crt"}
	if _, _, err := minter.MintToken(context.Background(), "ldndrc-x", "x"); err == nil {
		t.Fatal("expected CA read error")
	}
}

func readyNode(name string, ready bool) *corev1.Node {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: status},
		}},
	}
}

func TestWatcherJoinsReadyNode(t *testing.T) {
	s, err := NewStore(filepath.Join(t.TempDir(), "nodes.json"))
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := s.Register("laptop-a", "linux", "amd64", 12, 32, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(n.ID); err != nil {
		t.Fatal(err)
	}
	client := fake.NewSimpleClientset(readyNode(n.NodeName, true))
	w := NewWatcher(s, client, 0)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(n.ID)
	if got.Status != StatusJoined {
		t.Fatalf("status = %s, want joined", got.Status)
	}
	labeled, _ := client.CoreV1().Nodes().Get(context.Background(), n.NodeName, metav1.GetOptions{})
	if labeled.Labels["node-role.kubernetes.io/role"] != "host" || labeled.Labels["ldndrc/host"] != "true" {
		t.Fatalf("labels missing: %#v", labeled.Labels)
	}
}

func TestWatcherSkipsUnreadyAndMissingNodes(t *testing.T) {
	s, err := NewStore(filepath.Join(t.TempDir(), "nodes.json"))
	if err != nil {
		t.Fatal(err)
	}
	a, _, _ := s.Register("laptop-a", "linux", "amd64", 12, 32, "")
	b, _, _ := s.Register("laptop-b", "linux", "amd64", 12, 32, "")
	if _, err := s.Approve(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(b.ID); err != nil {
		t.Fatal(err)
	}
	// only b exists in the cluster, and it is not Ready; a never joined
	client := fake.NewSimpleClientset(readyNode(b.NodeName, false))
	w := NewWatcher(s, client, 0)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		want Status
	}{{a.ID, StatusApproved}, {b.ID, StatusApproved}} {
		got, _ := s.Get(tc.id)
		if got.Status != tc.want {
			t.Fatalf("node %s status = %s, want %s", tc.id, got.Status, tc.want)
		}
	}
}
