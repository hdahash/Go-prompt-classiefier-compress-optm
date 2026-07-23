// Package compressor reduces the token footprint of a prompt while trying
// to preserve its meaning. Implementations range from a dependency-free
// local heuristic to a client for an external model-backed compression
// service.
package compressor

import (
	"context"
	"time"
)

// Options configures a single compression call.
type Options struct {
	// TargetRatio is the desired fraction (0,1] of tokens to retain.
	// 1.0 keeps everything except obvious redundancy; smaller values
	// compress harder. Values outside (0,1] fall back to 0.75.
	TargetRatio float64
	// Preserve holds substrings that must never be dropped, e.g. a
	// specific instruction or delimiter the caller depends on downstream.
	Preserve []string
}

func (o Options) normalized() Options {
	if o.TargetRatio <= 0 || o.TargetRatio > 1 {
		o.TargetRatio = 0.75
	}
	return o
}

// Result is the outcome of compressing a single prompt.
type Result struct {
	Compressed       string        `json:"compressed_prompt"`
	OriginalTokens   int           `json:"original_tokens"`
	CompressedTokens int           `json:"compressed_tokens"`
	Ratio            float64       `json:"compression_ratio"`
	Latency          time.Duration `json:"-"`
	LatencyMS        float64       `json:"latency_ms"`
}

// Compressor reduces a prompt's token footprint.
type Compressor interface {
	Compress(ctx context.Context, prompt string, opts Options) (Result, error)
}
