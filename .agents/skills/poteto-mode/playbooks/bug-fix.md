### Bug fix

**You own this task. Plan, review, verify.** Use an available Pi subagent tool when it materially helps; otherwise investigate and fix directly.

Be scientific. Every shipped line traces to runtime evidence. Belt-and-suspenders that "might help" is a hypothesis, not a fix. It does not ship. When evidence refutes a hypothesis, revert what it motivated. The smallest change the evidence justifies ships, nothing more.

1. Reproduce it yourself on the matching surface via the control skill (Non-negotiables), even when a debug or instrumentation protocol says to ask the user to reproduce. Ask the user only with a stated, specific reason the control surface cannot reach the target, and only after driving it as far as it goes. If it won't reproduce directly, synthesize the trigger, tighten conditions, or instrument until it fires.
2. Binary-search the cause. Form candidate hypotheses, then rule them out with runtime evidence. Seed them with a trace of the affected subsystem and the **why** skill for regression history. When program state is unclear, add instrumentation and observe it as the code runs. For a long hunt, use only a scheduler or agent tool actually available in Pi. Confirm the surviving mechanism before planning the fix.
3. If the fix crosses a function boundary, use `architect` first. Delegate only when Pi exposes a suitable subagent tool; otherwise implement directly.
4. Verify on the same surface. The original repro now passes. "Inconclusive" or wrong-surface is not a pass. Flag it. Unit tests show branch behavior, not bug absence.
5. Stage the commits so the failing repro lands before the fix in git history. See the **tdd** skill for the failing-test-first cadence when the bug has a cheap local test path. Skip it when the test would be expensive, integration-heavy, or unclear.
   This is the canonical **sequence-verifiable-units** principle skill, the failing test first and the fix on top.
6. Run **Opening a PR**.

**Reply:** what was broken, root cause, fix, how you verified. Paste failing-then-passing repro output verbatim.
