---
title: "Intern Guide - Architecture and Implementation"
status: active
intent: long-term
topics: [go, bubbletea, glazed, tui, networking]
created: 2026-05-07
---

# Intern Guide: Listen Killer — Architecture & Implementation

> **Target audience**: A new intern who knows Go basics but hasn't built a Bubbletea TUI or used Glazed before.  
> **Goal**: After reading this, you should understand every file, every design decision, and be able to extend the app.

---

## Table of Contents

1. [What We're Building](#1-what-were-building)
2. [The Big Picture: Three Layers](#2-the-big-picture-three-layers)
3. [Layer 1: Data Gathering (`pkg/listener/`)](#3-layer-1-data-gathering-pkglistener)
4. [Layer 2: The TUI (`pkg/tui/`)](#4-layer-2-the-tui-pkgtui)
5. [Layer 3: The CLI (`cmd/listen-killer/`)](#5-layer-3-the-cli-cmdlisten-killer)
6. [The Bubbletea Architecture (Elm Architecture)](#6-the-bubbletea-architecture-elm-architecture)
7. [The Glazed Command Framework](#7-the-glazed-command-framework)
8. [Key Bindings Reference](#8-key-bindings-reference)
9. [File-by-File Walkthrough](#9-file-by-file-walkthrough)
10. [How to Extend](#10-how-to-extend)

---

## 1. What We're Building

**Listen Killer** is a terminal dashboard that shows every TCP daemon listening on your machine. Think of it as `ss -tlnp` meets `htop` — but focused on network listeners. You can:

- See all listening processes in a sortable, filterable table
- Kill any process with TERM, KILL, or INT
- See how long each daemon has been running
- Export the list as JSON/YAML/CSV for scripting

The app has **two modes**:

| Mode | Command | Output |
|------|---------|--------|
| TUI (interactive) | `listen-killer` | Bubbletea dashboard |
| CLI (scripting) | `listen-killer list --output json` | Structured data via Glazed |

---

## 2. The Big Picture: Three Layers

```
┌──────────────────────────────────────────────────┐
│  LAYER 3: CLI (cmd/listen-killer/)                │
│  Cobra root + Glazed wiring + help system         │
│  Responsibility: parse flags, decide TUI vs CLI    │
├──────────────────────────────────────────────────┤
│  LAYER 2: TUI (pkg/tui/)                          │
│  Bubbletea Model/Update/View + table + dialogs    │
│  Responsibility: render dashboard, handle keys     │
├──────────────────────────────────────────────────┤
│  LAYER 1: Data (pkg/listener/)                    │
│  Scanner + process info + types                   │
│  Responsibility: talk to the OS, gather facts      │
└──────────────────────────────────────────────────┘
```

**Why three layers?** Separation of concerns. The scanner doesn't know about Bubbletea. The TUI doesn't know about Cobra flags. The CLI layer glues them together. This means you can:
- Reuse the scanner in a web API
- Swap the TUI for a different framework without touching data gathering
- Test each layer independently

---

## 3. Layer 1: Data Gathering (`pkg/listener/`)

### 3.1 `types.go` — The Data Model

```go
// pkg/listener/types.go

package listener

// ListenerInfo is the central data type. Every listening TCP socket
// produces one of these.
type ListenerInfo struct {
    PID           int32   `json:"pid"            glazed:"pid"`
    Name          string  `json:"name"           glazed:"name"`
    Cmdline       string  `json:"cmdline"        glazed:"cmdline"`
    Exe           string  `json:"exe"            glazed:"exe"`
    Username      string  `json:"username"       glazed:"username"`
    Port          uint32  `json:"port"           glazed:"port"`
    Address       string  `json:"address"        glazed:"address"`
    Uptime        string  `json:"uptime"         glazed:"uptime"`
    UptimeSeconds int64   `json:"uptime_seconds" glazed:"uptime_seconds"`
    CPUPercent    float64 `json:"cpu_percent"    glazed:"cpu_percent"`
    RSSBytes      uint64  `json:"rss_bytes"      glazed:"rss_bytes"`
    RSSHuman      string  `json:"rss_human"      glazed:"rss_human"`
}
```

**Note the `glazed` struct tags**: These let the Glazed framework auto-map fields to table columns and JSON keys. This is the contract between the data layer and the output layer.

### 3.2 `scanner.go` — Finding Listeners

**Pseudocode**:

```
function ScanListeners():
    connections = gopsutil.net.Connections("inet")  // all IPv4 + IPv6 sockets
    listeners = []

    for each conn in connections:
        if conn.Type == SOCK_STREAM AND conn.Status == "LISTEN":
            proc = gopsutil.process.NewProcess(conn.Pid)
            info = gatherProcessInfo(proc, conn)
            listeners.append(info)

    return listeners
```

**Key library**: [`github.com/shirou/gopsutil/v3`](https://github.com/shirou/gopsutil) — reads `/proc/net/tcp`, `/proc/<pid>/stat`, etc. without shelling out.

**Why not shell out to `ss` or `lsof`?**
- Parsing text output is fragile (column widths change)
- gopsutil gives us typed structs
- Works cross-platform (the same API works on macOS via different sysctls)

### 3.3 `process.go` — Process Details

For each listening process, we gather:

| Field | Source (gopsutil call) | Notes |
|-------|----------------------|-------|
| PID | `conn.Pid` | From the socket table |
| Name | `proc.Name()` | The executable name |
| Cmdline | `proc.Cmdline()` | Full command line |
| Exe | `proc.Exe()` | Path to binary |
| Username | `proc.Username()` | Who owns it |
| Port | `conn.Laddr.Port` | Local port |
| Address | `conn.Laddr.IP` | Bound address (0.0.0.0, 127.0.0.1, etc.) |
| CreateTime | `proc.CreateTime()` | Unix millis → compute uptime |
| CPUPercent | `proc.CPUPercent()` | Requires two calls ~1s apart; use cached value |
| Memory | `proc.MemoryInfo()` | → RSS in bytes |

**Important gotcha with `CPUPercent()`**:
- gopsutil's `CPUPercent()` blocks for ~1 second on first call (it needs two samples)
- Use `proc.Percent(0)` with a 0 duration to get the last cached value
- Or call it once on startup and accept the delay

---

## 4. Layer 2: The TUI (`pkg/tui/`)

### 4.1 The Bubbletea Elm Architecture

Bubbletea follows the **Elm Architecture** — three functions that form a loop:

```
         ┌──────────┐
         │   Model   │  ← holds all state (listeners list, cursor, filter, etc.)
         └─────┬─────┘
               │
    ┌──────────┴──────────┐
    │       Update()       │  ← receives messages, returns new Model + commands
    └──────────┬──────────┘
               │
    ┌──────────┴──────────┐
    │       View()         │  ← renders Model as a string
    └──────────┬──────────┘
               │
               ▼   (sent to terminal)
```

**Messages** are the key concept. Everything is a message:

```go
type tea.Msg interface{}  // could be tea.KeyMsg, tea.WindowSizeMsg, or your custom type

// Examples of custom messages:
type ListenersLoadedMsg []listener.ListenerInfo   // scanner finished
type KillConfirmedMsg struct { PID int32; Signal string }  // user confirmed kill
type TickMsg time.Time                              // auto-refresh timer
```

### 4.2 `model.go` — The Main Model

```go
package tui

import (
    "github.com/charmbracelet/bubbletea"
    "github.com/go-go-golems/bubble-table/table"
    "github.com/wesen/listen-killer/pkg/listener"
)

type Model struct {
    // The table component (from bubble-table)
    table      table.Model

    // Raw data
    listeners  []listener.ListenerInfo

    // UI state
    width      int
    height     int
    ready      bool
    loading    bool
    showDetail bool
    selected   *listener.ListenerInfo  // for detail view

    // Dialogs
    showKillDialog bool
    killSignal     string  // "TERM", "KILL", "INT"

    // Auto-refresh
    autoRefresh    bool
    refreshSeconds int

    // Key bindings
    keys      KeyMap
    styles    Styles
}
```

**State diagram for the Model**:

```
                  ┌──────────┐
     Startup ───▶ │ loading  │
                  └────┬─────┘
                       │ ListenersLoadedMsg
                       ▼
                  ┌──────────┐
         ┌──────▶ │  table   │ ◀──── refresh (r / auto)
         │        └────┬─────┘
         │             │ k (kill key)
         │             ▼
         │        ┌──────────┐
         │   ┌─── │kill dialog│
         │   │    └────┬─────┘
         │   │         │ confirm / cancel
         │   │         ▼
         │   │    ┌──────────┐
         │   └──▶ │ killing  │ ──▶ KillConfirmedMsg ──▶ refresh
         │        └──────────┘
         │             │ enter (detail)
         │             ▼
         │        ┌──────────┐
         │   ┌─── │ detail   │
         │   │    └────┬─────┘
         │   │         │ esc / q
         │   └─────────┘
         │
         │   q / ctrl+c ──▶ tea.Quit
         └────────────────────────┘
```

### 4.3 `keymap.go` — Key Bindings

Following the bobatea REPL pattern:

```go
type KeyMap struct {
    Up        key.Binding  // up/k       — move cursor up
    Down      key.Binding  // down/j     — move cursor down
    Kill      key.Binding  // k          — kill selected process
    Refresh   key.Binding  // r          — refresh list
    Detail    key.Binding  // enter      — show process detail
    Filter    key.Binding  // /          — filter by name
    Sort      key.Binding  // s          — change sort column
    AutoRefresh key.Binding // a         — toggle auto-refresh
    Quit      key.Binding  // q/ctrl+c   — quit
    Help      key.Binding  // ?          — show help
    Back      key.Binding  // esc        — go back from detail/dialog
}

func (k KeyMap) ShortHelp() []key.Binding {
    return []key.Binding{k.Up, k.Down, k.Kill, k.Refresh, k.Quit, k.Help}
}

func (k KeyMap) FullHelp() [][]key.Binding {
    return [][]key.Binding{ /* all bindings */ }
}
```

### 4.4 `styles.go` — Visual Theme

Uses **lipgloss** for declarative styling:

```go
type Styles struct {
    TableHeader   lipgloss.Style  // bold white on dark blue
    TableCell     lipgloss.Style  // normal text
    SelectedRow   lipgloss.Style  // highlighted background
    Title         lipgloss.Style  // "Listen Killer" header
    Footer        lipgloss.Style  // help bar at bottom
    Dialog        lipgloss.Style  // popup border
    DialogButton  lipgloss.Style  // "Yes"/"No" buttons
    PortCell      lipgloss.Style  // ports in cyan
    PidCell       lipgloss.Style  // PIDs in yellow
    DangerCell    lipgloss.Style  // kill-related in red
}
```

### 4.5 `update.go` — Message Handling

**Pseudocode for the Update function**:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    // --- Window resize ---
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.table = m.table.WithTargetWidth(msg.Width)

    // --- Keyboard ---
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit

        case "k":
            // Show kill confirmation dialog
            if m.table.HasSelection() {
                m.showKillDialog = true
            }

        case "r":
            // Trigger refresh
            return m, m.loadListeners()

        case "/":
            // Activate filter
            m.table = m.table.WithFilterInput(true)

        case "enter":
            // Show detail for selected process
            rowIdx := m.table.GetSelectedRowIndex()
            m.selected = &m.listeners[rowIdx]
            m.showDetail = true
        }

    // --- Custom messages ---
    case ListenersLoadedMsg:
        m.listeners = msg
        m.loading = false
        rows := listenersToRows(m.listeners)
        return m, m.table.SetRows(rows)

    case TickMsg:
        if m.autoRefresh {
            return m, m.loadListeners()
        }
    }

    // --- Delegate to table ---
    // Bubbletea composition: forward unknown messages to child components
    var cmd tea.Cmd
    m.table, cmd = m.table.Update(msg)
    return m, cmd
}
```

### 4.6 `view.go` — Rendering

```go
func (m Model) View() string {
    if !m.ready {
        return "Loading...\n"
    }
    if m.loading {
        return m.styles.Title.Render("Listen Killer") + "\n\nRefreshing..."
    }

    // Main layout:
    // ┌────────────────────────────┐
    // │  Title bar                 │
    // ├────────────────────────────┤
    // │  Table                     │
    // │  (listeners)               │
    // │                            │
    // ├────────────────────────────┤
    // │  Footer (help / status)    │
    // └────────────────────────────┘

    title := m.styles.Title.Render("Listen Killer") +
             fmt.Sprintf("  %d listeners", len(m.listeners))

    table := m.table.View()

    footer := m.renderFooter()

    // If kill dialog is showing, overlay it
    if m.showKillDialog {
        overlay := m.renderKillDialog()
        return lipgloss.Place(m.width, m.height,
            lipgloss.Center, lipgloss.Center, overlay)
    }

    return lipgloss.JoinVertical(lipgloss.Left, title, table, footer)
}
```

### 4.7 `table.go` — Table Setup

```go
func NewTable(width int) table.Model {
    columns := []table.Column{
        table.NewColumn("pid", "PID", 7),
        table.NewColumn("name", "Process", 20),
        table.NewColumn("username", "User", 10),
        table.NewColumn("port", "Port", 6),
        table.NewColumn("address", "Address", 16),
        table.NewColumn("uptime", "Uptime", 12),
        table.NewColumn("cpu", "CPU%", 7),
        table.NewColumn("rss", "Memory", 10),
    }

    return table.New(columns).
        WithTargetWidth(width).
        WithSelectableRows(true).
        WithSortableColumns(true).
        WithPageSize(50).
        WithPaginationWrapping(true).
        WithBaseStyle(lipgloss.NewStyle().Padding(0, 1))
}
```

### 4.8 `dialogs.go` — Kill Confirmation

Uses **huh** forms (same pattern as lsof-who):

```go
func (m Model) showKillForm() tea.Cmd {
    var confirm bool
    var signal string = "TERM"

    form := huh.NewForm(
        huh.NewGroup(
            huh.NewConfirm().
                Title("Kill process?").
                Description(fmt.Sprintf("PID %d — %s", m.selected.PID, m.selected.Name)).
                Value(&confirm),
            huh.NewSelect[string]().
                Title("Signal").
                Options(
                    huh.NewOption("TERM (graceful)", "TERM"),
                    huh.NewOption("KILL (force)", "KILL"),
                    huh.NewOption("INT (interrupt)", "INT"),
                ).
                Value(&signal),
        ),
    )

    return func() tea.Msg {
        err := form.Run()
        if err != nil {
            return KillCancelledMsg{}
        }
        if confirm {
            return KillConfirmedMsg{PID: m.selected.PID, Signal: signal}
        }
        return KillCancelledMsg{}
    }
}
```

---

## 5. Layer 3: The CLI (`cmd/listen-killer/`)

### 5.1 `main.go` — Root Command

```go
package main

import (
    "fmt"
    "os"

    "github.com/go-go-golems/glazed/pkg/cmds/logging"
    "github.com/go-go-golems/glazed/pkg/help"
    help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
    "github.com/spf13/cobra"

    tui_cmd "github.com/wesen/listen-killer/cmd/listen-killer/cmds/tui"
)

var rootCmd = &cobra.Command{
    Use:   "listen-killer",
    Short: "Terminal dashboard for managing listening TCP daemons",
    Long: `Listen Killer shows all TCP listening sockets in an interactive table.
You can sort, filter, kill processes, and export data as JSON/YAML/CSV.`,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        return logging.InitLoggerFromCobra(cmd)
    },
}

func main() {
    // 1. Add logging flags
    _ = logging.AddLoggingSectionToRootCommand(rootCmd, "listen-killer")

    // 2. Register the TUI command
    listCmd, err := tui_cmd.NewListCommand()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    cobraListCmd, err := cli.BuildCobraCommand(listCmd,
        cli.WithParserConfig(cli.CobraParserConfig{
            AppName: "listen-killer",
        }),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    rootCmd.AddCommand(cobraListCmd)

    // 3. Setup help system
    helpSystem := help.NewHelpSystem()
    help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

    // 4. Run
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### 5.2 `cmds/tui/tui.go` — The Glazed Command

```go
package tui

type ListCommand struct {
    *cmds.CommandDescription
}

type ListSettings struct {
    TUI      bool `glazed:"tui"`
    NoTUI    bool `glazed:"no-tui"`
}

func NewListCommand() (*ListCommand, error) {
    // ... setup Glazed sections, fields ...
    return &ListCommand{CommandDescription: desc}, nil
}

func (c *ListCommand) RunIntoGlazeProcessor(
    ctx context.Context,
    vals *values.Values,
    gp middlewares.Processor,
) error {
    settings := &ListSettings{}
    vals.DecodeSectionInto(schema.DefaultSlug, settings)

    // Decide: TUI or CLI mode?
    useTUI := settings.TUI || (!settings.NoTUI && isTerminal())

    if useTUI {
        // Launch Bubbletea
        m := tui.NewModel()
        p := tea.NewProgram(m, tea.WithAltScreen())
        _, err := p.Run()
        return err
    }

    // CLI mode: scan and emit rows via Glazed
    listeners, err := listener.ScanListeners()
    for _, l := range listeners {
        row := types.NewRow(
            types.MRP("pid", l.PID),
            types.MRP("name", l.Name),
            // ...
        )
        gp.AddRow(ctx, row)
    }
    return nil
}
```

**Dual-mode design**: One command, two behaviors:

| Condition | Behavior |
|-----------|----------|
| `--tui` flag or TTY detected | Launch Bubbletea dashboard |
| `--no-tui` or piped/redirected | Emit structured rows via Glazed (supports `--output json`, `--output csv`, etc.) |

---

## 6. The Bubbletea Architecture (Elm Architecture)

### 6.1 The Three Functions

Every Bubbletea program has:

```go
// Model — holds all state
type Model struct { ... }

// Init — returns the initial command (e.g., load data)
func (m Model) Init() tea.Cmd { ... }

// Update — handles messages, returns new model + optional commands
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }

// View — renders model as string
func (m Model) View() string { ... }
```

### 6.2 Commands (`tea.Cmd`)

A **command** is `func() tea.Msg` — a function that runs asynchronously and returns a message. You don't call it directly; you return it from `Update()` or `Init()`, and Bubbletea runs it in a goroutine.

```go
// Example: load data asynchronously
func loadListeners() tea.Cmd {
    return func() tea.Msg {
        listeners, err := listener.ScanListeners()
        if err != nil {
            return ErrorMsg{err}
        }
        return ListenersLoadedMsg(listeners)
    }
}
```

### 6.3 Composition Pattern

Don't put everything in one Model. Delegate to child components:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Handle messages I care about
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
    }

    // Delegate everything else to the table
    var cmd tea.Cmd
    m.table, cmd = m.table.Update(msg)
    return m, cmd
}
```

This is how `bubble-table` works — it's a self-contained component that you embed in your model.

### 6.4 Auto-Refresh with Ticks

```go
func autoRefreshTick(seconds int) tea.Cmd {
    return tea.Tick(time.Duration(seconds)*time.Second, func(t time.Time) tea.Msg {
        return TickMsg(t)
    })
}

// In Update:
case TickMsg:
    if m.autoRefresh {
        return m, tea.Batch(
            m.loadListeners(),           // reload data
            autoRefreshTick(m.refreshSeconds), // schedule next tick
        )
    }
```

---

## 7. The Glazed Command Framework

### 7.1 What Glazed Does

Glazed is a layer on top of Cobra that adds:

1. **Structured output**: Your command emits `Row` objects, and Glazed renders them as JSON, YAML, CSV, or table — without you writing format-specific code.
2. **Field definitions**: Declare flags with types, defaults, and help text in a schema-driven way.
3. **Sections**: Group related flags into reusable sections.
4. **Help system**: Auto-generates help from field definitions.

### 7.2 Key Types

```go
// cmds.CommandDescription — metadata + fields + sections
type CommandDescription struct {
    Name     string
    Short    string
    Long     string
    Flags    []*fields.Field
    Sections []schema.Section
}

// The interface you implement:
type GlazeCommand interface {
    RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error
}

// middlewares.Processor — receives rows
type Processor interface {
    AddRow(ctx context.Context, row *types.Row) error
}
```

### 7.3 The Constructor Pattern

Every Glazed command follows this pattern:

```go
func NewFooCommand() (*FooCommand, error) {
    // 1. Create the output section (adds --output, --fields, etc.)
    glazedSection, _ := settings.NewGlazedSchema()

    // 2. Create the debug section (adds --print-schema, etc.)
    cmdSection, _ := cli.NewCommandSettingsSection()

    // 3. Build the command description
    desc := cmds.NewCommandDescription(
        "foo",
        cmds.WithShort("Short help"),
        cmds.WithLong("Long help with examples..."),
        cmds.WithFlags(
            fields.New("my-flag", fields.TypeString, fields.WithDefault("x")),
        ),
        cmds.WithSectionsList(glazedSection, cmdSection),
    )

    return &FooCommand{CommandDescription: desc}, nil
}
```

### 7.4 Integration with Bubbletea

The trick: one command, two code paths:

```go
func (c *ListCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
    if useTUI {
        // Path A: launch Bubbletea (bypasses Glazed output)
        return runTUI()
    }

    // Path B: scan and emit rows through Glazed
    listeners, _ := listener.ScanListeners()
    for _, l := range listeners {
        gp.AddRow(ctx, listenerToRow(l))
    }
    return nil
}
```

The Bubbletea path uses `tea.NewProgram().Run()`, which takes over the terminal. The CLI path uses Glazed's `Processor.AddRow()`, which handles `--output json` etc.

---

## 8. Key Bindings Reference

| Key | Context | Action |
|-----|---------|--------|
| `↑` / `k` | Table | Move cursor up |
| `↓` / `j` | Table | Move cursor down |
| `Enter` | Table | Show process detail |
| `k` | Table | Kill selected process (opens confirm dialog) |
| `r` | Table | Refresh list |
| `/` | Table | Filter by process name |
| `s` | Table | Cycle sort column |
| `a` | Table | Toggle auto-refresh |
| `?` | Any | Show help overlay |
| `q` / `Ctrl+C` | Any | Quit |
| `Esc` | Dialog/Detail | Go back to table |
| `y` | Kill dialog | Confirm kill |
| `n` | Kill dialog | Cancel kill |

---

## 9. File-by-File Walkthrough

### `pkg/listener/types.go`
- **Purpose**: Define `ListenerInfo` struct with JSON and Glazed tags
- **Dependencies**: None (pure data)
- **Key symbols**: `ListenerInfo`

### `pkg/listener/scanner.go`
- **Purpose**: `ScanListeners() ([]ListenerInfo, error)` — the main entry point
- **Algorithm**: Iterate `net.Connections("inet")`, filter TCP LISTEN, gather process info
- **Key symbols**: `ScanListeners()`
- **Dependencies**: `gopsutil/net`, `gopsutil/process`

### `pkg/listener/process.go`
- **Purpose**: `gatherProcessInfo(proc, conn) (*ListenerInfo, error)` — enriches a socket with process metadata
- **Key symbols**: `gatherProcessInfo()`, `formatUptime()`, `formatBytes()`
- **Dependencies**: `gopsutil/process`

### `pkg/tui/model.go`
- **Purpose**: Define `Model` struct and `NewModel()` constructor
- **Key symbols**: `Model`, `NewModel()`, `Init()`
- **Dependencies**: `bubble-table/table`, `pkg/listener`

### `pkg/tui/update.go`
- **Purpose**: `Model.Update(msg) (tea.Model, tea.Cmd)` — all message handling
- **Key symbols**: `Update()`, `ListenersLoadedMsg`, `TickMsg`, `KillConfirmedMsg`
- **Dependencies**: `bubbletea`, `pkg/listener`

### `pkg/tui/view.go`
- **Purpose**: `Model.View() string` — renders the entire UI
- **Key symbols**: `View()`, `renderFooter()`, `renderKillDialog()`, `renderDetail()`
- **Dependencies**: `lipgloss`, `bubble-table`

### `pkg/tui/keymap.go`
- **Purpose**: Define all key bindings with help text
- **Key symbols**: `KeyMap`, `NewKeyMap()`, `ShortHelp()`, `FullHelp()`
- **Dependencies**: `bubbletea/key`

### `pkg/tui/styles.go`
- **Purpose**: Define all lipgloss styles
- **Key symbols**: `Styles`, `DefaultStyles()`
- **Dependencies**: `lipgloss`

### `pkg/tui/table.go`
- **Purpose**: `NewTable(width)` — create and configure the bubble-table
- **Key symbols**: `NewTable()`, `listenersToRows()`
- **Dependencies**: `bubble-table/table`, `pkg/listener`

### `pkg/tui/dialogs.go`
- **Purpose**: Kill confirmation dialog using huh forms
- **Key symbols**: `showKillForm()`, `KillConfirmedMsg`, `KillCancelledMsg`
- **Dependencies**: `huh`

### `cmd/listen-killer/main.go`
- **Purpose**: Root Cobra command, logging, help system, register subcommands
- **Key symbols**: `rootCmd`, `main()`
- **Dependencies**: `cobra`, `glazed/cli`, `glazed/logging`, `glazed/help`

### `cmd/listen-killer/cmds/tui/tui.go`
- **Purpose**: Glazed `ListCommand` — the bridge between CLI and TUI
- **Key symbols**: `ListCommand`, `NewListCommand()`, `RunIntoGlazeProcessor()`
- **Dependencies**: `glazed/cmds`, `bubbletea`, `pkg/tui`, `pkg/listener`

---

## 10. How to Extend

### Add a new column
1. Add field to `ListenerInfo` in `types.go`
2. Add a `table.NewColumn(...)` in `table.go`
3. Add the field to `listenersToRows()` in `table.go`
4. Add a style for it in `styles.go` (optional)

### Add a new key binding
1. Add to `KeyMap` struct in `keymap.go`
2. Add to `NewKeyMap()` with key + help text
3. Handle in `Update()` switch in `update.go`
4. Add to `ShortHelp()` or `FullHelp()` in `keymap.go`

### Add UDP listeners
1. In `scanner.go`, change filter from `Status == "LISTEN"` to also include `Type == SOCK_DGRAM`
2. Add a `Protocol` field to `ListenerInfo`
3. Add to table columns

### Add a new output format
- Glazed handles this automatically via `--output`. No code changes needed.
- If you need a custom format, implement a custom `middlewares.Processor` and pass it to `RunIntoGlazeProcessor`.

### Add configuration file
1. Add a `--config` flag in `NewListCommand()`
2. Use Glazed's config file loading (already available via `cli.CobraParserConfig`)
3. Define settings in YAML and read via `vals.DecodeSectionInto()`