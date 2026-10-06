package authn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestMCPScopeStepUpReturnsInsufficientScopeChallenge(t *testing.T) {
	verifier := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Scopes:     []string{ScopeProfileRead},
			Expiration: time.Now().Add(time.Hour),
			UserID:     "user-1",
		}, nil
	}

	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	stepUp := MCPScopeStepUp("https://api.example.com/.well-known/oauth-protected-resource/mcp", map[string][]string{
		"yar_team_run": {ScopeTeamRun},
	})(base)
	handler := mcpauth.RequireBearerToken(verifier, nil)(stepUp)

	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/mcp", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set(mcpMethodHeader, "tools/call")
	req.Header.Set(mcpNameHeader, "yar_team_run")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	challenge := w.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, `error="insufficient_scope"`) || !strings.Contains(challenge, `scope="yar:team:run"`) {
		t.Fatalf("challenge = %q", challenge)
	}
	if !strings.Contains(challenge, `resource_metadata="https://api.example.com/.well-known/oauth-protected-resource/mcp"`) {
		t.Fatalf("challenge missing resource metadata: %q", challenge)
	}
}

func TestMCPScopeStepUpAllowsGrantedScopeThroughBearerMiddleware(t *testing.T) {
	verifier := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Scopes:     []string{ScopeTeamRun},
			Expiration: time.Now().Add(time.Hour),
			UserID:     "user-1",
		}, nil
	}
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	stepUp := MCPScopeStepUp("https://api.example.com/.well-known/oauth-protected-resource/mcp", map[string][]string{
		"yar_team_run": {ScopeTeamRun},
	})(base)
	handler := mcpauth.RequireBearerToken(verifier, nil)(stepUp)

	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/mcp", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set(mcpMethodHeader, "tools/call")
	req.Header.Set(mcpNameHeader, "yar_team_run")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestMCPScopeStepUpFallsBackWhenStandardHeadersMissing(t *testing.T) {
	verifier := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Scopes:     []string{ScopeProfileRead},
			Expiration: time.Now().Add(time.Hour),
			UserID:     "user-1",
		}, nil
	}
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	stepUp := MCPScopeStepUp("https://api.example.com/.well-known/oauth-protected-resource/mcp", map[string][]string{
		"yar_team_run": {ScopeTeamRun},
	})(base)
	handler := mcpauth.RequireBearerToken(verifier, nil)(stepUp)

	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/mcp", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want fallback 204", w.Code)
	}
}
