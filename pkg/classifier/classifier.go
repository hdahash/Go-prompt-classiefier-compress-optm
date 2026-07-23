// Package classifier assigns a domain, complexity, and recommended model
// tier to a prompt, so a caller can route it to the cheapest model capable
// of handling it well.
package classifier

import (
	"strings"

	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/tokenizer"
)

// Domain is the broad subject-matter category of a prompt.
type Domain string

const (
	DomainCode     Domain = "code"
	DomainMath     Domain = "math"
	DomainCreative Domain = "creative"
	DomainAnalysis Domain = "analysis"
	DomainQA       Domain = "qa"
	DomainChat     Domain = "chat"
)

// Complexity is a coarse estimate of how much reasoning a prompt requires.
type Complexity string

const (
	ComplexitySimple   Complexity = "simple"
	ComplexityModerate Complexity = "moderate"
	ComplexityComplex  Complexity = "complex"
)

// Tier is the model tier recommended for handling a prompt.
type Tier string

const (
	TierLocal Tier = "local" // small/local model, e.g. Ollama
	TierMid   Tier = "mid"   // e.g. gpt-4o-mini / claude-haiku
	TierLarge Tier = "large" // e.g. gpt-4o / claude-sonnet
)

// Result is the outcome of classifying a single prompt.
type Result struct {
	Domain          Domain             `json:"domain"`
	Complexity      Complexity         `json:"complexity"`
	Intent          string             `json:"intent"`
	RecommendedTier Tier               `json:"recommended_tier"`
	TokenCount      int                `json:"token_count"`
	Scores          map[Domain]float64 `json:"domain_scores,omitempty"`
}

// Classifier classifies prompts using weighted keyword/regex rules plus
// length- and structure-based complexity heuristics. It holds no external
// state and is safe for concurrent use.
type Classifier struct {
	tok tokenizer.Tokenizer
}

// New builds a Classifier. tok may be nil, in which case tokenizer.Approximate
// is used.
func New(tok tokenizer.Tokenizer) *Classifier {
	if tok == nil {
		tok = tokenizer.Approximate{}
	}
	return &Classifier{tok: tok}
}

// Classify inspects prompt and returns its domain, complexity, intent, and
// recommended model tier.
func (c *Classifier) Classify(prompt string) Result {
	trimmed := strings.TrimSpace(prompt)
	tokenCount := c.tok.CountTokens(trimmed)

	scores := make(map[Domain]float64, len(domainRules))
	for _, rule := range domainRules {
		var s float64
		for _, re := range rule.patterns {
			if re.MatchString(trimmed) {
				s += rule.weight
			}
		}
		if s > 0 {
			scores[rule.domain] += s
		}
	}

	domain := DomainChat
	var best float64
	for d, s := range scores {
		if s > best {
			best, domain = s, d
		}
	}

	intent := "chat"
	for _, ir := range intentRules {
		matched := false
		for _, re := range ir.patterns {
			if re.MatchString(trimmed) {
				matched = true
				break
			}
		}
		if matched {
			intent = ir.intent
			break
		}
	}

	complexity := classifyComplexity(trimmed, tokenCount)
	tier := recommendTier(domain, complexity)

	return Result{
		Domain:          domain,
		Complexity:      complexity,
		Intent:          intent,
		RecommendedTier: tier,
		TokenCount:      tokenCount,
		Scores:          scores,
	}
}

func classifyComplexity(prompt string, tokenCount int) Complexity {
	codeBlocks := strings.Count(prompt, "```")
	questions := strings.Count(prompt, "?") + strings.Count(prompt, "؟")
	steps := strings.Count(strings.ToLower(prompt), "step") + strings.Count(prompt, "خطوة")

	score := 0
	switch {
	case tokenCount > 600:
		score += 2
	case tokenCount > 150:
		score++
	}
	if codeBlocks >= 2 {
		score += 2
	}
	if questions > 2 {
		score++
	}
	if steps > 1 {
		score++
	}

	switch {
	case score >= 3:
		return ComplexityComplex
	case score >= 1:
		return ComplexityModerate
	default:
		return ComplexitySimple
	}
}

func recommendTier(domain Domain, complexity Complexity) Tier {
	if complexity == ComplexityComplex {
		return TierLarge
	}
	if complexity == ComplexitySimple && (domain == DomainChat || domain == DomainQA) {
		return TierLocal
	}
	return TierMid
}
