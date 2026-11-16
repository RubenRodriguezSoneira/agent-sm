# agent-sm

A state machine that takes one GitHub issue to a reviewed branch. The program owns the loop:
each step returns a typed outcome, one table (`next()` in `main.go`) decides the transition, and
agents (`copilot -p` by default, or `claude -p`) only run inside single steps. Every step is declared once in the `steps`
table in `main.go`: its function, outcomes, role prompt, output schema, allow list and model.

    prepare -> plan -> investigate (one session per unit) -> execute -> check -> review -> publish

Exits: `ready`, `gave-up`, `errored`, `split`, `escalated`, `stopped`.

## Usage

    go build
    agent-sm --config /trusted/go.json <repo-dir> <issue-number>
    agent-sm --config /trusted/go.json serve [--repo DIR] [issue] [addr]
    agent-sm stats [file]                       # per-state cost, models, tools used, denials
    agent-sm allow [file]                       # allow lists suggested by the stats, as SM_ALLOW_FILE JSON

Sub-issues: on the page, an issue with GitHub sub-issues shows its progress. Selecting it queues
its open sub-issues in its place, one after another; the issue itself never runs. A sub-issue is
skipped when GitHub lists an open issue as blocking it, when its body's Dependencies section names
an open issue, or when it has sub-issues of its own (select that one directly). The command line
still runs exactly the one issue it is given.

Re-running an issue: a run that does not merge leaves its worktree and its `sm/issue-<n>` branch,
because for a local-only run the staged changes there are the result. The page marks such an
issue "old work"; selecting it turns Run into "Discard old work and run", which deletes that
worktree and local branch first. A branch on origin or an existing PR is never deleted.

The repo's working tree must be clean; the run creates the branch `sm/issue-<n>`. It needs
`copilot` (or `claude`), `git` and `gh` on the PATH, authenticated independently of the desktop
app, plus the target project's build/test tools. Checks must be configured; there is no
implicit Cargo default. `stats` and `allow` do not require a project profile.

There is no resume: a run always starts at `prepare`. A stopped or failed run leaves its branch
in place; delete it before running the issue again.

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `SM_PUBLISH` | empty | empty: stop at a local staged branch. `pr`: commit, rebase, push, open a draft PR. `merge`: also merge it. |
| `SM_BASE` | `origin/main` | Branch that worktrees start from and publish rebases onto. |
| `SM_PROVIDER` | `copilot` | Runner: `copilot` or `claude`. |
| `SM_SESSION_LIMIT` | `2` | Session limit in the provider's native units: Copilot AI credits or Claude USD. |
| `SM_RUN_LIMIT` | `20` | Run limit in the same native units; `0` disables it. Unknown usage prevents another step under a run limit. |
| `SM_BUDGET`, `SM_RUN_BUDGET` | none | Legacy Claude-only dollar-limit aliases. Rejected for Copilot rather than converted to credits. |
| `SM_TIMEOUT` | `60` | Minutes before an agent session and its child processes are killed. |
| `SM_MODEL_<STATE>` | Copilot: `auto`; Claude: sonnet 5.5, opus 5.5 for review | Explicit CLI model ID per step. The web input wins for queued runs. |
| `SM_CHECKS` | profile (required) | Check commands, one per line. Empty checks fail configuration. |
| `SM_FMT` | profile or none | Formatter run before staging/checks; failure is explicit. Setting `SM_CHECKS` without `SM_FMT` disables the profile formatter. |
| `SM_ALLOW_FILE` | none | Claude-only JSON `{"state": "tools"}`; invalid files fail startup. Prefer profile `commands` for portable shell tools. |
| `SM_EXTRA_ALLOW` | none | Claude-only allow entries added to every agent step. |
| `SM_ADD_DIRS` | none | Comma-separated directories agents may read outside the repo. |
| `SM_NOTES_DIR` | none | Operator notes added to dispatches: `all.md`, or `<role>.md` for one step. |
| `SM_AUDIT` | off | Claude only: load every tool and bypass permissions (outward commands stay denied) to measure tool use. Not a sandbox. |
| `SM_GHIDRA_COPY` | `none` | Legacy-profile-only Ghidra project copy. |
| `SM_EXPORT` | `~/rescue-re/export` | Legacy-profile-only export; generic runs do not inspect it. |
| `SM_EPIC` | none | Epic that a follow-up issue joins (split, or three failed fix rounds). |
| `SM_LINKS` | none | Comma-separated gitignored files to symlink into a worktree (serve). |
| `SM_STATS` | `agent-sm-stats.jsonl` | Where session stats are appended. |
| `SM_SCRATCH_ROOT`, `SM_SCRATCH`, `SM_EVENTS` | `~/.local/state/agent-sm` (or `$XDG_STATE_HOME/agent-sm`), `<root>/<issue>`, `<scratch>/events.jsonl` | Where run data goes. |

