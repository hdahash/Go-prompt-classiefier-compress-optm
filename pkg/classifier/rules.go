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

// Go's regexp \b word-boundary assertion is ASCII-only (confirmed: it does
// not fire around Arabic letters), so Arabic patterns below are plain
// substring matches instead of \b-delimited ones. False positives from
// partial-word matches are an accepted tradeoff given the phrase lengths
// involved.
var domainRules = []domainRule{
	{DomainCode, compileAll(
		`\bfunc\b`, `\bclass\b`, `\bimport\b`, "```", `\bdef\b`, `\bstack ?trace\b`,
		`\berror:`, `\bcompile\b`, `\bnil pointer\b`, `\bsegfault\b`, `\brefactor\b`,
		`\bunit test\b`, `\bgolang\b`, `\bpython\b`, `\bjavascript\b`, `\btypescript\b`, `\brust\b`,
		`برمجة`, `الكود`, `الشيفرة`, `دالة`, `خطأ برمجي`, `خطأ في الكود`, `تصحيح الأخطاء`, `استثناء`,
	), 1.0},
	{DomainMath, compileAll(
		`\d+\s*[+\-*/^]\s*\d+`, `\bequation\b`, `\bderivative\b`, `\bintegral\b`,
		`\bsolve for\b`, `\bmatrix\b`, `\bprobability\b`, `\btheorem\b`,
		`معادلة`, `مشتقة`, `تكامل`, `مصفوفة`, `احتمال`, `نظرية`, `حل المعادلة`,
	), 1.0},
	{DomainCreative, compileAll(
		`\bwrite a (poem|story|song|novel)\b`, `\bonce upon a time\b`,
		`\bbrainstorm\b`, `\bcreative writing\b`, `\bshort story\b`,
		`اكتب قصة`, `اكتب قصيدة`, `كان يا ما كان`, `عصف ذهني`, `كتابة إبداعية`, `قصة قصيرة`,
	), 1.0},
	{DomainAnalysis, compileAll(
		`\bcompare\b`, `\banalyz(e|is)\b`, `\bpros and cons\b`, `\bsummariz(e|ation)\b`,
		`\btrend\b`, `\bevaluate\b`, `\breport\b`,
		`قارن`, `حلل`, `إيجابيات وسلبيات`, `لخص`, `اتجاه`, `قيّم`, `تقرير`,
	), 1.0},
	{DomainQA, compileAll(
		`^\s*(what|why|how|when|where|who|which)\b`, `[?؟]\s*$`,
		`^\s*(ما|ماذا|لماذا|كيف|متى|أين|من|أي)\s`,
	), 0.6},
}

var intentRules = []struct {
	intent   string
	patterns []*regexp.Regexp
}{
	{"debug", compileAll(
		`\bfix\b`, `\bdebug\b`, `\bbug\b`, `\berror\b`, `\bnot working\b`, `\bcrash(es|ing)?\b`,
		`أصلح`, `صحح`, `تصحيح`, `عطل`, `لا يعمل`, `يتعطل`, `يتوقف`,
	)},
	{"summarize", compileAll(
		`\bsummariz(e|ation)\b`, `\btl;?dr\b`, `\bshorten\b`, `\bcondense\b`,
		`لخص`, `تلخيص`, `اختصر`, `باختصار`,
	)},
	{"translate", compileAll(`\btranslate\b`, `ترجم`, `ترجمة`)},
	{"extract", compileAll(`\bextract\b`, `\bparse\b`, `\bpull out\b`, `استخرج`, `استخلص`)},
	{"explain", compileAll(
		`\bexplain\b`, `\bwhat is\b`, `\bhow does\b`, `\bwhy\b`,
		`اشرح`, `وضح`, `ما هو`, `ما هي`, `كيف يعمل`, `لماذا`,
	)},
	{"generate", compileAll(
		`\bwrite\b`, `\bgenerate\b`, `\bcreate\b`, `\bdraft\b`,
		`اكتب`, `أنشئ`, `انشئ`, `أنتج`, `صمم`,
	)},
}
