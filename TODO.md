# preflt — Roadmap

## v1.0 — Distribution

- [ ] Tag `v1.0.0` and push to trigger the release workflow
- [ ] Verify install script works end-to-end: `curl -sSL .../install.sh | sh`

## Near-term

**`preflt new` wizard**
Interactive YAML generator using Bubbletea form inputs. Outputs a valid `.yaml` to `~/.preflt/` or the current directory.

**Homebrew tap**
Once a tap repo exists, re-add the `brews:` block to `.goreleaser.yml` and add `HOMEBREW_TAP_GITHUB_TOKEN` to repo secrets.

## Backlog

**`--crew` mode** — two-terminal challenge/response. One terminal is "Pilot" (reads items), one is "Co-Pilot" (confirms). Both must confirm each item before advancing. Coordination via local socket or file.

**Registry** — `preflt install pre-deploy` pulls from a community checklist repo.

**Log sync** — push run history to S3 or a Git repo.

**Multi-user shared run state** — multiple people running the same checklist in sync.

**Web UI for history** — browse past runs in the browser.

**Checklist dependencies** — declare that checklist A must complete before B can start.

**Nested checklists (call-stack model)**
Current chaining is linear: A ends → B starts → B ends → done. A never resumes.

Nesting would work like a call stack: A pauses at item N → B runs to completion → A resumes at item N+1. Design notes:
- Runner needs a call stack (`[]runFrame`) instead of a flat `runChain`
- Each frame holds its own `*RunState`, `*Checklist`, and current item index
- A new item type (e.g. `type: checklist`) triggers a push; completion pops and resumes the parent
- Loop protection: a name can't appear twice in the stack
- Run logs: parent references child run IDs; whole stack shares a `chain_id`
