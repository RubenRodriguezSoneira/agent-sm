---
name: how
description: Explain how code works, how components connect, and where a change belongs. Use for "how does X work" or architecture and ownership questions.
disable-model-invocation: true
---

# How

Explore the relevant code and explain the runtime flow, ownership boundaries, and important data shapes. If the scope is ambiguous, state the interpretation and proceed with the most likely one.

Use the tools available in the active Pi session. When a subagent tool is exposed and the codebase is large enough to justify delegation, give it a read-only, bounded question. Otherwise investigate directly. Present a concise explanation with source paths and distinguish observed behavior from inference.
