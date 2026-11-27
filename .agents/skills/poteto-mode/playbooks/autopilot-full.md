### Autopilot-full

**Own the task's exit condition. Do not assume Pi can run background agents, schedules, or PR operations.**

1. Define a checkable completion predicate before work starts.
2. Use only Pi tools and integrations visible in the active session. Delegate independent slices only when an actual subagent tool is available; otherwise work serially.
3. Make the smallest evidence-backed change and verify each meaningful unit against the real artifact. Keep a decision trail for long or multi-phase work with **show-me-your-work**.
4. Do not merge, deploy, delete user data, or send external messages unless the user explicitly requested that action. A green check is not authorization.
5. If the session has no authorized scheduler, remain interactive and do not promise future monitoring. Stop when the predicate is met or report a concrete blocker; do not relax the predicate to claim success.

**Reply:** the exit condition, work performed, verification evidence, and any remaining gap.
