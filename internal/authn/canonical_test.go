package authn

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/gjellerup1857/yun-agent-runtime/internal/identity"
)

type stubIdentityResolver struct {
	principal identity.Principal
	err       error
}

func (s stubIdentityResolver) Resolve(context.Context, string) (identity.Principal, error) {
	return s.principal, s.err
}

func TestRequireCanonicalUser(t *testing.T) {
	base := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			UserID:     "00000000-0000-0000-0000-000000000101",
			Expiration: time.Now().Add(time.Hour),
		}, nil
	}
	verify := RequireCanonicalUser(base, stubIdentityResolver{principal: identity.Principal{
		TenantID:    "00000000-0000-0000-0000-000000000001",
		UserID:      "00000000-0000-0000-0000-000000000101",
		DisplayName: "YAR Developer",
	}})

	info, err := verify(context.Background(), "token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.Extra["yar_tenant_id"] != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("tenant extra = %#v", info.Extra["yar_tenant_id"])
	}
}

func TestRequireCanonicalUserRejectsUnknownUser(t *testing.T) {
	base := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{UserID: "missing", Expiration: time.Now().Add(time.Hour)}, nil
	}
	verify := RequireCanonicalUser(base, stubIdentityResolver{err: identity.ErrUserNotFound})

	_, err := verify(context.Background(), "token", nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}
