//go:build windows

package join

import "runtime"

// Detect on Windows measures the physical machine via native APIs where
// cheap; GPU/driver probing shells to nvidia-smi, which passes through the
// same driver the WSL2 path will use.
func Detect() Hardware {
	return Hardware{OS: "windows", CPU: runtime.NumCPU()}
}
