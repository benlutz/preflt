# preflt

Aviation-style checklist runner for the terminal and browser. Write YAML, run structured checklists, keep history.

> Checklists for things that matter — deploys, routines, incident response — not to-do lists.

---

## Install

```sh
go install github.com/benlutz/preflt/cmd/preflt@latest
```

Requires Go 1.21+. Make sure `$(go env GOPATH)/bin` is on your `$PATH` — then `preflt` is available immediately. Pre-built binaries (no Go required) coming in v1.0.

---

## Quick start

Create a checklist at `~/.preflt/deploy.yaml`:

```yaml
name: deploy
description: Pre-deployment checklist

phases:
  - name: CODE
    items:
      - id: tests
        label: All tests passing?
        type: do

      - id: version
        label: What version are we deploying?
        type: check

  - name: DEPLOY
    items:
      - id: env-vars
        label: Environment variables set in production?
        type: do

      - id: rollback
        label: Rollback plan documented?
        note: Know the exact steps to revert if something goes wrong
        type: do

      - id: smoke-url
        label: Which URL will you smoke-test after deploy?
        type: check
```

Run it:

```
preflt run deploy
```

The TUI steps through each item. Press `enter` to confirm, `n` to mark N/A, `q` to save progress and quit. Kill and restart — your place is saved.

Run it in the browser instead:

```
preflt web deploy
```

---

## How it works

Checklists live in `~/.preflt/` as YAML files. Each run writes a timestamped log to `~/.preflt/runs/`. Progress is saved after every item so you can always resume.

```
preflt list                    # show all checklists and their last run
preflt run <name>              # run in the terminal
preflt web <name>              # run in the browser (default: localhost:8080)
preflt history <name>          # show past runs
preflt schedule <name> --frequency daily   # register a recurring schedule
preflt                         # startup screen — shows what's due today
```

---

## YAML reference

```yaml
name: my-checklist
description: Optional description shown in list and web UI
type: normal              # normal (default) | emergency (no N/A, no skips)

# Items can be at the top level or grouped into phases.
phases:
  - name: PHASE NAME
    items:
      - id: unique-id           # auto-assigned if omitted
        label: What to do       # shown in TUI and web UI
        response: Confirmation  # shown when item is done
        note: Context hint      # shown while the item is current
        type: do                # do (default) | check (prompts for a text value)
        na_allowed: true        # show [n] N/A option

        # Fire shell commands or webhooks when this item is confirmed.
        on_complete:
          - shell: echo "done"
          - webhook: https://your-endpoint.com/hook

        # Make this a yes/no decision point.
        condition:
          if_yes:
            trigger_checklist: next-checklist
          if_no:
            skip: true          # mark N/A and continue

# Fire after the whole checklist completes.
on_complete:
  - shell: echo "all done"

# Start another checklist automatically after this one finishes.
trigger_checklist: post-deploy

# Register a recurring schedule (or use `preflt schedule` instead).
schedule:
  frequency: daily              # daily | weekly | monthly
  on: monday                    # weekday for weekly (comma-separated for multiple)
  period: morning               # display hint: morning | afternoon | evening
  cooldown: 12h                 # minimum gap between runs (e.g. 7d, 12h)
```

---

## Features

**Two interfaces, same state** — the terminal TUI and browser UI write to the same run state. Start on one, switch to the other.

**Phases** — group items under named headings. Phases show as section dividers in both interfaces.

**Resume** — progress is saved after every confirmed item. Kill the process, restart, pick up exactly where you left off.

**Conditions** — items can branch on yes/no. Trigger a different checklist, skip ahead, or abort and hand off immediately.

**Chaining** — a checklist can trigger the next one automatically on completion. Chains share a `chain_id` in the logs. Loop protection prevents cycles.

**Automations** — run shell commands or POST to a webhook when an item is confirmed or the checklist completes. Errors are logged but never block completion.

**Scheduling** — register daily, weekly, or monthly schedules with `preflt schedule`. The startup screen (`preflt` with no args) shows what's due and greets you by name.

**History** — every completed run is logged with timestamps, duration, who ran it, and per-item values. `preflt history <name>` shows the last 10 runs.

**Emergency mode** — `type: emergency` disables N/A and skipping. Every item must be explicitly confirmed.

---

## Examples

The [`examples/`](examples/) directory includes:

- `pre-deploy.yaml` — phased deploy checklist with automations and a version capture step
- `morning-routine.yaml` — daily morning routine with scheduling and cooldown
- `chain-deploy.yaml` / `chain-hotfix.yaml` / `chain-rollback.yaml` / `chain-verify.yaml` — a four-checklist chain that branches on a go/no-go condition

---

## License

MIT — see [LICENSE](LICENSE).
