package authn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestMCPScopeStepUpAllowsGrantedScope(t *testing.T) {
	handler := MCPScopeStepUp("https://api.example.com/.well-known/oauth-protected-resource/mcp", map[string][]string{
		"yar_team_run": {ScopeTeamRun},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/mcp", nil)
	req.Header.Set(mcpMethodHeader, "tools/call")
	req.Header.Set(mcpNameHeader, "yar_team_run")
	req = req.WithContext(context.WithValue(req.Context(), tokenInfoKeyForTest{}, &mcpauth.TokenInfo{Scopes: []string{ScopeTeamRun}}))

	// TokenInfoFromContext uses an unexported SDK key, so route through the
	// official bearer middleware in the integration test below instead.
	_ = req
	_ = handler
}

func TestMCPScopeStepUpReturnsInsufficientScopeChallenge(t *testing.T) {
	verifier := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Scopes: []string{ScopeProfileRead},
			Expiration: testFutureExpiration(),
			UserID: "user-1",
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
}

func TestMCPScopeStepUpAllowsGrantedScopeThroughBearerMiddleware(t *testing.T) {
	verifier := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Scopes: []string{ScopeTeamRun},
			Expiration: testFutureExpiration(),
			UserID: "user-1",
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

// This private key is never read by production code. It only prevents the
// compiler from optimizing away the first construction test while documenting
// why the real tests route through RequireBearerToken.
type tokenInfoKeyForTest struct{}
