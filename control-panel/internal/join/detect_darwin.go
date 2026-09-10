//go:build darwin

package join

import "runtime"

// Detect on macOS exists only to feed the OS gate: macOS laptops are
// always Guests (no K3s agents, no NVIDIA). The binary ships so users get
// the browser URL instead of a missing-download error.
func Detect() Hardware {
	return Hardware{OS: runtime.GOOS, CPU: runtime.NumCPU()}
}
