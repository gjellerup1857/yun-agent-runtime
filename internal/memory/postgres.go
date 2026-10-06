package memory

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) Relevant(ctx context.Context, q Query) ([]Memory, error) {
	if q.Limit <= 0 { q.Limit = 12 }
	rows, err := r.db.Query(ctx, `
		SELECT id::text, tenant_id::text, user_id::text,
		       COALESCE(team_id,''), COALESCE(project_id,''), COALESCE(agent_id,''), COALESCE(task_id,''),
		       scope, memory_type, COALESCE(memory_key,''), content,
		       importance, confidence, trust_score, source_type, status,
		       COALESCE(supersedes_memory_id::text,''), created_at, updated_at
		FROM memories
		WHERE tenant_id=$1::uuid AND user_id=$2::uuid AND status='active' AND importance >= $3
		  AND (scope='user'
		    OR (scope='team' AND team_id=$4)
		    OR (scope='project' AND project_id=$5)
		    OR (scope='agent' AND agent_id=$6)
		    OR (scope='task' AND task_id=$7))
		ORDER BY CASE memory_type WHEN 'decision' THEN 1 WHEN 'preference' THEN 2 ELSE 3 END,
		         importance DESC, updated_at DESC
		LIMIT $8`, q.TenantID, q.UserID, q.MinImportance, q.TeamID, q.ProjectID, q.AgentID, q.TaskID, q.Limit)
	if err != nil { return nil, err }
	defer rows.Close()

	items := make([]Memory, 0, q.Limit)
	for rows.Next() {
		var m Memory
		var typ string
		if err := rows.Scan(&m.ID, &m.TenantID, &m.UserID, &m.TeamID, &m.ProjectID, &m.AgentID, &m.TaskID,
			&m.Scope, &typ, &m.Key, &m.Content, &m.Importance, &m.Confidence, &m.TrustScore,
			&m.SourceType, &m.Status, &m.SupersedesID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Type = Type(typ)
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Save(ctx context.Context, m Memory) (Memory, error) {
	if m.Status == "" { m.Status = "active" }
	err := r.db.QueryRow(ctx, `
		INSERT INTO memories (
			tenant_id,user_id,team_id,project_id,agent_id,task_id,scope,memory_type,memory_key,content,
			importance,confidence,trust_score,source_type,status
		) VALUES (
			$1::uuid,$2::uuid,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7,$8,NULLIF($9,''),$10,$11,$12,$13,$14,$15
		) RETURNING id::text,created_at,updated_at`,
		m.TenantID,m.UserID,m.TeamID,m.ProjectID,m.AgentID,m.TaskID,m.Scope,string(m.Type),m.Key,m.Content,
		m.Importance,m.Confidence,m.TrustScore,m.SourceType,m.Status).Scan(&m.ID,&m.CreatedAt,&m.UpdatedAt)
	if err != nil { return Memory{}, err }
	return m, nil
}

func (r *PostgresRepository) SaveDecision(ctx context.Context, m Memory) (Memory, error) {
	if m.Key == "" { return Memory{}, fmt.Errorf("decision memory requires key") }
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Memory{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()

	var previousID string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM memories
		WHERE tenant_id=$1::uuid AND user_id=$2::uuid AND memory_key=$3 AND scope=$4
		  AND project_id IS NOT DISTINCT FROM NULLIF($5,'')
		  AND team_id IS NOT DISTINCT FROM NULLIF($6,'')
		  AND status='active'
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`,
		m.TenantID,m.UserID,m.Key,m.Scope,m.ProjectID,m.TeamID).Scan(&previousID)
	if err == nil {
		if _, err = tx.Exec(ctx, `UPDATE memories SET status='superseded',updated_at=now() WHERE id=$1::uuid`, previousID); err != nil {
			return Memory{}, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Memory{}, err
	}

	m.Status = "active"
	m.SupersedesID = previousID
	err = tx.QueryRow(ctx, `
		INSERT INTO memories (
			tenant_id,user_id,team_id,project_id,agent_id,task_id,scope,memory_type,memory_key,content,
			importance,confidence,trust_score,source_type,status,supersedes_memory_id
		) VALUES (
			$1::uuid,$2::uuid,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,$11,$12,$13,$14,'active',NULLIF($15,'')::uuid
		) RETURNING id::text,created_at,updated_at`,
		m.TenantID,m.UserID,m.TeamID,m.ProjectID,m.AgentID,m.TaskID,m.Scope,string(m.Type),m.Key,m.Content,
		m.Importance,m.Confidence,m.TrustScore,m.SourceType,m.SupersedesID).Scan(&m.ID,&m.CreatedAt,&m.UpdatedAt)
	if err != nil { return Memory{}, err }
	if err := tx.Commit(ctx); err != nil { return Memory{}, err }
	return m, nil
}

var _ Repository = (*PostgresRepository)(nil)
