### Shipping

**Verify the exact change that will land, merge only when asked, and confirm the forge's result.**

1. Verify each PR independently on its current head. Green CI alone is not proof of user-facing behavior; run the repository's verification skill or named driver when one exists.
2. For a stack, identify the dependency order and verify from the bottom up. A PR whose base or patch changed since verification needs fresh checks for the affected behavior.
3. Use `gh` only when installed and authenticated. Inspect each PR's base, head, review state, and checks immediately before landing.
4. Merge only when the user explicitly requested it. Merge bottom-up with the repository's configured strategy, then read the PR back to confirm it is merged rather than queued.
5. Stop at the first unverified or blocked PR. Do not retarget branches, force-push, or bypass required approvals to make the stack appear green.

**Reply:** the verified run and its ceiling, evidence for each PR, what landed and how it was confirmed, and any remaining gap.
