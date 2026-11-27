---
name: arena
description: Compare independent candidate solutions against a concrete rubric, then synthesize and verify the strongest result. Use for /skill:arena or "try this in an arena".
disable-model-invocation: true
---

# Arena

Use an arena only when competing approaches can produce meaningfully different artifacts.

1. Define the artifact and write 3–6 concrete criteria for choosing it. Choose `first pass`, `rank all`, or `best-of` before starting.
2. Check whether the active Pi session exposes a subagent tool. If it does, give every candidate the same standalone brief and an isolated output location. Select only models the tool and current Pi configuration actually support. Do not assume a provider or model family.
3. If no subagent tool is available, do not pretend parallel candidates ran. Work the best-supported direction directly and state that a multi-candidate comparison was unavailable.
4. Read each candidate's full output, compare it with the criteria, and choose the base a maintainer can extend safely.
5. Graft only specific improvements that fit coherently. Verify the synthesized result against the real artifact.

Use an independent judge only when Pi exposes a separate reviewer and an available model choice; otherwise omit that step and say so. Report the selection rule, chosen base, useful grafts, rejected alternatives, gaps, and verification.
