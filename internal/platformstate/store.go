package platformstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ReceiptProcessing = "processing"
	ReceiptCompleted  = "completed"
	ReceiptFailed     = "failed"
)

type Conversation struct {
	ID                     string    `json:"id"`
	TenantID               string    `json:"tenant_id"`
	UserID                 string    `json:"user_id"`
	SourceClient            string    `json:"source_client"`
	ExternalConversationID string    `json:"external_conversation_id"`
	TeamID                  string    `json:"team_id"`
	ProjectID               string    `json:"project_id,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	LastSeenAt              time.Time `json:"last_seen_at"`
}

type Receipt struct {
	ID                string          `json:"id"`
	TenantID          string          `json:"tenant_id"`
	UserID            string          `json:"user_id"`
	SourceClient       string          `json:"source_client"`
	ExternalMessageID string          `json:"external_message_id"`
	Status            string          `json:"status"`
	AttemptCount      int             `json:"attempt_count"`
	LeaseExpiresAt    *time.Time       `json:"lease_expires_at,omitempty"`
	Response          json.RawMessage `json:"response,omitempty"`
	ErrorCode         string          `json:"error_code,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type AcquireResult struct {
	Receipt  Receipt
	Acquired bool
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) ResolveOrCreateConversation(
	ctx context.Context,
	tenantID, userID, sourceClient, externalConversationID, teamID, projectID string,
) (Conversation, error) {
	sourceClient = normalizeSourceClient(sourceClient)
	externalConversationID = strings.TrimSpace(externalConversationID)
	if sourceClient == "" || externalConversationID == "" || strings.TrimSpace(teamID) == "" {
		return Conversation{}, fmt.Errorf("source client, external conversation ID, and team ID are required")
	}

	var item Conversation
	err := s.db.QueryRow(ctx, `
		INSERT INTO platform_conversations (
			tenant_id,user_id,source_client,external_conversation_id,team_id,project_id
		)
		VALUES ($1::uuid,$2::uuid,$3,$4,$5,NULLIF($6,''))
		ON CONFLICT (tenant_id,user_id,source_client,external_conversation_id)
		DO UPDATE SET last_seen_at=now()
		RETURNING id::text,tenant_id::text,user_id::text,source_client,external_conversation_id,
		          team_id,COALESCE(project_id,''),created_at,last_seen_at`,
		tenantID, userID, sourceClient, externalConversationID, teamID, strings.TrimSpace(projectID),
	).Scan(
		&item.ID, &item.TenantID, &item.UserID, &item.SourceClient,
		&item.ExternalConversationID, &item.TeamID, &item.ProjectID,
		&item.CreatedAt, &item.LastSeenAt,
	)
	if err != nil {
		return Conversation{}, err
	}
	return item, nil
}

