package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"

	"github.com/wesen/listen-killer/pkg/listener"
)

// KillCommand kills one or more listening server processes and emits one
// structured result row per target PID.
type KillCommand struct {
	*cmds.CommandDescription
}

type KillSettings struct {
	Targets []string `glazed:"targets"`
	PIDs    []string `glazed:"pid"`
	Ports   []string `glazed:"port"`
	Name    string   `glazed:"name"`
	User    string   `glazed:"user"`
	Path    string   `glazed:"path"`
	Signal  string   `glazed:"signal"`
	DryRun  bool     `glazed:"dry-run"`
	Yes     bool     `glazed:"yes"`
}

type killTarget struct {
	Info      listener.ListenerInfo
	Ports     map[uint32]bool
	Addresses map[string]bool
	MatchedBy map[string]bool
}

func NewKillCommand() (*KillCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"kill",
		cmds.WithShort("Kill one or more TCP listening server processes"),
		cmds.WithLong(`
Kill one or more processes that currently own TCP listening sockets.

Targets may be PIDs or ports. Bare numeric positional targets are interpreted
as PIDs. Use :PORT or port:PORT to select by listening port.

The command emits structured Glazed result rows, so scripts and LLM agents can
inspect exactly what was matched and whether the signal was delivered.

For safety, destructive execution requires --yes. Use --dry-run to resolve and
inspect targets without sending a signal.

Examples:
  # Resolve targets by port without killing them
  listen-killer kill --port 3000 --dry-run --output json

  # Gracefully stop the process listening on port 3000
  listen-killer kill --port 3000 --signal TERM --yes

  # Force-kill multiple PIDs and ports, with JSON result rows
  listen-killer kill 1234 pid:5678 :8080 --signal KILL --yes --output json

  # Kill all listening node processes started from a project directory
  listen-killer kill --name node --path /home/me/project --signal TERM --yes
`),
		cmds.WithArguments(
			fields.New(
				"targets",
				fields.TypeStringList,
				fields.WithDefault([]string{}),
				fields.WithHelp("Targets: PID, pid:PID, :PORT, or port:PORT"),
				fields.WithIsArgument(true),
			),
		),
		cmds.WithFlags(
			fields.New(
				"pid",
				fields.TypeStringList,
				fields.WithDefault([]string{}),
				fields.WithHelp("PIDs to kill (repeatable or comma-separated)"),
			),
			fields.New(
				"port",
				fields.TypeStringList,
				fields.WithDefault([]string{}),
				fields.WithHelp("Listening ports to kill (repeatable or comma-separated)"),
			),
			fields.New(
				"name",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only kill listeners whose process name contains this string"),
			),
			fields.New(
				"user",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only kill listeners owned by this user substring"),
			),
			fields.New(
				"path",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only kill listeners whose executable, cwd, or cmdline contains this string"),
			),
			fields.New(
				"signal",
				fields.TypeChoice,
				fields.WithDefault("TERM"),
				fields.WithChoices("TERM", "KILL", "INT", "term", "kill", "int"),
				fields.WithHelp("Signal to send: TERM, KILL, or INT"),
			),
			fields.New(
				"dry-run",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Resolve and emit matching targets without sending a signal"),
			),
			fields.New(
				"yes",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Actually send the signal; required unless --dry-run is set"),
			),
		),
		cmds.WithSections(glazedSection, cmdSection),
	)

	return &KillCommand{CommandDescription: cmdDesc}, nil
}

var _ cmds.GlazeCommand = &KillCommand{}

func (c *KillCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	settings := &KillSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
		return err
	}

	if !settings.DryRun && !settings.Yes {
		return fmt.Errorf("refusing to kill without --yes; use --dry-run to inspect targets")
	}

	pidFilter, portFilter, err := parseKillSelectors(settings)
	if err != nil {
		return err
	}
	if len(pidFilter) == 0 && len(portFilter) == 0 && settings.Name == "" && settings.User == "" && settings.Path == "" {
		return fmt.Errorf("no kill targets specified; use PIDs, :PORT/port:PORT, --pid, --port, --name, --user, or --path")
	}

	listeners, err := listener.ScanListeners()
	if err != nil {
		return err
	}

	targets := resolveKillTargets(listeners, pidFilter, portFilter, settings)
	if len(targets) == 0 {
		return fmt.Errorf("no matching listening processes found")
	}

	var failed []string
	for _, target := range sortedKillTargets(targets) {
		killed := false
		errText := ""
		if !settings.DryRun {
			if err := listener.KillProcess(target.Info.PID, settings.Signal); err != nil {
				errText = err.Error()
				failed = append(failed, fmt.Sprintf("PID %d: %v", target.Info.PID, err))
			} else {
				killed = true
			}
		}

		row := types.NewRow(
			types.MRP("pid", target.Info.PID),
			types.MRP("name", target.Info.Name),
			types.MRP("username", target.Info.Username),
			types.MRP("ports", joinUint32Set(target.Ports)),
			types.MRP("addresses", joinStringSet(target.Addresses)),
			types.MRP("exe", target.Info.Exe),
			types.MRP("cwd", target.Info.Cwd),
			types.MRP("cmdline", target.Info.Cmdline),
			types.MRP("signal", strings.ToUpper(settings.Signal)),
			types.MRP("dry_run", settings.DryRun),
			types.MRP("killed", killed),
			types.MRP("matched_by", joinStringSet(target.MatchedBy)),
			types.MRP("error", errText),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to kill %d target(s): %s", len(failed), strings.Join(failed, "; "))
	}
	return nil
}

