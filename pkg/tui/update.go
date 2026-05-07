package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// --- Window resize ---
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(msg.Height - 6)
		m.table.SetWidth(msg.Width - 2)
		if !m.ready {
			m.ready = true
		}
		return m, nil

	// --- Keyboard ---
	case tea.KeyMsg:
		key := msg.String()

		// ctrl+c always quits
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		// ? toggles help in any mode
		if key == "?" {
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}

		switch m.mode {
		case modeTable:
			// In table mode: intercept our action keys,
			// delegate everything else to the table widget
			// for navigation (up/down/j/k/pgup/pgdn/home/end).
			var cmd tea.Cmd
			m, handled, cmd := m.handleTableKey(key)
			if handled {
				return m, cmd
			}
			// Not one of our keys — let the table handle it
			var tCmd tea.Cmd
			m.table, tCmd = m.table.Update(msg)
			return m, tCmd

		case modeDetail:
			return m.updateDetail(key)

		case modeKill:
			return m.updateKill(key)
		}

		return m, nil

	// --- Data loaded ---
	case ListenersLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.Err)
		} else {
			m.listeners = msg.Listeners
			m.table.SetRows(listenersToRows(m.listeners))
			m.statusMsg = fmt.Sprintf("%d listeners", len(m.listeners))
		}
		return m, nil

	// --- Kill result ---
	case KillResultMsg:
		m.mode = modeTable
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Failed: %v", msg.Err)
		} else {
			m.statusMsg = fmt.Sprintf("Sent %s to PID %d", msg.Signal, msg.PID)
			return m, m.loadListeners()
		}
		return m, nil

	// --- Auto-refresh tick ---
	case RefreshTickMsg:
		if m.autoRefresh && m.mode == modeTable {
			return m, tea.Batch(m.loadListeners(), autoRefreshTick(m.refreshSeconds))
		}
		return m, nil
	}

	// Delegate non-key messages (e.g. focus) to the table
	var tCmd tea.Cmd
	m.table, tCmd = m.table.Update(msg)
	return m, tCmd
}

// ---------------------------------------------------------------------------
// Table mode key handling
// ---------------------------------------------------------------------------

// handleTableKey intercepts action keys in table mode.
// Returns the (possibly modified) model, whether the key was consumed,
// and an optional command.
func (m Model) handleTableKey(key string) (Model, bool, tea.Cmd) {
	switch key {

	case "q":
		return m, true, tea.Quit

	case "K":
		row := m.table.SelectedRow()
		if len(row) == 0 {
			m.statusMsg = "No process selected"
			return m, true, nil
		}
		var pid int32
		fmt.Sscanf(row[0], "%d", &pid)
		m.killPID = pid
		m.killName = row[1]
		m.killSignal = m.killSignals[0]
		m.killIdx = 0
		m.mode = modeKill
		return m, true, nil

	case "r":
		m.loading = true
		m.statusMsg = "Refreshing..."
		return m, true, m.loadListeners()

	case "a":
		m.autoRefresh = !m.autoRefresh
		if m.autoRefresh {
			m.statusMsg = fmt.Sprintf("Auto-refresh: %ds", m.refreshSeconds)
			return m, true, autoRefreshTick(m.refreshSeconds)
		}
		m.statusMsg = "Auto-refresh off"
		return m, true, nil

	case "enter":
		row := m.table.SelectedRow()
		if len(row) == 0 {
			return m, true, nil
		}
		var pid int32
		fmt.Sscanf(row[0], "%d", &pid)
		for _, l := range m.listeners {
			if l.PID == pid {
				info := l
				m.detailInfo = &info
				m.mode = modeDetail
				return m, true, nil
			}
		}
		return m, true, nil

	case "/":
		m.statusMsg = "Filter: not yet implemented"
		return m, true, nil
	}

	// Not our key — let the table handle navigation (up/down/j/k/pgup/pgdn/etc.)
	return m, false, nil
}

// ---------------------------------------------------------------------------
// Detail mode
// ---------------------------------------------------------------------------

func (m Model) updateDetail(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		m.mode = modeTable
		m.detailInfo = nil
		return m, nil
	case "K":
		if m.detailInfo != nil {
			m.killPID = m.detailInfo.PID
			m.killName = m.detailInfo.Name
			m.killSignal = m.killSignals[0]
			m.killIdx = 0
			m.mode = modeKill
		}
		return m, nil
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Kill confirmation mode
// ---------------------------------------------------------------------------

func (m Model) updateKill(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		m.mode = modeTable
		return m, nil

	case "y", "Y", "enter":
		m.mode = modeTable
		m.statusMsg = fmt.Sprintf("Killing PID %d with %s...", m.killPID, m.killSignal)
		return m, m.killProcess(m.killPID, m.killSignal)

	case "n", "N":
		m.mode = modeTable
		m.statusMsg = "Kill cancelled"
		return m, nil

	case "up":
		if m.killIdx > 0 {
			m.killIdx--
			m.killSignal = m.killSignals[m.killIdx]
		}
		return m, nil

	case "down":
		if m.killIdx < len(m.killSignals)-1 {
			m.killIdx++
			m.killSignal = m.killSignals[m.killIdx]
		}
		return m, nil
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Footer
// ---------------------------------------------------------------------------

func (m Model) renderFooter() string {
	status := m.styles.StatusOK.Render(m.statusMsg)
	if strings.HasPrefix(m.statusMsg, "Error") || strings.HasPrefix(m.statusMsg, "Failed") {
		status = m.styles.StatusDanger.Render(m.statusMsg)
	}

	helpView := m.help.View(m.keys)

	// Guard against zero/negative width before first WindowSizeMsg
	avail := m.width - lipgloss.Width(status) - lipgloss.Width(helpView) - 4
	if avail < 0 {
		avail = 0
	}
	spacer := lipgloss.NewStyle().Width(avail).Render("")
	footer := lipgloss.JoinHorizontal(lipgloss.Left, status, spacer, helpView)

	fw := m.width - 2
	if fw < 10 {
		fw = 10
	}
	return m.styles.Footer.Width(fw).Render(footer)
}
