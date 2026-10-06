package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
)

func main() {
	router := routing.New()
	llm := provider.NewMock()
	runtime := yarruntime.New(router, llm)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"provider": llm.Name(),
		})
	})

	mux.HandleFunc("POST /v1/team/run", func(w http.ResponseWriter, r *http.Request) {
		var req yarruntime.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}
		result, err := runtime.Run(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	addr := os.Getenv("YAR_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("YAR listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
