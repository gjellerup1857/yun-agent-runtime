package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjellerup1857/yun-agent-runtime/internal/memory"
	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
	"github.com/gjellerup1857/yun-agent-runtime/internal/task"
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
	router := routing.New()
	llm := provider.NewMock()
	runtime := yarruntime.NewStateful(router, llm, memoryRepo, taskRepo, memory.NewExtractor())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(checkCtx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "database": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "database": "ok", "provider": llm.Name()})
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
			if tenantID == "" { tenantID = devTenantID }
			if userID == "" { userID = devUserID }
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

	addr := os.Getenv("YAR_ADDR")
	if addr == "" { addr = ":8080" }
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
