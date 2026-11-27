---
name: make-bot-ui
description: Build a small UI that calls an existing, authorized agent endpoint. Use when a user wants buttons or a dashboard to trigger an agent workflow.
disable-model-invocation: true
---

# Make an agent UI

First identify an existing agent endpoint or integration in the repository or active Pi session. Pi does not guarantee a webhook, automation, secret-store, or public server API. Do not invent a URL, connector, authentication header, or user-facing control.

1. If no supported endpoint is available, explain the missing integration and stop before building a UI that cannot work.
2. Treat endpoint input as untrusted data. Validate the fields and actions on the server, keep credentials server-side, and never place secrets in browser code, logs, or chat.
3. Reuse the repository's existing server, authentication, and UI conventions. Do not expose a local agent service to the network or install/configure a tunneling service unless the user explicitly requests that exposure.
4. Keep requests bounded and idempotent where possible. Add a timeout, show errors honestly, and do not retry an action that could run twice unless the endpoint guarantees idempotency.
5. Verify the complete user path with a harmless request against the real integration before calling the UI live.

If a needed secret cannot be supplied through an approved secret mechanism visible in this environment, do not ask for it in chat. Report the blocker and leave the integration unconfigured.
