# Yun Agent Runtime (YAR)

Portable, governed multi-agent runtime for running the same AI team across model providers and MCP-compatible clients.

> Status: MVP implementation in progress on `build/yar-mvp`.

## Core principles

- Agent != LLM
- Tool != Tool Provider
- YAR memory is the canonical source of truth
- Default-deny tool permissions
- Human approval for write/high-risk actions
- Model-provider independence with fallback
- Modular monolith first

## What already works in the MVP branch

- Go HTTP runtime
- `GET /healthz`
- `POST /v1/team/run`
- deterministic 9-agent routing
- zero-cost Mock provider
- PostgreSQL + pgvector schema
- persistent tasks
- project/user/agent/task memory scopes
- decision supersession (`Go -> Rust` keeps history)
- governed tool registry
- default-deny policy engine
- human approval records
- idempotent write-tool execution
- audit logging
- Docker Compose local stack
- GitHub Actions CI

Remote MCP, real model providers and the Preact admin console are still being added.

## Requirements

- Docker + Docker Compose
- Optional: Go 1.25+ for running outside Docker

No OpenAI, Anthropic or Gemini API key is required for the current MVP because the Mock provider is always available.

## Quick start

```bash
git clone https://github.com/gjellerup1857/yun-agent-runtime.git
cd yun-agent-runtime
git checkout build/yar-mvp
make up
```

Then verify:

```bash
curl http://localhost:8080/healthz
```

Expected shape:

```json
{
  "status": "ok",
  "database": "ok",
  "provider": "mock"
}
```

Run the whole MVP smoke flow:

```bash
make smoke
```

## Stateful team run

```bash
curl -X POST http://localhost:8080/v1/team/run \
  -H 'Content-Type: application/json' \
  -d '{
    "project_id": "travel-ai",
    "message": "這個專案 Backend 預設使用 Go"
  }'
```

The response returns a `task_id`. Reuse that ID to continue the same task:

```bash
curl -X POST http://localhost:8080/v1/team/run \
  -H 'Content-Type: application/json' \
  -d '{
    "project_id": "travel-ai",
    "task_id": "<TASK_ID>",
    "message": "繼續這個 Task"
  }'
```

If a later request says `Backend 改成 Rust`, the previous `backend.default_language` memory is marked `superseded` and the Rust decision becomes active.

## Governed tool flow

Read-only tool:

```bash
curl -X POST http://localhost:8080/v1/dev/tools/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "product-manager",
    "tool_id": "mock.note.read",
    "arguments": {}
  }'
```

Write tool first returns `APPROVAL_REQUIRED`:

```bash
curl -X POST http://localhost:8080/v1/dev/tools/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "backend-engineer",
    "tool_id": "mock.note.create",
    "idempotency_key": "demo-note-001",
    "arguments": {"text": "Backend uses Go"}
  }'
```

Approve it:

```bash
curl -X POST http://localhost:8080/v1/dev/approvals/<APPROVAL_ID>/approve
```

Then repeat the write request with the same `idempotency_key` plus `approval_id`.

Repeating the exact approved write again returns the cached execution result instead of performing the write twice.

## Development commands

```bash
make fmt
make vet
make test
make build
make ci
make up
make down
make logs
make smoke
```

## Current architecture

```text
Client
  -> YAR HTTP Gateway
  -> Agent Router
  -> Stateful Runtime
       -> PostgreSQL / pgvector memory
       -> Task continuation
       -> Mock model provider
  -> Tool Control Plane
       -> Registry
       -> Schema validation
       -> Default-deny policy
       -> Approval
       -> Idempotency
       -> Audit
       -> Backend
```

## Next implementation targets

1. Provider Registry + OpenAI / Anthropic / Gemini adapters
2. Agent tool-calling loop
3. Remote MCP server
4. OAuth canonical identity
5. Preact admin console
6. Cross-platform ChatGPT / Claude / Gemini E2E

Development work is reviewed in PRs before merging to `main`.
