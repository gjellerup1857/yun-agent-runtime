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
- stateless Streamable HTTP MCP endpoint at `POST /mcp`
- MCP tools: `yar_profile_get`, `yar_team_run`
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

Real model providers, OAuth canonical identity and the Preact admin console are still being added.

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
  "provider": "mock",
  "mcp": "/mcp"
}
```

Run the whole MVP smoke flow:

```bash
make smoke
```

## Remote MCP

YAR exposes the official Model Context Protocol Streamable HTTP transport at:

```text
http://localhost:8080/mcp
```

The server uses the official `github.com/modelcontextprotocol/go-sdk` and runs the HTTP transport in stateless mode for the 2026-07-28 protocol revision.

Current MCP tools:

- `yar_profile_get` — returns the canonical YAR identity connected to the endpoint.
- `yar_team_run` — runs or resumes the persistent YAR product engineering team.

The MVP currently binds `/mcp` to the development identity. Production OAuth/OIDC identity resolution is intentionally not enabled yet.

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
MCP Client / REST Client
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
2. Agent model tool-calling loop
3. OAuth/OIDC canonical identity
4. Preact admin console
5. Cross-platform ChatGPT / Claude / Gemini E2E

Development work is reviewed in PRs before merging to `main`.
