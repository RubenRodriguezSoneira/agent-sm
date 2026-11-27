---
name: interrogate
description: Adversarially review a change or design and report only defects proven by a run. Use for /skill:interrogate, "stress test this", or "find blind spots".
disable-model-invocation: true
---

# Interrogate

Break the stated change or design. Do not apply fixes.

1. Define the scope and intent from the user's request and the current diff. If intent is unclear, state the ambiguity and inspect what can be proven without it.
2. If the active Pi session exposes independent reviewer agents, give each the same bounded adversarial brief and isolated scratch space. Use only supported tools and models.
3. Without reviewer agents, do a separate self-review pass after the ordinary implementation pass. Do not claim independent or cross-model review.
4. Every finding must be proven against the real code with a failing test, reproduction, or command output. Rerun each proposed finding and drop anything that does not reproduce or violate the stated intent.

Report the intent, verified findings with exact evidence, and dropped or unavailable review paths. Do not edit the product code.