func (s *Store) AttachTask(ctx context.Context, conversationID, taskID string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO conversation_tasks (conversation_id,task_id)
		VALUES ($1::uuid,$2::uuid)
		ON CONFLICT (conversation_id,task_id) DO NOTHING`,
		conversationID, taskID,
	)
	return err
}

func (s *Store) AcquireReceipt(
	ctx context.Context,
	tenantID, userID, sourceClient, externalMessageID string,
	lease time.Duration,
) (AcquireResult, error) {
	sourceClient = normalizeSourceClient(sourceClient)
	externalMessageID = strings.TrimSpace(externalMessageID)
	if sourceClient == "" || externalMessageID == "" {
		return AcquireResult{}, fmt.Errorf("source client and external message ID are required")
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AcquireResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var receipt Receipt
	inserted := true
	err = tx.QueryRow(ctx, `
		INSERT INTO platform_message_receipts (
			tenant_id,user_id,source_client,external_message_id,status,lease_expires_at
		)
		VALUES ($1::uuid,$2::uuid,$3,$4,'processing',now()+$5::interval)
		ON CONFLICT (tenant_id,user_id,source_client,external_message_id) DO NOTHING
		RETURNING id::text,tenant_id::text,user_id::text,source_client,external_message_id,
		          status,attempt_count,lease_expires_at,response,COALESCE(error_code,''),created_at,updated_at`,
		tenantID, userID, sourceClient, externalMessageID, intervalLiteral(lease),
	).Scan(
		&receipt.ID, &receipt.TenantID, &receipt.UserID, &receipt.SourceClient,
		&receipt.ExternalMessageID, &receipt.Status, &receipt.AttemptCount,
		&receipt.LeaseExpiresAt, &receipt.Response, &receipt.ErrorCode,
		&receipt.CreatedAt, &receipt.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		inserted = false
	} else if err != nil {
		return AcquireResult{}, err
	}

	if inserted {
		if err := tx.Commit(ctx); err != nil {
			return AcquireResult{}, err
		}
		return AcquireResult{Receipt: receipt, Acquired: true}, nil
	}

	err = tx.QueryRow(ctx, `
		SELECT id::text,tenant_id::text,user_id::text,source_client,external_message_id,
		       status,attempt_count,lease_expires_at,response,COALESCE(error_code,''),created_at,updated_at
		FROM platform_message_receipts
		WHERE tenant_id=$1::uuid AND user_id=$2::uuid AND source_client=$3 AND external_message_id=$4
		FOR UPDATE`,
		tenantID, userID, sourceClient, externalMessageID,
	).Scan(
		&receipt.ID, &receipt.TenantID, &receipt.UserID, &receipt.SourceClient,
		&receipt.ExternalMessageID, &receipt.Status, &receipt.AttemptCount,
		&receipt.LeaseExpiresAt, &receipt.Response, &receipt.ErrorCode,
		&receipt.CreatedAt, &receipt.UpdatedAt,
	)
	if err != nil {
		return AcquireResult{}, err
	}

	if receipt.Status == ReceiptCompleted {
		if err := tx.Commit(ctx); err != nil {
			return AcquireResult{}, err
		}
		return AcquireResult{Receipt: receipt, Acquired: false}, nil
	}

	now := time.Now()
	if receipt.Status == ReceiptProcessing && receipt.LeaseExpiresAt != nil && receipt.LeaseExpiresAt.After(now) {
		if err := tx.Commit(ctx); err != nil {
			return AcquireResult{}, err
		}
		return AcquireResult{Receipt: receipt, Acquired: false}, nil
	}

	err = tx.QueryRow(ctx, `
		UPDATE platform_message_receipts
		SET status='processing',attempt_count=attempt_count+1,
		    lease_expires_at=now()+$5::interval,response=NULL,error_code=NULL,updated_at=now()
		WHERE tenant_id=$1::uuid AND user_id=$2::uuid AND source_client=$3 AND external_message_id=$4
		RETURNING id::text,tenant_id::text,user_id::text,source_client,external_message_id,
		          status,attempt_count,lease_expires_at,response,COALESCE(error_code,''),created_at,updated_at`,
		tenantID, userID, sourceClient, externalMessageID, intervalLiteral(lease),
	).Scan(
		&receipt.ID, &receipt.TenantID, &receipt.UserID, &receipt.SourceClient,
		&receipt.ExternalMessageID, &receipt.Status, &receipt.AttemptCount,
		&receipt.LeaseExpiresAt, &receipt.Response, &receipt.ErrorCode,
		&receipt.CreatedAt, &receipt.UpdatedAt,
	)
	if err != nil {
		return AcquireResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AcquireResult{}, err
	}
	return AcquireResult{Receipt: receipt, Acquired: true}, nil
}

func (s *Store) CompleteReceipt(ctx context.Context, receiptID string, response json.RawMessage) error {
	cmd, err := s.db.Exec(ctx, `
		UPDATE platform_message_receipts
		SET status='completed',response=$2::jsonb,error_code=NULL,lease_expires_at=NULL,updated_at=now()
		WHERE id=$1::uuid AND status='processing'`,
		receiptID, response,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("receipt is not processing")
	}
	return nil
}

func (s *Store) FailReceipt(ctx context.Context, receiptID, errorCode string) error {
	cmd, err := s.db.Exec(ctx, `
		UPDATE platform_message_receipts
		SET status='failed',error_code=NULLIF($2,''),lease_expires_at=NULL,updated_at=now()
		WHERE id=$1::uuid AND status='processing'`,
		receiptID, strings.TrimSpace(errorCode),
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("receipt is not processing")
	}
	return nil
}

func normalizeSourceClient(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func intervalLiteral(d time.Duration) string {
	return fmt.Sprintf("%f seconds", d.Seconds())
}
