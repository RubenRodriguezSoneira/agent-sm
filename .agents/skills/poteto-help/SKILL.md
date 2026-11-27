---
name: poteto-help
description: Guides users through pstack setup, /skill:poteto-mode, and picking the skill, playbook, or principle for a task. Type /skill:poteto-help with a question.
disable-model-invocation: true
---

# Poteto help

Answer the user's question about pstack, hand them a prompt they can send, and link the file the answer came from. For a help question, don't start the work. The user asked how, and a pstack run spends real tokens, so let them send the prompt.

A message that asks for work, such as "use pstack to fix this bug", is not a help question. Read [`poteto-mode`](../poteto-mode/SKILL.md) and do the work under it.

This file maps questions to the bundled skills that hold the answers. Those files own the details. Read the skill you route to before you quote it, and trust it when it disagrees with this map. The skills are in `.agents/skills` and Pi can invoke them with `/skill:<name>`.

## Find out what they need

Infer the need from the message and conversation. A named situation, such as "which skill reviews a PR?", goes straight to its section. If the need is still unclear, ask one concise question in the Pi conversation:

- Get set up
- Start a task with `/skill:poteto-mode`
- Pick a skill for a situation
- Fix a run that went wrong
- Make pstack my own

Check the state that changes the answer, and mention it only when it does:

- Pi has no built-in per-role PStack model map. `/skill:setup-pstack` explains the active provider/model settings and only changes them when the user asks.
- No verification skill or app harness means there is no scripted user-path proof. Mention `/skill:create-verification-skill` when the question is about proving a change works.
- No formal verification skill in the project means no property is checked past what tests reach. Mention `/skill:create-formal-verification-skill` when the question is about a race, an invariant, or another case tests rarely hit. When one exists and the code it covers changed after the commit it records as last in sync, mention `/skill:maintain-formal-verification-skill` if the question touches that code.

## Get set up

1. Run Pi from the repository root so it discovers `.agents/skills`.
2. Invoke [`/skill:setup-pstack`](../setup-pstack/SKILL.md) only to inspect or adjust Pi's actual model/provider settings.
3. Start a real task with `/skill:poteto-mode`, a goal, and a check that can pass or fail.

Pi loads skill descriptions at startup and full instructions when invoked. Offer to word a first prompt using [`references/prompting.md`](references/prompting.md).

If cost is the worry, use the provider and model actually selected in Pi and avoid optional multi-agent work. Pi's settings do not provide this collection with a per-role budget map.

This repository targets Pi. Unsupported integrations are not silently emulated: when a skill needs a tool that Pi does not expose, use its documented serial fallback or report the limitation.

Pi does not guarantee subagents, MCP servers, scheduling, or external automation tools. The PStack workflows check for those capabilities and otherwise work serially or identify the missing integration.

## Start a task with `/skill:poteto-mode`

`/skill:poteto-mode` matches the task to a playbook and uses its steps as a checklist. A good prompt states the goal and how to tell it's done. Read [`references/prompting.md`](references/prompting.md) before helping word one. Pi skills are loaded on demand, so invoke `/skill:poteto-mode` for each task that should use the playbook.

## Pick a skill

The default answer is `/skill:poteto-mode`, which runs most of the others when its steps need them. Name a skill directly when the user wants more or less of something than the playbook gives. Read the skill before you recommend it, and give one example prompt.

