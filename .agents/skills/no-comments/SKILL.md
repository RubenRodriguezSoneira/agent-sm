---
name: no-comments
description: Review scoped code comments for dead explanations, workarounds, and constraints that should be encoded in tests or types. Use for /skill:no-comments or a comment cleanup request.
disable-model-invocation: true
---

# No comments

Review only the caller's files or diff; otherwise use the current diff against its base branch.

1. Inspect comments in scope and classify each as stale, redundant, workaround, contract, or constraint.
2. Remove comments only when code, tests, or repository documentation make their meaning unnecessary. Do not delete correctness or safety context just to reduce comment count.
3. For a constraint that can be encoded in a type, test, or lint, propose the smallest in-scope change. Do not broaden the scope or invent an enforcement mechanism.
4. Verify behavior after any code change. If an independent reviewer tool is not available, review the final diff yourself and do not claim independent review.

Report the meaningful removals, retained constraints, changes made, and anything still open.
