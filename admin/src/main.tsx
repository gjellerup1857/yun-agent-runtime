import { render } from 'preact'
import { useCallback, useEffect, useMemo, useState } from 'preact/hooks'
import { api, type Approval, type AuditEvent, type Health, type Memory, type RunResponse, type Task } from './api'
import './styles.css'

type Page = 'overview' | 'run' | 'tasks' | 'memory' | 'approvals' | 'audit'

const nav: Array<{ id: Page; label: string; icon: string }> = [
  { id: 'overview', label: 'Overview', icon: '◈' },
  { id: 'run', label: 'Run Team', icon: '▶' },
  { id: 'tasks', label: 'Tasks', icon: '✓' },
  { id: 'memory', label: 'Memory', icon: '◇' },
  { id: 'approvals', label: 'Approvals', icon: '!' },
  { id: 'audit', label: 'Audit', icon: '≡' },
]

function App() {
  const [page, setPage] = useState<Page>('overview')
  const [health, setHealth] = useState<Health | null>(null)
  const [tasks, setTasks] = useState<Task[]>([])
  const [memories, setMemories] = useState<Memory[]>([])
  const [approvals, setApprovals] = useState<Approval[]>([])
  const [audit, setAudit] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const refresh = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [nextHealth, nextTasks, nextMemories, nextApprovals, nextAudit] = await Promise.all([
        api.health(), api.tasks(), api.memories(), api.approvals(), api.audit(),
      ])
      setHealth(nextHealth)
      setTasks(nextTasks)
      setMemories(nextMemories)
      setApprovals(nextApprovals)
      setAudit(nextAudit)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  const pendingApprovals = useMemo(() => approvals.filter(item => item.status === 'pending'), [approvals])
  const activeTasks = useMemo(() => tasks.filter(item => !['completed', 'cancelled'].includes(item.status)), [tasks])
  const activeMemories = useMemo(() => memories.filter(item => item.status === 'active'), [memories])

  return (
    <div class="app-shell">
      <aside class="sidebar">
        <div class="brand">
          <div class="brand-mark">Y</div>
          <div>
            <strong>YAR</strong>
            <span>Agent Runtime</span>
          </div>
        </div>
        <nav>
          {nav.map(item => (
            <button class={page === item.id ? 'nav-item active' : 'nav-item'} onClick={() => setPage(item.id)}>
              <span>{item.icon}</span>{item.label}
              {item.id === 'approvals' && pendingApprovals.length > 0 && <b class="nav-badge">{pendingApprovals.length}</b>}
            </button>
          ))}
        </nav>
        <div class="sidebar-foot">
          <span class={health?.status === 'ok' ? 'dot ok' : 'dot'} />
          <div><strong>{health?.status === 'ok' ? 'Runtime online' : 'Connecting…'}</strong><small>{health?.mcp ?? '/mcp'}</small></div>
        </div>
      </aside>

      <main class="main">
        <header class="topbar">
          <div>
            <p class="eyebrow">Yun Product Engineering Team</p>
            <h1>{nav.find(item => item.id === page)?.label}</h1>
          </div>
          <button class="secondary" disabled={loading} onClick={() => void refresh()}>{loading ? 'Refreshing…' : 'Refresh'}</button>
        </header>

        {error && <div class="error-banner">{error}</div>}

        {page === 'overview' && <Overview health={health} tasks={tasks} memories={memories} approvals={pendingApprovals} audit={audit} />}
        {page === 'run' && <RunTeam onCompleted={refresh} />}
        {page === 'tasks' && <Tasks items={tasks} />}
        {page === 'memory' && <Memories items={memories} />}
        {page === 'approvals' && <Approvals items={approvals} onChanged={refresh} />}
        {page === 'audit' && <Audit items={audit} />}
      </main>
    </div>
  )
}

