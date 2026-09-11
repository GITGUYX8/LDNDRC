package join

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen is the visible TUI state. The model renders state and forwards
// keypresses only — detection and install logic live in the engine.
type Screen int

const (
	ScreenChecks Screen = iota
	ScreenFix
	ScreenConfirm
	ScreenJoin
	ScreenDone
)

// CheckRow is one pre-flight matrix row.
type CheckRow struct {
	Name   string
	State  string // green, amber, red
	Detail string
}

// Model holds TUI state; zero value is usable in tests.
type Model struct {
	Screen   Screen
	Rows     []CheckRow
	FixIndex int
	StepLog  []string
	DoneMsg  string
	Master   string
	// NeedToolkit requests the consent prompt before joining (D4): the
	// toolkit install never runs silently.
	NeedToolkit bool
	// PendingPrompt is shown on ScreenConfirm.
	PendingPrompt string
	// JoinStarted marks the flow launched; the cmd/join glue watches this
	// transition to start the engine goroutine exactly once.
	JoinStarted bool
	quitting    bool
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	greenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	amberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	redStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	dimStyle   = lipgloss.NewStyle().Faint(true)
)

// NewModel builds the checks screen from engine rows.
func NewModel(rows []CheckRow) Model {
	return Model{Screen: ScreenChecks, Rows: rows}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Msg types the model handles.
type (
	// StepMsg appends a join step log line.
	StepMsg string
	// DoneMsgS finishes the flow with a message.
	DoneMsgS string
)

// Update implements tea.Model: pure keypress-to-state transitions.
// A quit decision returns tea.Quit — setting the flag alone only swaps the
// view and would leave the program hanging (fixed after the guest reported
// an unexitable TUI).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m = m.handleKey(msg.String())
		if m.quitting {
			return m, tea.Quit
		}
		return m, nil
	case StepMsg:
		m.StepLog = append(m.StepLog, string(msg))
		return m, nil
	case DoneMsgS:
		m.Screen = ScreenDone
		m.DoneMsg = string(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(key string) Model {
	switch key {
	case "ctrl+c", "q":
		m.quitting = true
		return m
	}
	switch m.Screen {
	case ScreenChecks:
		hasRed := false
		for i, r := range m.Rows {
			if r.State == "red" && !hasRed {
				m.FixIndex = i
				hasRed = true
			}
		}
		if key == "enter" && hasRed {
			m.Screen = ScreenFix
		} else if key == "j" && !hasRed {
			if m.NeedToolkit {
				m.PendingPrompt = "Install the NVIDIA container toolkit now? (needs sudo)  [y/N]"
				m.Screen = ScreenConfirm
			} else {
				m.Screen = ScreenJoin
				m.JoinStarted = true
			}
		}
	case ScreenFix:
		if key == "esc" || key == "backspace" {
			m.Screen = ScreenChecks
		}
	case ScreenConfirm:
		switch key {
		case "y", "Y":
			m.Screen = ScreenJoin
			m.JoinStarted = true
		case "n", "N", "esc":
			m.Screen = ScreenChecks
		}
	case ScreenJoin:
		// join steps stream in as StepMsg; nothing to do on keys
	case ScreenDone:
		if key == "enter" {
			m.quitting = true
		}
	}
	return m
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return "bye.\n"
	}
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("ldndrc-join") + "\n\n")
	switch m.Screen {
	case ScreenChecks:
		sb.WriteString("Pre-flight checks (Join locked until all blocking rows are green):\n")
		for _, r := range m.Rows {
			dot := greenStyle.Render("●")
			switch r.State {
			case "amber":
				dot = amberStyle.Render("●")
			case "red":
				dot = redStyle.Render("●")
			}
			fmt.Fprintf(&sb, "  %s %-10s %s\n", dot, r.Name, dimStyle.Render(r.Detail))
		}
		sb.WriteString("\n[enter on red row: fix card]  [j: join]  [q: quit]\n")
	case ScreenFix:
		r := m.Rows[m.FixIndex]
		sb.WriteString(redStyle.Render("Fix: "+r.Name) + "\n")
		sb.WriteString(r.Detail + "\n")
		sb.WriteString("\n[esc: back]\n")
	case ScreenConfirm:
		sb.WriteString(amberStyle.Render("Confirm") + "\n")
		sb.WriteString(m.PendingPrompt + "\n")
	case ScreenJoin:
		sb.WriteString("Joining via master " + m.Master + ":\n")
		for _, s := range m.StepLog {
			sb.WriteString("  " + s + "\n")
		}
	case ScreenDone:
		sb.WriteString(m.DoneMsg + "\n")
		sb.WriteString("\n[enter: quit]\n")
	}
	return sb.String()
}
