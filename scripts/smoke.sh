#!/bin/sh
set -eu

BASE_URL="${YAR_BASE_URL:-http://localhost:8080}"

printf '\n[1/5] health\n'
curl -fsS "$BASE_URL/healthz"
printf '\n'

printf '\n[2/5] create stateful team run\n'
RUN_RESPONSE=$(curl -fsS -X POST "$BASE_URL/v1/team/run" \
  -H 'Content-Type: application/json' \
  -d '{"project_id":"yar-smoke","message":"這個專案 Backend 預設使用 Go"}')
printf '%s\n' "$RUN_RESPONSE"

printf '\n[3/5] read allowed mock tool\n'
curl -fsS -X POST "$BASE_URL/v1/dev/tools/execute" \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"product-manager","tool_id":"mock.note.read","arguments":{}}'
printf '\n'

printf '\n[4/5] request approval for write tool\n'
APPROVAL_RESPONSE=$(curl -sS -X POST "$BASE_URL/v1/dev/tools/execute" \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"backend-engineer","tool_id":"mock.note.create","idempotency_key":"smoke-note-001","arguments":{"text":"smoke test note"}}')
printf '%s\n' "$APPROVAL_RESPONSE"

APPROVAL_ID=$(printf '%s' "$APPROVAL_RESPONSE" | sed -n 's/.*"approval_id":"\([^"]*\)".*/\1/p')
if [ -z "$APPROVAL_ID" ]; then
  echo "approval_id not returned" >&2
  exit 1
fi

printf '\n[5/5] approve and execute write\n'
curl -fsS -X POST "$BASE_URL/v1/dev/approvals/$APPROVAL_ID/approve"
printf '\n'
curl -fsS -X POST "$BASE_URL/v1/dev/tools/execute" \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"backend-engineer\",\"tool_id\":\"mock.note.create\",\"idempotency_key\":\"smoke-note-001\",\"approval_id\":\"$APPROVAL_ID\",\"arguments\":{\"text\":\"smoke test note\"}}"
printf '\n\nSmoke test completed.\n'