A session that hits `SM_TIMEOUT` returns `timeout`; an observed native limit exhaustion
returns `over-budget`. Both end with `gave-up`. Unknown/missing usage is never reported as
free and stops a bounded run before its next step. Unknown CLI errors fail closed as invalid,
not as completed work. Native billing limits can be soft/provider-enforced, not exact money caps.

## Profiles and tools

Pass an explicit operator-owned JSON file; configuration is never auto-discovered in a target
branch. See [Go](examples/go.json), [.NET unit testing](examples/dotnet-unit.json), and
[legacy Claude/Rust/Ghidra](examples/claude-legacy.json). Copy and adapt the examples outside
the target checkout. Precedence is defaults, profile, environment, then validated web overrides.
Relative `notes_dir` and `add_dirs` paths resolve against the profile's directory. Queued runs
receive an immutable resolved snapshot, including models and limits.

Supported fields are `provider`, `models` (state to CLI model ID), `checks`, `format`,
`required_test_reports`, `session_limit`, `run_limit`, `timeout_minutes`, `commands`,
`notes_dir`, `add_dirs`, `base`, `jira`, `management`, and the Claude-specific `claude_allow`,
`claude_extra_allow`, `audit`, and `legacy_rust_ghidra`. Unknown fields fail startup.
The server uses one provider per profile; restart with another profile to change it.

`checks` are trusted shell commands run deterministically in the worktree, including after a
publish rebase. They have timeout/Stop cancellation and must not mutate the checked candidate.
`format` runs before staging. `commands` separately grants agent access per role, for example
`{"execute":["dotnet build","dotnet test"]}`. Prefixes use plain command/subcommand words,
not shell expressions. They are translated to each runner's permission syntax and RTK forms;
granting `dotnet test` does not grant every `dotnet` command. Program-owned GitHub/history
mutations stay outside worker command profiles. Permission lists are not an OS sandbox:
execute untrusted repositories/tests in a disposable, appropriately isolated environment.

The .NET example is for the reference project's .NET 10 Microsoft.Testing.Platform runner
and its UnitTest project's installed TRX extension, not a universal .NET invocation. It does
not cover IntegrationTest, which needs its own infrastructure and reporting configuration.
`SM_CHECK_RESULTS` is a fresh directory for every check pass; required TRX report globs are
relative to it. Missing reports, zero executed tests, failed tests, or skipped required tests
fail validation. Enable equivalent evidence for every additional required test suite.
`SM_SCRATCH` is also available to check commands. SDKs, feeds, Docker, and packages remain
the target project's prerequisites; agent-sm does not install them.

Generic roles read the target's applicable instructions; use shared `AGENTS.md` and explicit
role notes for portable guidance. Copilot also loads its normal repository instructions.
Do not pass app configuration as an agent-sm profile: `.github/github-app.yml` remains owned
by the Copilot app. Old rescue-specific prompts are selected only by `legacy_rust_ghidra`.

Copilot uses documented silent final-response mode, strict local JSON validation, and
`--usage-output-file` for native credits. Its live feed currently shows start/end, not tool
calls; unavailable model/tool/turn/usage data is marked unknown. Claude retains its stream
feed. Both reject missing, duplicate, malformed, or off-schema result fields and failed
subprocesses. Stats separate providers and units; old Claude stats remain readable.
`allow` emits Claude syntax only; it does not infer a Copilot policy from absent telemetry.

