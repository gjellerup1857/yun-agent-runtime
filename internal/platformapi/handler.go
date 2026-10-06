package platformapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gjellerup1857/yun-agent-runtime/internal/identity"
	"github.com/gjellerup1857/yun-agent-runtime/internal/platformgateway"
)

type Gateway interface {
	Run(ctx context.Context, req platformgateway.Request) (platformgateway.Response, error)
}

type PrincipalResolver func(*http.Request) (identity.Principal, error)

type Handler struct {
	gateway          Gateway
	resolvePrincipal PrincipalResolver
}

func New(gateway Gateway, resolvePrincipal PrincipalResolver) *Handler {
	return &Handler{gateway: gateway, resolvePrincipal: resolvePrincipal}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.gateway == nil || h.resolvePrincipal == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "platform gateway unavailable"})
		return
	}

	principal, err := h.resolvePrincipal(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authenticated identity required"})
		return
	}

	var req platformgateway.Request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	// Canonical identity is server-derived. Any client-supplied identity fields
	// are impossible to bind because TenantID/UserID are not JSON fields.
	req.TenantID = principal.TenantID
	req.UserID = principal.UserID

	result, err := h.gateway.Run(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, platformgateway.ErrMessageInProgress):
			w.Header().Set("Retry-After", "2")
			writeJSON(w, http.StatusConflict, map[string]any{"error": "MESSAGE_IN_PROGRESS"})
		case errors.Is(err, platformgateway.ErrUnsupportedSourceClient):
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "UNSUPPORTED_SOURCE_CLIENT"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "platform run failed"})
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
