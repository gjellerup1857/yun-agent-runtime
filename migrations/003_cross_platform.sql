CREATE TABLE IF NOT EXISTS platform_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_client TEXT NOT NULL,
    external_conversation_id TEXT NOT NULL,
    team_id TEXT NOT NULL,
    project_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, user_id, source_client, external_conversation_id)
);

CREATE INDEX IF NOT EXISTS idx_platform_conversations_user
ON platform_conversations(tenant_id, user_id, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS conversation_tasks (
    conversation_id UUID NOT NULL REFERENCES platform_conversations(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    attached_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, task_id)
);

CREATE INDEX IF NOT EXISTS idx_conversation_tasks_task
ON conversation_tasks(task_id);

CREATE TABLE IF NOT EXISTS platform_message_receipts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_client TEXT NOT NULL,
    external_message_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('processing', 'completed', 'failed')),
    attempt_count INTEGER NOT NULL DEFAULT 1 CHECK (attempt_count > 0),
    lease_expires_at TIMESTAMPTZ,
    response JSONB,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, user_id, source_client, external_message_id)
);

CREATE INDEX IF NOT EXISTS idx_platform_message_receipts_lease
ON platform_message_receipts(status, lease_expires_at)
WHERE status = 'processing';
