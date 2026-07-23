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
