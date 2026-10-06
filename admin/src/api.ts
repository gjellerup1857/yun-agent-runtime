export type Health = {
  status: string
  database: string
  mcp: string
  providers: Record<string, boolean>
}

export type Task = {
  id: string
  team_id: string
  project_id: string
  title: string
  status: string
  current_step: string
  created_at: string
  updated_at: string
}

export type Memory = {
  id: string
  project_id: string
  agent_id: string
  task_id: string
  scope: string
  type: string
  key: string
  content: string
  importance: number
  confidence: number
  status: string
  supersedes_id: string
  created_at: string
  updated_at: string
}

export type Approval = {
  id: string
  task_id: string
  agent_id: string
  tool_id: string
  idempotency_key: string
  risk: string
  status: string
  expires_at: string
  created_at: string
  resolved_at: string
}

export type AuditEvent = {
  id: string
  task_id: string
  agent_id: string
  event_type: string
  resource_type: string
  resource_id: string
  action: string
  result: string
  payload_hash: string
  created_at: string
}

export type RunResponse = {
  run_id: string
  task_id: string
  project_id?: string
  agents_used: string[]
  answer: string
  pending_approval_id?: string
  memories_read?: number
  memories_written?: number
  results?: Array<{
    agent_id: string
    provider: string
    model: string
    output: string
  }>
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error((body as { error?: string }).error || `${response.status} ${response.statusText}`)
  }
  return body as T
}

export const api = {
  health: () => request<Health>('/healthz'),
  tasks: () => request<Task[]>('/v1/dev/tasks'),
  memories: () => request<Memory[]>('/v1/dev/memories'),
  approvals: () => request<Approval[]>('/v1/dev/approvals'),
  audit: () => request<AuditEvent[]>('/v1/dev/audit'),
  runTeam: (input: { project_id?: string; task_id?: string; message: string }) =>
    request<RunResponse>('/v1/team/run', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  approve: (id: string) =>
    request<{ approved: boolean; approval_id: string }>(`/v1/dev/approvals/${id}/approve`, {
      method: 'POST',
      body: '{}',
    }),
}
