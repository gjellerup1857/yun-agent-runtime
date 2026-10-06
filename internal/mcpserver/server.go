package mcpserver

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
)

type Server struct {
	mcp       *mcp.Server
	runtime   *yarruntime.Runtime
	tenantID  string
	userID    string
}

type ProfileInput struct{}

type ProfileOutput struct {
	TenantID string `json:"tenant_id" jsonschema:"canonical YAR tenant identifier"`
	UserID   string `json:"user_id" jsonschema:"canonical YAR user identifier"`
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

func New(runtime *yarruntime.Runtime, tenantID, userID string) *Server {
	s := &Server{
		runtime:  runtime,
		tenantID: tenantID,
		userID:   userID,
	}

	s.mcp = mcp.NewServer(
		&mcp.Implementation{
			Name:    "yar-agent-runtime",
			Version: "0.1.0",
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
	_ context.Context,
	_ *mcp.CallToolRequest,
	_ ProfileInput,
) (*mcp.CallToolResult, ProfileOutput, error) {
	return nil, ProfileOutput{
		TenantID: s.tenantID,
		UserID:   s.userID,
	}, nil
}

func (s *Server) teamRun(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input TeamRunInput,
) (*mcp.CallToolResult, TeamRunOutput, error) {
	result, err := s.runtime.Run(ctx, yarruntime.RunRequest{
		TenantID:  s.tenantID,
		UserID:    s.userID,
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
