---
name: recall
description: Rebuild recent working context for a project or topic from Pi sessions, repository history, and available issue records. Use for "catch me up", "where did I leave off", or "recall my work on X".
disable-model-invocation: true
---

# Recall

Give a concise current-state brief. Read only the sessions and records needed for the named topic.

1. If the user already supplied a complete state capsule, use it and skip history mining.
2. Define the time window, topic, and workspace before searching. Default to the active project and a bounded recent period; never turn "all" into an arbitrary subset.
3. Use the Pi session picker or current session when available. Session files default to `~/.pi/agent/sessions/` but may be elsewhere; use the configured location or an exact path the user provides. Do not scan sessions from other workspaces. If history is unavailable, state that rather than guessing.
4. For a named feature or bug, check current source control and relevant issue/PR records when the corresponding tools are available. Do not assume MCP access or GitHub authentication.
5. Verify surfaced branches, PRs, and issues against live state with available tools. If what an earlier agent did matters, inspect the original record when accessible; summaries alone are not proof.
6. Use a subagent only if the active Pi session exposes one and the history is large enough to justify delegation. Otherwise search directly.

## Reply

Lead with the capsule, then one status line per thread, recurring problems, and the single next move. Keep the capsule to five bullets, cite session IDs or source records when available, and separate facts from inference.
