package task

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) Create(ctx context.Context, t Task) (Task, error) {
	if t.Status == "" { t.Status = StatusRunning }
	err := r.db.QueryRow(ctx, `
		INSERT INTO tasks (tenant_id,user_id,team_id,project_id,title,status,current_step)
		VALUES ($1::uuid,$2::uuid,$3,NULLIF($4,''),NULLIF($5,''),$6,NULLIF($7,''))
		RETURNING id::text,created_at,updated_at`,
		t.TenantID,t.UserID,t.TeamID,t.ProjectID,t.Title,string(t.Status),t.CurrentStep,
	).Scan(&t.ID,&t.CreatedAt,&t.UpdatedAt)
	if err != nil { return Task{}, err }
	return t, nil
}

func (r *PostgresRepository) Get(ctx context.Context, tenantID, userID, taskID string) (Task, error) {
	var t Task
	var status string
	err := r.db.QueryRow(ctx, `
		SELECT id::text,tenant_id::text,user_id::text,team_id,COALESCE(project_id,''),COALESCE(title,''),status,
		       COALESCE(current_step,''),created_at,updated_at
		FROM tasks WHERE id=$1::uuid AND tenant_id=$2::uuid AND user_id=$3::uuid`,
		taskID,tenantID,userID,
	).Scan(&t.ID,&t.TenantID,&t.UserID,&t.TeamID,&t.ProjectID,&t.Title,&status,&t.CurrentStep,&t.CreatedAt,&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return Task{}, ErrNotFound }
	if err != nil { return Task{}, err }
	t.Status = Status(status)
	return t, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, taskID string, status Status) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE tasks SET status=$2,updated_at=now(),
		completed_at=CASE WHEN $2='completed' THEN now() ELSE completed_at END
		WHERE id=$1::uuid`, taskID, string(status))
	if err != nil { return err }
	if cmd.RowsAffected() == 0 { return ErrNotFound }
	return nil
}

var _ Repository = (*PostgresRepository)(nil)
