# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`preflt` is a pilot-style CLI checklist runner — a single Go binary for running structured YAML checklists with state persistence, TUI and web interfaces, automations, and checklist chaining. Think aviation checklists, not to-do apps.

Module: `github.com/benlutz/preflt`

## Commands

```bash
go build ./...          # build
go run ./cmd/preflt     # run without building
go test ./...           # all tests
go test ./internal/store/... -run TestName  # single test
PREFLT_HOME=/tmp/test-preflt go run ./cmd/preflt run morgenroutine  # isolate state
```

The compiled binary is `./preflt`. Checklists are loaded from `~/.preflt/*.yaml` (global) or `./*.yaml` (cwd).

## Architecture

Data flows: YAML file → `checklist.Load()` → `Checklist` struct → `runner.Run()` or `web.Serve()` → `tui.Model` (Bubbletea) or HTTP handlers → `store` writes to `~/.preflt/runs/<uuid>/`.

**`internal/checklist/`** — YAML schema types and loader. `Checklist.Flatten()` normalises `phases[].items` and top-level `items[]` into a flat `[]FlatItem` that all other code uses. Always work with `FlatItem` slices, never the raw struct fields.

**`internal/store/`** — all disk I/O. Two JSON shapes: `RunState` (in-progress, lives at `state.json`, deleted on completion) and `RunLog` (completed/aborted record at `run.json`). Base dir is `~/.preflt/`; override with `PREFLT_HOME` env var.

**`internal/runner/`** — CLI run loop. `runChain()` is the recursive engine: it handles resume prompts, drives the Bubbletea TUI, inspects the finished `tui.Model`, and recurses for chained checklists. Loop protection via a `visited []string` list.

**`internal/tui/`** — Bubbletea model. Five screens: `screenList` (main view), `screenInput` (text entry for `type:check` items), `screenCondition` (yes/no branch), `screenQuit` (save prompt), `screenDone`. The runner reads `Model.Done`, `Model.Aborted`, `Model.TriggerNext`, and `Model.Saved` after `p.Run()` returns.

**`internal/web/`** — HTTP server for `preflt web`. Embeds `static/index.html` via `embed.FS`. The `server` struct mirrors the runner's chain logic: `loadNext()` swaps the active checklist in place, keeping the server alive across a chain. API: `GET /api/state`, `POST /api/item/confirm`, `POST /api/item/na`.

**`internal/automation/`** — fires `shell` and `webhook` steps from `on_complete` blocks. Item-level automations are async (goroutine); list-level are synchronous post-run. Errors are logged but never block completion.

**`internal/schedule/`** — reads `schedule` blocks from YAML and decides what's due. This is v0.2 behaviour: the YAML block drives scheduling. v0.4 will replace this with `~/.preflt/schedules.json` as the source of truth (see TODO.md).

**`internal/cmd/root.go`** — Cobra command wiring. `preflt` with no args calls `startupScreen()` which shows due checklists.

## YAML schema key points

```yaml
name: vor-dem-deploy
type: normal          # normal | emergency (emergency = no skips, no N/A)
phases:               # optional; if absent, use top-level items[]
  - name: CODE
    items:
      - id: tests
        label: All tests green
        type: do          # do | check (check prompts for a text value)
        na_allowed: true
        on_complete:
          - webhook: https://...
        condition:
          if_yes:
            trigger_checklist: rollback
            abort: true   # abandon current list immediately
          if_no:
            skip: true
on_complete:
  - shell: "echo done"
trigger_checklist: post-deploy  # starts after this list completes
schedule:             # hint only in v0.2, will be ignored in v0.4
  frequency: daily
  cooldown: 7d
```

## Current version and roadmap

v0.3.0 is complete (CLI + Web + automations + chaining). v0.4 (scheduling rework with `schedules.json`) and v1.0 (goreleaser / distribution) are next. See TODO.md for task-level detail.
