package memory

import (
	"context"
	"time"
)

type Type string

const (
	TypeSemantic   Type = "semantic"
	TypeEpisodic   Type = "episodic"
	TypeDecision   Type = "decision"
	TypePreference Type = "preference"
	TypeProcedural Type = "procedural"
	TypeArtifact   Type = "artifact"
)

type Memory struct {
	ID string
	TenantID string
	UserID string
	TeamID string
	ProjectID string
	AgentID string
	TaskID string
	Scope string
	Type Type
	Key string
	Content string
	Importance float64
	Confidence float64
	TrustScore float64
	SourceType string
	Status string
	SupersedesID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Query struct {
	TenantID string
	UserID string
	TeamID string
	ProjectID string
	AgentID string
	TaskID string
	MinImportance float64
	Limit int
}

type Repository interface {
	Relevant(ctx context.Context, query Query) ([]Memory, error)
	Save(ctx context.Context, m Memory) (Memory, error)
	SaveDecision(ctx context.Context, m Memory) (Memory, error)
}
