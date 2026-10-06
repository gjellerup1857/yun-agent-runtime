package task

import (
	"context"
	"errors"
	"time"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusWaitingApproval Status = "waiting_approval"
	StatusBlocked Status = "blocked"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
)

var ErrNotFound = errors.New("task not found")

type Task struct {
	ID string
	TenantID string
	UserID string
	TeamID string
	ProjectID string
	Title string
	Status Status
	CurrentStep string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repository interface {
	Create(ctx context.Context, t Task) (Task, error)
	Get(ctx context.Context, tenantID, userID, taskID string) (Task, error)
	UpdateStatus(ctx context.Context, taskID string, status Status) error
}
