package compressor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPCompressor calls an external compression endpoint that speaks the
// {prompt, target_token_ratio} -> {compressed_prompt, original_tokens,
// compressed_tokens, compression_ratio, latency_ms} contract, e.g. a
// LLMLingua-2-backed microservice. This lets a Go caller reuse a
// higher-quality, model-backed compressor without linking a Python runtime
// into the binary.
type HTTPCompressor struct {
	Endpoint string
	Client   *http.Client
}

// NewHTTPCompressor builds an HTTPCompressor targeting endpoint. client may
// be nil, in which case a client with a 10s timeout is used.
func NewHTTPCompressor(endpoint string, client *http.Client) *HTTPCompressor {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPCompressor{Endpoint: endpoint, Client: client}
}

type remoteRequest struct {
	Prompt           string  `json:"prompt"`
	TargetTokenRatio float64 `json:"target_token_ratio"`
}

type remoteResponse struct {
	CompressedPrompt string  `json:"compressed_prompt"`
	OriginalTokens   int     `json:"original_tokens"`
	CompressedTokens int     `json:"compressed_tokens"`
	CompressionRatio float64 `json:"compression_ratio"`
	LatencyMS        float64 `json:"latency_ms"`
}

func (h *HTTPCompressor) Compress(ctx context.Context, prompt string, opts Options) (Result, error) {
	opts = opts.normalized()
	body, err := json.Marshal(remoteRequest{Prompt: prompt, TargetTokenRatio: opts.TargetRatio})
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := h.Client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("compress request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Result{}, fmt.Errorf("compress service returned %d: %s", resp.StatusCode, string(data))
	}

	var rr remoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}
	latency := time.Since(start)

	return Result{
		Compressed:       rr.CompressedPrompt,
		OriginalTokens:   rr.OriginalTokens,
		CompressedTokens: rr.CompressedTokens,
		Ratio:            rr.CompressionRatio,
		Latency:          latency,
		LatencyMS:        rr.LatencyMS,
	}, nil
}

// Fallback tries Primary and, if it errors, retries with Secondary.
// The typical use is HTTPCompressor as Primary (a remote model-backed
// service) with Heuristic as Secondary, so compression degrades gracefully
// rather than failing outright when the remote service is unavailable.
type Fallback struct {
	Primary   Compressor
	Secondary Compressor
}

func (f Fallback) Compress(ctx context.Context, prompt string, opts Options) (Result, error) {
	res, err := f.Primary.Compress(ctx, prompt, opts)
	if err == nil {
		return res, nil
	}
	if f.Secondary == nil {
		return Result{}, err
	}
	return f.Secondary.Compress(ctx, prompt, opts)
}
