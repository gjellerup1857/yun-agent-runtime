package mcpserver

import (
	"context"
	"net/http/httptest"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gjellerup1857/yun-agent-runtime/internal/authn"
)

func TestProfileToolOverStreamableHTTP(t *testing.T) {
	server := New(nil, nil, "tenant-test", "user-test")
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "yar-test-client", Version: "0.1.0"},
		nil,
	)

	session, err := client.Connect(
		context.Background(),
		&mcp.StreamableClientTransport{Endpoint: httpServer.URL},
		nil,
	)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "yar_profile_get",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call profile tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("profile tool returned MCP error: %+v", result.Content)
	}
}

func TestToolScopesAreIndependent(t *testing.T) {
	server := New(nil, nil, "", "")

	profileRequest := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &mcpauth.TokenInfo{
		Scopes: []string{authn.ScopeProfileRead},
	}}}
	if err := server.requireScopes(profileRequest, authn.ScopeProfileRead); err != nil {
		t.Fatalf("profile scope rejected: %v", err)
	}
	if err := server.requireScopes(profileRequest, authn.ScopeTeamRun); err == nil {
		t.Fatal("profile-only token unexpectedly authorized for team run")
	}

	teamRequest := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &mcpauth.TokenInfo{
		Scopes: []string{authn.ScopeTeamRun},
	}}}
	if err := server.requireScopes(teamRequest, authn.ScopeTeamRun); err != nil {
		t.Fatalf("team-run scope rejected: %v", err)
	}
	if err := server.requireScopes(teamRequest, authn.ScopeProfileRead); err == nil {
		t.Fatal("team-run-only token unexpectedly authorized for profile read")
	}
}
