package tools

import (
	"errors"
	"fmt"
)

var (
	ErrToolNotFound = errors.New("tool not found")
	ErrToolDenied = errors.New("tool denied")
	ErrInvalidArguments = errors.New("invalid tool arguments")
	ErrIdempotencyRequired = errors.New("idempotency key required")
	ErrExecutionInProgress = errors.New("tool execution already in progress")
)

type ApprovalRequiredError struct{ ApprovalID string }

func (e *ApprovalRequiredError) Error() string {
	return fmt.Sprintf("approval required: %s", e.ApprovalID)
}
