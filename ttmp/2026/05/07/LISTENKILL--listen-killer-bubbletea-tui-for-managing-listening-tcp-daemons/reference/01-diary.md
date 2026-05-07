---
title: "Diary"
status: active
intent: long-term
topics: [go, bubbletea, glazed, tui, networking]
created: 2026-05-07
---

# Diary

## Goal

Build a Bubbletea-based Go TUI app ("listen-killer") that displays all listening TCP daemons with process details and allows killing them. Uses the Glazed command framework for CLI integration.

## Step 1: Project Scaffold and Dependency Resolution

Scaffolding the Go module, directory structure, and pulling in all dependencies. We need bubbletea, bubble-table (from go-go-golems), gopsutil, glazed, cobra, lipgloss, and huh.

### Prompt Context

**User prompt (verbatim):** "Create a new bubbletea based golang TUI to show me all the running listening HTTP/TCP daemons, including their path, port, etc... (see lsof-who -p XXX in a tmux) and allow me to kill them, see how long they're running, etc... Use the galzed command framework to build it. First, create a new docmgr ticket and then Create a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable. Keep a diary as you work."

**Assistant interpretation:** Build a full TUI dashboard for TCP listeners, modeled after the existing lsof-who tool but with a dashboard view instead of single-port lookup. Use Glazed for CLI wrapping, bubble-table for the table widget, gopsutil for OS data. Create comprehensive intern documentation and track everything in docmgr.

**Inferred user intent:** The user wants a practical utility (manage stale listening daemons on their dev machine) that also serves as an educational artifact — hence the intern guide. They also want everything tracked and deliverable via docmgr and reMarkable.

**Commit (code):** (none yet — scaffolding in progress)

### What I did
- Created docmgr ticket LISTENKILL with topics: go, bubbletea, glazed, tui, networking
- Created 3 docs: analysis, intern guide (design), diary (reference)
- Created 12 tasks in the ticket covering all implementation steps
- Wrote comprehensive analysis document covering problem statement, existing tools comparison, requirements, architecture, data flow, tech stack, design decisions, API contract
- Wrote comprehensive intern guide covering: what we're building, the three-layer architecture, data gathering layer (pkg/listener/), TUI layer (pkg/tui/), CLI layer (cmd/), Bubbletea Elm Architecture deep-dive, Glazed framework deep-dive, key bindings reference, file-by-file walkthrough, extension guide
- Read lsof-who source code at `/home/manuel/code/wesen/corporate-headquarters/go-go-labs/cmd/apps/lsof-who/main.go` for reference patterns
- Read bobatea keymap.go, styles.go, listbox example for Bubbletea patterns
- Read bubble-table doc.go, model.go for table widget API
- Examined `ss -tlnp` and `lsof -iTCP -sTCP:LISTEN` output for data shape
- Examined `ps -eo pid,etime,comm` for process uptime format
- Initialized Go module: `go mod init github.com/wesen/listen-killer`
- Created directory structure: `cmd/listen-killer/cmds/tui/`, `pkg/listener/`, `pkg/tui/`

### Why
- lsof-who is the closest ancestor — reuses its gopsutil patterns
- bubble-table is in-house and well-tested — avoids writing a table from scratch
- The three-layer architecture keeps scanner, TUI, and CLI separable for testing and reuse

### What worked
- lsof-who source provided exact gopsutil API calls to replicate
- bobatea keymap/styles patterns are clean and directly applicable
- bubble-table API is well-documented in doc.go

### What didn't work
- Could not find lsof-who source at first; it's in go-go-labs, not a standalone project
- bobatea corporate-headquarters and go-go-golems have different content; go-go-golems has the actual library code

### What I learned
- `gopsutil/net.Connections("inet")` returns all sockets; filter by `Type == SOCK_STREAM && Status == "LISTEN"` for TCP listeners
- `gopsutil/process.CPUPercent()` blocks ~1s on first call; use `Percent(0)` for cached value
- `bubble-table` uses `table.New(columns).WithRows(rows).WithTargetWidth(w)` API

### What was tricky to build
- Navigating the two bobatea repos to find the right reference code (corporate-headquarters has ttmp docs, go-go-golems has the library code)

### What warrants a second pair of eyes
- The decision to use bubble-table vs bubbles/table — bubble-table is in-house but less known

### What should be done in the future
- N/A (scaffolding step)

### Code review instructions
- Review the analysis and intern guide documents for completeness and accuracy
- Verify the technology choices

### Technical details
- lsof-who reference: `/home/manuel/code/wesen/corporate-headquarters/go-go-labs/cmd/apps/lsof-who/main.go`
- bobatea reference: `/home/manuel/code/wesen/go-go-golems/bobatea/`
- bubble-table reference: `/home/manuel/code/wesen/go-go-golems/bubble-table/`
- Glazed tutorial: `/home/manuel/code/wesen/corporate-headquarters/glazed/pkg/doc/tutorials/05-build-first-command.md`