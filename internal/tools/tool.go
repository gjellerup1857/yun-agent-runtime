package tools

import (
	"context"
	"encoding/json"
	"sync"
)

type Operation string

type Risk string

const (
	OperationRead Operation = "read"
	OperationWrite Operation = "write"
	OperationExecute Operation = "execute"
	OperationDestructive Operation = "destructive"

	RiskLow Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh Risk = "high"
	RiskCritical Risk = "critical"
)

type Tool struct {
	ID string
	Description string
	Operation Operation
	Risk Risk
	Backend string
	InputSchema json.RawMessage
}

type Invocation struct {
	TenantID string
	UserID string
	TaskID string
	AgentID string
	ToolID string
	Arguments json.RawMessage
	IdempotencyKey string
	ApprovalID string
}

type Result struct {
	Content string `json:"content,omitempty"`
	Structured json.RawMessage `json:"structured,omitempty"`
}

type Backend interface {
	Name() string
	Execute(ctx context.Context, tool Tool, arguments json.RawMessage) (Result, error)
}

type Registry struct {
	mu sync.RWMutex
	items map[string]Tool
}

func NewRegistry() *Registry { return &Registry{items: map[string]Tool{}} }
func (r *Registry) Register(tool Tool) { r.mu.Lock(); defer r.mu.Unlock(); r.items[tool.ID] = tool }
func (r *Registry) Get(id string) (Tool, bool) { r.mu.RLock(); defer r.mu.RUnlock(); t, ok := r.items[id]; return t, ok }
func (r *Registry) List() []Tool {
	r.mu.RLock(); defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.items))
	for _, t := range r.items { out = append(out, t) }
	return out
}
