---
title: "System Analysis and Requirements"
status: active
intent: long-term
topics: [go, bubbletea, glazed, tui, networking]
created: 2026-05-07
---

# System Analysis and Requirements: Listen Killer

## Problem Statement

When developing on a Linux workstation, many daemons and servers accumulate listening on TCP ports — development servers, database proxies, API mock servers, Obsidian local servers, Discord RPC, etc. Over time, stale processes consume ports and resources. There is no simple TUI tool that:

1. Shows **all** listening TCP daemons in a single dashboard
2. Displays relevant metadata: PID, process name, user, port, bound address, uptime, CPU, memory
3. Allows **killing** processes with selectable signals (TERM, KILL, INT)
4. Provides sorting and filtering

## Existing Tools

| Tool | Pros | Cons |
|------|------|------|
| `ss -tlnp` | Fast, built-in | No process details beyond PID/name; no interactive kill |
| `lsof -iTCP -sTCP:LISTEN` | Detailed, shows process path | Slow, noisy output, no interactivity |
| `lsof-who` (go-go-labs) | Single-port lookup, process tree, huh-based kill prompt | Only one port at a time; not a dashboard |
| `htop` | Process dashboard | Not network-centric; no port listing |

**`lsof-who`** (at `/home/manuel/code/wesen/corporate-headquarters/go-go-labs/cmd/apps/lsof-who/main.go`) is the closest ancestor. It uses `gopsutil` to find a process by port, gathers rich info (PID, name, cmdline, exe, username, parent tree, CPU%, RSS, open files, start time), and offers a `huh` form to kill. But it only handles **one port at a time** and has no dashboard view.

## Requirements

### Functional

- **F1**: Scan all TCP listening sockets on the system
- **F2**: Display in a scrollable, sortable table with columns: PID, Process Name, User, Port, Address, Uptime, CPU%, RSS
- **F3**: Refresh the list manually (r) or on a configurable interval (auto-refresh)
- **F4**: Kill a selected process with configurable signal (k → select TERM/KILL/INT)
- **F5**: Show detailed info for selected process (Enter)
- **F6**: Filter by process name or port (/) 
- **F7**: Sort by any column by clicking header or key binding
- **F8**: CLI mode: `listen-killer list --output json` for scripting (Glazed output)
- **F9**: TUI mode: `listen-killer` launches the interactive dashboard

### Non-Functional

- **NF1**: Startup under 500ms (lsof-who takes ~200ms for one port)
- **NF2**: Refresh under 100ms
- **NF3**: Works on Linux (uses `/proc` via gopsutil)
- **NF4**: Memory under 50MB

## Architecture Overview

```
┌────────────────────────────────────────────────────┐
│                    main.go                          │
│  Cobra root + Glazed help/logging wiring            │
├────────────────────────────────────────────────────┤
│  cmd/listen-killer/cmds/tui/tui.go                  │
│  GlazeCommand: RunIntoGlazeProcessor                │
│  → launches Bubbletea Program or emits rows          │
├────────────────────┬───────────────────────────────┤
│  pkg/listener/     │  pkg/tui/                      │
│  scanner.go        │  model.go     (main model)     │
│  process.go        │  keymap.go    (key bindings)   │
│  types.go          │  styles.go    (lipgloss)        │
│                    │  update.go    (msg handling)    │
│                    │  view.go      (render)          │
│                    │  table.go     (table widget)    │
│                    │  dialogs.go   (kill confirm)    │
└────────────────────┴───────────────────────────────┘
```

### Data Flow

```
1. scanner.ScanListeners()
   → gopsutil/net.Connections("inet")
   → filter for TCP LISTEN (SOCK_STREAM + status "LISTEN")
   → for each: gopsutil/process.NewProcess(pid)
   → gather: Name, Cmdline, Username, CreateTime, CPUPercent, MemoryInfo

2. []ListenerInfo → table.Row[] → bubble-table Model

3. User interactions → tea.Msg → Update() → bubble-table actions

4. Kill action → syscall.Kill(pid, signal)
```

### Technology Stack

| Layer | Library | Why |
|-------|---------|-----|
| TUI framework | `github.com/charmbracelet/bubbletea` v1.3+ | Elm Architecture, mature ecosystem |
| Table widget | `github.com/go-go-golems/bubble-table` | Custom in-house table with sort/filter/select |
| Styling | `github.com/charmbracelet/lipgloss` v1.1+ | Declarative terminal styling |
| CLI framework | `github.com/go-go-golems/glazed` | Structured output, Cobra integration |
| CLI parsing | `github.com/spf13/cobra` | Industry standard |
| System info | `github.com/shirou/gopsutil/v3` | Cross-platform /proc access |
| Confirm dialog | `github.com/charmbracelet/huh` | Form-based prompts |

## Key Design Decisions

### 1. Scanner: gopsutil vs shelling out to `ss`
- **Decision**: Use `gopsutil` (same as lsof-who)
- **Rationale**: Structured data, no parsing, cross-platform potential, proven in lsof-who

### 2. Table: bubble-table vs bubbles/table
- **Decision**: Use `bubble-table` from go-go-golems
- **Rationale**: In-house, supports sorting, filtering, selection, pagination

### 3. Glazed command: TUI mode vs CLI mode
- **Decision**: Single `GlazeCommand` with a `--tui` flag (default true when TTY)
- **Rationale**: Same command works both ways; Glazed handles `--output json` automatically in CLI mode

### 4. Refresh: polling vs inotify on /proc
- **Decision**: Configurable polling interval (default 2s)
- **Rationale**: Simple, reliable; /proc/net/tcp doesn't support inotify

## API Contract: ListenerInfo

```go
type ListenerInfo struct {
    PID        int32   `json:"pid"`
    Name       string  `json:"name"`
    Cmdline    string  `json:"cmdline,omitempty"`
    Exe        string  `json:"exe,omitempty"`
    Username   string  `json:"username,omitempty"`
    Port       uint32  `json:"port"`
    Address    string  `json:"address"`
    Uptime     string  `json:"uptime"`       // human-readable, e.g. "3h 15m"
    UptimeSeconds int64 `json:"uptime_seconds"`
    CPUPercent float64 `json:"cpu_percent,omitempty"`
    RSSBytes   uint64  `json:"rss_bytes,omitempty"`
    RSSHuman   string  `json:"rss_human,omitempty"` // e.g. "45.2 MB"
}
```