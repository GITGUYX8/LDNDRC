package join

import "fmt"

// CheckRows builds the Screen 1 pre-flight matrix from detected hardware.
// MasterOnline reflects the TCP 6443 probe; MasterHost empty means no
// master address is configured yet (typed or discovered).
func CheckRows(hw Hardware, masterHost string, masterOnline bool) []CheckRow {
	th := DefaultThresholds()
	row := func(name, state, detail string) CheckRow {
		return CheckRow{Name: name, State: state, Detail: detail}
	}
	rows := []CheckRow{}

	if hw.OS == "darwin" {
		return []CheckRow{row("OS", "red", "macOS: Guest only (no K3s/NVIDIA possible)")}
	}
	rows = append(rows, row("OS", "green", hw.OS))

	if hw.CPU >= th.MinCPU {
		rows = append(rows, row("CPU", "green", fmt.Sprintf("%d cores", hw.CPU)))
	} else {
		rows = append(rows, row("CPU", "red", fmt.Sprintf("%d < %d cores → Guest", hw.CPU, th.MinCPU)))
	}
	if hw.RAMGB >= th.MinRAMGB {
		rows = append(rows, row("RAM", "green", fmt.Sprintf("%.0f GB", hw.RAMGB)))
	} else {
		rows = append(rows, row("RAM", "red", fmt.Sprintf("%.1f < %.0f GB → Guest", hw.RAMGB, th.MinRAMGB)))
	}
	if hw.GPU != "" {
		rows = append(rows, row("GPU", "green", hw.GPU))
	} else {
		rows = append(rows, row("GPU", "red", "absent → Guest"))
	}
	switch DriverBand(hw.DriverMajor, th.MinDriver, th.WantDriver) {
	case "green":
		rows = append(rows, row("Driver", "green", fmt.Sprintf("%d (550+ recommended)", hw.DriverMajor)))
	case "amber":
		rows = append(rows, row("Driver", "amber", fmt.Sprintf("%d works, 550+ recommended", hw.DriverMajor)))
	default:
		rows = append(rows, row("Driver", "red", fmt.Sprintf("%d < %d; lspci proves existence only, nvidia-smi proves the driver works", hw.DriverMajor, th.MinDriver)))
	}
	if hw.DiskFreeGB >= th.MinDiskGB {
		rows = append(rows, row("Disk", "green", fmt.Sprintf("%.0f GB free", hw.DiskFreeGB)))
	} else {
		rows = append(rows, row("Disk", "red", fmt.Sprintf("%.1f < %.0f GB free → Guest", hw.DiskFreeGB, th.MinDiskGB)))
	}
	if hw.HasSudo {
		rows = append(rows, row("Sudo", "green", "can elevate"))
	} else {
		rows = append(rows, row("Sudo", "red", "agent + toolkit install need sudo → stop"))
	}
	switch {
	case masterHost == "":
		rows = append(rows, row("Master", "red", "unset: set MASTER_IP (mDNS fills this later) → Join locked"))
	case masterOnline:
		rows = append(rows, row("Network", "green", masterHost+":6443 reachable"))
	default:
		rows = append(rows, row("Network", "red", masterHost+":6443 unreachable (laptop: sudo ufw allow out 6443/tcp; master: sudo ufw allow 6443/tcp)"))
	}
	return rows
}
