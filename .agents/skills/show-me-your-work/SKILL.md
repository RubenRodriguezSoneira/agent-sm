---
name: show-me-your-work
description: Keep a reviewable decision trail for long-running or unattended work. Use for /skill:show-me-your-work, multi-phase runs, or work reviewed after someone steps away.
disable-model-invocation: true
---

# Show me your work

Keep one append-only TSV log. Copy the header from `references/decision-log-template.tsv`; store it as `decisions.tsv` in the work directory or `.audit/<task-slug>.tsv` when several efforts run at once. Leave it untracked unless a reviewer needs the trail.

Columns are `ts`, `phase`, `decision`, `why`, `evidence`, and `result`. Keep cells on one line and evidence as a pointer. Use `scripts/log.sh` or append a validated row. Log decisions and checkpoints, not every action.

At the end, verify each row against the visible conversation and its evidence. In interactive Pi, `/session` identifies the current session file. Session storage defaults to `~/.pi/agent/sessions/` but may be configured differently. Inspect only the active project/session; if its transcript is not accessible, state that the transcript audit could not be completed rather than searching other workspaces.

Use an independent reviewer only when the active Pi session exposes one. Otherwise self-review the log and do not claim cross-model review. Report the trail location, the verified outcome, and any evidence or review gaps.
