package join

import (
	"strings"
	"testing"
)

func TestCheckRowsAllGreen(t *testing.T) {
	hw := Hardware{OS: "linux", CPU: 12, RAMGB: 32, GPU: "NVIDIA RTX", DriverMajor: 550, DiskFreeGB: 100, HasSudo: true}
	rows := CheckRows(hw, "master", true)
	for _, r := range rows {
		if r.State == "red" {
			t.Fatalf("row %s red: %s", r.Name, r.Detail)
		}
	}
}

func TestCheckRowsRedAndGuidance(t *testing.T) {
	hw := Hardware{OS: "linux", CPU: 4, RAMGB: 32, GPU: "", DriverMajor: 0, DiskFreeGB: 100, HasSudo: true}
	rows := CheckRows(hw, "", false)
	byName := map[string]CheckRow{}
	for _, r := range rows {
		byName[r.Name] = r
	}
	if byName["CPU"].State != "red" || byName["GPU"].State != "red" {
		t.Fatalf("cpu/gpu should be red: %#v", rows)
	}
	if byName["Master"].State != "red" || !strings.Contains(byName["Master"].Detail, "MASTER_IP") {
		t.Fatalf("master row should demand MASTER_IP: %#v", byName["Master"])
	}
	if byName["Driver"].State != "red" || !strings.Contains(byName["Driver"].Detail, "nvidia-smi") {
		t.Fatalf("driver row should carry the lspci lesson: %#v", byName["Driver"])
	}
}

func TestCheckRowsDarwinShortCircuits(t *testing.T) {
	rows := CheckRows(Hardware{OS: "darwin", CPU: 12}, "", false)
	if len(rows) != 1 || rows[0].State != "red" {
		t.Fatalf("darwin should yield one red OS row: %#v", rows)
	}
}
