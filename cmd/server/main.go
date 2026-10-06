package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjellerup1857/yun-agent-runtime/internal/approval"
	"github.com/gjellerup1857/yun-agent-runtime/internal/audit"
	"github.com/gjellerup1857/yun-agent-runtime/internal/inference"
	"github.com/gjellerup1857/yun-agent-runtime/internal/mcpserver"
	"github.com/gjellerup1857/yun-agent-runtime/internal/memory"
	"github.com/gjellerup1857/yun-agent-runtime/internal/policy"
	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	anthropicprovider "github.com/gjellerup1857/yun-agent-runtime/internal/provider/anthropic"
	googleprovider "github.com/gjellerup1857/yun-agent-runtime/internal/provider/google"
	openaiprovider "github.com/gjellerup1857/yun-agent-runtime/internal/provider/openai"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
	"github.com/gjellerup1857/yun-agent-runtime/internal/task"
	"github.com/gjellerup1857/yun-agent-runtime/internal/toolruntime"
	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
	mocktools "github.com/gjellerup1857/yun-agent-runtime/internal/tools/mock"
)

const (
	devTenantID = "00000000-0000-0000-0000-000000000001"
	devUserID   = "00000000-0000-0000-0000-000000000101"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://yar:yar_dev@localhost:5432/yar"
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := db.Ping(pingCtx); err != nil {
		log.Fatalf("database unavailable: %v", err)
	}

	memoryRepo := memory.NewPostgresRepository(db)
	taskRepo := task.NewPostgresRepository(db)
	agentRouter := routing.New()

	providerRegistry := provider.NewRegistry()
	providerRegistry.Register(provider.NewMock())
	models := map[string]provider.ModelRef{
		"mock": {ID: "mock", Provider: "mock", Model: "mock-balanced"},
	}
	httpClient := provider.NewHTTPClient()

	if key, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL"); key != "" && model != "" {
		providerRegistry.Register(openaiprovider.New(key, httpClient))
		models["openai"] = provider.ModelRef{ID: "openai", Provider: "openai", Model: model}
	}
	if key, model := os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL"); key != "" && model != "" {
		providerRegistry.Register(anthropicprovider.New(key, httpClient))
		models["anthropic"] = provider.ModelRef{ID: "anthropic", Provider: "anthropic", Model: model}
	}
	if key, model := os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL"); key != "" && model != "" {
		providerRegistry.Register(googleprovider.New(key, httpClient))
		models["google"] = provider.ModelRef{ID: "google", Provider: "google", Model: model}
	}

	modelRouter := routing.NewModelRouter(models)
	inferenceService := inference.New(providerRegistry)
	runtime := yarruntime.NewMulti(
		agentRouter,
		modelRouter,
		inferenceService,
		memoryRepo,
		taskRepo,
		memory.NewExtractor(),
	)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.Tool{
		ID:          "mock.note.read",
		Description: "Read mock development notes.",
		Operation:   tools.OperationRead,
		Risk:        tools.RiskLow,
		Backend:     "mock",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	})
	toolRegistry.Register(tools.Tool{
		ID:          "mock.note.create",
		Description: "Create a mock development note.",
		Operation:   tools.OperationWrite,
		Risk:        tools.RiskMedium,
		Backend:     "mock",
		InputSchema: json.RawMessage(`{"type":"object","required":["text"]}`),
	})

	policyEngine := policy.New(map[string]policy.AgentPolicy{
		"product-manager": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{"mock.note.create": {}},
			Deny:             map[string]struct{}{},
		},
		"technical-product-manager": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{"mock.note.create": {}},
			Deny:             map[string]struct{}{},
		},
		"backend-engineer": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{"mock.note.create": {}},
			Deny:             map[string]struct{}{},
		},
		"software-architect": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{},
			Deny:             map[string]struct{}{"mock.note.create": {}},
		},
	})

	approvalRepo := approval.NewPostgresRepository(db)
	approvalService := approval.NewService(approvalRepo)
	executionRepo := tools.NewPostgresExecutionRepository(db)
	auditRepo := audit.NewPostgresRepository(db)
	toolExecutor := toolruntime.NewExecutor(
		toolRegistry,
		tools.NewBasicSchemaValidator(),
		policyEngine,
		approvalService,
		executionRepo,
		auditRepo,
		[]tools.Backend{mocktools.New()},
	)
	runtime.WithTools(toolRegistry, toolExecutor, policyEngine)

	mux := http.NewServeMux()
	mcpEndpoint := mcpserver.New(runtime, devTenantID, devUserID)
	mux.Handle("/mcp", mcpEndpoint.Handler())

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(checkCtx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "database": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"database": "ok",
			"mcp":      "/mcp",
			"providers": map[string]bool{
				"mock":      providerRegistry.Has("mock"),
				"openai":    providerRegistry.Has("openai"),
				"anthropic": providerRegistry.Has("anthropic"),
				"google":    providerRegistry.Has("google"),
			},
		})
	})

	mux.HandleFunc("POST /v1/team/run", func(w http.ResponseWriter, r *http.Request) {
		var req yarruntime.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}

		tenantID := r.Header.Get("X-YAR-Tenant-ID")
		userID := r.Header.Get("X-YAR-User-ID")
		if os.Getenv("YAR_DEV_MODE") == "true" {
			if tenantID == "" {
				tenantID = devTenantID
			}
			if userID == "" {
				userID = devUserID
			}
		}
		if tenantID == "" || userID == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "identity required"})
			return
		}
		req.TenantID = tenantID
		req.UserID = userID

		result, err := runtime.Run(r.Context(), req)
		if err != nil {
			log.Printf("run failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /v1/dev/tools/execute", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("YAR_DEV_MODE") != "true" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			AgentID        string          `json:"agent_id"`
			ToolID         string          `json:"tool_id"`
			TaskID         string          `json:"task_id"`
			IdempotencyKey string          `json:"idempotency_key"`
			ApprovalID     string          `json:"approval_id"`
			Arguments      json.RawMessage `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}
		result, err := toolExecutor.Execute(r.Context(), tools.Invocation{
			TenantID:       devTenantID,
			UserID:         devUserID,
			TaskID:         body.TaskID,
			AgentID:        body.AgentID,
			ToolID:         body.ToolID,
			Arguments:      body.Arguments,
			IdempotencyKey: body.IdempotencyKey,
			ApprovalID:     body.ApprovalID,
		})
		if err != nil {
			var approvalErr *tools.ApprovalRequiredError
			if errors.As(err, &approvalErr) {
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":       "APPROVAL_REQUIRED",
					"approval_id": approvalErr.ApprovalID,
				})
				return
			}
			if errors.Is(err, tools.ErrToolDenied) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "TOOL_DENIED"})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /v1/dev/approvals/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("YAR_DEV_MODE") != "true" {
			http.NotFound(w, r)
			return
		}
		approvalID := r.PathValue("id")
		if err := approvalRepo.Resolve(r.Context(), devTenantID, devUserID, approvalID, approval.StatusApproved); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"approved": true, "approval_id": approvalID})
	})

	addr := os.Getenv("YAR_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("YAR listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown failed: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
