//go:build !linux

package join

import "errors"

// InstallToolkit is Linux-only (apt + nvidia-ctk). Windows hosts hand off
// into WSL2, where the Linux build runs instead.
func InstallToolkit() error {
	return errors.New("toolkit install is Linux-only")
}

// InstallAgent is Linux-only for the same reason.
func InstallAgent(k3sURL, token, nodeName string) error {
	return errors.New("agent install is Linux-only")
}

// ToolkitPresent reports false off Linux; the WSL2 handoff governs there.
func ToolkitPresent() bool {
	return false
}
