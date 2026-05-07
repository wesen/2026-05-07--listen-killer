package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"golang.org/x/term"

	"github.com/wesen/listen-killer/pkg/listener"
	apptui "github.com/wesen/listen-killer/pkg/tui"
)

// ListCommand is a Glazed command that either launches the TUI or outputs
// structured data, depending on the --tui / --no-tui flags and whether
// stdin is a TTY.
type ListCommand struct {
	*cmds.CommandDescription
}

// ListSettings maps CLI flags to a struct.
type ListSettings struct {
	TUI   bool `glazed:"tui"`
	NoTUI bool `glazed:"no-tui"`
}

// NewListCommand creates the Glazed command with field definitions and sections.
func NewListCommand() (*ListCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}

	cmdSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"list",
		cmds.WithShort("List TCP listening daemons"),
		cmds.WithLong(`
Show all TCP listening sockets on the system with process details.

In TUI mode (default when running in a terminal), launches an interactive
dashboard where you can browse, sort, filter, and kill listening processes.

In CLI mode (--no-tui or when piped), outputs structured data that can be
formatted as JSON, YAML, CSV, or a table.

Examples:
  # Interactive TUI dashboard
  listen-killer

  # CLI: output as JSON for scripting
  listen-killer list --no-tui --output json

  # CLI: table output
  listen-killer list --no-tui

  # Force TUI even when piped
  listen-killer list --tui
`),
		cmds.WithFlags(
			fields.New(
				"tui",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Force TUI mode even when piped"),
			),
			fields.New(
				"no-tui",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Force CLI mode even when in a terminal"),
			),
		),
		cmds.WithSections(glazedSection, cmdSection),
	)

	return &ListCommand{CommandDescription: cmdDesc}, nil
}

var _ cmds.GlazeCommand = &ListCommand{}

// RunIntoGlazeProcessor implements the GlazeCommand interface.
func (c *ListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &ListSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
		return err
	}

	// Decide mode: TUI unless explicitly disabled or not a TTY
	useTUI := settings.TUI
	if !settings.NoTUI && !settings.TUI {
		useTUI = isTerminal()
	}

	if useTUI {
		return runTUI()
	}

	return runCLI(ctx, gp)
}

// isTerminal returns true if stdin is a terminal.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// runTUI launches the Bubbletea TUI.
func runTUI() error {
	m := apptui.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// runCLI scans listeners and emits them through the Glazed processor.
func runCLI(ctx context.Context, gp middlewares.Processor) error {
	listeners, err := listener.ScanListeners()
	if err != nil {
		return err
	}

	for _, l := range listeners {
		addr := l.Address
		if addr == "" || addr == "::" {
			addr = "*"
		}
		cpuStr := ""
		if l.CPUPercent > 0 {
			cpuStr = formatFloat(l.CPUPercent)
		}

		row := types.NewRow(
			types.MRP("pid", l.PID),
			types.MRP("name", l.Name),
			types.MRP("cmdline", l.Cmdline),
			types.MRP("exe", l.Exe),
			types.MRP("username", l.Username),
			types.MRP("port", l.Port),
			types.MRP("address", addr),
			types.MRP("protocol", l.Protocol),
			types.MRP("uptime", l.Uptime),
			types.MRP("uptime_seconds", l.UptimeSeconds),
			types.MRP("cpu_percent", cpuStr),
			types.MRP("rss_bytes", l.RSSBytes),
			types.MRP("rss_human", l.RSSHuman),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f", f)
}