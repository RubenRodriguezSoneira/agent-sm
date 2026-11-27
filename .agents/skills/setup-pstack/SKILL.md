---
name: setup-pstack
description: Explain or adjust Pi's model and reasoning settings for PStack. Use for /skill:setup-pstack, "configure PStack", or questions about model/provider setup.
disable-model-invocation: true
---

# Set up PStack for Pi

Pi owns provider and model selection. This skill does not define per-role model IDs or write a role-specific model rule.

1. Inspect the active model and available providers using Pi's `/model` and `/settings` commands. If running from a shell, `pi --list-models` can show the models available in that installation.
2. Read the relevant Pi settings only when the user asks to change them. `defaultProvider`, `defaultModel`, `enabledModels`, and thinking-level settings are user configuration; do not overwrite unrelated values or credentials.
3. If the user wants a recommendation, explain the tradeoffs using models actually listed in their Pi environment. Never assume a provider, model name, or reasoning level is available.
4. PStack workflows that need multiple models or per-role choices can use them only when the active Pi session exposes a subagent or model-selection tool that supports those choices. Otherwise use the current model serially. Do not start nested Pi processes to imitate subagents.
5. Apply a setting only after the user clearly asks to change it. Prefer Pi's `/settings` UI; when editing a file, preserve unrelated settings and validate the resulting JSON.

Pi's `defaultThinkingLevel` and `modelThinkingLevels` are not a replacement for a per-role PStack configuration. See [the Pi adapter](../pstack/README.md) for the repository's tool and worker boundaries.
