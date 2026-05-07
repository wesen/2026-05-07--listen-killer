# Tasks

## TODO

- [x] Add tasks here

- [x] Scaffold Go module, project structure, go.mod with all dependencies
- [x] Implement pkg/tui/keymap.go and pkg/tui/styles.go
- [x] Implement pkg/listener/types.go - ListenerInfo struct with glazed+json tags
- [x] Implement pkg/tui/dialogs.go - kill confirmation dialog
- [x] Implement cmd/listen-killer/cmds/tui/tui.go - Glazed command with dual TUI/CLI mode
- [x] Implement pkg/tui/model.go, update.go, view.go - main Bubbletea model
- [x] Implement cmd/listen-killer/main.go - root Cobra command with Glazed wiring
- [x] Implement pkg/tui/table.go - bubble-table setup with columns and row conversion
- [x] Implement pkg/listener/process.go - gatherProcessInfo, uptime, memory formatting
- [x] Implement pkg/listener/scanner.go - ScanListeners() using gopsutil
- [x] Upload intern guide and analysis to reMarkable
- [ ] Test in tmux: verify scan, table, kill flow, CLI JSON output