| The user wants to | Skill |
|---|---|
| Do any non-trivial task with rigor | [`/skill:poteto-mode`](../poteto-mode/SKILL.md) |
| Check that the agent understood their goal and the problem before it starts | [`/skill:in-your-own-words`](../in-your-own-words/SKILL.md) |
| Get a prompt rewritten deeper, not bigger, before sending it: what a top professional would add, their raw material and methods, and an outside check. It returns the prompt and doesn't do the task | [`/skill:sharpen`](../sharpen/SKILL.md) |
| Learn the expert words for what they're building, including words borrowed from neighboring fields, with every cited source checked, and get their description rewritten with them | [`/skill:reframe`](../reframe/SKILL.md) |
| Know how code works now, or where new code should live | [`/skill:how`](../how/SKILL.md) |
| Know why code is shaped this way, or where a number came from | [`/skill:why`](../why/SKILL.md) |
| Understand a change or subsystem, explained plainly | [`/skill:teach`](../teach/SKILL.md) |
| Catch up on their own recent work on a topic | [`/skill:recall`](../recall/SKILL.md) |
| Catch up on what changed in this chat since their last message, in plain words and small visuals, with every item waiting on them listed and decidable from the reply alone | [`/skill:ketchup`](../ketchup/SKILL.md) |
| Catch up like `/skill:ketchup`, in a short subtitled video of slides, with the waiting calls as text under it | [`/skill:ketchup-video`](../ketchup-video/SKILL.md) |
| Know what a small diff could break outside itself | [`/skill:blast-radius`](../blast-radius/SKILL.md) |
| Settle types and module shape before code that crosses a function boundary | [`/skill:architect`](../architect/SKILL.md) |
| Get several attempts at one brief, merged into the best one | [`/skill:arena`](../arena/SKILL.md) |
| Split independent checks into parallel slices when an agent tool exists | [`/skill:swarm`](../swarm/SKILL.md) |
| Adversarially review a diff and prove any findings | [`/skill:interrogate`](../interrogate/SKILL.md) |
| Fix a bug test-first when a cheap local test exists | [`/skill:tdd`](../tdd/SKILL.md) |
| Apply TypeScript rules to `.ts` or `.tsx` work | [`/skill:typescript-best-practices`](../typescript-best-practices/SKILL.md) |
| Review scoped comments for dead explanations and unencoded constraints | [`/skill:no-comments`](../no-comments/SKILL.md) |
| Clean AI tells out of prose | [`/skill:unslop`](../unslop/SKILL.md) |
| Write docs, an RFC, a README, a PR description, or a commit message to a standard | [`/skill:technical-writing`](../technical-writing/SKILL.md) |
| Hear the last reply again in plain words | [`/skill:bro`](../bro/SKILL.md) |
| Give agents a scripted way to drive the app and prove behavior | [`/skill:create-verification-skill`](../create-verification-skill/SKILL.md) |
| Bring a verification skill and its feature map back in line with the app | [`/skill:maintain-verification-skill`](../maintain-verification-skill/SKILL.md) |
| Formally verify a race, invariant, or other property that tests rarely reach | [`/skill:create-formal-verification-skill`](../create-formal-verification-skill/SKILL.md) |
| Bring a formal verification back in line after the code it covers changed, and learn whether a failure is drift or a bug | [`/skill:maintain-formal-verification-skill`](../maintain-formal-verification-skill/SKILL.md) |
| Show a PM or designer what the user sees as a chart checked by a TLA+ model, with the shortest path to each state that breaks a rule | [`/skill:statechart`](../statechart/SKILL.md) |
| Vet a performance number before reporting or acting on it | [`/skill:benchmark-checklist`](../benchmark-checklist/SKILL.md) |
| Run a large or cross-cutting change, or one to review after stepping away | [`/skill:figure-it-out`](../figure-it-out/SKILL.md) |
| Keep a decision log during a run, and review it afterward | [`/skill:show-me-your-work`](../show-me-your-work/SKILL.md) |
| Inspect or adjust Pi's actual model/provider settings | [`/skill:setup-pstack`](../setup-pstack/SKILL.md) |
| Turn their own working habits into a personal mode skill | [`/skill:automate-me`](../automate-me/SKILL.md) |
| Turn what a finished task taught into skill edits | [`/skill:reflect`](../reflect/SKILL.md) |
| Stop agents from repeating the same mistakes in this repo | [`/skill:correct`](../correct/SKILL.md) |
| Build a UI for an existing, authorized agent endpoint | [`/skill:make-bot-ui`](../make-bot-ui/SKILL.md) |
| Find their way around pstack | `/skill:poteto-help` |

