package join

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Detect gathers the laptop's physical profile on Linux. TEST_* env vars
// mirror join-cluster.sh overrides so the same scenarios run in Go tests.
func Detect() Hardware {
	hw := Hardware{OS: runtime.GOOS, CPU: runtime.NumCPU(), HasSudo: canSudo()}
	if v := os.Getenv("TEST_CPU"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			hw.CPU = n
		}
	}
	hw.RAMGB = physRAMGB()
	if v := os.Getenv("TEST_RAM"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			hw.RAMGB = f
		}
	}
	hw.GPU, hw.DriverMajor = nvidiaProbe()
	if v := os.Getenv("TEST_GPU"); v != "" {
		if v == "none" {
			hw.GPU, hw.DriverMajor = "", 0
		} else {
			hw.GPU, hw.DriverMajor = "NVIDIA (test override)", 550
		}
	}
	hw.DiskFreeGB = diskFreeGB("/")
	if v := os.Getenv("TEST_DISK"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			hw.DiskFreeGB = f
		}
	}
	return hw
}

func physRAMGB() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		var key string
		var kb float64
		if _, err := fmt.Sscanf(sc.Text(), "%s %f", &key, &kb); err == nil && key == "MemTotal:" {
			return kb / 1024 / 1024
		}
	}
	return 0
}

// nvidiaProbe returns the GPU model line and driver major version.
// lspci proves existence only; nvidia-smi proves the driver works.
func nvidiaProbe() (string, int) {
	out, err := exec.Command("nvidia-smi", "-L").Output()
	if err != nil {
		return "", 0
	}
	model := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	major := 0
	if v, err := exec.Command("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader").Output(); err == nil {
		if parts := strings.Split(strings.TrimSpace(strings.SplitN(string(v), "\n", 2)[0]), "."); len(parts) > 0 {
			major, _ = strconv.Atoi(parts[0])
		}
	}
	return model, major
}

func canSudo() bool {
	if os.Geteuid() == 0 {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "sudo", "-n", "true").Run() == nil
}

func diskFreeGB(_ string) float64 {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil {
		return 0
	}
	return float64(stat.Bavail*uint64(stat.Bsize)) / 1e9
}
