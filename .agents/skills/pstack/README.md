# PStack skills for Pi and agent-sm

The 59 skills in this directory were adapted from [Cursor's PStack collection](https://github.com/cursor/plugins/tree/b7b93c36be9bcc36a74db438e0fd385d660bc559/pstack/skills), pinned at commit `b7b93c36be9bcc36a74db438e0fd385d660bc559`. The upstream MIT license is in this directory.

## Pi compatibility

- The skills live in `.agents/skills`, a project skill location Pi discovers from the repository root. Invoke a skill with `/skill:<name>` or let Pi select it from its description. `disable-model-invocation: true` skills require explicit invocation.
- Use only tools available in the active Pi session. Pi's built-ins include `read`, `bash`, `edit`, `write`, `grep`, `find`, and `ls`; extensions and MCP tools are optional and must be visibly available before use.
- Pi does not guarantee a subagent tool or per-role model settings. Use a subagent or model selector only when the current session exposes one with the needed capability. Otherwise work serially and state when a workflow's parallel or cross-model check could not run. Never start nested Pi processes to imitate agents.
- Pi can use different providers and model IDs. Rely on the current session's model and the models/settings Pi actually exposes; do not hard-code vendor names or assume a model is available. `setup-pstack` explains the supported Pi settings.
- Ask for choices in the interactive conversation. In non-interactive runs, use a safe default when possible and report any decision that genuinely blocks the work.
- Pi session files default to `~/.pi/agent/sessions/` and can be relocated with `PI_CODING_AGENT_SESSION_DIR`, `--session-dir`, or `sessionDir`. Treat them as private; inspect only the active project/session and never scan unrelated workspaces.

## agent-sm worker boundary

`agent-sm` deliberately starts isolated Pi workers with `--no-skills --no-mcp`. Do not change those flags or suggest that worker steps rely on these skills, MCP servers, GitHub credentials, or tools that the worker profile does not grant. The repository-specific verification skills remain under `.claude/skills`; load one explicitly in Pi with `pi --skill .claude/skills/<skill-name>` when needed.
