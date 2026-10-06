CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    display_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id TEXT,
    project_id TEXT,
    agent_id TEXT,
    task_id TEXT,
    scope TEXT NOT NULL,
    memory_type TEXT NOT NULL,
    memory_key TEXT,
    content TEXT NOT NULL,
    importance DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    trust_score DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    source_type TEXT NOT NULL DEFAULT 'user',
    status TEXT NOT NULL DEFAULT 'active',
    supersedes_memory_id UUID REFERENCES memories(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_memories_scope
ON memories(tenant_id, user_id, scope, status);

CREATE INDEX IF NOT EXISTS idx_memories_key
ON memories(tenant_id, user_id, memory_key)
WHERE status = 'active';

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id TEXT NOT NULL,
    project_id TEXT,
    title TEXT,
    status TEXT NOT NULL CHECK (
        status IN ('pending', 'running', 'waiting_approval', 'blocked', 'completed', 'cancelled')
    ),
    current_step TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tasks_user
ON tasks(tenant_id, user_id, status);

INSERT INTO tenants (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'YAR Development')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, tenant_id, display_name)
VALUES (
    '00000000-0000-0000-0000-000000000101',
    '00000000-0000-0000-0000-000000000001',
    'YAR Developer'
)
ON CONFLICT (id) DO NOTHING;
