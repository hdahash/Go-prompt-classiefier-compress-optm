package optimizer

import (
	"context"
	"strings"
	"testing"

	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/classifier"
	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/compressor"
	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/tokenizer"
)

func newTestOptimizer(budget int) *Optimizer {
	tok := tokenizer.Approximate{}
	return New(Config{
		Tokenizer:   tok,
		Classifier:  classifier.New(tok),
		Compressor:  compressor.NewHeuristic(tok),
		TokenBudget: budget,
	})
}

func TestOptimizeSkipsCompressionUnderBudget(t *testing.T) {
	o := newTestOptimizer(1000)

	plan, err := o.Optimize(context.Background(), "hello, how are you?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Compression != nil {
		t.Fatalf("expected no compression under budget")
	}
	if !plan.FitsBudget {
		t.Fatalf("expected prompt to fit budget")
	}
}

func TestOptimizeCompressesOverBudget(t *testing.T) {
	o := newTestOptimizer(20)

	prompt := strings.Repeat("This is a long filler sentence that pads out the prompt considerably. ", 20) + "What is 2 plus 2?"
	plan, err := o.Optimize(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Compression == nil {
		t.Fatalf("expected compression to run over budget")
	}
	if plan.Compression.CompressedTokens >= plan.Compression.OriginalTokens {
		t.Fatalf("expected fewer tokens after compression: original=%d compressed=%d",
			plan.Compression.OriginalTokens, plan.Compression.CompressedTokens)
	}
}

func TestCacheKeyStableAcrossWhitespaceAndCase(t *testing.T) {
	a := CacheKey("mid", "Hello   World")
	b := CacheKey("mid", "hello world")
	if a != b {
		t.Fatalf("expected cache keys to match after normalization: %s vs %s", a, b)
	}

	c := CacheKey("large", "hello world")
	if a == c {
		t.Fatalf("expected cache keys to differ across tiers")
	}
}
