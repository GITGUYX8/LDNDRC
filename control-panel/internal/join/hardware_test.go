package join

import (
	"testing"
)

func TestQualifyGates(t *testing.T) {
	good := Hardware{OS: "linux", CPU: 12, RAMGB: 32, GPU: "NVIDIA RTX", DriverMajor: 550, DiskFreeGB: 100, HasSudo: true, MasterOnline: true}
	if v, _ := Qualify(good, DefaultThresholds()); v != VerdictHost {
		t.Fatalf("good hardware = %v, want host", v)
	}
	cases := []struct {
		name string
		mut  func(*Hardware)
	}{
		{"darwin", func(h *Hardware) { h.OS = "darwin" }},
		{"cpu", func(h *Hardware) { h.CPU = 4 }},
		{"ram", func(h *Hardware) { h.RAMGB = 4 }},
		{"gpu", func(h *Hardware) { h.GPU = "" }},
		{"driver", func(h *Hardware) { h.DriverMajor = 470 }},
		{"disk", func(h *Hardware) { h.DiskFreeGB = 5 }},
		{"sudo", func(h *Hardware) { h.HasSudo = false }},
		{"network", func(h *Hardware) { h.MasterOnline = false }},
	}
	for _, tc := range cases {
		hw := good
		tc.mut(&hw)
		if v, reason := Qualify(hw, DefaultThresholds()); v != VerdictGuest || reason == "" {
			t.Fatalf("%s: verdict=%v reason=%q, want guest with reason", tc.name, v, reason)
		}
	}
}

func TestDriverBand(t *testing.T) {
	th := DefaultThresholds()
	if DriverBand(470, th.MinDriver, th.WantDriver) != "red" {
		t.Fatal("470 should be red")
	}
	if DriverBand(535, th.MinDriver, th.WantDriver) != "amber" {
		t.Fatal("535 should be amber")
	}
	if DriverBand(550, th.MinDriver, th.WantDriver) != "green" {
		t.Fatal("550 should be green")
	}
}

func TestPlainOracleLines(t *testing.T) {
	// Byte-for-byte contract with test-join.sh grep patterns.
	hw := Hardware{CPU: 12, RAMGB: 16, GPU: "x"}
	if got := PlainDetectedLine(hw); got != "[join-cluster] Detected: CPU=12 cores, RAM=16GB, GPU=yes" {
		t.Fatalf("detected line = %q", got)
	}
	if got := PlainDecisionLine(VerdictHost); got != "[join-cluster] High-end laptop detected. Joining as HOST." {
		t.Fatal("host line mismatch")
	}
	if got := PlainDecisionLine(VerdictGuest); got != "[join-cluster] Low-end laptop detected. Do not join cluster. Use browser to access platform." {
		t.Fatal("guest line mismatch")
	}
}

func TestDetectOverrides(t *testing.T) {
	t.Setenv("TEST_CPU", "12")
	t.Setenv("TEST_RAM", "16")
	t.Setenv("TEST_GPU", "nvidia")
	t.Setenv("TEST_DISK", "100")
	hw := Detect()
	if hw.CPU != 12 || hw.RAMGB != 16 || hw.GPU == "" || hw.DiskFreeGB != 100 {
		t.Fatalf("overrides not applied: %#v", hw)
	}
	t.Setenv("TEST_GPU", "none")
	if hw := Detect(); hw.GPU != "" {
		t.Fatalf("gpu=none should clear GPU: %#v", hw)
	}
}
