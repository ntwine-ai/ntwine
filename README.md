# Socratic Slopinar

A local web app where multiple AI models hold a roundtable discussion about your prompt over your codebase. They collaborate on shared notes and produce an execution prompt you can paste into any model to actually do the work.

## Build

Prerequisites: Go 1.24+, Node.js 18+

```sh
cd frontend && npm install && npm run build && cd ..
go build -o dist/socratic-slopinar ./cmd/server
./dist/socratic-slopinar
```

Opens at http://localhost:8080

## Configuration

Click the gear icon in the UI to configure:

- **OpenRouter API key** -- required, all model calls go through OpenRouter
- **Tavily API key** -- optional, enables web search during discussions
- **Models** -- add models by pasting OpenRouter model IDs (e.g. `anthropic/claude-sonnet-4-6`)

## How it works

Models discuss your prompt in rounds. Each model sees the full conversation history and has access to tools: reading files from your codebase, searching code, and web search.

Throughout the discussion, models collaborate on a shared notes document (markdown) to track findings, decisions, and open questions.

When the discussion wraps up, an execution prompt is generated -- a self-contained instruction you can copy and paste into any model to carry out the plan.
