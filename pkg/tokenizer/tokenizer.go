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
// URLs. The character heuristic is script-aware (see charsPerToken): most
// BPE vocabularies are English-centric and fragment non-Latin scripts into
// more subword tokens per character, sometimes drastically more (CJK). It
// is not a substitute for a real tokenizer when exact counts matter, but is
// accurate enough to drive compression-ratio and budget decisions across
// scripts.
type Approximate struct{}

func (Approximate) CountTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	counts := make(map[script]float64, len(charsPerToken))
	var totalChars, spacelessChars float64
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		s := scriptOf(r)
		counts[s]++
		totalChars++
		if s == scriptCJK || s == scriptThai {
			spacelessChars++
		}
	}

	var charEstimate float64
	for s, n := range counts {
		charEstimate += n / charsPerToken[s]
	}

	// CJK and Thai are conventionally written without spaces between
	// words, so a whitespace-derived word count is meaningless for them:
	// lean on the character estimate alone once they dominate the text
	// instead of diluting it with a near-empty word count.
	if totalChars > 0 && spacelessChars/totalChars > 0.3 {
		tokens := int(charEstimate)
		if tokens < 1 {
			tokens = 1
		}
		return tokens
	}

	words := wordSplit.FindAllString(text, -1)
	wordEstimate := float64(len(words)) * 1.3
	tokens := int((charEstimate + wordEstimate) / 2)
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// script buckets characters into broad Unicode script families that tend to
// fragment similarly under English-centric BPE vocabularies.
type script int

const (
	scriptLatin script = iota // default bucket: Latin and anything unrecognized
	scriptArabic
	scriptHebrew
	scriptCJK
	scriptCyrillic
	scriptDevanagari
	scriptThai
)

// charsPerToken is an approximate, commonly-cited average
// characters-per-BPE-token ratio per script family, based on how
// English-centric tokenizers (GPT/Claude-style) typically fragment other
// scripts. These are heuristics, not measurements against any specific
// tokenizer, and exist to keep budget/compression-ratio decisions roughly
// sane across languages rather than to produce exact counts.
var charsPerToken = map[script]float64{
	scriptLatin:      4.0,
	scriptArabic:     2.2,
	scriptHebrew:     2.2,
	scriptCJK:        1.0,
	scriptCyrillic:   2.6,
	scriptDevanagari: 1.8,
	scriptThai:       1.7,
}

func scriptOf(r rune) script {
	switch {
	case isArabicScript(r):
		return scriptArabic
	case isHebrewScript(r):
		return scriptHebrew
	case isCJKScript(r):
		return scriptCJK
	case isCyrillicScript(r):
		return scriptCyrillic
	case isDevanagariScript(r):
		return scriptDevanagari
	case isThaiScript(r):
		return scriptThai
	default:
		return scriptLatin
	}
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

// isHebrewScript reports whether r falls in a Unicode block used by
// Hebrew-script text.
func isHebrewScript(r rune) bool {
	return (r >= 0x0590 && r <= 0x05FF) || // Hebrew
		(r >= 0xFB1D && r <= 0xFB4F) // Hebrew presentation forms
}

// isCJKScript reports whether r falls in a Unicode block used by Chinese,
// Japanese, or Korean text (Han ideographs, Hiragana, Katakana, Hangul).
func isCJKScript(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
		(r >= 0x1100 && r <= 0x11FF) // Hangul Jamo
}

// isCyrillicScript reports whether r falls in a Unicode block used by
// Cyrillic-script text (Russian, Ukrainian, Bulgarian, and related
// languages).
func isCyrillicScript(r rune) bool {
	return (r >= 0x0400 && r <= 0x04FF) || // Cyrillic
		(r >= 0x0500 && r <= 0x052F) // Cyrillic Supplement
}

// isDevanagariScript reports whether r falls in the Unicode block used by
// Devanagari-script text (Hindi, Marathi, Nepali, Sanskrit).
func isDevanagariScript(r rune) bool {
	return r >= 0x0900 && r <= 0x097F
}

// isThaiScript reports whether r falls in the Unicode block used by Thai
// text.
func isThaiScript(r rune) bool {
	return r >= 0x0E00 && r <= 0x0E7F
}

// IsRTL reports whether text should be treated as right-to-left, using a
// simplified version of the Unicode Bidirectional Algorithm's "first strong
// character" heuristic (the same rule HTML's dir="auto" uses): it scans for
// the first character with strong directionality and reports whether that
// direction is right-to-left. Digits, punctuation, and whitespace are
// direction-neutral and skipped. Of the scripts this package recognizes,
// Arabic and Hebrew are right-to-left; the rest are left-to-right.
func IsRTL(text string) bool {
	for _, r := range text {
		switch {
		case isArabicScript(r), isHebrewScript(r):
			return true
		case unicode.IsLetter(r):
			return false
		}
	}
	return false
}
