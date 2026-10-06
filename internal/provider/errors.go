package provider

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorUnknown     ErrorKind = "unknown"
	ErrorTimeout     ErrorKind = "timeout"
	ErrorRateLimit   ErrorKind = "rate_limit"
	ErrorServer      ErrorKind = "server"
	ErrorAuth        ErrorKind = "auth"
	ErrorBadRequest  ErrorKind = "bad_request"
	ErrorUnavailable ErrorKind = "unavailable"
)

type ProviderError struct {
	Provider   string
	Kind       ErrorKind
	StatusCode int
	Message    string
	Retryable  bool
	Err        error
}

func (e *ProviderError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s provider error: %s", e.Provider, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s provider error: %v", e.Provider, e.Err)
	}
	return fmt.Sprintf("%s provider error", e.Provider)
}

func (e *ProviderError) Unwrap() error { return e.Err }

func IsRetryable(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Retryable
}

func ErrorKindOf(err error) ErrorKind {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Kind
	}
	return ErrorUnknown
}

func HTTPError(providerName string, status int, message string) error {
	err := &ProviderError{Provider: providerName, StatusCode: status, Message: message}
	switch {
	case status == 408:
		err.Kind, err.Retryable = ErrorTimeout, true
	case status == 429:
		err.Kind, err.Retryable = ErrorRateLimit, true
	case status >= 500:
		err.Kind, err.Retryable = ErrorServer, true
	case status == 401 || status == 403:
		err.Kind = ErrorAuth
	case status >= 400:
		err.Kind = ErrorBadRequest
	default:
		err.Kind = ErrorUnknown
	}
	return err
}
