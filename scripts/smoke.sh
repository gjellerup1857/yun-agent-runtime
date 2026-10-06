#!/bin/sh
set -eu

BASE_URL="${YAR_BASE_URL:-http://localhost:8080}"
SMOKE_ID="$(date +%s)-$$"

printf '\n[1/7] health\n'
curl -fsS "$BASE_URL/healthz"
printf '\n'

printf '\n[2/7] create stateful team run\n'
RUN_RESPONSE=$(curl -fsS -X POST "$BASE_URL/v1/team/run" \
  -H 'Content-Type: application/json' \
  -d '{"project_id":"yar-smoke","message":"這個專案 Backend 預設使用 Go"}')
printf '%s\n' "$RUN_RESPONSE"

printf '\n[3/7] platform run through replay-safe gateway\n'
PLATFORM_BODY="{\"source_client\":\"web\",\"external_conversation_id\":\"smoke-conversation-$SMOKE_ID\",\"external_message_id\":\"smoke-message-$SMOKE_ID\",\"project_id\":\"yar-smoke-platform\",\"message\":\"請規劃這個跨平台任務\"}"
PLATFORM_FIRST=$(curl -fsS -X POST "$BASE_URL/v1/platform/run" \
  -H 'Content-Type: application/json' \
  -d "$PLATFORM_BODY")
printf '%s\n' "$PLATFORM_FIRST"

FIRST_TASK_ID=$(printf '%s' "$PLATFORM_FIRST" | sed -n 's/.*"task_id":"\([^"]*\)".*/\1/p')
if [ -z "$FIRST_TASK_ID" ]; then
  echo "platform run did not return task_id" >&2
  exit 1
fi
if ! printf '%s' "$PLATFORM_FIRST" | grep -q '"cached":false'; then
  echo "first platform run should not be cached" >&2
  exit 1
fi

printf '\n[4/7] replay same platform message without rerunning agents\n'
PLATFORM_SECOND=$(curl -fsS -X POST "$BASE_URL/v1/platform/run" \
  -H 'Content-Type: application/json' \
  -d "$PLATFORM_BODY")
printf '%s\n' "$PLATFORM_SECOND"

SECOND_TASK_ID=$(printf '%s' "$PLATFORM_SECOND" | sed -n 's/.*"task_id":"\([^"]*\)".*/\1/p')
if [ "$SECOND_TASK_ID" != "$FIRST_TASK_ID" ]; then
  echo "replayed platform run returned a different task_id" >&2
  exit 1
fi
if ! printf '%s' "$PLATFORM_SECOND" | grep -q '"cached":true'; then
  echo "replayed platform run was not served from cache" >&2
  exit 1
fi

printf '\n[5/7] read allowed mock tool\n'
curl -fsS -X POST "$BASE_URL/v1/dev/tools/execute" \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"product-manager","tool_id":"mock.note.read","arguments":{}}'
printf '\n'

printf '\n[6/7] request approval for write tool\n'
APPROVAL_RESPONSE=$(curl -sS -X POST "$BASE_URL/v1/dev/tools/execute" \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"backend-engineer\",\"tool_id\":\"mock.note.create\",\"idempotency_key\":\"smoke-note-$SMOKE_ID\",\"arguments\":{\"text\":\"smoke test note\"}}")
printf '%s\n' "$APPROVAL_RESPONSE"

APPROVAL_ID=$(printf '%s' "$APPROVAL_RESPONSE" | sed -n 's/.*"approval_id":"\([^"]*\)".*/\1/p')
if [ -z "$APPROVAL_ID" ]; then
  echo "approval_id not returned" >&2
  exit 1
fi

printf '\n[7/7] approve and execute write\n'
curl -fsS -X POST "$BASE_URL/v1/dev/approvals/$APPROVAL_ID/approve"
printf '\n'
curl -fsS -X POST "$BASE_URL/v1/dev/tools/execute" \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"backend-engineer\",\"tool_id\":\"mock.note.create\",\"idempotency_key\":\"smoke-note-$SMOKE_ID\",\"approval_id\":\"$APPROVAL_ID\",\"arguments\":{\"text\":\"smoke test note\"}}"
printf '\n\nSmoke test completed.\n'
