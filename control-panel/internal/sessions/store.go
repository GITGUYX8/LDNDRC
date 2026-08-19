// Package sessions manages workspace pods via the Kubernetes client-go API.
//
// This is the Go equivalent of the reference control panel's
// kubernetes.service.ts: it provisions a session pod, reports status, and
// tears pods down. The client-go wiring is intentionally isolated here so the
// rest of the server compiles and runs without a cluster (the store seam in
// internal/httpapi stays nil until a kubeconfig is provided).
//
// TODO(sessions): wire k8s.io/client-go once the ros2-platform StatefulSet
// manifest (Task 5) is final. Provision will create the per-user pod from
// that manifest template and return the pod endpoint for the gateway.
package sessions

// Store provisions and reports Kubernetes workspace sessions.
type Store struct {
	// kubeconfigPath is empty when running without a cluster.
	kubeconfigPath string
}

// NewStore returns a Store. Pass an empty kubeconfigPath to run in
// "no cluster" mode (provisioning returns ErrNoCluster).
func NewStore(kubeconfigPath string) *Store {
	return &Store{kubeconfigPath: kubeconfigPath}
}

// ErrNoCluster is returned when Provision is called without a kubeconfig.
var ErrNoCluster = errNoCluster{}

type errNoCluster struct{}

func (errNoCluster) Error() string { return "no kubeconfig configured" }

// Provision creates a workspace for username.
func (s *Store) Provision(username string) error {
	if s.kubeconfigPath == "" {
		return ErrNoCluster
	}
	// TODO(sessions): create the pod from the ros2-platform template and
	// register its endpoint for the gateway's resolve func.
	_ = username
	return nil
}
