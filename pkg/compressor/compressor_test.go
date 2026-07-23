package compressor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/tokenizer"
)

func TestHeuristicCompressReducesTokens(t *testing.T) {
	tok := tokenizer.Approximate{}
	h := NewHeuristic(tok)

	prompt := strings.Repeat("Please kindly note that this is a filler sentence that repeats unimportant context. ", 8) +
		"What is the capital of France?"

	res, err := h.Compress(context.Background(), prompt, Options{TargetRatio: 0.4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CompressedTokens >= res.OriginalTokens {
		t.Fatalf("expected compression to reduce tokens: original=%d compressed=%d", res.OriginalTokens, res.CompressedTokens)
	}
	if !strings.Contains(res.Compressed, "capital of France") {
		t.Fatalf("expected the salient question to survive compression, got: %q", res.Compressed)
	}
}

func TestHeuristicPreservesCodeBlocks(t *testing.T) {
	tok := tokenizer.Approximate{}
	h := NewHeuristic(tok)
	code := "```go\nfunc main() { fmt.Println(\"hi\") }\n```"
	prompt := strings.Repeat("This is filler text that should be dropped when compressing hard. ", 10) + code

	res, err := h.Compress(context.Background(), prompt, Options{TargetRatio: 0.3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Compressed, code) {
		t.Fatalf("expected code block to survive compression verbatim, got: %q", res.Compressed)
	}
}

func TestHeuristicRespectsPreserve(t *testing.T) {
	tok := tokenizer.Approximate{}
	h := NewHeuristic(tok)
	mustKeep := "Respond only in JSON."
	prompt := strings.Repeat("This is unrelated filler text padding out the prompt. ", 10) + mustKeep

	res, err := h.Compress(context.Background(), prompt, Options{TargetRatio: 0.2, Preserve: []string{mustKeep}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Compressed, mustKeep) {
		t.Fatalf("expected preserved instruction to survive compression, got: %q", res.Compressed)
	}
}

type failingCompressor struct{}

func (failingCompressor) Compress(context.Context, string, Options) (Result, error) {
	return Result{}, errors.New("primary always fails")
}

func TestFallbackUsesSecondaryOnError(t *testing.T) {
	fb := Fallback{Primary: failingCompressor{}, Secondary: NewHeuristic(tokenizer.Approximate{})}

	res, err := fb.Compress(context.Background(), "hello there, this is a test prompt", Options{TargetRatio: 0.5})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if res.Compressed == "" {
		t.Fatalf("expected non-empty compressed output from fallback")
	}
}
