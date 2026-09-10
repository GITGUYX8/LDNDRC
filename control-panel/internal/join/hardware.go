// Package join implements the laptop-side onboarding engine for ldndrc-join.
//
// All detection and install logic lives in testable non-UI functions; the
// Bubble Tea model renders state and forwards keypresses only. Thresholds
// mirror join-cluster.sh: >= 8 cores, >= 16 GB RAM, NVIDIA GPU present,
// driver >= 535 (550+ recommended), plus disk, sudo, and network gates.
package join

import "fmt"

// Verdict is the outcome of hardware qualification.
type Verdict int

const (
	// VerdictHost means the laptop may join as a compute Host.
	VerdictHost Verdict = iota
	// VerdictGuest means the laptop stays a browser-only Guest.
	VerdictGuest
)

// Thresholds gate Host qualification. Zero values select the defaults.
type Thresholds struct {
	MinCPU     int
	MinRAMGB   float64
	MinDriver  int
	WantDriver int
	MinDiskGB  float64
}

// DefaultThresholds carries the project-wide Host bars.
func DefaultThresholds() Thresholds {
	return Thresholds{MinCPU: 8, MinRAMGB: 16, MinDriver: 535, WantDriver: 550, MinDiskGB: 25}
}

// Hardware is one laptop's detected (or faked, in tests) profile.
type Hardware struct {
	OS           string
	CPU          int
	RAMGB        float64
	GPU          string // empty when absent
	DriverMajor  int    // 0 when unknown
	DiskFreeGB   float64
	HasSudo      bool
	MasterOnline bool
}

// Qualify applies every gate; the first failure decides Guest with a reason.
// There is deliberately no override — strictness is a locked decision.
func Qualify(hw Hardware, th Thresholds) (Verdict, string) {
	if hw.OS == "darwin" {
		return VerdictGuest, "macOS cannot host K3s or NVIDIA workloads"
	}
	if hw.CPU < th.MinCPU {
		return VerdictGuest, fmt.Sprintf("CPU %d < %d cores", hw.CPU, th.MinCPU)
	}
	if hw.RAMGB < th.MinRAMGB {
		return VerdictGuest, fmt.Sprintf("RAM %.1f < %.0f GB", hw.RAMGB, th.MinRAMGB)
	}
	if hw.GPU == "" {
		return VerdictGuest, "no NVIDIA GPU detected"
	}
	if hw.DriverMajor < th.MinDriver {
		return VerdictGuest, fmt.Sprintf("driver %d < %d", hw.DriverMajor, th.MinDriver)
	}
	if hw.DiskFreeGB < th.MinDiskGB {
		return VerdictGuest, fmt.Sprintf("disk %.1f < %.0f GB free", hw.DiskFreeGB, th.MinDiskGB)
	}
	if !hw.HasSudo {
		return VerdictGuest, "cannot elevate (agent and toolkit install need sudo)"
	}
	if !hw.MasterOnline {
		return VerdictGuest, "master unreachable on TCP 6443"
	}
	return VerdictHost, ""
}

// DriverBand classifies the driver for the fix card: red blocks, amber
// warns (works, upgrade recommended), green is current.
func DriverBand(major, floor, want int) string {
	switch {
	case major < floor:
		return "red"
	case major < want:
		return "amber"
	default:
		return "green"
	}
}

// Plain-mode oracle lines. --plain must reproduce these byte-for-byte: they
// are the strings test-join.sh greps for.
func PlainDetectedLine(hw Hardware) string {
	gpu := "no"
	if hw.GPU != "" {
		gpu = "yes"
	}
	return fmt.Sprintf("[join-cluster] Detected: CPU=%d cores, RAM=%.0fGB, GPU=%s", hw.CPU, hw.RAMGB, gpu)
}

// PlainDecisionLine renders the Host/Guest decision line for --plain mode.
func PlainDecisionLine(v Verdict) string {
	if v == VerdictHost {
		return "[join-cluster] High-end laptop detected. Joining as HOST."
	}
	return "[join-cluster] Low-end laptop detected. Do not join cluster. Use browser to access platform."
}
