package classifier

import "regexp"

type domainRule struct {
	domain   Domain
	patterns []*regexp.Regexp
	weight   float64
}

func compileAll(patterns ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(`(?i)`+p))
	}
	return out
}

var domainRules = []domainRule{
	{DomainCode, compileAll(
		`\bfunc\b`, `\bclass\b`, `\bimport\b`, "```", `\bdef\b`, `\bstack ?trace\b`,
		`\berror:`, `\bcompile\b`, `\bnil pointer\b`, `\bsegfault\b`, `\brefactor\b`,
		`\bunit test\b`, `\bgolang\b`, `\bpython\b`, `\bjavascript\b`, `\btypescript\b`, `\brust\b`,
	), 1.0},
	{DomainMath, compileAll(
		`\d+\s*[+\-*/^]\s*\d+`, `\bequation\b`, `\bderivative\b`, `\bintegral\b`,
		`\bsolve for\b`, `\bmatrix\b`, `\bprobability\b`, `\btheorem\b`,
	), 1.0},
	{DomainCreative, compileAll(
		`\bwrite a (poem|story|song|novel)\b`, `\bonce upon a time\b`,
		`\bbrainstorm\b`, `\bcreative writing\b`, `\bshort story\b`,
	), 1.0},
	{DomainAnalysis, compileAll(
		`\bcompare\b`, `\banalyz(e|is)\b`, `\bpros and cons\b`, `\bsummariz(e|ation)\b`,
		`\btrend\b`, `\bevaluate\b`, `\breport\b`,
	), 1.0},
	{DomainQA, compileAll(
		`^\s*(what|why|how|when|where|who|which)\b`, `\?\s*$`,
	), 0.6},
}

var intentRules = []struct {
	intent   string
	patterns []*regexp.Regexp
}{
	{"debug", compileAll(`\bfix\b`, `\bdebug\b`, `\bbug\b`, `\berror\b`, `\bnot working\b`, `\bcrash(es|ing)?\b`)},
	{"summarize", compileAll(`\bsummariz(e|ation)\b`, `\btl;?dr\b`, `\bshorten\b`, `\bcondense\b`)},
	{"translate", compileAll(`\btranslate\b`)},
	{"extract", compileAll(`\bextract\b`, `\bparse\b`, `\bpull out\b`)},
	{"explain", compileAll(`\bexplain\b`, `\bwhat is\b`, `\bhow does\b`, `\bwhy\b`)},
	{"generate", compileAll(`\bwrite\b`, `\bgenerate\b`, `\bcreate\b`, `\bdraft\b`)},
}

var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "on": true, "is": true, "are": true, "it": true,
	"that": true, "this": true, "for": true, "with": true, "as": true,
	"be": true, "was": true, "were": true, "i": true, "you": true, "we": true,
}
