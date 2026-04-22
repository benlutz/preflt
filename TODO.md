# preflt — Build Plan

> New to Go? Every section starts with the commands and concepts you need.
> Work top-to-bottom. Check off items as you go.

---

## v0.0 — Go Setup & Project Scaffold

*Before writing a single line of preflt, get your Go environment ready.*

### Install Go
- [ ] Download and install Go from https://go.dev/dl/ (pick the latest stable, Linux/Mac/Windows)
- [ ] Verify: `go version` should print something like `go version go1.22.x`
- [ ] Understand: Go compiles to a single binary — no runtime needed on the target machine

### Install tools
- [ ] Install `goreleaser` (cross-platform release builder): `go install github.com/goreleaser/goreleaser/v2@latest`
- [ ] Install the Bubbletea example to verify TUI works: `go run github.com/charmbracelet/bubbletea/examples/list-simple@latest`
- [ ] (Optional) Install `gopls` for IDE autocomplete: `go install golang.org/x/tools/gopls@latest`

### Initialize the project
- [x] Run `go mod init github.com/benlutz/preflt`
  - This creates `go.mod` — Go's equivalent of `package.json`
- [x] Create the folder structure:
  ```
  preflt/
  ├── cmd/
  │   └── preflt/
  │       └── main.go        ← entry point
  ├── internal/
  │   ├── checklist/         ← YAML parsing + domain types
  │   ├── runner/            ← run logic, state, resume
  │   ├── store/             ← read/write ~/.preflt/
  │   └── tui/               ← Bubbletea UI
  ├── go.mod
  └── go.sum
  ```
- [x] Write a minimal `main.go` that prints "preflt v0.0" and compiles: `go build ./...`
- [x] Run it: `go run ./cmd/preflt`

### Add CLI framework
- [x] Add `cobra` (the standard Go CLI library, used by kubectl, gh, etc.):
  `go get github.com/spf13/cobra`
- [x] Wire up a root command with a `--version` flag
- [x] Add stub subcommands: `run`, `list`, `history` (just print "not yet implemented")
- [x] Verify: `go build ./...` and `./preflt --help` shows the subcommands

---

## v0.1 — MVP: Run a Checklist End-to-End

*Goal: be able to run your first real checklist tomorrow.*

### YAML Schema + Parser
- [x] Add `gopkg.in/yaml.v3`: `go get gopkg.in/yaml.v3`
- [x] Define Go structs for the checklist schema:
  - `Checklist` (name, description, version, type, items, phases)
  - `Item` (id, label, response, note, type, na_allowed)
  - `Phase` (name, items)
- [x] Write a `Load(path string) (*Checklist, error)` function
- [x] Handle the two load modes: by name (`~/.preflt/<name>.yaml`) and by direct path

### State & Persistence
- [x] Define a `RunState` struct:
  - run id (uuid), checklist name, started_at, current_item_index
  - per-item: status (`pending | checked | na | skipped`), checked_at, note
- [x] Add `github.com/google/uuid`: `go get github.com/google/uuid`
- [x] Write `SaveState(state *RunState)` → `~/.preflt/runs/<id>/state.json`
- [x] Write `LoadState(checklistName string) (*RunState, error)` — finds latest in-progress run
- [x] Ctrl+C safety: state is saved in TUI before `tea.Quit`

### Bubbletea TUI
- [x] Add Bubbletea + Lipgloss: `bubbletea`, `lipgloss`, `bubbles/textinput`
- [x] Build the item view:
  - Show current phase name (if phases defined)
  - Show item label + response text
  - Show progress: `[3/12]`
  - Show keybindings: `[enter] confirm  [n] N/A  [q] quit`
- [x] Handle `type: check` — text input screen after pressing enter
- [x] Handle `na_allowed: true` — show `[n]` keybinding, log as `na` not `skipped`
- [x] Show checklist complete screen with duration and stats

### Resume Flow
- [x] On `preflt run <name>`: check for existing in-progress state
- [x] If found: show "Session found (23 min ago). Resume? [y/n]"
- [x] `y` → restore item index and continue
- [x] `n` → start fresh (new run state)

### Run Log (JSON)
- [x] On completion: write final log to `~/.preflt/runs/<id>/run.json`
- [x] Log contains: checklist name, run_id, started_at, completed_at, completed_by (git user or hostname), items with status + timestamps
- [x] State file removed on completion

### `preflt list`
- [x] Scan `~/.preflt/*.yaml` for global checklists
- [x] Scan `./` (cwd) for local `.yaml` files that match the preflt schema
- [x] Print a formatted table: name, description, last run date

