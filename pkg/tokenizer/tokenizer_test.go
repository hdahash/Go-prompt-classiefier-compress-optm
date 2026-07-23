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