Worker MCP servers, delegation, and broad permission bypasses are not enabled for Copilot:
its visible tools are explicitly restricted to file/search/shell tools. Add trusted command
tools through `commands`; Jira uses the separate program-owned transport below. The runner
does not pass `--trust-folder` or alter your saved CLI trust settings. Review inherited CLI
configuration/hooks and use a clean automation account for untrusted targets.

## GitHub and linked Jira

GitHub remains the queue; Jira tickets are not independent run inputs. An issue may declare:

```markdown
## Jira
- **Jira issue:** https://your-team.atlassian.net/browse/TEAM-42
- **Jira Epic:** https://your-team.atlassian.net/browse/TEAM-1
- **Base branch:** feature/integration
```

Only `Jira issue` identifies this ticket's counterpart. The declared base is used for worktree
creation, the task branch, rebasing, and the draft PR's actual base. Otherwise `SM_BASE`/profile
`base` applies. A linked Jira key is included in the PR title.

Jira is off by default. Adapt [the Action-owned example](examples/jira-actions.json), including
your workflow's exact status names. It assumes the target repository already has a trusted
lifecycle Action, like the reference project's `sync-jira-status.yml`; agent-sm neither
installs nor replaces it. Use this independently after human review/merge:

```sh
agent-sm --config /trusted/project.json reconcile <repo-dir> <github-issue-number>
```

Standalone reconciliation uses `JIRA_USER_EMAIL` and `JIRA_API_TOKEN` from the process
environment, never profile credentials or the desktop Atlassian connection. Optional `jira.api_base`
supports a scoped-token gateway (`https://api.atlassian.com/ex/jira/<cloudId>`); `jira.site`
must still be the browser site. Both URLs must refer to the same tenant. Redirects are rejected.
Credentials are not forwarded to worker agents.

Reconciliation is **read-only status verification**: it verifies the explicit counterpart and
exact GitHub backlink, closing-issue reference, Jira PR-title token, and intended PR base.
Assignment establishes the implementation phase; only an open non-draft closing PR
establishes review, and its verified merge establishes delivery. Partial/Refs PRs do not advance
delivery, and child completion never advances a parent. Conflicts are reported, not overwritten.
Assignee selection, Start date initialization, transitions, and failed-Action recovery stay with
the existing Action/agent integration; status alignment is not a claim that those are verified.

Results are `already aligned`, `conflict`, `blocked`, or `skipped-unavailable`; only aligned
status exits reconciliation with status 0. During intake, conflicts/invalid mappings block that
run; unavailable integrations are recorded without blocking independent code work.
Off/Action-owned modes do not add ticket creation, labels, hierarchy, sprint edits, automatic
review readiness, or additional merge permissions. Service-owned management is separate below.

## Service-owned tracker management

This opt-in mode makes the **native Go service** the tracker writer, using `gh` for GitHub
and native HTTP for Jira. LLMs only propose structured plans; they do not get tools for tracker
CRUD, retries, or monitoring. Existing `off` and `actions` profiles keep their behavior;
only `management.mode: "service"` enables the new management writes. Tracker operations and
recovery do not launch agent sessions; live tenant behavior still requires sandbox verification.

Adapt [the service profile](examples/service-management.json) in an operator-owned file outside
the target checkout. Set `management.host` and `repo` to the intended GitHub scope. Jira uses
`jira.mode: "service"`, the browser `site`, project/epic, exact issue-type/status names, and the
Start date field ID. `initial_statuses` explicitly lists earlier statuses from which the
service may advance work; unknown states block rather than guess. The service discovers a
unique direct transition to the destination status, never an implicit multi-step workflow.
`identities` explicitly maps GitHub logins to Jira account IDs;
`github_assignee` selects the configured GitHub assignee. Use only `gh auth` and environment
`JIRA_USER_EMAIL` / `JIRA_API_TOKEN` for credentials, never profile secrets or desktop connections.
The example contains placeholders, not real identities or working tenant settings. Jira-required
custom creation fields can be supplied per exact type name, for example
`"create_fields": {"Epic": {"customfield_123": "Epic name"}}`; discover actual field IDs and
values in your project, not from this example. No Jira labels are added.