function Overview({ health, tasks, memories, approvals, audit }: {
  health: Health | null; tasks: Task[]; memories: Memory[]; approvals: Approval[]; audit: AuditEvent[]
}) {
  const active = tasks.filter(item => !['completed', 'cancelled'].includes(item.status)).length
  const activeMemory = memories.filter(item => item.status === 'active').length
  return (
    <div class="stack">
      <section class="metrics">
        <Metric label="Runtime" value={health?.status === 'ok' ? 'Online' : 'Unknown'} detail="Go + PostgreSQL" />
        <Metric label="Active tasks" value={String(active)} detail={`${tasks.length} total`} />
        <Metric label="Active memories" value={String(activeMemory)} detail={`${memories.length} historical`} />
        <Metric label="Pending approvals" value={String(approvals.length)} detail="Human-in-the-loop" />
      </section>

      <section class="grid-two">
        <article class="panel">
          <PanelTitle title="Model providers" subtitle="Registered at runtime" />
          <div class="provider-list">
            {Object.entries(health?.providers ?? {}).map(([name, enabled]) => (
              <div class="provider-row"><span class={enabled ? 'dot ok' : 'dot'} /><strong>{name}</strong><span>{enabled ? 'Ready' : 'Not configured'}</span></div>
            ))}
          </div>
        </article>
        <article class="panel">
          <PanelTitle title="Team" subtitle="9 permanent agents · dynamic routing" />
          <div class="agent-cloud">
            {['Coordinator','Product Manager','Technical PM','Architect','Frontend','Full-stack','Backend','SDET','AI Engineer'].map(agent => <span>{agent}</span>)}
          </div>
        </article>
      </section>

      <article class="panel">
        <PanelTitle title="Recent activity" subtitle={`${audit.length} audit events loaded`} />
        <DataTable headers={['Agent','Event','Resource','Result','Time']} rows={audit.slice(0, 8).map(item => [item.agent_id || 'system', item.event_type, item.resource_id || '—', item.result || '—', date(item.created_at)])} />
      </article>
    </div>
  )
}

function RunTeam({ onCompleted }: { onCompleted: () => Promise<void> }) {
  const [projectID, setProjectID] = useState('yar')
  const [taskID, setTaskID] = useState('')
  const [message, setMessage] = useState('請分析目前 YAR 的後端架構與下一個最重要的工程工作。')
  const [result, setResult] = useState<RunResponse | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: Event) {
    event.preventDefault()
    if (!message.trim()) return
    setBusy(true)
    setError('')
    try {
      const response = await api.runTeam({ project_id: projectID || undefined, task_id: taskID || undefined, message })
      setResult(response)
      if (!taskID && response.task_id) setTaskID(response.task_id)
      await onCompleted()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div class="grid-run">
      <form class="panel run-form" onSubmit={submit}>
        <PanelTitle title="Run the team" subtitle="Router selects agents and models automatically" />
        <label>Project ID<input value={projectID} onInput={e => setProjectID((e.currentTarget as HTMLInputElement).value)} placeholder="travel-ai" /></label>
        <label>Task ID <small>Optional — reuse to continue a task</small><input value={taskID} onInput={e => setTaskID((e.currentTarget as HTMLInputElement).value)} placeholder="Leave empty for a new task" /></label>
        <label>Message<textarea rows={9} value={message} onInput={e => setMessage((e.currentTarget as HTMLTextAreaElement).value)} /></label>
        {error && <div class="inline-error">{error}</div>}
        <button class="primary" disabled={busy}>{busy ? 'Running agents…' : 'Run YAR Team'}</button>
      </form>

      <article class="panel result-panel">
        <PanelTitle title="Result" subtitle={result ? `Run ${short(result.run_id)} · Task ${short(result.task_id)}` : 'Waiting for a run'} />
        {!result && <Empty text="Run the team to see routed agents, models, memory usage and the synthesized answer." />}
        {result && <>
          <div class="chips">{result.agents_used?.map(agent => <span>{agent}</span>)}</div>
          <div class="run-meta"><span>Memory read <b>{result.memories_read ?? 0}</b></span><span>Memory written <b>{result.memories_written ?? 0}</b></span></div>
          {result.pending_approval_id && <div class="approval-callout">Waiting for approval <code>{result.pending_approval_id}</code></div>}
          <pre class="answer">{result.answer}</pre>
          {result.results?.map(agent => <details><summary>{agent.agent_id} · {agent.provider}/{agent.model}</summary><pre>{agent.output}</pre></details>)}
        </>}
      </article>
    </div>
  )
}

