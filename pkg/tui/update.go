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
	var cmds []tea.Cmd

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

	// --- Keyboard ---
	case tea.KeyMsg:
		// Global keys (work in any mode)
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "?":
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}

		// Mode-specific key handling
		switch m.mode {
		case modeTable:
			return m.updateTable(msg)
		case modeDetail:
			return m.updateDetail(msg)
		case modeKill:
			return m.updateKill(msg)
		}

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
			cmds = append(cmds, m.loadListeners())
		}
		return m, tea.Batch(cmds...)

	// --- Auto-refresh tick ---
	case RefreshTickMsg:
		if m.autoRefresh && m.mode == modeTable {
			cmds = append(cmds, m.loadListeners(), autoRefreshTick(m.refreshSeconds))
		}
		return m, tea.Batch(cmds...)
	}

	// Delegate to table for its own navigation keys
	var tCmd tea.Cmd
	m.table, tCmd = m.table.Update(msg)
	cmds = append(cmds, tCmd)

	return m, tea.Batch(cmds...)
}

// ---------------------------------------------------------------------------
// Table mode
// ---------------------------------------------------------------------------

func (m Model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch {
	case key == "q":
		return m, tea.Quit

	case key == "K":
		row := m.table.SelectedRow()
		if len(row) == 0 {
			m.statusMsg = "No process selected"
			return m, nil
		}
		pidStr := row[0]
		var pid int32
		fmt.Sscanf(pidStr, "%d", &pid)
		m.killPID = pid
		m.killName = row[1]
		m.killSignal = m.killSignals[0]
		m.killIdx = 0
		m.mode = modeKill
		return m, nil

	case key == "r":
		m.loading = true
		m.statusMsg = "Refreshing..."
		return m, m.loadListeners()

	case key == "a":
		m.autoRefresh = !m.autoRefresh
		if m.autoRefresh {
			m.statusMsg = fmt.Sprintf("Auto-refresh: %ds", m.refreshSeconds)
			return m, autoRefreshTick(m.refreshSeconds)
		}
		m.statusMsg = "Auto-refresh off"
		return m, nil

	case key == "enter":
		row := m.table.SelectedRow()
		if len(row) == 0 {
			return m, nil
		}
		pidStr := row[0]
		var pid int32
		fmt.Sscanf(pidStr, "%d", &pid)
		for _, l := range m.listeners {
			if l.PID == pid {
				info := l
				m.detailInfo = &info
				m.mode = modeDetail
				return m, nil
			}
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Detail mode
// ---------------------------------------------------------------------------

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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

func (m Model) updateKill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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

	case "up", "k":
		if m.killIdx > 0 {
			m.killIdx--
			m.killSignal = m.killSignals[m.killIdx]
		}
		return m, nil

	case "down", "j":
		if m.killIdx < len(m.killSignals)-1 {
			m.killIdx++
			m.killSignal = m.killSignals[m.killIdx]
		}
		return m, nil
	}
	return m, nil
}

// renderFooter builds the bottom status/help bar.
func (m Model) renderFooter() string {
	status := m.styles.StatusOK.Render(m.statusMsg)
	if strings.HasPrefix(m.statusMsg, "Error") || strings.HasPrefix(m.statusMsg, "Failed") {
		status = m.styles.StatusDanger.Render(m.statusMsg)
	}

	helpView := m.help.View(m.keys)
	spacer := lipgloss.NewStyle().Width(m.width - lipgloss.Width(status) - lipgloss.Width(helpView) - 4).Render("")
	footer := lipgloss.JoinHorizontal(lipgloss.Left, status, spacer, helpView)
	return m.styles.Footer.Width(m.width - 2).Render(footer)
}