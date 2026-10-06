package approval

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusExpired Status = "expired"
)

type Approval struct {
	ID string
	TenantID string
	UserID string
	TaskID string
	AgentID string
	ToolID string
	IdempotencyKey string
	ArgumentsHash string
	Risk tools.Risk
	Status Status
	ExpiresAt time.Time
}

type Repository interface {
	CreateOrGet(ctx context.Context, a Approval) (Approval, error)
	Get(ctx context.Context, tenantID, approvalID string) (Approval, error)
	Resolve(ctx context.Context, tenantID, userID, approvalID string, status Status) error
}

type Service struct {
	repo Repository
	ttl time.Duration
}

func NewService(repo Repository) *Service { return &Service{repo: repo, ttl: 15 * time.Minute} }

func (s *Service) Request(ctx context.Context, a Approval) (Approval, error) {
	if a.Status == "" { a.Status = StatusPending }
	if a.ExpiresAt.IsZero() { a.ExpiresAt = time.Now().UTC().Add(s.ttl) }
	return s.repo.CreateOrGet(ctx, a)
}

func (s *Service) IsApproved(ctx context.Context, tenantID, approvalID string) (bool, error) {
	a, err := s.repo.Get(ctx, tenantID, approvalID)
	if err != nil { return false, err }
	return a.Status == StatusApproved && time.Now().UTC().Before(a.ExpiresAt), nil
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) CreateOrGet(ctx context.Context, a Approval) (Approval, error) {
	var risk, status string
	err := r.db.QueryRow(ctx, `
		INSERT INTO approvals (tenant_id,user_id,task_id,agent_id,tool_id,idempotency_key,arguments_hash,risk,status,expires_at)
		VALUES ($1::uuid,$2::uuid,NULLIF($3,''),$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (tenant_id,idempotency_key,tool_id)
		DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key
		RETURNING id::text,risk,status,expires_at`,
		a.TenantID,a.UserID,a.TaskID,a.AgentID,a.ToolID,a.IdempotencyKey,a.ArgumentsHash,string(a.Risk),string(a.Status),a.ExpiresAt,
	).Scan(&a.ID,&risk,&status,&a.ExpiresAt)
	if err != nil { return Approval{}, err }
	a.Risk = tools.Risk(risk)
	a.Status = Status(status)
	return a, nil
}

func (r *PostgresRepository) Get(ctx context.Context, tenantID, approvalID string) (Approval, error) {
	var a Approval
	var risk, status string
	err := r.db.QueryRow(ctx, `
		SELECT id::text,tenant_id::text,user_id::text,COALESCE(task_id,''),agent_id,tool_id,idempotency_key,arguments_hash,risk,status,expires_at
		FROM approvals WHERE id=$1::uuid AND tenant_id=$2::uuid`, approvalID, tenantID,
	).Scan(&a.ID,&a.TenantID,&a.UserID,&a.TaskID,&a.AgentID,&a.ToolID,&a.IdempotencyKey,&a.ArgumentsHash,&risk,&status,&a.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) { return Approval{}, errors.New("approval not found") }
	if err != nil { return Approval{}, err }
	a.Risk = tools.Risk(risk)
	a.Status = Status(status)
	return a, nil
}

func (r *PostgresRepository) Resolve(ctx context.Context, tenantID, userID, approvalID string, status Status) error {
	_, err := r.db.Exec(ctx, `
		UPDATE approvals SET status=$4,resolved_at=now()
		WHERE id=$1::uuid AND tenant_id=$2::uuid AND user_id=$3::uuid AND status='pending' AND expires_at>now()`,
		approvalID,tenantID,userID,string(status))
	return err
}

var _ Repository = (*PostgresRepository)(nil)
