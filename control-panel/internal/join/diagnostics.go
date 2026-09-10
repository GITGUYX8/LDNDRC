package join

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Bundle is a diagnostics export: OS, check outputs, and step log.
// Tokens and secrets are excluded by construction — callers must never add
// them, and there is no field that could carry one.
type Bundle struct {
	Hostname string
	OS       string
	Checks   []string
	Steps    []string
}

// Render formats the bundle as text.
func (b Bundle) Render() []byte {
	var sb strings.Builder
	fmt.Fprintf(&sb, "ldndrc diagnostics %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "hostname: %s\nos: %s\n", b.Hostname, b.OS)
	sb.WriteString("--- checks ---\n")
	for _, c := range b.Checks {
		sb.WriteString(c + "\n")
	}
	sb.WriteString("--- steps ---\n")
	for _, s := range b.Steps {
		sb.WriteString(s + "\n")
	}
	return []byte(sb.String())
}

// WriteLocal saves the bundle to ldndrc-diagnostics-<timestamp>.txt in dir
// and returns the filename. The local file is always written first so the
// manual path never depends on the network.
func (b Bundle) WriteLocal(dir string) (string, error) {
	name := fmt.Sprintf("ldndrc-diagnostics-%s.txt", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b.Render(), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// MaybeUpload posts the bundle when the network proved healthy (approval
// polls succeeding). Callers pass networkHealthy from the poll loop outcome;
// on false it returns nil having uploaded nothing, and the caller prints
// the manual-share message with the local filename.
func (c *Client) MaybeUpload(ctx context.Context, nodeID string, bundle []byte, networkHealthy bool) error {
	if !networkHealthy {
		return nil
	}
	return c.UploadDiagnostics(ctx, nodeID, bundle)
}