### `preflt history <name>`
- [x] Scan `~/.preflt/runs/` for completed runs matching the checklist name
- [x] Print last N runs: date, duration, status (completed / aborted), completed_by
- [x] Default to last 10 runs

### Write example checklists
- [x] Write `vor-dem-deploy.yaml` — 10 items, 2 phases (CODE, DEPLOY), mix of do/check
- [x] Write `morgenroutine.yaml` — 8-item flat daily routine
- [x] Copied to `~/.preflt/` — discoverable by `preflt list`

### v0.1 Done Criteria
- [x] `go build ./...` produces a clean binary
- [x] Run `vor-dem-deploy.yaml` end-to-end without errors
- [x] Kill mid-run with Ctrl+C, restart, resume works
- [x] `preflt list` shows both global and local checklists
- [x] `preflt history vor-dem-deploy` shows past runs after a completed run

---

## v0.2 — Automations, Chaining & Scheduling

### Automations (on_complete)
- [x] Parse `on_complete` block in YAML (list-level and item-level)
- [x] Implement `shell` automation: run arbitrary shell command (30s timeout)
- [x] Implement `webhook` automation: POST JSON to any URL (10s timeout)
  - Item-level body: event, run_id, checklist_name, completed_by, timestamp, item (id, label, status, value)
  - List-level body: event, run_id, checklist_name, started_at, completed_at, duration_seconds, completed_by, items[]
- [x] Run automations sequentially — errors are logged, never block completion
- [x] Item-level automations fire immediately when item is confirmed (async, non-blocking)
- [x] List-level automations fire after run is logged as complete

### Checklist Chaining
- [x] Parse `condition` block per item: `if_yes`, `if_no` with `skip`, `trigger_checklist`, `abort`
- [x] In TUI: condition items show `[y] yes  [n] no` instead of normal confirm
- [x] Implement `trigger_checklist`: load and start another checklist after current
- [x] Implement `abort: true`: mark current run as aborted, start next immediately
- [x] Chain metadata (chain_id, triggered_by) logged in run.json
- [x] Loop protection: refuse to trigger a checklist already in the current chain
- [x] List-level `trigger_checklist` field on the checklist itself

### Scheduling + Startup Screen (v0.2 provisional — superseded by v0.4)
- [x] Parse `schedule` block in YAML: `frequency`, `on`, `period`, `cooldown`
- [x] Implement `preflt` (no args): scan all known checklists for due schedules
- [x] Check run history to determine "already done today"
- [x] Implement `cooldown`: compare last completed run date to now
- [x] Show the startup screen with due checklists and `[1-n/s]` prompt
- [x] Greet by git user name with time-of-day salutation
- NOTE: v0.2 scheduling reads directly from YAML `schedule` blocks. Requirements
  updated: scheduling should be user-intent stored in `~/.preflt/schedules.json`,
  YAML block becomes a hint only. This is redesigned in v0.4.

### v0.2 Done Criteria
- [x] `vor-dem-deploy.yaml` fires a shell command on list completion
- [x] A specific item fires an automation when confirmed
- [x] Condition item branches to a different checklist on yes/no
- [x] Running `preflt` in the morning shows due checklists
- [x] `morgenroutine.yaml` is not re-suggested after completion (cooldown + daily check)

---

## v0.3 — Web UI

### Embed + Server
- [x] Create `internal/web/` with `embed.FS` setup
- [x] Write `index.html` + vanilla JS (no framework — keep the binary small)
- [x] Implement HTTP server in Go: `net/http`, serve embedded assets
- [x] `preflt web <name>` starts server + opens browser automatically (`os/exec` with `open`/`xdg-open`)
- [x] Add `--port` flag (default 8080) and `--host` flag (default localhost)
- [x] Document: with `--host 0.0.0.0` the checklist is reachable in the local network

### Web UI Features
- [x] Render checklist items with the same logic as TUI (phases, N/A, check type)
- [x] Submit item completions via `POST /api/item/confirm` and `POST /api/item/na`
- [x] Show live progress bar
- [x] Run automations on completion (same code path as CLI)
- [x] Show completion screen with duration

### Shared State
- [x] Both CLI and Web write to the same `RunState` — same store layer, no duplication
- [x] (Real-time sync between CLI and Web is deferred — noted as TBD in requirements)

