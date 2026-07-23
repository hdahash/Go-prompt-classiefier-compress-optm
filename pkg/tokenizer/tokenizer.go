// Package tokenizer provides token-count estimation for prompt text.
package tokenizer

import (
	"regexp"
	"strings"
	"unicode"
)

// Tokenizer estimates how many LLM tokens a piece of text will consume.
type Tokenizer interface {
	CountTokens(text string) int
}

var wordSplit = regexp.MustCompile(`\S+`)

// Approximate is a dependency-free token estimator. It blends a
// character-based heuristic with a word-based heuristic (~1.3 tokens/word)
// so it degrades gracefully for both prose and dense text like code or
// URLs. The character heuristic is script-aware: Latin-script text is
// estimated at ~4 chars/token (the commonly cited average for English BPE
// tokenizers), while Arabic-script text is estimated at ~2.2 chars/token,
// since most BPE vocabularies are English-centric and fragment Arabic into
// more subword tokens per character. It is not a substitute for a real
// tokenizer when exact counts matter, but is accurate enough to drive
// compression-ratio and budget decisions across both scripts.
type Approximate struct{}

func (Approximate) CountTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	words := wordSplit.FindAllString(text, -1)

	var arabicChars, otherChars float64
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			continue
		case isArabicScript(r):
			arabicChars++
		default:
			otherChars++
		}
	}

	charEstimate := otherChars/4.0 + arabicChars/2.2
	wordEstimate := float64(len(words)) * 1.3
	tokens := int((charEstimate + wordEstimate) / 2)
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// isArabicScript reports whether r falls in a Unicode block used by
// Arabic-script text (Arabic, Persian, Urdu, and related languages).
func isArabicScript(r rune) bool {
	return (r >= 0x0600 && r <= 0x06FF) || // Arabic
		(r >= 0x0750 && r <= 0x077F) || // Arabic Supplement
		(r >= 0x08A0 && r <= 0x08FF) || // Arabic Extended-A
		(r >= 0xFB50 && r <= 0xFDFF) || // Arabic Presentation Forms-A
		(r >= 0xFE70 && r <= 0xFEFF) // Arabic Presentation Forms-B
}
