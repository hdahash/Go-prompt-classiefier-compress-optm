// Package tokenizer provides token-count estimation for prompt text.
package tokenizer

import (
	"regexp"
	"strings"
)

// Tokenizer estimates how many LLM tokens a piece of text will consume.
type Tokenizer interface {
	CountTokens(text string) int
}

var wordSplit = regexp.MustCompile(`\S+`)

// Approximate is a dependency-free token estimator. It blends a
// character-based heuristic (~4 chars/token, the commonly cited average for
// English BPE tokenizers) with a word-based heuristic (~1.3 tokens/word) so
// it degrades gracefully for both prose and dense text like code or URLs.
// It is not a substitute for a real tokenizer when exact counts matter, but
// is accurate enough to drive compression-ratio and budget decisions.
type Approximate struct{}

func (Approximate) CountTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	words := wordSplit.FindAllString(text, -1)
	charEstimate := float64(len([]rune(text))) / 4.0
	wordEstimate := float64(len(words)) * 1.3
	tokens := int((charEstimate + wordEstimate) / 2)
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}
