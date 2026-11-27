### Opening a PR

Invoked at the end of every other playbook.

Work from the current task's intended branch and keep unrelated work out of it. Rebase into small, landable commits only when the user requested or the branch workflow requires it. Run the **unslop** skill over the diff before commit.

Title: `type(scope): subject`, naming a real symbol when one carries the change. Body: a one-minute briefing under 40 lines with these `##` headings in order, dropping empty ones: `## Why`, `## What changed`, `## Scope`, `## Tradeoffs` (only rejected alternatives a reviewer would otherwise ask about), `## Blast Radius`, `## Verification`.

Stack follow-ups by base branch. Open PRs ready, through the run's PR tool with `draft: false`. Without an app-native PR tool, use `gh pr create` only when GitHub CLI is installed and authenticated. If the repository uses another forge, use its configured CLI only when available. Opening a PR does not start a babysit.
