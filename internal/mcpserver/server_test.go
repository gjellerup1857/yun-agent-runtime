package mcpserver

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProfileToolOverStreamableHTTP(t *testing.T) {
	server := New(nil, "tenant-test", "user-test")
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
