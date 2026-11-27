### Worktree and simulator cleanup

1. Snapshot and audit. Record `df -h /`, then run `scripts/worktree-audit.sh [-b <pr-base-ref>] [repo-path]`. The audit reads paths from `git worktree list` and reports size, age, merge state, uncommitted work, and GitHub PR state when `gh` is available.
2. The audit is advice, not permission. Check the Pi `/resume` picker and current branch to identify pinned or active work. The script does not inspect session files.
3. Verify each candidate before removal. Show tracked changes and name untracked files. Any uncommitted work, active session, open PR, or uncertain ownership is a hold; do not infer that a worktree is disposable from age or size.
4. Removal is destructive. Remove only exact paths the user has confirmed, with `git worktree remove <path>`. Do not add `--force` or manually delete residual files without explicit approval. Run `git worktree prune` only after the selected removal.
5. Recheck `git worktree list` and `df -h /`. Do not clear caches or unrelated user state as part of this cleanup.

This is the one playbook that deletes user state with no code review to catch a slip, so the gates above are the review.

**Reply:** `df -h /` before and after, the exact worktrees removed, and the reason each other candidate was held.
