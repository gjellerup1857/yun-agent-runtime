package platformapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gjellerup1857/yun-agent-runtime/internal/identity"
	"github.com/gjellerup1857/yun-agent-runtime/internal/platformgateway"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
)

type fakeGateway struct {
	lastReq platformgateway.Request
	resp    platformgateway.Response
	err     error
}

func (f *fakeGateway) Run(_ context.Context, req platformgateway.Request) (platformgateway.Response, error) {
	f.lastReq = req
	return f.resp, f.err
}

func TestHandlerUsesServerDerivedCanonicalIdentity(t *testing.T) {
	gateway := &fakeGateway{resp: platformgateway.Response{
		ConversationID: "conversation-1",
		Run:            yarruntime.RunResponse{RunID: "run-1", TaskID: "task-1", Answer: "ok"},
	}}
	handler := New(gateway, func(*http.Request) (identity.Principal, error) {
		return identity.Principal{TenantID: "tenant-canonical", UserID: "user-canonical"}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/platform/run", strings.NewReader(`{
		"source_client":"chatgpt",
		"external_conversation_id":"conv-ext",
		"external_message_id":"msg-ext",
		"message":"continue"
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if gateway.lastReq.TenantID != "tenant-canonical" || gateway.lastReq.UserID != "user-canonical" {
		t.Fatalf("canonical identity not injected: %+v", gateway.lastReq)
	}
}

func TestHandlerRejectsClientSuppliedCanonicalIdentityFields(t *testing.T) {
	gateway := &fakeGateway{}
	handler := New(gateway, func(*http.Request) (identity.Principal, error) {
		return identity.Principal{TenantID: "tenant-canonical", UserID: "user-canonical"}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/platform/run", strings.NewReader(`{
		"tenant_id":"tenant-spoof",
		"user_id":"user-spoof",
		"source_client":"chatgpt",
		"external_conversation_id":"conv-ext",
		"external_message_id":"msg-ext",
		"message":"continue"
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerMapsInProgressToConflict(t *testing.T) {
	gateway := &fakeGateway{err: platformgateway.ErrMessageInProgress}
	handler := New(gateway, func(*http.Request) (identity.Principal, error) {
		return identity.Principal{TenantID: "tenant", UserID: "user"}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/platform/run", strings.NewReader(`{
		"source_client":"claude",
		"external_conversation_id":"conv-ext",
		"external_message_id":"msg-ext",
		"message":"continue"
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict || w.Header().Get("Retry-After") != "2" {
		t.Fatalf("status=%d retry-after=%q body=%s", w.Code, w.Header().Get("Retry-After"), w.Body.String())
	}
}

func TestHandlerRejectsUnauthenticatedPrincipal(t *testing.T) {
	gateway := &fakeGateway{}
	handler := New(gateway, func(*http.Request) (identity.Principal, error) {
		return identity.Principal{}, errors.New("no identity")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/platform/run", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
