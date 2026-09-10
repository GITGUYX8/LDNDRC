// Command ldndrc-join onboards a laptop as a cluster Host.
//
// Default mode is the keyboard-driven terminal UI; --plain reproduces the
// join-cluster.sh log lines byte-for-byte for scripts and CI.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldndrc/control-panel/internal/join"
)

func main() {
	plain := flag.Bool("plain", false, "script-friendly log output instead of the TUI")
	master := flag.String("master", "", "master address (skips mDNS discovery)")
	flag.Parse()

	if *plain {
		os.Exit(runPlain(*master))
	}
	rows := []join.CheckRow{{Name: "OS", State: "green", Detail: "checks run on Join"}}
	p := tea.NewProgram(join.NewModel(rows))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}
}

// runPlain mirrors join-cluster.sh: one detection line, one decision line,
// exit 0 either way (guests just use the browser). Only the hardware gates
// (CPU/RAM/GPU/driver) apply here — disk, sudo, and network are TUI-mode
// pre-flight rows, and the script never checked them.
func runPlain(masterFlag string) int {
	_ = masterFlag
	th := join.DefaultThresholds()
	hw := join.Detect()
	hw.MasterOnline, hw.HasSudo, hw.DiskFreeGB = true, true, th.MinDiskGB
	fmt.Println(join.PlainDetectedLine(hw))
	verdict, _ := join.Qualify(hw, th)
	fmt.Println(join.PlainDecisionLine(verdict))
	return 0
}
