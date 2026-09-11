package join

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyModel(rows []CheckRow, key string) Model {
	m := NewModel(rows)
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestQuitCommandIssued(t *testing.T) {
	m := NewModel([]CheckRow{{Name: "OS", State: "green"}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q must return a quit command, else the TUI hangs on the bye screen")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	joined := next.(Model)
	if !joined.JoinStarted {
		t.Fatal("expected join to start")
	}
}

func TestChecksToFixOnEnter(t *testing.T) {
	rows := []CheckRow{{Name: "OS", State: "green"}, {Name: "Driver", State: "red", Detail: "470 < 535"}}
	m := keyModel(rows, "enter")
	if m.Screen != ScreenFix || m.FixIndex != 1 {
		t.Fatalf("screen=%v fix=%d, want fix/1", m.Screen, m.FixIndex)
	}
	m2 := keyModel(rows, "j")
	if m2.Screen != ScreenChecks {
		t.Fatalf("j with red rows must not join: screen=%v", m2.Screen)
	}
}

func TestChecksToJoinWhenGreen(t *testing.T) {
	rows := []CheckRow{{Name: "OS", State: "green"}}
	if m := keyModel(rows, "j"); m.Screen != ScreenJoin {
		t.Fatalf("screen=%v, want join", m.Screen)
	}
}

func TestStepAndDoneFlow(t *testing.T) {
	m := NewModel(nil)
	next, _ := m.Update(StepMsg("registered as abc123"))
	m = next.(Model)
	if len(m.StepLog) != 1 {
		t.Fatalf("steps = %v", m.StepLog)
	}
	next, _ = m.Update(DoneMsgS("joined!"))
	if m = next.(Model); m.Screen != ScreenDone || m.DoneMsg != "joined!" {
		t.Fatalf("done = %#v", m)
	}
}

func TestViewRenders(t *testing.T) {
	rows := []CheckRow{{Name: "CPU", State: "green", Detail: "12 cores"}}
	out := NewModel(rows).View()
	if out == "" {
		t.Fatal("empty view")
	}
}

func TestConfirmToolkitFlow(t *testing.T) {
	rows := []CheckRow{{Name: "OS", State: "green"}}
	m := NewModel(rows)
	m.NeedToolkit = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = next.(Model)
	if m.Screen != ScreenConfirm || m.JoinStarted {
		t.Fatalf("screen=%v started=%v, want confirm/not-started", m.Screen, m.JoinStarted)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m = next.(Model); m.Screen != ScreenJoin || !m.JoinStarted {
		t.Fatalf("screen=%v started=%v, want join/started", m.Screen, m.JoinStarted)
	}
}

func TestConfirmDeclineReturns(t *testing.T) {
	rows := []CheckRow{{Name: "OS", State: "green"}}
	m := NewModel(rows)
	m.NeedToolkit = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m = next.(Model); m.Screen != ScreenChecks || m.JoinStarted {
		t.Fatalf("screen=%v started=%v, want checks/not-started", m.Screen, m.JoinStarted)
	}
}

func TestNoToolkitSkipsConfirm(t *testing.T) {
	rows := []CheckRow{{Name: "OS", State: "green"}}
	m := NewModel(rows)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m = next.(Model); m.Screen != ScreenJoin || !m.JoinStarted {
		t.Fatalf("screen=%v started=%v, want join/started", m.Screen, m.JoinStarted)
	}
}
