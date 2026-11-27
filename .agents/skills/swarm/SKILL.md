---
name: swarm
description: Split a task into independent slices and combine verified results. Use for /skill:swarm or "swarm this".
disable-model-invocation: true
---

# Swarm

Use parallel work only when slices are independent and each can return verifiable evidence.

1. Define the done condition and result format. Partition the work so concurrent workers do not edit the same files or state.
2. Check whether the active Pi session exposes a subagent tool. If so, send standalone briefs with exact scope, verification steps, and separate writable outputs. Use only tools and model choices supported by that session.
3. If no subagent tool is available, do the slices serially. Do not spawn nested Pi processes to imitate agents.
4. Verify each result against the named evidence and reconcile overlaps. A missing result is a gap, not a pass.
5. Return one report with `PASS`, `ISSUES`, or `BLOCKED` per slice, evidence, and any gaps or dropouts.
