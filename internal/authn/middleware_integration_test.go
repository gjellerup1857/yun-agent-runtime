package authn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/gjellerup1857/yun-agent-runtime/internal/identity"
)

func TestBearerMiddlewareWithCanonicalIdentity(t *testing.T) {
	introspection := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("token") != "good-token" {
			_, _ = w.Write([]byte(`{"active":false}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active":true,
			"scope":"yar:mcp",
			"sub":"00000000-0000-0000-0000-000000000101",
			"aud":"https://api.yar.example/mcp",
			"exp":4102444800
		}`))
	}))
	defer introspection.Close()

	verifier, err := NewIntrospectionVerifier(IntrospectionConfig{
		Endpoint:         introspection.URL,
		ClientID:         "client",
		ClientSecret:     "secret",
		ExpectedAudience: "https://api.yar.example/mcp",
	}, introspection.Client())
	if err != nil {
		t.Fatal(err)
	}
	canonical := RequireCanonicalUser(verifier.Verify, stubIdentityResolver{principal: identity.Principal{
		TenantID: "00000000-0000-0000-0000-000000000001",
		UserID:   "00000000-0000-0000-0000-000000000101",
	}})

	handler := mcpauth.RequireBearerToken(canonical, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: "https://api.yar.example/.well-known/oauth-protected-resource/mcp",
		Scopes:              []string{"yar:mcp"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := mcpauth.TokenInfoFromContext(r.Context())
		if info == nil || info.UserID == "" {
			http.Error(w, "identity missing", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "https://api.yar.example/mcp", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("status without token = %d, want 401", unauthorized.Code)
	}
	if got := unauthorized.Header().Get("WWW-Authenticate"); !strings.Contains(got, "resource_metadata=") {
		t.Fatalf("WWW-Authenticate = %q", got)
	}

	authorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://api.yar.example/mcp", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	handler.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusOK {
		t.Fatalf("status with valid token = %d, want 200; body=%s", authorized.Code, authorized.Body.String())
	}
}

func TestBearerMiddlewareRejectsUnknownCanonicalUser(t *testing.T) {
	base := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{UserID: "unknown"}, nil
	}
	canonical := RequireCanonicalUser(base, stubIdentityResolver{err: identity.ErrUserNotFound})
	if _, err := canonical(context.Background(), "token", nil); err == nil {
		t.Fatal("expected unknown canonical user to be rejected")
	}
}
