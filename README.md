# ntwine

AI agents that code together.

multiple models. one codebase. real-time collaboration with presence, attribution, and shared tools.

## what is this

ntwine is a collaborative AI IDE where multiple AI models work on the same codebase simultaneously. they can see each other's cursors, call each other out on bad code, and spawn sub-agents when they need help.

## the harness

the core of ntwine is its tool execution harness. 2,500+ lines of Go incorporating patterns from 20+ products:

**tools:**
- `read_file` / `edit_file` / `write_file` - file operations with read-first enforcement
- `search_code` / `list_files` - codebase navigation
- `shell` - command execution with dangerous pattern blocking
- `think` - forced reflection before critical decisions
- `update_notes` / `pin_message` - shared spec and pinning
- `create_memory` / `recall_memory` - persistent context across sessions
- `use_skill` - load markdown skill files
- `web_search` - tavily web search
- `mcp:*:*` - any tool from any connected MCP server

**harness features:**
- parallel tool execution (models call multiple tools at once)
- observation masking (old tool results replaced with [masked] to save context)
- loop detection with escalation (warn -> force text -> abort)
- git worktree isolation for parallel agents
- append-only event log with pub/sub
- hooks system (pre/post tool execution)
- project config loading (.ntwine/config.md, AGENTS.md, CLAUDE.md)
- dangerous command blocking (rm -rf /, fork bombs, etc)
- environment variable sanitization

## setup

```
go build ./cmd/server
./server --port 8080
```

## mcp config

create `.ntwine/mcp.json` or `.mcp.json` in your project:

```json
{
  "servers": [
    {
      "name": "filesystem",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."]
    }
  ]
}
```

## project config

create `.ntwine/config.md` in your project with rules for the agents:

```markdown
this is a go project using gin for the api layer.
always run `go test ./...` after making changes.
prefer table-driven tests.
```

## skills

drop markdown files in `.ntwine/skills/` or `~/.ntwine/skills/`:

```markdown
---
name: my-skill
description: does a thing
---

instructions for the agent when this skill is loaded...
```

## license

MIT
