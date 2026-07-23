package compressor

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/tokenizer"
)

// Heuristic is a dependency-free, extractive compressor. It strips filler
// phrasing, collapses whitespace, and drops the least salient sentences
// until the prompt fits the target token budget. Fenced code blocks are
// always preserved verbatim and never count against sentence dropping.
//
// It trades compression quality for zero external dependencies and
// predictable latency, making it a safe default and fallback for a
// model-backed Compressor such as HTTPCompressor.
type Heuristic struct {
	Tokenizer tokenizer.Tokenizer
}

// NewHeuristic builds a Heuristic compressor. tok may be nil, in which case
// tokenizer.Approximate is used.
func NewHeuristic(tok tokenizer.Tokenizer) *Heuristic {
	if tok == nil {
		tok = tokenizer.Approximate{}
	}
	return &Heuristic{Tokenizer: tok}
}

// Arabic entries below are plain substring matches rather than \b-delimited
// ones: Go's regexp \b word-boundary assertion is ASCII-only and does not
// fire around Arabic letters.
var fillerPhrases = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bkindly\b\s*`),
	regexp.MustCompile(`(?i)\bplease\b\s*`),
	regexp.MustCompile(`(?i)\bi would like you to\b\s*`),
	regexp.MustCompile(`(?i)\bi want you to\b\s*`),
	regexp.MustCompile(`(?i)\bjust to (be clear|clarify),?\s*`),
	regexp.MustCompile(`(?i)\bbasically\b,?\s*`),
	regexp.MustCompile(`(?i)\bactually\b,?\s*`),
	regexp.MustCompile(`(?i)\bin order to\b\s*`),
	regexp.MustCompile(`(?i)\bfeel free to\b\s*`),
	regexp.MustCompile(`(?i)\bit would be great if you could\b\s*`),
	regexp.MustCompile(`من فضلك\s*`),
	regexp.MustCompile(`لو سمحت\s*`),
	regexp.MustCompile(`أرجو منك\s*`),
	regexp.MustCompile(`أريد منك أن\s*`),
	regexp.MustCompile(`بشكل أساسي\s*`),
	regexp.MustCompile(`في الواقع\s*`),
	regexp.MustCompile(`من أجل\s*`),
	regexp.MustCompile(`لا تتردد في\s*`),
}

var (
	codeBlockRe      = regexp.MustCompile("(?s)```.*?```")
	sentenceSplitRe  = regexp.MustCompile(`[^.!?؟]+[.!?؟]+\s*|[^.!?؟]+$`)
	wordRe           = regexp.MustCompile(`[\p{L}\p{N}']+`)
	extraSpaceRe     = regexp.MustCompile(`[ \t]+`)
	extraBlankLineRe = regexp.MustCompile(`\n{3,}`)
)

