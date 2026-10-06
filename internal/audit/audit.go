package audit

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	TenantID string
	UserID string
	TaskID string
	AgentID string
	EventType string
	ResourceType string
	ResourceID string
	Action string
	Result string
	PayloadHash string
}

type Repository interface {
	Record(ctx context.Context, event Event) error
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) Record(ctx context.Context, e Event) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_logs (
			tenant_id,user_id,task_id,agent_id,event_type,resource_type,resource_id,action,result,payload_hash
		) VALUES (
			$1::uuid,NULLIF($2,'')::uuid,NULLIF($3,''),NULLIF($4,''),$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,'')
		)`, e.TenantID,e.UserID,e.TaskID,e.AgentID,e.EventType,e.ResourceType,e.ResourceID,e.Action,e.Result,e.PayloadHash)
	return err
}

var _ Repository = (*PostgresRepository)(nil)