If a skill directory next to this one is missing from the table, read its frontmatter and route by its description. The `principle-*` directories are covered under principles below.

Close calls:

- `/skill:how` explains what the code does. `/skill:why` explains the reasons. `/skill:teach` runs one or both and explains the result plainly.
- `/skill:arena` compares candidates when Pi exposes the needed agent tools. `/skill:swarm` splits independent work into slices and returns one report.
- `/skill:interrogate` reviews the diff. `/skill:blast-radius` looks for breakage outside the diff and proves the one fact that makes the change safe.
- `/skill:recall` rebuilds context across recent chats. `/skill:ketchup` covers only this chat since the user's last message. Resuming one specific chat or branch is the Session pickup playbook.
- `/skill:figure-it-out` designs one rigorous run. The Autonomous run playbook drives one task to a finish condition.

Not included in this repository's PStack bundle:

- Pi commands such as `/model`, `/settings`, `/session`, and `/resume` belong to Pi, not this skill bundle.
- `/orchestrate` is not part of this bundle. Use it only if the active Pi session exposes an integration with that name.

## Playbooks and principles

Playbooks are step lists inside `/skill:poteto-mode`, not skills, so they have no slash command. Inside `/skill:poteto-mode`, describing the task picks one, and these phrases name one directly:

- "babysit this pr" or "check on pr 123" runs Babysit. It reports blockers and stops before merge.
- "land the stack" runs Shipping.
- "take over this branch" runs Session pickup.
- "pause safely" runs Pause safely.
- "full autopilot on this queue" runs Autopilot-full. "autopilot, don't merge" runs Autopilot-stack.
- "run the eval playbook" runs Eval.

The Playbooks section of [`poteto-mode`](../poteto-mode/SKILL.md) lists every playbook.

pstack has no separate planning command. Use the current Pi conversation to plan when asked; for a design question, the Prototype playbook or `/skill:architect` can settle it in code first.

Principles are one-rule skills that `/skill:poteto-mode` reads and cites in its replies. The user rarely invokes one. They steer with the names instead, as in "apply prove it works. show me the real output." Typing `/skill:principle-<name>` loads one on demand.

## Fix a run that went wrong

| Symptom | Fix |
|---|---|
| A skill did not load | Run Pi from the repository root, confirm the `.agents/skills` directory is visible, and invoke `/skill:<name>` explicitly. |
| A question got treated as the next step of the last task | Say "new task", or say the turn doesn't need the mode. |
| A model choice had no effect | Check Pi's current `/model` and `/settings`; model availability and settings are provider-specific. |
| Runs cost more than expected | Use the currently selected Pi model and skip optional independent-agent work. |
| Parallel agents overwrote each other | Use isolated worktrees when a real agent tool supports them; otherwise serialize shared writes. |
| A long run made no progress | Define a checkable exit condition and use only a scheduler or automation exposed and authorized in this environment. |
| The reply claims success from a green build | Ask for the real command, flow, stored value, or profile. That's the prove-it-works principle. |

For a run that drifts, [`references/prompting.md`](references/prompting.md) has one-line steers.

## Make pstack my own

- [`/skill:automate-me`](../automate-me/SKILL.md) drafts a personal mode skill from the user's own history, to use alongside `/skill:poteto-mode`.
- [`/skill:reflect`](../reflect/SKILL.md) after a session turns its lessons into skill edits the user approves.
- `/skill:poteto-mode write a skill for <workflow>` runs the authoring playbook. The eval playbook tests a skill change blind.
- Fix a misbehaving skill in its own PR, not inside the feature work where it went wrong.

## Reply

Lead with the answer. Give at most one example prompt in a code block, adapted from [`references/recipes.md`](references/recipes.md) when one fits, then the link to that file. Keep it short unless the user asked for the whole map.
