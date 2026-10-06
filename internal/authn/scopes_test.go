package authn

import (
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestRequireScopesAllowsGrantedScopes(t *testing.T) {
	info := &mcpauth.TokenInfo{Scopes: []string{ScopeProfileRead, ScopeTeamRun}}
	if err := RequireScopes(info, ScopeTeamRun); err != nil {
		t.Fatalf("granted scope rejected: %v", err)
	}
}

func TestRequireScopesRejectsMissingScope(t *testing.T) {
	info := &mcpauth.TokenInfo{Scopes: []string{ScopeProfileRead}}
	if err := RequireScopes(info, ScopeTeamRun); err == nil {
		t.Fatal("expected missing scope to be rejected")
	}
}

func TestRequireScopesRejectsMissingTokenInfo(t *testing.T) {
	if err := RequireScopes(nil, ScopeProfileRead); err == nil {
		t.Fatal("expected missing token info to be rejected")
	}
}
