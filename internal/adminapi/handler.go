package adminapi

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db       *pgxpool.Pool
	tenantID string
	userID   string
}

func New(db *pgxpool.Pool, tenantID, userID string) *Handler {
	return &Handler{db: db, tenantID: tenantID, userID: userID}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/dev/tasks", h.tasks)
	mux.HandleFunc("GET /v1/dev/memories", h.memories)
	mux.HandleFunc("GET /v1/dev/approvals", h.approvals)
	mux.HandleFunc("GET /v1/dev/audit", h.audit)
}

func (h *Handler) tasks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id::text,team_id,COALESCE(project_id,''),COALESCE(title,''),status,COALESCE(current_step,''),created_at,updated_at
		FROM tasks
		WHERE tenant_id=$1::uuid AND user_id=$2::uuid
		ORDER BY updated_at DESC
		LIMIT 100`, h.tenantID, h.userID)
	if err != nil { writeError(w, err); return }
	defer rows.Close()

	type item struct {
		ID string `json:"id"`
		TeamID string `json:"team_id"`
		ProjectID string `json:"project_id"`
		Title string `json:"title"`
		Status string `json:"status"`
		CurrentStep string `json:"current_step"`
		CreatedAt any `json:"created_at"`
		UpdatedAt any `json:"updated_at"`
	}
	out := make([]item, 0)
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.ID,&v.TeamID,&v.ProjectID,&v.Title,&v.Status,&v.CurrentStep,&v.CreatedAt,&v.UpdatedAt); err != nil { writeError(w, err); return }
		out = append(out, v)
	}
	if err := rows.Err(); err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) memories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id::text,COALESCE(project_id,''),COALESCE(agent_id,''),COALESCE(task_id,''),scope,memory_type,
		       COALESCE(memory_key,''),content,importance,confidence,status,COALESCE(supersedes_memory_id::text,''),created_at,updated_at
		FROM memories
		WHERE tenant_id=$1::uuid AND user_id=$2::uuid
		ORDER BY updated_at DESC
		LIMIT 200`, h.tenantID, h.userID)
	if err != nil { writeError(w, err); return }
	defer rows.Close()

	type item struct {
		ID string `json:"id"`
		ProjectID string `json:"project_id"`
		AgentID string `json:"agent_id"`
		TaskID string `json:"task_id"`
		Scope string `json:"scope"`
		Type string `json:"type"`
		Key string `json:"key"`
		Content string `json:"content"`
		Importance float64 `json:"importance"`
		Confidence float64 `json:"confidence"`
		Status string `json:"status"`
		SupersedesID string `json:"supersedes_id"`
		CreatedAt any `json:"created_at"`
		UpdatedAt any `json:"updated_at"`
	}
	out := make([]item, 0)
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.ID,&v.ProjectID,&v.AgentID,&v.TaskID,&v.Scope,&v.Type,&v.Key,&v.Content,&v.Importance,&v.Confidence,&v.Status,&v.SupersedesID,&v.CreatedAt,&v.UpdatedAt); err != nil { writeError(w, err); return }
		out = append(out, v)
	}
	if err := rows.Err(); err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) approvals(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id::text,COALESCE(task_id,''),agent_id,tool_id,idempotency_key,risk,status,expires_at,created_at,COALESCE(resolved_at,created_at)
		FROM approvals
		WHERE tenant_id=$1::uuid AND user_id=$2::uuid
		ORDER BY created_at DESC
		LIMIT 100`, h.tenantID, h.userID)
	if err != nil { writeError(w, err); return }
	defer rows.Close()

	type item struct {
		ID string `json:"id"`
		TaskID string `json:"task_id"`
		AgentID string `json:"agent_id"`
		ToolID string `json:"tool_id"`
		IdempotencyKey string `json:"idempotency_key"`
		Risk string `json:"risk"`
		Status string `json:"status"`
		ExpiresAt any `json:"expires_at"`
		CreatedAt any `json:"created_at"`
		ResolvedAt any `json:"resolved_at"`
	}
	out := make([]item, 0)
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.ID,&v.TaskID,&v.AgentID,&v.ToolID,&v.IdempotencyKey,&v.Risk,&v.Status,&v.ExpiresAt,&v.CreatedAt,&v.ResolvedAt); err != nil { writeError(w, err); return }
		out = append(out, v)
	}
	if err := rows.Err(); err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) audit(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id::text,COALESCE(task_id,''),COALESCE(agent_id,''),event_type,COALESCE(resource_type,''),COALESCE(resource_id,''),
		       COALESCE(action,''),COALESCE(result,''),COALESCE(payload_hash,''),created_at
		FROM audit_logs
		WHERE tenant_id=$1::uuid AND (user_id=$2::uuid OR user_id IS NULL)
		ORDER BY created_at DESC
		LIMIT 200`, h.tenantID, h.userID)
	if err != nil { writeError(w, err); return }
	defer rows.Close()

	type item struct {
		ID string `json:"id"`
		TaskID string `json:"task_id"`
		AgentID string `json:"agent_id"`
		EventType string `json:"event_type"`
		ResourceType string `json:"resource_type"`
		ResourceID string `json:"resource_id"`
		Action string `json:"action"`
		Result string `json:"result"`
		PayloadHash string `json:"payload_hash"`
		CreatedAt any `json:"created_at"`
	}
	out := make([]item, 0)
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.ID,&v.TaskID,&v.AgentID,&v.EventType,&v.ResourceType,&v.ResourceID,&v.Action,&v.Result,&v.PayloadHash,&v.CreatedAt); err != nil { writeError(w, err); return }
		out = append(out, v)
	}
	if err := rows.Err(); err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, out)
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
