package classifier

import (
	"regexp"
	"strings"
)

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

// questionMarks lists sentence-final question-mark variants recognized
// across scripts: Latin "?", Arabic/Persian/Urdu "؟", full-width CJK "？",
// and Ethiopic "፧". Greek's question mark is a semicolon, which is
// deliberately excluded since treating ";" as a question mark would badly
// misfire on English prose and code.
const questionMarks = "?؟？፧"

// stepWords are per-language words for "step" (as in "step 1, step 2"),
// used as a complexity signal alongside code-block and question density.
// English is matched case-insensitively by the caller; the rest are not,
// since the scripts involved have no case.
var stepWords = []string{"خطوة", "שלב"}

func countAnyRune(s, runes string) int {
	n := 0
	for _, r := range s {
		if strings.ContainsRune(runes, r) {
			n++
		}
	}
	return n
}

// Go's regexp \b word-boundary assertion is ASCII-only (confirmed: it does
// not fire around Arabic or Hebrew letters), so non-Latin patterns below
// are plain substring matches instead of \b-delimited ones. False
// positives from partial-word matches are an accepted tradeoff given the
// phrase lengths involved.
//
// Domain/intent detection only has keyword rules for the languages listed
// here (English, Arabic, Hebrew); other languages still classify, just
// without keyword-driven domain/intent — they fall back to DomainChat with
// complexity and tier decided by script-agnostic structural signals (length,
// code blocks, question density). Add another language by appending its
// keywords to the relevant rule below.
var domainRules = []domainRule{
	{DomainCode, compileAll(
		`\bfunc\b`, `\bclass\b`, `\bimport\b`, "```", `\bdef\b`, `\bstack ?trace\b`,
		`\berror:`, `\bcompile\b`, `\bnil pointer\b`, `\bsegfault\b`, `\brefactor\b`,
		`\bunit test\b`, `\bgolang\b`, `\bpython\b`, `\bjavascript\b`, `\btypescript\b`, `\brust\b`,
		`برمجة`, `الكود`, `الشيفرة`, `دالة`, `خطأ برمجي`, `خطأ في الكود`, `تصحيح الأخطاء`, `استثناء`,
		`תכנות`, `קוד`, `פונקציה`, `שגיאה`, `באג`, `לתקן`, `דיבוג`,
	), 1.0},
	{DomainMath, compileAll(
		`\d+\s*[+\-*/^]\s*\d+`, `\bequation\b`, `\bderivative\b`, `\bintegral\b`,
		`\bsolve for\b`, `\bmatrix\b`, `\bprobability\b`, `\btheorem\b`,
		`معادلة`, `مشتقة`, `تكامل`, `مصفوفة`, `احتمال`, `نظرية`, `حل المعادلة`,
		`משוואה`, `נגזרת`, `אינטגרל`, `מטריצה`, `הסתברות`, `משפט`, `פתור את המשוואה`,
	), 1.0},
	{DomainCreative, compileAll(
		`\bwrite a (poem|story|song|novel)\b`, `\bonce upon a time\b`,
		`\bbrainstorm\b`, `\bcreative writing\b`, `\bshort story\b`,
		`اكتب قصة`, `اكتب قصيدة`, `كان يا ما كان`, `عصف ذهني`, `كتابة إبداعية`, `قصة قصيرة`,
		`כתוב סיפור`, `כתוב שיר`, `היה היה`, `סיעור מוחות`, `כתיבה יצירתית`, `סיפור קצר`,
	), 1.0},
	{DomainAnalysis, compileAll(
		`\bcompare\b`, `\banalyz(e|is)\b`, `\bpros and cons\b`, `\bsummariz(e|ation)\b`,
		`\btrend\b`, `\bevaluate\b`, `\breport\b`,
		`قارن`, `حلل`, `إيجابيات وسلبيات`, `لخص`, `اتجاه`, `قيّم`, `تقرير`,
		`השווה`, `נתח`, `יתרונות וחסרונות`, `סכם`, `מגמה`, `העריך`, `דוח`,
	), 1.0},
	{DomainQA, compileAll(
		`^\s*(what|why|how|when|where|who|which)\b`, `[`+questionMarks+`]\s*$`,
		`^\s*(ما|ماذا|لماذا|كيف|متى|أين|من|أي)\s`,
		`^\s*(מה|למה|איך|מתי|איפה|מי|איזה)\s`,
	), 0.6},
}

var intentRules = []struct {
	intent   string
	patterns []*regexp.Regexp
}{
	{"debug", compileAll(
		`\bfix\b`, `\bdebug\b`, `\bbug\b`, `\berror\b`, `\bnot working\b`, `\bcrash(es|ing)?\b`,
		`أصلح`, `صحح`, `تصحيح`, `عطل`, `لا يعمل`, `يتعطل`, `يتوقف`,
		`לתקן`, `דיבוג`, `באג`, `שגיאה`, `לא עובד`, `קורס`, `נתקע`,
	)},
	{"summarize", compileAll(
		`\bsummariz(e|ation)\b`, `\btl;?dr\b`, `\bshorten\b`, `\bcondense\b`,
		`لخص`, `تلخيص`, `اختصر`, `باختصار`,
		`סכם`, `תמצת`, `בקיצור`,
	)},
	{"translate", compileAll(`\btranslate\b`, `ترجم`, `ترجمة`, `תרגם`)},
	{"extract", compileAll(`\bextract\b`, `\bparse\b`, `\bpull out\b`, `استخرج`, `استخلص`, `חלץ`, `לשלוף`)},
	{"explain", compileAll(
		`\bexplain\b`, `\bwhat is\b`, `\bhow does\b`, `\bwhy\b`,
		`اشرح`, `وضح`, `ما هو`, `ما هي`, `كيف يعمل`, `لماذا`,
		`הסבר`, `מה זה`, `איך זה עובד`, `למה`,
	)},
	{"generate", compileAll(
		`\bwrite\b`, `\bgenerate\b`, `\bcreate\b`, `\bdraft\b`,
		`اكتب`, `أنشئ`, `انشئ`, `أنتج`, `صمم`,
		`כתוב`, `צור`, `הפק`, `עצב`,
	)},
}
