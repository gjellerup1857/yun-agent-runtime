package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutionStatus string

const (
	ExecutionProcessing ExecutionStatus = "processing"
	ExecutionCompleted ExecutionStatus = "completed"
	ExecutionFailed ExecutionStatus = "failed"
)

type Execution struct {
	ID string
	TenantID string
	UserID string
	TaskID string
	AgentID string
	ToolID string
	IdempotencyKey string
	Status ExecutionStatus
	Result json.RawMessage
	ErrorCode string
	LeaseExpiresAt time.Time
}

type ExecutionRepository interface {
	Acquire(ctx context.Context, inv Invocation, lease time.Duration) (Execution, bool, error)
	Complete(ctx context.Context, tenantID, idempotencyKey string, result json.RawMessage) error
	Fail(ctx context.Context, tenantID, idempotencyKey, errorCode string) error
}

type PostgresExecutionRepository struct{ db *pgxpool.Pool }

func NewPostgresExecutionRepository(db *pgxpool.Pool) *PostgresExecutionRepository {
	return &PostgresExecutionRepository{db: db}
}

func (r *PostgresExecutionRepository) Acquire(ctx context.Context, inv Invocation, lease time.Duration) (Execution, bool, error) {
	leaseExpires := time.Now().UTC().Add(lease)
	var e Execution
	var status string
	err := r.db.QueryRow(ctx, `
		INSERT INTO tool_executions (
			tenant_id,user_id,task_id,agent_id,tool_id,idempotency_key,status,lease_expires_at
		) VALUES (
			$1::uuid,$2::uuid,NULLIF($3,''),$4,$5,$6,'processing',$7
		)
		ON CONFLICT (tenant_id,idempotency_key)
		DO UPDATE SET
			status=CASE
				WHEN tool_executions.status='failed' THEN 'processing'
				WHEN tool_executions.status='processing' AND tool_executions.lease_expires_at < now() THEN 'processing'
				ELSE tool_executions.status END,
			lease_expires_at=CASE
				WHEN tool_executions.status='failed' THEN EXCLUDED.lease_expires_at
				WHEN tool_executions.status='processing' AND tool_executions.lease_expires_at < now() THEN EXCLUDED.lease_expires_at
				ELSE tool_executions.lease_expires_at END,
			updated_at=now()
		RETURNING id::text,tenant_id::text,user_id::text,COALESCE(task_id,''),agent_id,tool_id,idempotency_key,status,
		          COALESCE(result,'null'::jsonb),COALESCE(error_code,''),lease_expires_at`,
		inv.TenantID,inv.UserID,inv.TaskID,inv.AgentID,inv.ToolID,inv.IdempotencyKey,leaseExpires,
	).Scan(&e.ID,&e.TenantID,&e.UserID,&e.TaskID,&e.AgentID,&e.ToolID,&e.IdempotencyKey,&status,&e.Result,&e.ErrorCode,&e.LeaseExpiresAt)
	if err != nil { return Execution{}, false, err }
	e.Status = ExecutionStatus(status)
	if e.Status == ExecutionCompleted { return e, false, nil }
	acquired := time.Until(e.LeaseExpiresAt) > lease/2
	return e, acquired, nil
}

func (r *PostgresExecutionRepository) Complete(ctx context.Context, tenantID, idempotencyKey string, result json.RawMessage) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tool_executions SET status='completed',result=$3::jsonb,error_code=NULL,updated_at=now()
		WHERE tenant_id=$1::uuid AND idempotency_key=$2`, tenantID,idempotencyKey,result)
	return err
}

func (r *PostgresExecutionRepository) Fail(ctx context.Context, tenantID, idempotencyKey, errorCode string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tool_executions SET status='failed',error_code=$3,updated_at=now()
		WHERE tenant_id=$1::uuid AND idempotency_key=$2`, tenantID,idempotencyKey,errorCode)
	return err
}

var _ ExecutionRepository = (*PostgresExecutionRepository)(nil)
