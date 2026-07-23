// Command promptoptd exposes classification, compression, and optimization
// over HTTP so non-Go callers (e.g. the FastAPI backend in hugeai) can use
// them as a sidecar service, the same way go-proxy exposes streaming.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/classifier"
	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/compressor"
	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/optimizer"
	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/tokenizer"
)

const defaultPort = "8003"

var (
	tok       tokenizer.Tokenizer = tokenizer.Approximate{}
	classify                      = classifier.New(tok)
	localComp                     = compressor.NewHeuristic(tok)
	comp                          = buildCompressor()

	requestCount uint64
	errorCount   uint64
	startTime    = time.Now()
)

// buildCompressor wires in a remote model-backed compressor (e.g. the
// LLMLingua-2 optimizer-service) when REMOTE_COMPRESSOR_URL is set, falling
// back to the local heuristic compressor on any remote error.
func buildCompressor() compressor.Compressor {
	endpoint := os.Getenv("REMOTE_COMPRESSOR_URL")
	if endpoint == "" {
		return localComp
	}
	remote := compressor.NewHTTPCompressor(endpoint, nil)
	return compressor.Fallback{Primary: remote, Secondary: localComp}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	atomic.AddUint64(&errorCount, 1)
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodePrompt(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return "", false
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "missing prompt")
		return "", false
	}
	return req.Prompt, true
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": time.Since(startTime).Seconds(),
		"requests_total": atomic.LoadUint64(&requestCount),
		"errors_total":   atomic.LoadUint64(&errorCount),
		"version":        "0.1.0",
	})
}

func handleClassify(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&requestCount, 1)
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	prompt, ok := decodePrompt(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, classify.Classify(prompt))
}

func handleCompress(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&requestCount, 1)
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		Prompt           string  `json:"prompt"`
		TargetTokenRatio float64 `json:"target_token_ratio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "missing prompt")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res, err := comp.Compress(ctx, req.Prompt, compressor.Options{TargetRatio: req.TargetTokenRatio})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func handleOptimize(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&requestCount, 1)
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		Prompt      string `json:"prompt"`
		TokenBudget int    `json:"token_budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "missing prompt")
		return
	}

	opt := optimizer.New(optimizer.Config{
		Classifier:  classify,
		Compressor:  comp,
		Tokenizer:   tok,
		TokenBudget: req.TokenBudget,
	})

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	plan, err := opt.Optimize(ctx, req.Prompt)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "go-prompt-classiefier-compress-optm",
		"version": "0.1.0",
		"docs":    "POST /classify, /compress, /optimize with a JSON body containing \"prompt\"",
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	port := os.Getenv("PROMPTOPTD_PORT")
	if port == "" {
		port = defaultPort
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/classify", handleClassify)
	mux.HandleFunc("/compress", handleCompress)
	mux.HandleFunc("/optimize", handleOptimize)

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("promptoptd starting on %s", addr)

	server := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
