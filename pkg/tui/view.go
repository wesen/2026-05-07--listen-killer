package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	w := m.width - 2
	if w < 20 {
		w = 20
	}

	// Title bar
	title := m.styles.Title.Render("🎧 Listen Killer")
	if m.loading {
		title = m.styles.Title.Render("🎧 Listen Killer  ⏳")
	}
	marked := len(m.marked)
	countText := m.styles.TitleBar.Render(fmt.Sprintf(" %d listeners", len(m.listeners)))
	if marked > 0 {
		countText = m.styles.TitleBar.Render(fmt.Sprintf(" %d listeners  %d marked", len(m.listeners), marked))
	}
	titleBar := lipgloss.NewStyle().Width(w).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, title, countText),
	)

	// Body
	var body string
	if m.mode == modeKill {
		body = m.renderKillOverlay()
	} else if m.showDetail {
		body = m.renderSplitPane()
	} else {
		body = m.renderTable()
	}

	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
}

// ---------------------------------------------------------------------------
// Table only (no detail pane)
// ---------------------------------------------------------------------------

func (m Model) renderTable() string {
	if m.loading && len(m.listeners) == 0 {
		return m.styles.Loading.Render("\n  Scanning for TCP listeners...")
	}
	if len(m.listeners) == 0 {
		return m.styles.Loading.Render("\n  No TCP listeners found.")
	}
	return m.table.View()
}

// ---------------------------------------------------------------------------
// Split pane: table on left, detail on right
// ---------------------------------------------------------------------------

func (m Model) renderSplitPane() string {
	// Left: table (already sized to half width by Update)
	tableView := m.renderTable()

	// Right: detail pane for the currently selected row
	detailView := m.renderDetailPane()

	// Join horizontally
	gap := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, tableView, gap, detailView)
}

// ---------------------------------------------------------------------------
// Detail pane (right side)
// ---------------------------------------------------------------------------

func (m Model) renderDetailPane() string {
	info := m.selectedListener()
	if info == nil {
		return lipgloss.NewStyle().
			Width(m.width/2 - 4).
			Height(m.height - 6).
			Foreground(lipgloss.Color("243")).
			Render("\n  No selection")
	}

	addr := info.Address
	if addr == "" || addr == "::" {
		addr = "*"
	}

	// Truncate long cmdlines
	cmdline := info.Cmdline
	if len(cmdline) > 60 {
		cmdline = cmdline[:57] + "..."
	}

	// Truncate long exe paths
	exe := info.Exe
	if len(exe) > 60 {
		exe = exe[:57] + "..."
	}

	// Browser URL
	url := listenerURL(info)

	rows := []string{
		m.renderKV("PID", fmt.Sprintf("%d", info.PID)),
		m.renderKV("Name", info.Name),
		m.renderKV("Cmdline", cmdline),
		m.renderKV("Binary", exe),
		m.renderKV("User", info.Username),
		m.renderKV("Port", fmt.Sprintf("%d (%s)", info.Port, info.Protocol)),
		m.renderKV("Address", addr),
		m.renderKV("Uptime", info.Uptime),
		m.renderKV("CPU %", fmt.Sprintf("%.1f", info.CPUPercent)),
		m.renderKV("Memory", info.RSSHuman),
		m.renderKV("URL", url),
	}

	title := m.styles.DetailTitle.Render(fmt.Sprintf("%d — %s", info.PID, info.Name))
	body := strings.Join(rows, "\n")

	// Mark indicator
	markLine := ""
	if m.marked[info.PID] {
		markLine = "\n" + m.styles.StatusDanger.Render("● MARKED")
	}

	actions := "\n\n" + m.styles.DetailLabel.Render("[K] Kill  [o] Open  [space] Mark")

	paneWidth := m.width/2 - 6
	if paneWidth < 20 {
		paneWidth = 20
	}

	content := title + "\n\n" + body + markLine + actions

	return m.styles.DetailView.Width(paneWidth).Render(content)
}

// ---------------------------------------------------------------------------
// Kill dialog overlay
// ---------------------------------------------------------------------------

func (m Model) renderKillOverlay() string {
	title := m.styles.DialogTitle.Render("⚠ Kill Process")

	// Build target description
	var target string
	if len(m.killPIDs) == 1 {
		target = fmt.Sprintf("PID %d — %s", m.killPIDs[0], m.killNames[0])
	} else {
		target = fmt.Sprintf("%d processes:", len(m.killPIDs))
		for i, pid := range m.killPIDs {
			name := m.killNames[i]
			target += fmt.Sprintf("\n  • PID %d — %s", pid, name)
		}
	}

	// Signal selector
	var signalLines []string
	for i, sig := range m.killSignals {
		prefix := "  "
		style := m.styles.ButtonInactive
		if i == m.killIdx {
			prefix = "▶ "
			style = m.styles.ButtonActive
		}
		desc := ""
		switch sig {
		case "TERM":
			desc = "(graceful)"
		case "KILL":
			desc = "(force)"
		case "INT":
			desc = "(interrupt)"
		}
		signalLines = append(signalLines, style.Render(prefix+sig+" "+desc))
	}

	signalsBlock := strings.Join(signalLines, "\n")
	footer := "\n\n[y] Confirm  •  [n] Cancel  •  [↑↓] Signal"

	dialog := m.styles.Dialog.Render(
		title + "\n\n" +
			m.styles.DialogText.Render(target) + "\n\n" +
			signalsBlock +
			footer,
	)

	w := m.width - 2
	h := m.height - 6
	if w < 20 {
		w = 20
	}
	if h < 10 {
		h = 10
	}

	return lipgloss.Place(w, h,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (m Model) renderKV(label, value string) string {
	return m.styles.DetailLabel.Render(label) + " " + m.styles.DetailValue.Render(value)
}