### v0.3 Done Criteria
- [x] `preflt web vor-dem-deploy` opens browser, checklist is completable
- [x] `--host 0.0.0.0` makes it accessible from another device on the same WiFi
- [x] Completion triggers the same automations as CLI

---

## v0.4 — Scheduling Rework ✓

*`schedules.json` is the primary source for user-managed schedules. YAML `schedule`
blocks remain functional as a self-contained fallback — checklists without a
`schedules.json` entry still appear on startup if their YAML block says they're due.
`schedules.json` takes precedence when both exist.*

### Store layer — `schedules.json`
- [x] Define `ScheduleEntry` struct: name, path, mode (`pending|date|recurring`), from, frequency, on (weekday), period, cooldown, created_at
- [x] Write `SaveSchedule(entry)`, `LoadSchedules() []ScheduleEntry`, `RemoveSchedule(name)` in store package
- [x] Store at `~/.preflt/schedules.json`
- [x] Unit tests for schedule store CRUD

### `preflt schedule` command
- [x] `preflt schedule <name> --pending` — add open/pending entry (show until completed once)
- [x] `preflt schedule <name> --from <date>` — suggest starting from a specific date (YYYY-MM-DD)
- [x] `preflt schedule <name> --frequency daily|weekly|monthly --on <weekday>` — recurring schedule
- [x] `--on` accepts comma-separated weekdays: `--on monday,wednesday`
- [x] `preflt schedule <name> --period morning|afternoon|evening` — time-of-day hint (display only)
- [x] `preflt schedule <name> --cooldown <duration>` — min gap between runs (e.g. `7d`, `12h`)
- [x] `preflt schedule` (no args) — list all active schedules
- [x] `preflt schedule <name> --remove` — delete a schedule entry
- [x] When `preflt schedule <name>` is called and the checklist has a YAML `schedule` hint, fill unset fields from it (frequency only when no mode flag is set; on/period/cooldown for recurring mode)

### Startup screen — reads from `schedules.json`
- [x] `schedules.json` entries checked first; YAML blocks are fallback for uncovered checklists
- [x] `isDue()` handles pending, date, and recurring modes against run history
- [x] Pending entries appear until completed once (since schedule was created)
- [x] Date-based entries appear on/after `from` date until completed once
- [x] Recurring entries use frequency + cooldown as before
- [x] Greeting + numbered prompt UX unchanged

---

## v1.0 — Polish & Distribution

### Goreleaser + Distribution
- [ ] Write `.goreleaser.yml` config
- [ ] Set up GitHub Actions workflow: build + release on git tag
- [ ] Test cross-compilation: `GOOS=darwin GOARCH=arm64 go build ./...`
- [ ] Write Homebrew tap formula (or use goreleaser's built-in tap support)
- [ ] Verify `brew install` works end-to-end

### `preflt new` Wizard (optional)
- [ ] Interactive YAML generator using Bubbletea form inputs
- [ ] Outputs a valid `.yaml` to `~/.preflt/` or current directory

### `--crew` Mode (two-terminal challenge/response)
- [ ] One terminal is "Pilot" (reads items), one is "Co-Pilot" (confirms)
- [ ] Use a local socket or file-based coordination
- [ ] Both must confirm each item before advancing

### v1.0 Done Criteria
- [ ] `goreleaser release` produces Mac/Linux/Windows binaries
- [ ] Homebrew install works: `brew install preflt`
- [ ] README covers install + quickstart

---

## Post-v1 (Backlog, Not Yet Planned)

- [ ] Registry: `preflt install vor-dem-deploy` from community repo
- [ ] Log sync: S3 / Git repo
- [ ] Multi-user shared run state
- [ ] Web UI for history
- [ ] Checklist dependencies (A must run before B)

### Nested Checklists (call-stack model)
Current chaining is linear: A ends → B starts → B ends → done. A never resumes.

Nesting would work like a call stack: A pauses at item N → B runs to completion →
A resumes at item N+1. This gives much more flexibility — a sub-checklist becomes
reusable middleware that any parent can embed at any point.

Design notes for when this is tackled:
- Runner needs a call stack (`[]runFrame`) instead of a flat `runChain`
- Each frame holds its own `*RunState`, `*Checklist`, and current item index
- A new item type (e.g. `type: checklist`) or YAML field (e.g. `sub_checklist: <name>`) triggers a push
- On sub-checklist completion, pop the frame and resume the parent
- Loop protection still needed: a name can't appear twice in the stack
- Run logs: parent run references child run IDs; child logs reference parent run ID
- The whole stack shares a `chain_id` as today, but nesting depth is also recorded