Service GitHub tokens (`GH_TOKEN`, `GITHUB_TOKEN` and Enterprise variants) and Jira credentials
are removed from agent/check environments. Agent tool policies also deny tracker commands,
including RTK forms; audit/extra-permission bypasses are rejected in service mode. These are
not an OS/network sandbox: use separate least-privilege agent authentication and do not expose
service credentials in files the workers can read.

Before enabling service mode, disable overlapping Jira lifecycle Actions, issue-close and
parent-roll-up Actions, and Jira-writing skills/other agents. List workflow filenames or IDs in
`management.workflows` for preflight checks. `writers_disabled: true` is the operator's
attestation that **all** competing writers are disabled, not an instruction that disables them.
An empty workflow list does not prove there are none. Preflight can inspect only configured
workflows; other writers require operator verification. No external repository is edited automatically.

**Management permission is independent of `SM_PUBLISH`.** That variable still owns push,
PR creation and merge authorization. Enabling management separately authorizes issue updates
**even when Publish is local-only**; local-only is not a tracker-write prohibition.

### Propose, preview, approve, apply

A WorkPlan is JSON with an `id` and `items`. Each item has `id`, `title`, `body`, and
`kind` (`epic`, `story`, or `task`). Optional fields are `number` (existing GitHub issue),
`jira_key` (existing Jira counterpart), `parent`, `depends_on`, `remove_dependencies`,
`base`, `assignee`, `github_type`, `remove_parent`, `labels`, and `remove_labels`.
Labels are additive unless explicitly listed for removal; unrelated labels are preserved.
Parent and dependency references are **local item IDs
in that plan**, not issue numbers, Jira keys, or cross-plan references. For example:

```json
{
  "id": "example-plan",
  "items": [
    {"id": "story", "title": "Example feature", "body": "Define the outcome", "kind": "story"},
    {"id": "first", "title": "Implement", "body": "Implement the feature", "kind": "task", "parent": "story"},
    {"id": "second", "title": "Verify", "body": "Verify the feature", "kind": "task", "parent": "story", "depends_on": ["first"]}
  ]
}
```

The CLI interface is:

```sh
agent-sm --config /trusted/service.json issues preview <repo-dir> PLAN.json
agent-sm --config /trusted/service.json issues apply <repo-dir> DIGEST
agent-sm --config /trusted/service.json issues status <repo-dir>
agent-sm --config /trusted/service.json issues cancel <repo-dir> DIGEST
agent-sm --config /trusted/service.json reconcile <repo-dir> <github-issue-number>
agent-sm --config /trusted/service.json reconcile <repo-dir> <github-issue-number> --apply
```

Preview is read-only. Review its scope, plan, expected remote state, operations and digest.
Explicitly apply **that digest**, never an automatically selected latest plan. Structural
create/edit, hierarchy and dependency changes require approval; configured routine lifecycle
updates are automatic. Optional coding-planner `issue_plan` output and follow-up work become
pending proposal files, not automatic ticket creation. They need the same preview/approval path.
Reconcile is read-only by default on the CLI; `--apply` explicitly authorizes lifecycle repair,
not unapproved structural changes. Off/Action-owned modes remain read-only.

Plans are limited to 100 items/1 MiB and must be acyclic; remote graph validation stops
explicitly at 1,000 nodes instead of accepting incomplete evidence. `title` is an explicit
replacement. An empty `body` preserves existing descriptions; a nonempty body explicitly
replaces them after approval, with canonical links added by the service. Omitted edges are
not removals. `remove_parent: true` explicitly detaches an existing non-task; Jira Sub-tasks
must retain their Story parent. Native issue types/labels must already be available.
Worker context keeps human comments but excludes the journal-identified service progress
comment, and rejects issue/comment context above 1 MiB. GitHub collections fail explicitly
above 16 MiB rather than accepting incomplete evidence.

New and changed proposals need stable, distinct plan IDs. Completed applies are no-ops.
Started, approved batches resume after restart and on eligible web reconciliation, using the
original approval and per-operation drift checks. Re-previewing that unchanged in-flight plan
returns its original artifact. Ambiguous creates search for the exact marker/backlink before
retry; uncertainty never authorizes another POST. Resolve unknown outcomes on the trackers,
then retry the same digest. `issues cancel` stops a batch's recovery **without rollback or
deleting anything**; inspect partial resources and use explicit existing issue numbers/Jira
keys with a new plan ID before proposing replacement work. Do not edit the journal to repair it.

