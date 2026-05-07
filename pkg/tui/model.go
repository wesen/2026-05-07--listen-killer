package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wesen/listen-killer/pkg/listener"
)

// ---------------------------------------------------------------------------
// Custom messages
// ---------------------------------------------------------------------------

// ListenersLoadedMsg is sent when a scan completes.
type ListenersLoadedMsg struct {
	Listeners []listener.ListenerInfo
	Err       error
}

// KillConfirmedMsg is sent when the user confirms a kill action.
type KillConfirmedMsg struct {
	PID    int32
	Signal string
}

// KillResultMsg is sent after attempting to kill a process.
type KillResultMsg struct {
	PID    int32
	Signal string
	Err    error
}

// RefreshTickMsg is sent by the auto-refresh timer.
type RefreshTickMsg time.Time

// ---------------------------------------------------------------------------
// View modes
// ---------------------------------------------------------------------------

type viewMode int

const (
	modeTable  viewMode = iota // showing the listener table
	modeDetail                 // showing detail for one process
	modeKill                   // showing kill confirmation
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the top-level Bubbletea model for the Listen Killer TUI.
type Model struct {
	// Data
	listeners []listener.ListenerInfo

	// Components
	table      table.Model
	help       help.Model
	keys       KeyMap
	styles     Styles

	// UI state
	width       int
	height      int
	ready       bool
	loading     bool
	mode        viewMode
	selectedIdx int

	// Detail view state
	detailInfo *listener.ListenerInfo

	// Kill dialog state
	killPID     int32
	killName    string
	killSignal  string
	killSignals []string
	killIdx     int

	// Auto-refresh
	autoRefresh    bool
	refreshSeconds int

	// Status message
	statusMsg string
}

// NewModel creates a new Model in loading state.
func NewModel() Model {
	columns := []table.Column{
		{Title: "PID", Width: 7},
		{Title: "Process", Width: 18},
		{Title: "User", Width: 10},
		{Title: "Port", Width: 6},
		{Title: "Address", Width: 18},
		{Title: "Uptime", Width: 10},
		{Title: "CPU%", Width: 7},
		{Title: "Memory", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)
	t.SetStyles(s)

	keys := NewKeyMap()

	return Model{
		table:          t,
		help:           help.New(),
		keys:           keys,
		styles:         DefaultStyles(),
		loading:        true,
		mode:           modeTable,
		killSignal:     "TERM",
		killSignals:    []string{"TERM", "KILL", "INT"},
		killIdx:        0,
		autoRefresh:    false,
		refreshSeconds: 3,
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadListeners(),
		tea.EnterAltScreen,
	)
}

// ---------------------------------------------------------------------------
// Row conversion
// ---------------------------------------------------------------------------

func listenersToRows(listeners []listener.ListenerInfo) []table.Row {
	rows := make([]table.Row, len(listeners))
	for i, l := range listeners {
		addr := l.Address
		if addr == "" || addr == "::" {
			addr = "*"
		}
		cpu := ""
		if l.CPUPercent > 0 {
			cpu = fmt.Sprintf("%.1f", l.CPUPercent)
		}

		rows[i] = table.Row{
			fmt.Sprintf("%d", l.PID),
			l.Name,
			l.Username,
			fmt.Sprintf("%d", l.Port),
			addr,
			l.Uptime,
			cpu,
			l.RSSHuman,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (m Model) loadListeners() tea.Cmd {
	return func() tea.Msg {
		listeners, err := listener.ScanListeners()
		return ListenersLoadedMsg{Listeners: listeners, Err: err}
	}
}

func (m Model) killProcess(pid int32, signal string) tea.Cmd {
	return func() tea.Msg {
		err := listener.KillProcess(pid, signal)
		return KillResultMsg{PID: pid, Signal: signal, Err: err}
	}
}

func autoRefreshTick(seconds int) tea.Cmd {
	return tea.Tick(time.Duration(seconds)*time.Second, func(t time.Time) tea.Msg {
		return RefreshTickMsg(t)
	})
}