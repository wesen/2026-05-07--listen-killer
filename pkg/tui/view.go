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
	countText := m.styles.TitleBar.Render(fmt.Sprintf(" %d listeners ", len(m.listeners)))
	titleBar := lipgloss.NewStyle().Width(w).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, title, countText),
	)

	// Body — always render the table as the base layer
	body := m.renderTable()

	// Overlay the detail or kill dialog on top
	switch m.mode {
	case modeDetail:
		body = m.renderDetailOverlay(body)
	case modeKill:
		body = m.renderKillOverlay(body)
	}

	// Footer
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
}

// ---------------------------------------------------------------------------
// Table view (always rendered as the base layer)
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
// Detail view (rendered as overlay on top of the table)
// ---------------------------------------------------------------------------

func (m Model) renderDetailOverlay(base string) string {
	if m.detailInfo == nil {
		return base
	}

	info := m.detailInfo
	addr := info.Address
	if addr == "" || addr == "::" {
		addr = "*"
	}

	// Truncate long cmdlines for display
	cmdline := info.Cmdline
	if len(cmdline) > 80 {
		cmdline = cmdline[:77] + "..."
	}

	rows := []string{
		m.renderKV("PID", fmt.Sprintf("%d", info.PID)),
		m.renderKV("Name", info.Name),
		m.renderKV("Cmdline", cmdline),
		m.renderKV("Binary", info.Exe),
		m.renderKV("User", info.Username),
		m.renderKV("Port", fmt.Sprintf("%d (%s)", info.Port, info.Protocol)),
		m.renderKV("Address", addr),
		m.renderKV("Uptime", info.Uptime),
		m.renderKV("CPU %", fmt.Sprintf("%.1f", info.CPUPercent)),
		m.renderKV("Memory RSS", info.RSSHuman),
	}

	title := m.styles.DetailTitle.Render(fmt.Sprintf("Process %d — %s", info.PID, info.Name))
	body := strings.Join(rows, "\n")
	footer := "\n\n" + m.styles.DetailLabel.Render("[K] Kill  •  [esc] Back")

	detailBox := m.styles.DetailView.Render(title + "\n\n" + body + footer)

	// Center the detail box on top of the base view
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
		detailBox,
	)
}

func (m Model) renderKV(label, value string) string {
	return m.styles.DetailLabel.Render(label) + " " + m.styles.DetailValue.Render(value)
}

// ---------------------------------------------------------------------------
// Kill dialog (rendered as overlay on top of the table)
// ---------------------------------------------------------------------------

func (m Model) renderKillOverlay(base string) string {
	title := m.styles.DialogTitle.Render("⚠ Kill Process")

	info := fmt.Sprintf("PID %d — %s", m.killPID, m.killName)

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
			desc = "(graceful — SIGTERM)"
		case "KILL":
			desc = "(force — SIGKILL)"
		case "INT":
			desc = "(interrupt — SIGINT)"
		}
		signalLines = append(signalLines, style.Render(prefix+sig+" "+desc))
	}

	signalsBlock := strings.Join(signalLines, "\n")
	footer := "\n\n[y] Confirm  •  [n] Cancel  •  [↑↓] Change signal"

	dialog := m.styles.Dialog.Render(
		title + "\n\n" +
			m.styles.DialogText.Render(info) + "\n\n" +
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
