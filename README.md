# Yun Agent Runtime (YAR)

Portable, governed multi-agent runtime for running the same AI team across model providers and MCP-compatible clients.

> Status: early MVP implementation.

## Core principles

- Agent != LLM
- Tool != Tool Provider
- YAR memory is the canonical source of truth
- Default-deny tool permissions
- Human approval for write/high-risk actions
- Model-provider independence with fallback
- Modular monolith first

## Planned MVP

- Go runtime and HTTP API
- PostgreSQL + pgvector memory
- Persistent task continuation
- 9-agent product engineering team
- Model router with Mock/OpenAI/Anthropic/Gemini providers
- Governed tool runtime with approval, idempotency and audit
- Remote MCP surface
- Docker Compose local deployment
- Preact admin console

Development work happens on feature branches and is merged to `main` through pull requests.
