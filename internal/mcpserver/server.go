package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gjellerup1857/yun-agent-runtime/internal/identity"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
)

type Server struct {
	mcp        *mcp.Server
	runtime    *yarruntime.Runtime
	identities identity.Resolver
	dev        *identity.Principal
}

type ProfileInput struct{}

type ProfileOutput struct {
	TenantID    string `json:"tenant_id" jsonschema:"canonical YAR tenant identifier"`
	UserID      string `json:"user_id" jsonschema:"canonical YAR user identifier"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"canonical YAR display name"`
}

type TeamRunInput struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"YAR project identifier"`
	TaskID    string `json:"task_id,omitempty" jsonschema:"existing YAR task identifier to continue"`
	Message   string `json:"message" jsonschema:"request for the YAR agent team"`
}

type TeamRunOutput struct {
	RunID           string   `json:"run_id"`
	TaskID          string   `json:"task_id"`
	ProjectID       string   `json:"project_id,omitempty"`
	AgentsUsed      []string `json:"agents_used"`
	Answer          string   `json:"answer"`
	MemoriesRead    int      `json:"memories_read"`
	MemoriesWritten int      `json:"memories_written"`
}

func New(runtime *yarruntime.Runtime, identities identity.Resolver, devTenantID, devUserID string) *Server {
	s := &Server{
		runtime:    runtime,
		identities: identities,
	}
	if devTenantID != "" && devUserID != "" {
		s.dev = &identity.Principal{TenantID: devTenantID, UserID: devUserID, DisplayName: "YAR Developer"}
	}

	s.mcp = mcp.NewServer(
		&mcp.Implementation{
			Name:    "yar-agent-runtime",
			Version: "0.2.0",
		},
		nil,
	)

	mcp.AddTool(
		s.mcp,
		&mcp.Tool{
			Name:        "yar_profile_get",
			Description: "Return the canonical YAR identity connected to this MCP endpoint.",
		},
		s.profileGet,
	)

	mcp.AddTool(
		s.mcp,
		&mcp.Tool{
			Name:        "yar_team_run",
			Description: "Run the persistent YAR product engineering agent team. Reuse task_id to continue an existing task.",
		},
		s.teamRun,
	)

	return s
}

func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return s.mcp
		},
		&mcp.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
		},
	)
}

func (s *Server) profileGet(
	ctx context.Context,
	req *mcp.CallToolRequest,
	_ ProfileInput,
) (*mcp.CallToolResult, ProfileOutput, error) {
	principal, err := s.resolvePrincipal(ctx, req)
	if err != nil {
		return nil, ProfileOutput{}, err
	}
	return nil, ProfileOutput{
		TenantID:    principal.TenantID,
		UserID:      principal.UserID,
		DisplayName: principal.DisplayName,
	}, nil
}

func (s *Server) teamRun(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TeamRunInput,
) (*mcp.CallToolResult, TeamRunOutput, error) {
	principal, err := s.resolvePrincipal(ctx, req)
	if err != nil {
		return nil, TeamRunOutput{}, err
	}
	if s.runtime == nil {
		return nil, TeamRunOutput{}, fmt.Errorf("YAR runtime is unavailable")
	}

	result, err := s.runtime.Run(ctx, yarruntime.RunRequest{
		TenantID:  principal.TenantID,
		UserID:    principal.UserID,
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
		Message:   input.Message,
	})
	if err != nil {
		return nil, TeamRunOutput{}, err
	}

	return nil, TeamRunOutput{
		RunID:           result.RunID,
		TaskID:          result.TaskID,
		ProjectID:       result.ProjectID,
		AgentsUsed:      result.AgentsUsed,
		Answer:          result.Answer,
		MemoriesRead:    result.MemoriesRead,
		MemoriesWritten: result.MemoriesWritten,
	}, nil
}

func (s *Server) resolvePrincipal(ctx context.Context, req *mcp.CallToolRequest) (identity.Principal, error) {
	if req != nil && req.Extra != nil && req.Extra.TokenInfo != nil {
		userID := strings.TrimSpace(req.Extra.TokenInfo.UserID)
		if userID == "" {
			return identity.Principal{}, fmt.Errorf("authenticated token is missing canonical user subject")
		}
		if s.identities == nil {
			return identity.Principal{}, fmt.Errorf("canonical identity resolver is unavailable")
		}
		principal, err := s.identities.Resolve(ctx, userID)
		if err != nil {
			return identity.Principal{}, fmt.Errorf("resolve canonical identity: %w", err)
		}
		return principal, nil
	}
	if s.dev != nil {
		return *s.dev, nil
	}
	return identity.Principal{}, fmt.Errorf("authenticated YAR identity is required")
}