func (h *Heuristic) Compress(_ context.Context, prompt string, opts Options) (Result, error) {
	start := time.Now()
	opts = opts.normalized()
	originalTokens := h.Tokenizer.CountTokens(prompt)

	if strings.TrimSpace(prompt) == "" {
		return Result{Latency: time.Since(start)}, nil
	}

	// Pull out code blocks so they're immune to filler-stripping and are
	// never candidates for dropping.
	codeBlocks := codeBlockRe.FindAllString(prompt, -1)
	withPlaceholders := prompt
	placeholders := make([]string, len(codeBlocks))
	for i, block := range codeBlocks {
		placeholders[i] = fmt.Sprintf("\x00CODE_BLOCK_%d\x00", i)
		withPlaceholders = strings.Replace(withPlaceholders, block, placeholders[i], 1)
	}

	cleaned := withPlaceholders
	for _, re := range fillerPhrases {
		cleaned = re.ReplaceAllString(cleaned, "")
	}
	cleaned = extraSpaceRe.ReplaceAllString(cleaned, " ")
	cleaned = extraBlankLineRe.ReplaceAllString(cleaned, "\n\n")

	codeBudgetTokens := 0
	for _, block := range codeBlocks {
		codeBudgetTokens += h.Tokenizer.CountTokens(block)
	}
	targetTokens := int(float64(originalTokens) * opts.TargetRatio)
	if targetTokens < codeBudgetTokens {
		targetTokens = codeBudgetTokens
	}

	sentences := dedupeSentences(sentenceSplitRe.FindAllString(cleaned, -1))
	freq := termFrequency(sentences)

	type scoredSentence struct {
		text  string
		score float64
		toks  int
	}
	items := make([]scoredSentence, len(sentences))
	for i, s := range sentences {
		items[i] = scoredSentence{
			text:  s,
			toks:  h.Tokenizer.CountTokens(s),
			score: salience(s, freq, opts.Preserve),
		}
	}

	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return items[order[a]].score > items[order[b]].score })

	budget := targetTokens - codeBudgetTokens
	keep := make(map[int]bool, len(items))
	used := 0
	for _, i := range order {
		if used > 0 && used+items[i].toks > budget {
			continue
		}
		keep[i] = true
		used += items[i].toks
	}
	if len(keep) == 0 && len(items) > 0 {
		keep[order[0]] = true
	}

	var b strings.Builder
	for i, it := range items {
		if keep[i] {
			b.WriteString(it.text)
		}
	}
	result := strings.TrimSpace(b.String())

	for i, ph := range placeholders {
		result = strings.Replace(result, ph, codeBlocks[i], 1)
	}

	compressedTokens := h.Tokenizer.CountTokens(result)
	ratio := 1.0
	if originalTokens > 0 {
		ratio = float64(compressedTokens) / float64(originalTokens)
	}
	latency := time.Since(start)

	return Result{
		Compressed:       result,
		OriginalTokens:   originalTokens,
		CompressedTokens: compressedTokens,
		Ratio:            ratio,
		Latency:          latency,
		LatencyMS:        float64(latency.Microseconds()) / 1000.0,
	}, nil
}

// dedupeSentences drops exact repeats (modulo whitespace/case), keeping the
// first occurrence. Verbatim-repeated instructions are common in pasted
// prompts and carry no marginal information after the first copy.
func dedupeSentences(sentences []string) []string {
	seen := make(map[string]bool, len(sentences))
	out := make([]string, 0, len(sentences))
	for _, s := range sentences {
		key := strings.ToLower(strings.Join(strings.Fields(s), " "))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

func termFrequency(sentences []string) map[string]int {
	freq := make(map[string]int)
	for _, s := range sentences {
		for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
			if stopwords[w] {
				continue
			}
			freq[w]++
		}
	}
	return freq
}

func salience(sentence string, freq map[string]int, preserve []string) float64 {
	words := wordRe.FindAllString(strings.ToLower(sentence), -1)
	var sum float64
	var count int
	for _, w := range words {
		if stopwords[w] {
			continue
		}
		sum += float64(freq[w])
		count++
	}
	score := 0.0
	if count > 0 {
		score = sum / float64(count)
	}
	if strings.ContainsAny(sentence, "?؟") {
		score *= 1.15
	}
	for _, p := range preserve {
		if p != "" && strings.Contains(sentence, p) {
			score += 1000 // force-keep
		}
	}
	return score
}

var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "on": true, "is": true, "are": true, "it": true,
	"that": true, "this": true, "for": true, "with": true, "as": true,
	"be": true, "was": true, "were": true, "i": true, "you": true, "we": true,
	// Arabic
	"في": true, "من": true, "إلى": true, "على": true, "عن": true,
	"هذا": true, "هذه": true, "ذلك": true, "التي": true, "الذي": true,
	"و": true, "أو": true, "لكن": true, "كما": true, "حتى": true,
	"إذا": true, "أن": true, "إن": true, "كان": true, "يكون": true,
	"لم": true, "لن": true, "قد": true, "هو": true, "هي": true,
	"هم": true, "نحن": true, "أنت": true, "مع": true, "بين": true,
	"عند": true, "بعد": true, "قبل": true,
}
