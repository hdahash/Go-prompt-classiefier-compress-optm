package tokenizer

import "testing"

func TestApproximateCountTokens(t *testing.T) {
	tok := Approximate{}

	if got := tok.CountTokens(""); got != 0 {
		t.Fatalf("empty text: want 0, got %d", got)
	}
	if got := tok.CountTokens("   \n\t  "); got != 0 {
		t.Fatalf("whitespace-only text: want 0, got %d", got)
	}

	short := tok.CountTokens("hello world")
	long := tok.CountTokens("hello world, this is a considerably longer sentence with many more words in it")

	if short <= 0 {
		t.Fatalf("expected positive token count for short text, got %d", short)
	}
	if long <= short {
		t.Fatalf("expected longer text to yield more tokens: short=%d long=%d", short, long)
	}
}

func TestApproximateCountTokensArabic(t *testing.T) {
	tok := Approximate{}

	arabic := "مرحبا كيف حالك اليوم وماذا تفعل"
	if got := tok.CountTokens(arabic); got <= 0 {
		t.Fatalf("expected positive token count for Arabic text, got %d", got)
	}

	// Arabic script is weighted at ~2.2 chars/token vs ~4 for Latin, so a
	// similar-length phrase should be estimated at more tokens.
	latin := "hello how are you today and what"
	arabicTokens := tok.CountTokens(arabic)
	latinTokens := tok.CountTokens(latin)
	if arabicTokens <= latinTokens {
		t.Fatalf("expected Arabic phrase to yield more tokens than a similar-length Latin phrase: arabic=%d latin=%d", arabicTokens, latinTokens)
	}
}

func TestApproximateCountTokensOtherScripts(t *testing.T) {
	tok := Approximate{}

	samples := map[string]string{
		"hebrew":  "שלום מה שלומך היום ומה אתה עושה",
		"chinese": "你好，你今天好吗，你在做什么",
		"russian": "привет как дела сегодня и что ты делаешь",
		"hindi":   "नमस्ते आज आप कैसे हैं और आप क्या कर रहे हैं",
		"thai":    "สวัสดีวันนี้เป็นอย่างไรบ้างและคุณกำลังทำอะไรอยู่",
	}
	for name, text := range samples {
		if got := tok.CountTokens(text); got <= 0 {
			t.Fatalf("%s: expected positive token count, got %d", name, got)
		}
	}

	// CJK is weighted far denser than Latin (~1 char/token vs ~4), so a
	// short Chinese phrase should already out-cost a similar-length Latin
	// one by a wide margin.
	chineseTokens := tok.CountTokens(samples["chinese"])
	latinTokens := tok.CountTokens("hello how are you today and what are")
	if chineseTokens <= latinTokens {
		t.Fatalf("expected Chinese phrase to yield more tokens than a similar-length Latin phrase: chinese=%d latin=%d", chineseTokens, latinTokens)
	}
}

func TestIsRTL(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"arabic", "مرحبا كيف حالك", true},
		{"hebrew", "שלום מה שלומך", true},
		{"english", "hello, how are you?", false},
		{"chinese", "你好，你好吗", false},
		{"empty", "", false},
		{"digits and punctuation only", "123, 456!", false},
		{"leading digits then arabic", "123 مرحبا", true},
	}
	for _, tc := range cases {
		if got := IsRTL(tc.text); got != tc.want {
			t.Errorf("%s: IsRTL(%q) = %v, want %v", tc.name, tc.text, got, tc.want)
		}
	}
}
