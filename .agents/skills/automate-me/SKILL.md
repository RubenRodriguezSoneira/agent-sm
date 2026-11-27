---
name: automate-me
description: Capture a user's stated working conventions in a small Pi skill. Use for "automate me", "make a mode skill", or "capture how I work".
disable-model-invocation: true
---

# Automate me

Turn explicit preferences into one concise `-mode` skill. Do not infer personal preferences from unrelated projects or private sessions.

1. Check for an existing skill in the active project's `.agents/skills/` and, only when the user asks for a personal skill, `~/.pi/agent/skills/`. If one exists, update it unless the user requested a fresh start; preserve rules that remain consistent.
2. Use the current conversation and any history the user explicitly identifies. Pi sessions default to `~/.pi/agent/sessions/`, but their location can vary. Do not scan unrelated session files. A behavioral preference needs repeated or clearly intentional evidence; otherwise ask in the interactive conversation or omit it.
3. Group only confirmed preferences that change the agent's choices: response style, autonomy, investigation, delegation, code discipline, verification, workflow, and skill maintenance.
4. Write the skill in the user's preferred scope: `.agents/skills/<handle>-mode/SKILL.md` for this project or `~/.pi/agent/skills/<handle>-mode/SKILL.md` when they explicitly want it globally. Follow the Agent Skills frontmatter format and do not assume custom mode UI.
5. Keep the skill short, reference other skills instead of copying them, and verify its links and frontmatter. Present the draft for feedback when the session is interactive.

In non-interactive runs, do not block waiting for preferences. Use only explicit evidence, omit uncertain rules, and report any choice that needs the user's decision.
