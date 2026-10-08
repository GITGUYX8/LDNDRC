package join

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// InstallToolkit installs the NVIDIA container toolkit on Debian-family
// hosts. It never runs silently: the caller must have obtained explicit
// user consent (TUI decision 4) before invoking it.
func InstallToolkit() error {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("automatic toolkit install needs Debian/Ubuntu (no apt-get on %s): install nvidia-container-toolkit manually per NVIDIA docs, then re-run", distroID())
	}
	for _, step := range [][]string{
		{"sudo", "apt-get", "update"},
		{"sudo", "apt-get", "install", "-y", "nvidia-container-toolkit"},
		{"sudo", "nvidia-ctk", "runtime", "configure", "--runtime=docker"},
	} {
		cmd := exec.Command(step[0], step[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%v: %w", step, err)
		}
	}
	return nil
}

// distroID reports the os-release ID for error messages (unknown when
// unreadable). Kept tiny on purpose: full multi-distro automation (pacman,
// AUR helpers) is a separate feature; failing loudly beats running apt-get
// to certain failure on Arch and friends.
func distroID() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown distro"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "ID="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return "unknown distro"
}

// ToolkitPresent reports whether the toolkit is already installed.
func ToolkitPresent() bool {
	return exec.Command("nvidia-container-toolkit", "--version").Run() == nil
}

// InstallAgent downloads the K3s installer and runs the agent join with the
// short-lived token. token comes from an approved poll response, lives in
// memory only, and is never logged (errors redact it).
func InstallAgent(k3sURL, token, nodeName string) error {
	if k3sURL == "" || token == "" || nodeName == "" {
		return fmt.Errorf("k3s URL, token and node name are required")
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get("https://get.k3s.io")
	if err != nil {
		return fmt.Errorf("download k3s installer: %w", err)
	}
	defer resp.Body.Close()
	tmp, err := os.CreateTemp("", "k3s-install-*.sh")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("save k3s installer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// NOTE: no --node-label here on purpose. kubelet --node-labels rejects
	// the kubernetes.io namespace (node-role.kubernetes.io/role is not
	// allow-listed), so passing our host label kills kubelet at flag
	// validation (seen live 2026-09-14). The master-side watcher labels
	// the node (node-role.kubernetes.io/role=host + ldndrc/host=true)
	// via the API — unrestricted — as soon as it goes Ready.
	cmd := exec.Command("sh", tmpName, "agent",
		"--node-name", nodeName)
	cmd.Env = append(os.Environ(),
		"K3S_URL="+k3sURL,
		"K3S_TOKEN="+token,
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("k3s agent install failed (token redacted)")
	}
	return nil
}
