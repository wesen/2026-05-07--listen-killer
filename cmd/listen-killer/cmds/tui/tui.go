package tui

import (
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
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
	TUI   bool     `glazed:"tui"`
	NoTUI bool     `glazed:"no-tui"`
	PIDs  []string `glazed:"pid"`
	Ports []string `glazed:"port"`
	Name  string   `glazed:"name"`
	User  string   `glazed:"user"`
	Path  string   `glazed:"path"`
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
dashboard where you can browse, mark, open, and kill listening processes.

In CLI mode (--no-tui or when piped), emits structured Glazed rows that can be
formatted as JSON, YAML, CSV, table, or field-selected output. This mode is
intended for scripts, CI diagnostics, and LLM-agent workflows.

Examples:
  # Interactive TUI dashboard
  listen-killer

  # CLI: output as JSON for scripting / LLM-agent workflows
  listen-killer list --no-tui --output json

  # CLI: show selected fields only
  listen-killer list --no-tui --fields pid,name,port,cwd,cmdline

  # CLI: find one port or process family
  listen-killer list --no-tui --port 3000 --output json
  listen-killer list --no-tui --name node --fields pid,port,cwd,cmdline

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
			fields.New(
				"pid",
				fields.TypeStringList,
				fields.WithDefault([]string{}),
				fields.WithHelp("Only show listeners owned by these PIDs (repeatable or comma-separated)"),
			),
			fields.New(
				"port",
				fields.TypeStringList,
				fields.WithDefault([]string{}),
				fields.WithHelp("Only show these listening ports (repeatable or comma-separated)"),
			),
			fields.New(
				"name",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only show listeners whose process name contains this string"),
			),
			fields.New(
				"user",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only show listeners owned by this user substring"),
			),
			fields.New(
				"path",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only show listeners whose executable, cwd, or cmdline contains this string"),
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

	// Decide mode: TUI unless explicitly disabled or not a TTY.
	useTUI := settings.TUI
	if !settings.NoTUI && !settings.TUI {
		useTUI = isTerminal()
	}

	if useTUI {
		return runTUI()
	}

	return runCLI(ctx, gp, settings)
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

// runCLI scans listeners, applies script-friendly filters, and emits them
// through the Glazed processor.
func runCLI(ctx context.Context, gp middlewares.Processor, settings *ListSettings) error {
	listeners, err := listener.ScanListeners()
	if err != nil {
		return err
	}

	pidFilter, err := parseInt32List(settings.PIDs, "pid")
	if err != nil {
		return err
	}
	portFilter, err := parsePortList(settings.Ports, "port")
	if err != nil {
		return err
	}

	for _, l := range listeners {
		if !matchesListFilters(l, pidFilter, portFilter, settings) {
			continue
		}
		if err := gp.AddRow(ctx, listenerRow(l)); err != nil {
			return err
		}
	}

	return nil
}

func matchesListFilters(l listener.ListenerInfo, pids map[int32]bool, ports map[uint32]bool, settings *ListSettings) bool {
	if len(pids) > 0 && !pids[l.PID] {
		return false
	}
	if len(ports) > 0 && !ports[l.Port] {
		return false
	}
	if settings.Name != "" && !containsFold(l.Name, settings.Name) {
		return false
	}
	if settings.User != "" && !containsFold(l.Username, settings.User) {
		return false
	}
	if settings.Path != "" && !containsFold(l.Exe, settings.Path) && !containsFold(l.Cwd, settings.Path) && !containsFold(l.Cmdline, settings.Path) {
		return false
	}
	return true
}