func parseKillSelectors(settings *KillSettings) (map[int32]bool, map[uint32]bool, error) {
	pids, err := parseInt32List(settings.PIDs, "pid")
	if err != nil {
		return nil, nil, err
	}
	ports, err := parsePortList(settings.Ports, "port")
	if err != nil {
		return nil, nil, err
	}

	for _, target := range expandStringList(settings.Targets) {
		switch {
		case strings.HasPrefix(target, "pid:"):
			pid, err := parseOneInt32(strings.TrimPrefix(target, "pid:"), "target pid")
			if err != nil {
				return nil, nil, err
			}
			pids[pid] = true
		case strings.HasPrefix(target, "port:"):
			port, err := parseOnePort(strings.TrimPrefix(target, "port:"), "target port")
			if err != nil {
				return nil, nil, err
			}
			ports[port] = true
		case strings.HasPrefix(target, ":"):
			port, err := parseOnePort(strings.TrimPrefix(target, ":"), "target port")
			if err != nil {
				return nil, nil, err
			}
			ports[port] = true
		default:
			pid, err := parseOneInt32(target, "target pid")
			if err != nil {
				return nil, nil, fmt.Errorf("invalid target %q (use PID, pid:PID, :PORT, or port:PORT)", target)
			}
			pids[pid] = true
		}
	}

	return pids, ports, nil
}

func parseOneInt32(value string, fieldName string) (int32, error) {
	n, err := strconv.ParseInt(value, 10, 32)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s %q", fieldName, value)
	}
	return int32(n), nil
}

func parseOnePort(value string, fieldName string) (uint32, error) {
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil || n == 0 || n > 65535 {
		return 0, fmt.Errorf("invalid %s %q", fieldName, value)
	}
	return uint32(n), nil
}

func resolveKillTargets(listeners []listener.ListenerInfo, pids map[int32]bool, ports map[uint32]bool, settings *KillSettings) map[int32]*killTarget {
	ret := map[int32]*killTarget{}
	for _, l := range listeners {
		matchedBy := map[string]bool{}

		if len(pids) > 0 && pids[l.PID] {
			matchedBy["pid"] = true
		}
		if len(ports) > 0 && ports[l.Port] {
			matchedBy["port"] = true
		}
		if len(pids) == 0 && len(ports) == 0 {
			matchedBy["filter"] = true
		}
		if len(matchedBy) == 0 {
			continue
		}

		if settings.Name != "" && !containsFold(l.Name, settings.Name) {
			continue
		}
		if settings.User != "" && !containsFold(l.Username, settings.User) {
			continue
		}
		if settings.Path != "" && !containsFold(l.Exe, settings.Path) && !containsFold(l.Cwd, settings.Path) && !containsFold(l.Cmdline, settings.Path) {
			continue
		}

		t, ok := ret[l.PID]
		if !ok {
			t = &killTarget{
				Info:      l,
				Ports:     map[uint32]bool{},
				Addresses: map[string]bool{},
				MatchedBy: map[string]bool{},
			}
			ret[l.PID] = t
		}
		t.Ports[l.Port] = true
		t.Addresses[normalizeAddress(l.Address)] = true
		for k := range matchedBy {
			t.MatchedBy[k] = true
		}
		if settings.Name != "" {
			t.MatchedBy["name"] = true
		}
		if settings.User != "" {
			t.MatchedBy["user"] = true
		}
		if settings.Path != "" {
			t.MatchedBy["path"] = true
		}
	}
	return ret
}

func sortedKillTargets(targets map[int32]*killTarget) []*killTarget {
	pids := make([]int, 0, len(targets))
	for pid := range targets {
		pids = append(pids, int(pid))
	}
	sort.Ints(pids)
	ret := make([]*killTarget, 0, len(targets))
	for _, pid := range pids {
		ret = append(ret, targets[int32(pid)])
	}
	return ret
}

func joinUint32Set(values map[uint32]bool) string {
	ints := make([]int, 0, len(values))
	for value := range values {
		ints = append(ints, int(value))
	}
	sort.Ints(ints)
	parts := make([]string, len(ints))
	for i, value := range ints {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, ",")
}

func joinStringSet(values map[string]bool) string {
	parts := make([]string, 0, len(values))
	for value := range values {
		parts = append(parts, value)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