function Tasks({ items }: { items: Task[] }) {
  return <article class="panel"><PanelTitle title="Tasks" subtitle="Persistent work can resume across clients" /><DataTable headers={['Status','Project','Title','Task ID','Updated']} rows={items.map(item => [<Status value={item.status} />, item.project_id || '—', item.title || 'Untitled', <code>{short(item.id)}</code>, date(item.updated_at)])} /></article>
}

function Memories({ items }: { items: Memory[] }) {
  const [filter, setFilter] = useState('active')
  const visible = filter === 'all' ? items : items.filter(item => item.status === filter)
  return <div class="stack">
    <div class="toolbar"><button class={filter === 'active' ? 'filter active' : 'filter'} onClick={() => setFilter('active')}>Active</button><button class={filter === 'superseded' ? 'filter active' : 'filter'} onClick={() => setFilter('superseded')}>Superseded</button><button class={filter === 'all' ? 'filter active' : 'filter'} onClick={() => setFilter('all')}>All</button></div>
    <article class="panel"><PanelTitle title="Canonical memory" subtitle="PostgreSQL is the source of truth" /><DataTable headers={['Status','Type / Scope','Key','Content','Project','Updated']} rows={visible.map(item => [<Status value={item.status} />, `${item.type} · ${item.scope}`, <code>{item.key || '—'}</code>, item.content, item.project_id || '—', date(item.updated_at)])} /></article>
  </div>
}

function Approvals({ items, onChanged }: { items: Approval[]; onChanged: () => Promise<void> }) {
  const [busy, setBusy] = useState('')
  async function approve(id: string) {
    setBusy(id)
    try { await api.approve(id); await onChanged() } finally { setBusy('') }
  }
  return <article class="panel"><PanelTitle title="Approvals" subtitle="Write/high-risk actions stay human-governed" /><div class="approval-list">
    {items.length === 0 && <Empty text="No approval requests yet." />}
    {items.map(item => <div class="approval-card"><div><div class="chips"><Status value={item.status} /><span>{item.risk} risk</span></div><h3>{item.tool_id}</h3><p>{item.agent_id} · Task {short(item.task_id)}</p><small>{date(item.created_at)}</small></div>{item.status === 'pending' && <button class="primary small" disabled={busy === item.id} onClick={() => void approve(item.id)}>{busy === item.id ? 'Approving…' : 'Approve'}</button>}</div>)}
  </div></article>
}

function Audit({ items }: { items: AuditEvent[] }) {
  return <article class="panel"><PanelTitle title="Audit log" subtitle="Arguments are represented by hashes, not plaintext" /><DataTable headers={['Agent','Event','Tool / Resource','Action','Result','Payload hash','Time']} rows={items.map(item => [item.agent_id || 'system', item.event_type, item.resource_id || '—', item.action || '—', <Status value={item.result || 'unknown'} />, <code>{short(item.payload_hash, 10)}</code>, date(item.created_at)])} /></article>
}

function Metric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return <article class="metric"><span>{label}</span><strong>{value}</strong><small>{detail}</small></article>
}

function PanelTitle({ title, subtitle }: { title: string; subtitle: string }) {
  return <div class="panel-title"><div><h2>{title}</h2><p>{subtitle}</p></div></div>
}

function Status({ value }: { value: string }) {
  const tone = ['active','running','approved','completed','success','ok'].includes(value) ? 'good' : ['pending','waiting_approval','processing'].includes(value) ? 'warn' : ['failed','rejected','blocked','denied'].includes(value) ? 'bad' : 'neutral'
  return <span class={`status ${tone}`}>{value}</span>
}

function DataTable({ headers, rows }: { headers: string[]; rows: Array<Array<unknown>> }) {
  if (rows.length === 0) return <Empty text="No data yet." />
  return <div class="table-wrap"><table><thead><tr>{headers.map(header => <th>{header}</th>)}</tr></thead><tbody>{rows.map(row => <tr>{row.map(cell => <td>{cell as any}</td>)}</tr>)}</tbody></table></div>
}

function Empty({ text }: { text: string }) { return <div class="empty">{text}</div> }
function short(value?: string, length = 8) { return value ? value.slice(0, length) : '—' }
function date(value?: string) { return value ? new Date(value).toLocaleString() : '—' }

render(<App />, document.getElementById('app')!)
