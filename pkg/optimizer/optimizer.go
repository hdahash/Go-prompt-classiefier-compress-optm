// Package optimizer ties together classification and compression into a
// single decision: does this prompt need to be shrunk to fit a token
// budget, and if so, by how much and via which backend.
package optimizer

import (
	"context"
	"fmt"

	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/classifier"
	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/compressor"
	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/tokenizer"
)

// Config wires the classifier, compressor, and tokenizer an Optimizer uses.
// All fields are optional; New fills in sane defaults.
type Config struct {
	Classifier *classifier.Classifier
	Compressor compressor.Compressor
	Tokenizer  tokenizer.Tokenizer

	// TokenBudget is the max tokens the final prompt should occupy. 0 means
	// no forced compression: the prompt is classified but left untouched.
	TokenBudget int

	// MinRatio floors how aggressively the compressor is allowed to cut,
	// regardless of how tight TokenBudget is. Defaults to 0.4 if unset.
	MinRatio float64
}

// Plan is the result of running Optimize on a prompt.
type Plan struct {
	Classification classifier.Result  `json:"classification"`
	Compression    *compressor.Result `json:"compression,omitempty"`
	CacheKey       string             `json:"cache_key"`
	FinalPrompt    string             `json:"final_prompt"`
	FitsBudget     bool               `json:"fits_budget"`
}

// Optimizer classifies a prompt and compresses it toward a token budget
// when it doesn't already fit.
type Optimizer struct {
	cfg Config
}

// New builds an Optimizer, defaulting any unset Config fields.
func New(cfg Config) *Optimizer {
	if cfg.Tokenizer == nil {
		cfg.Tokenizer = tokenizer.Approximate{}
	}
	if cfg.Classifier == nil {
		cfg.Classifier = classifier.New(cfg.Tokenizer)
	}
	if cfg.Compressor == nil {
		cfg.Compressor = compressor.NewHeuristic(cfg.Tokenizer)
	}
	if cfg.MinRatio <= 0 {
		cfg.MinRatio = 0.4
	}
	return &Optimizer{cfg: cfg}
}

// Optimize classifies prompt and, if it exceeds the configured TokenBudget,
// compresses it toward that budget (never below MinRatio of the original
// size) using the configured Compressor.
func (o *Optimizer) Optimize(ctx context.Context, prompt string) (Plan, error) {
	class := o.cfg.Classifier.Classify(prompt)

	plan := Plan{
		Classification: class,
		FinalPrompt:    prompt,
		CacheKey:       CacheKey(string(class.RecommendedTier), prompt),
		FitsBudget:     o.cfg.TokenBudget <= 0 || class.TokenCount <= o.cfg.TokenBudget,
	}
	if plan.FitsBudget {
		return plan, nil
	}

	ratio := float64(o.cfg.TokenBudget) / float64(class.TokenCount)
	if ratio > 1 {
		ratio = 1
	}
	if ratio < o.cfg.MinRatio {
		ratio = o.cfg.MinRatio
	}

	res, err := o.cfg.Compressor.Compress(ctx, prompt, compressor.Options{TargetRatio: ratio})
	if err != nil {
		return plan, fmt.Errorf("compress: %w", err)
	}

	plan.Compression = &res
	plan.FinalPrompt = res.Compressed
	plan.FitsBudget = res.CompressedTokens <= o.cfg.TokenBudget
	plan.CacheKey = CacheKey(string(class.RecommendedTier), res.Compressed)
	return plan, nil
}
