# Yun Agent Runtime (YAR)

Portable, governed multi-agent runtime for running the same AI team across model providers and MCP-compatible clients.

> Status: interactive MVP is Docker/E2E verified on `build/yar-mvp`.

## Core principles

- Agent != LLM
- Tool != Tool Provider
- YAR memory is the canonical source of truth
- Default-deny tool permissions
- Human approval for write/high-risk actions
- Model-provider independence with retry/failover
- Modular monolith first

## Verified MVP capabilities

- Go HTTP runtime
- Preact/Vite admin console at `http://localhost:3000`
- `GET /healthz`
- `POST /v1/team/run`
- stateless Streamable HTTP MCP endpoint at `/mcp`
- MCP tools: `yar_profile_get`, `yar_team_run`
- deterministic 9-agent routing
- agent-aware model routing
- optional OpenAI Responses provider (`store:false`)
- optional Anthropic Messages provider
- optional Gemini Interactions provider (`store:false`)
- zero-cost Mock provider fallback
- retry/failover for retryable provider failures
- PostgreSQL + pgvector schema
- persistent tasks
- project/user/agent/task memory scopes
- decision supersession (`Go -> Rust` keeps history)
- governed tool registry
- policy-filtered tool discovery
- Agentic tool loop with hard call/round budgets
- default-deny policy engine
- human approval records and approval resume
- idempotent write-tool execution
- audit logging with argument hashes instead of plaintext payloads
- Docker Compose local stack
- GitHub Actions Go/Admin builds
- Docker end-to-end CI including health, admin and smoke flow

Production OAuth/OIDC identity, provider-native tool calling for every real provider, portable `.yar` packages, and public/cloud deployment remain production-hardening work.

## Requirements

- Docker + Docker Compose
- Optional: Go 1.25+ and Node 22+ for running components outside Docker

No OpenAI, Anthropic or Gemini API key is required. The Mock provider is always available.

## Quick start

```bash
git clone https://github.com/gjellerup1857/yun-agent-runtime.git
cd yun-agent-runtime
git checkout build/yar-mvp
make up
```

Open the interactive admin console:

```text
http://localhost:3000
```

YAR API/MCP:

```text
http://localhost:8080
http://localhost:8080/mcp
```

Verify health:

```bash
curl http://localhost:8080/healthz
```

Expected shape without paid provider credentials:

```json
{
  "status": "ok",
  "database": "ok",
  "mcp": "/mcp",
  "providers": {
    "mock": true,
    "openai": false,
    "anthropic": false,
    "google": false
  }
}
```

Run the whole MVP smoke flow:

```bash
make smoke
```

## Admin console

The Preact admin currently includes:

- Overview — runtime/provider/task/memory/approval status
- Run Team — start or continue a persistent task
- Tasks — inspect persistent tasks and statuses
- Memory — inspect active/superseded canonical memory
- Approvals — approve pending write-tool operations
- Audit — inspect governed tool activity and payload hashes

The production container serves the Preact bundle through Nginx. `/api/*` is reverse-proxied to the YAR Go service, so the browser does not need cross-origin configuration.

## Optional real model providers

Copy or export values from `.env.example` before `make up`:

```bash
export OPENAI_API_KEY='...'
export OPENAI_MODEL='...'

export ANTHROPIC_API_KEY='...'
export ANTHROPIC_MODEL='...'

export GEMINI_API_KEY='...'
export GEMINI_MODEL='...'
```

Only providers that have both a key and model configured are registered. Mock remains available as the final fallback.

## Remote MCP

YAR exposes the official Model Context Protocol Streamable HTTP transport at:

```text
http://localhost:8080/mcp
```

The server uses `github.com/modelcontextprotocol/go-sdk` and stateless Streamable HTTP.

Current MCP tools:

- `yar_profile_get` — returns the canonical YAR identity connected to the endpoint.
- `yar_team_run` — runs or resumes the persistent YAR product engineering team.

The MVP currently binds `/mcp` to the development identity. Production OAuth/OIDC subject resolution is intentionally still pending.

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

Approve it from the Admin console or:

```bash
curl -X POST http://localhost:8080/v1/dev/approvals/<APPROVAL_ID>/approve
```

Repeating an already completed write with the same idempotency key returns the cached result rather than performing the side effect twice.

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
ChatGPT / Claude / Gemini / MCP Client / Admin
                 |
                 v
          YAR HTTP + MCP Gateway
                 |
        +--------+---------+
        |                  |
        v                  v
   Agent Runtime       Tool Control Plane
        |                  |
   Agent Router          Registry
   Model Router          Schema
   Memory Context        Default Deny
   Task Continuation     Approval
        |                Idempotency
        v                Audit
 OpenAI / Anthropic         |
 Gemini / Mock              v
                         Backends
        |
        v
 PostgreSQL + pgvector
```

## CI definition of the current MVP

Every branch/PR run verifies:

1. `go mod tidy`
2. `gofmt`
3. `go vet ./...`
4. `go test ./...`
5. `go build ./cmd/server`
6. Preact TypeScript/Vite production build
7. Docker Compose build/start
8. PostgreSQL migrations
9. YAR health endpoint
10. Admin HTTP endpoint
11. Stateful team run
12. Governed read tool
13. Approval request + approval
14. Approved write tool execution
15. Clean stack shutdown

## Next production targets

1. OAuth/OIDC canonical identity for Remote MCP and REST
2. Provider-native tool/function calling for OpenAI, Anthropic and Gemini
3. MCP connector backends for GitHub/ClickUp/Figma/etc.
4. `.yar` team package build/import/signature verification
5. encrypted `.yarmem` export/import
6. public/cloud deployment and cross-platform ChatGPT/Claude/Gemini E2E

Development work is reviewed in PRs before merging to `main`.
