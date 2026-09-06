package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// Watcher polls approved join requests, labels nodes that turned Ready, and
// marks them joined. Labeling is also done by the joining agent itself, so
// the watcher is the verifier, not the only path.
type Watcher struct {
	store    *Store
	client   kubernetes.Interface
	interval time.Duration
}

// NewWatcher builds a watcher that ticks every interval.
func NewWatcher(store *Store, client kubernetes.Interface, interval time.Duration) *Watcher {
	return &Watcher{store: store, client: client, interval: interval}
}

// Run loops until ctx is done.
func (w *Watcher) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := w.RunOnce(ctx); err != nil {
				log.Printf("nodes watcher: %v", err)
			}
		}
	}
}

// RunOnce reconciles every approved node a single time.
func (w *Watcher) RunOnce(ctx context.Context) error {
	for _, n := range w.store.List() {
		if n.Status != StatusApproved {
			continue
		}
		node, err := w.client.CoreV1().Nodes().Get(ctx, n.NodeName, metav1.GetOptions{})
		if err != nil {
			continue // not joined yet; try next tick
		}
		if !nodeReady(node) {
			continue
		}
		patch, _ := json.Marshal(map[string]any{
			"metadata": map[string]any{"labels": map[string]string{
				"node-role.kubernetes.io/role": "host",
				"ldndrc/host":                  "true",
			}},
		})
		if _, err := w.client.CoreV1().Nodes().Patch(ctx, n.NodeName, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
			return fmt.Errorf("label node %s: %w", n.NodeName, err)
		}
		if err := w.store.MarkJoined(n.ID); err != nil {
			return err
		}
		log.Printf("nodes watcher: %s joined", n.NodeName)
	}
	return nil
}

func nodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
