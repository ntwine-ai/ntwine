# ntwine Harness - Research Synthesis

## The Best Pattern From Every Source

| Pattern | Source | What to implement |
|---------|--------|-------------------|
| Concurrent-safe tool registry with middleware | mcp-go, Goose | Already built in harness/registry.go |
| Error as output, not exception | Zed | Already built - ToolResult.IsError |
| Hardcoded catastrophic deny rules | Zed, Codex | Already built - DenyRule list |
| Tool event streaming | Zed, Void | Already built - EventCallback |
| ModelDescription vs Description | Void | Already built - ToolDef.ModelDescription |
| Three-level permissions | Void | Already built - PermAllow/Confirm/Deny |
| Observation masking | JetBrains | Replace old tool outputs with [masked, N tokens] instead of LLM summarization. 52% cheaper. |
| Append-only context | Manus | Never modify prior messages. Only append. Maximizes KV-cache hits. |
| Stable system prompt prefix | Cursor, Manus | Keep system prompt identical across calls for 75% cheaper cached tokens. |
| todo.md attention hack | Manus | Write the plan to a file, push it into recent attention window. Zero cost. |
| Living shared spec | Augment | All agents read/write one canonical spec document. Already have shared notes - evolve into this. |
| Adversarial critic role | Devin | One of the N agents should be a dedicated reviewer using a different model family. |
| Cross-model auditing | consensus-loop | Reviewer should be a different model (GPT reviews Claude's code, etc.) to avoid shared biases. |
| Git worktree isolation | Cursor, Augment, Claude Code | Each agent gets its own worktree for parallel execution. |
| Event-sourced coordination | ESAA paper | Append-only event log as source of truth. State is a projection. |
| Blackboard for task selection | Robotics/RTS research | Shared state where agents self-select tasks based on capability. |
| Contract-first interfaces | MetaGPT | Architect defines interfaces first, agents implement against them. |
| XML tool invocation | Cline, bolt.new | Simpler than function calling for some models. Support both formats. |
| Speculative edits | Cursor | For the apply step - deterministic algorithm predicts unchanged code, 13x speedup. |
| Shadow workspace | Cursor | Hidden editor instance for LSP verification before showing edits to user. |
| Dual planning/execution split | Windsurf | Background planner continuously refines strategy while execution model does tool calls. |
| Trajectory-based intent | Windsurf | Monitor user actions to infer intent without explicit prompts. |
| Persistent REPL for testing | Replit | Variable persistence across interactions for testing. |
| Code-as-tool-invocation | Manus, Replit | Model generates code to call tools instead of JSON. More reliable for complex args. |
| Textcode (text IDE for LLMs) | Amazon Q | Token-efficient text representations of code and workspace. |
| Editable intermediate artifacts | Copilot Workspace | Plans and specs are user-facing, editable documents. |
| Incremental JSON parsing | Aha.io | Stateful parser for streaming tool call args. O(n) not O(n^2). |
| Loop detection via structure | JetBrains Koog | Change message structure, not prompt text. XML wrapping breaks pattern addiction. |
| Circuit breaker with degraded state | AWS research | Semantic failure detection, not just HTTP errors. |
| Diff-only updates | Advanced patterns | When reading same file repeatedly, send only the diff. |

## System Prompt Design

### What Works (from every major agent)

1. Be terse. Claude Code: "Go straight to the point. Be extra concise."
2. Never lie or fabricate. Every agent says this.
3. Read before modifying. Claude Code and Cursor both enforce this.
4. No premature abstractions. Claude Code: "Three similar lines of code is better than a premature abstraction."
5. No unnecessary additions. Don't add comments, docstrings, or type annotations to unchanged code.
6. No unnecessary error handling. Trust internal code, validate at boundaries only.
7. State the role explicitly. "You are X" not "You should act as X".
8. Constrain capabilities at the tool level, not just in the prompt. Remove tools agents shouldn't use.
9. One tool per message with confirmation (Cline) vs free tool calling (Claude Code). We want free for speed.
10. Anti-laziness: "You NEVER leave comments describing code without implementing it!"
11. Anti-sycophancy: "Never delegate understanding" to sub-agents.

### ntwine's Unique Prompt Needs

Our system prompt must handle something no other agent does: multiple models talking to each other in real-time. From the original Socratic Slopinar:

- Models act like developers in a Slack channel
- Ban AI-speak: no "I'd be happy to", "Great question", markdown headers in chat
- Short messages (2-4 sentences)
- Encourage disagreement
- Use shared notes for decisions (evolves into the living spec)
- Call each other out on bad code with specific line numbers
- Don't ask permission, just do it

### Multi-Agent Prompt Structure

For each agent in a discussion:

```
[SYSTEM] You are {model_name}, a developer on this team.

Your team: {list of other models with their roles}
Your role: {assigned role - architect, frontend, backend, reviewer, etc.}
The project: {codebase path, tech stack}

Rules:
- Talk like a real developer. Short messages. No AI-speak.
- Use tools when you need to read code, not from memory.
- Write to the shared spec when making decisions.
- Call out bad code with file:line references.
- Disagree when you think something is wrong.
- Don't repeat what others said. Build on it or challenge it.
- When in execution mode, edit files directly. Don't describe what you'd do.

Current plan: {contents of spec.md / todo.md}
```

The plan/spec is injected at the END of the system prompt to maximize attention (Manus's todo.md trick).

## Implementation Priority

### Already Done
- Tool registry with middleware, permissions, events
- Built-in tools (read, list, search, notes, pins, web search)
- Error-as-output pattern
- Hardcoded deny rules
- ModelDescription vs Description

### Next: Wire Into Orchestrator
- Replace AllTools/AllToolDefinitions/ExecuteTool with registry
- Pass registry through Discussion struct
- Update event broadcasting to use ToolEvent

### Then: Advanced Harness Features
- Observation masking (replace old tool outputs with [masked])
- Append-only context management
- Loop detection (track last N tool calls, detect repeats)
- Diff-only updates for repeated file reads
- Git worktree manager for parallel agent execution

### Then: Multi-Agent Coordination
- Event log (append-only JSONL)
- Blackboard state projection
- Role assignment (static + auction-based)
- Shared spec document (evolve from notes)
- Cross-model auditing

### Then: MCP Integration
- MCP client (official Go SDK)
- Tool aggregation with mcp:{server}:{tool} namespacing
- Parallel discovery with caching
- Per-agent tool filtering
