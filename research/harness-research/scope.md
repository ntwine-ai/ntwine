# Harness Research - Scope

## Sub-Questions

### Fundamentals
1. What makes a great AI agent tool execution harness? What are the core primitives?
2. What is the minimal viable harness architecture for multi-model collaborative coding?

### System Prompts
3. What system prompts do the best coding agents use? (Claude Code, Cursor, Windsurf, Aider, Cline)
4. How do multi-agent systems prompt models to collaborate vs compete?
5. What prompt patterns make models better at using tools?
6. How do you prompt models to have distinct personalities while coding together?

### Harnesses We Haven't Covered
7. What harness patterns does Cursor use internally?
8. What does Windsurf/Codeium's tool execution look like?
9. How does Copilot Workspace handle multi-step tool execution?
10. What does Amazon Q Developer's agent harness look like?
11. How does Devin (Cognition) orchestrate its agent loop?
12. What does Replit Agent's harness look like?
13. How does bolt.new/Stackblitz handle tool execution?

### Advanced Patterns
14. How do the best harnesses handle tool call streaming (partial input)?
15. What are the best patterns for tool result truncation and summarization?
16. How do harnesses handle tool call loops (agent calling same tool repeatedly)?
17. What retry and fallback strategies work best for failed tool calls?
18. How do harnesses manage context window budget across multiple tool results?

### Multi-Agent Specific
19. How should multiple models share tool state (file locks, edit conflicts)?
20. What communication protocols work best for inter-model messaging?
21. How do you prevent models from undoing each other's work?
22. What role assignment strategies produce the best collaborative output?

### Security & Permissions
23. What are the best permission models for AI tool execution?
24. How do you sandbox tool execution safely across platforms?
25. What are the most common security vulnerabilities in agent harnesses?
