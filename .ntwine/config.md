this is ntwine, a collaborative AI IDE written in Go.

the backend is in internal/ with these packages:
- harness/ - tool execution engine (registry, builtins, mcp, context management)
- orchestrator/ - multi-model discussion engine
- openrouter/ - LLM provider client (supports 13+ providers)
- api/ - HTTP/WebSocket API
- config/ - encrypted config storage

the desktop app will be Avalonia UI (C#/.NET) talking to this Go backend.

rules:
- keep files under 400 lines
- no premature abstractions
- test after changes with `go build ./...`
- never hardcode API keys
- use context.Context for cancellation everywhere
