// Command ldndrc-join onboards a laptop as a cluster Host.
//
// Default mode is the keyboard-driven terminal UI; --plain reproduces the
// join-cluster.sh log lines byte-for-byte for scripts and CI.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldndrc/control-panel/internal/join"
)

func main() {
	plain := flag.Bool("plain", false, "script-friendly log output instead of the TUI")
	masterFlag := flag.String("master", "", "master address (skips mDNS discovery)")
	joinKey := flag.String("join-key", os.Getenv("JOIN_KEY"), "optional shared join key")
	flag.Parse()

	if *plain {
		os.Exit(runPlain())
	}
	os.Exit(runTUI(*masterFlag, *joinKey))
}

// runPlain mirrors join-cluster.sh: one detection line, one decision line,
// exit 0 either way (guests just use the browser). Only the hardware gates
// (CPU/RAM/GPU/driver) apply here — disk, sudo, and network are TUI-mode
// pre-flight rows, and the script never checked them.
func runPlain() int {
	th := join.DefaultThresholds()
	hw := join.Detect()
	hw.MasterOnline, hw.HasSudo, hw.DiskFreeGB = true, true, th.MinDiskGB
	fmt.Println(join.PlainDetectedLine(hw))
	verdict, _ := join.Qualify(hw, th)
	fmt.Println(join.PlainDecisionLine(verdict))
	return 0
}

// appModel wraps the join model to launch the engine exactly once when the
// Join screen is entered. The engine streams StepMsg/DoneMsgS back.
type appModel struct {
	m   join.Model
	p   *tea.Program
	cfg join.RunConfig
}

func (a *appModel) Init() tea.Cmd { return nil }

func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := a.m.Update(msg)
	a.m = next.(join.Model)
	// The model sets JoinStarted on the Checks→Join transition; launch the
	// engine goroutine exactly once (Client nils after launch).
	if a.m.JoinStarted && a.cfg.Client != nil {
		cfg := a.cfg
		a.cfg.Client = nil // launch once
		go func() {
			emit := func(s string) { a.p.Send(join.StepMsg(s)) }
			final, err := join.RunJoin(context.Background(), cfgWithEmit(cfg, emit))
			if err != nil {
				a.p.Send(join.DoneMsgS("failed: " + err.Error()))
				return
			}
			a.p.Send(join.DoneMsgS(final))
		}()
		return a, cmd
	}
	return a, cmd
}

func cfgWithEmit(cfg join.RunConfig, emit func(string)) join.RunConfig {
	cfg.Emit = emit
	return cfg
}

func (a *appModel) View() string { return a.m.View() }

func runTUI(masterFlag, joinKey string) int {
	hw := join.Detect()
	master := join.DiscoverMaster(masterFlag, 8082)
	online := false
	if master.Host != "" {
		online = join.ProbeMaster(master.Host) == nil
	}
	hw.MasterOnline = online

	hostname, _ := os.Hostname()
	rows := join.CheckRows(hw, master.Host, online)
	m := join.NewModel(rows)
	m.Master = master.Addr()
	m.NeedToolkit = hw.GPU != "" && !join.ToolkitPresent()

	base := fmt.Sprintf("http://%s", master.Addr())
	cfg := join.RunConfig{
		Client:      join.NewClient(base),
		Hostname:    hostname,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		HW:          hw,
		JoinKey:     joinKey,
		Master:      master,
		ControlURL:  base,
		NeedToolkit: m.NeedToolkit,
		Timeout:     15 * time.Minute,
		BundleDir:   os.TempDir(),
	}
	app := &appModel{m: m, cfg: cfg}
	p := tea.NewProgram(app)
	app.p = p
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		return 1
	}
	return 0
}