In the web panel, paste the WorkPlan, choose **Preview (read-only)**, inspect the displayed
JSON/digest, check its approval box, then choose **Apply approved digest**. Nothing auto-applies.
Editing the input, another preview, reconciliation, or an apply attempt clears approval.
Use **Refresh status** to inspect managed operation/item maps and sync errors, and
**Reconcile / retry** to explicitly request reconciliation/eligible retries. The web reconcile
action can write configured lifecycle repairs; it is not the read-only CLI verification command.
After an uncertain apply response, inspect status rather than blindly submitting it again.

The UI uses `GET /api/management` for `{enabled, state}`; state includes preview/item maps
and may include operations and sync errors. `POST /api/management/preview` takes the WorkPlan
directly and returns `{digest, scope, plan, expected, operations}`. Apply takes `{digest}` at
`POST /api/management/apply`; reconcile takes `{}` at `POST /api/management/reconcile`.
Both return state or an explicit non-success error. POSTs use the existing `X-SM-Token`.
Proposal and remote content is rendered as text, never trusted HTML.

### Lifecycle and limits

While `serve` runs, the service watches registered PRs and resumes managed tracking after
restart. A one-shot command runs no daemon and cannot observe later review/merge events;
keep `serve` running or reconcile manually. This does not resume a stopped coding run.
Lifecycle follows verified assignment, review-ready closing PR and merge evidence; drafts
and partial/Refs PRs do not imply delivery, and completing a child never rolls up its parent.
Start dates use a verified assignment event's UTC date and never overwrite an existing date.
Missing events remain pending rather than substituting today's date. Stopping/failing a run
pauses its assignment/date automation until a new implementation starts; existing PR monitoring
continues. Automatic polling stops after verified delivery; explicit reconcile still checks
completed items. A human reopening delivered work requires intervention, not an automatic re-close.

GitHub/Jira changes are **not atomic**: inspect per-operation partial results and sync errors.
Ambiguous creates block for reconciliation/operator resolution instead of blind retry.
The state-root lock coordinates processes on the same host only; there is no multi-host
coordination. Do not run competing instances from separate state roots or hosts for the same
trackers. No sprint management, broad bidirectional mirroring, automatic external-repo edits,
or extra publication/merge authorization is implied. Local syntax/fixture checks do not verify
real GitHub/Jira behavior, live credentials, tenant configuration, or integration performance.
The durable journal is under `<state-root>/management/<repository-hash>/`, separate from
rotating run logs, with private files and a per-repository process lock. It currently rewrites
the complete history; large installations should compact/split this state before scaling it.

## Run data

- `<scratch>/events.jsonl`: one line per step, per agent session and the exit. When a new run of the issue starts, the old log is renamed to `events-<date>-<time>.jsonl`, not deleted.
- The scratch root is durable, so run history survives a reboot. Agents also write probe output (traces, dumps) there; delete an issue's directory when you no longer need it.
- `<scratch>/status.json`: the running session's live state. `<scratch>/run.log`: output of a run started from the page.
- `agent-sm-stats.jsonl` (`SM_STATS`): one line per agent session. It is local output and is gitignored.
- `runs/`: archived attempts kept in the repo as evidence for the issues that cite them. Nothing writes there.

## Platforms

Linux and macOS. The code uses process groups, `sh -c` and symlinks, so it does not build on
Windows; use WSL.

## Tests

GitHub Actions runs formatting checks, `go vet`, race-enabled tests, and a build on pull
requests and pushes to `main`, using the Go version in `go.mod`. Live agent smoke tests
remain opt-in and are not run in CI.

    go vet ./... && go test ./...
    SM_SMOKE=1 go test -run Smoke -v                     # opt-in: one real Copilot session
    SM_SMOKE=1 SM_PROVIDER=claude go test -run Smoke -v  # opt-in: one real Claude session

Normal tests use fake CLIs and temporary repositories; they do not call paid agents or mutate Jira.
CLI flags/usage follow the installed CLI's help. Live provider and target-.NET compatibility
must be checked with these explicit smoke tests in your authorized environment.
