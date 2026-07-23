package classifier

import (
	"strings"
	"testing"

	"github.com/hdahash/go-prompt-classiefier-compress-optm/pkg/tokenizer"
)

func TestClassifyCode(t *testing.T) {
	c := New(tokenizer.Approximate{})
	res := c.Classify("I'm getting a nil pointer error in this Go func, can you help debug it?\n```go\nfunc main() {}\n```")

	if res.Domain != DomainCode {
		t.Fatalf("expected code domain, got %s", res.Domain)
	}
	if res.Intent != "debug" {
		t.Fatalf("expected debug intent, got %s", res.Intent)
	}
}

func TestClassifySimpleChatGetsLocalTier(t *testing.T) {
	c := New(tokenizer.Approximate{})
	res := c.Classify("hey, how's it going?")

	if res.Complexity != ComplexitySimple {
		t.Fatalf("expected simple complexity, got %s", res.Complexity)
	}
	if res.RecommendedTier != TierLocal {
		t.Fatalf("expected local tier, got %s", res.RecommendedTier)
	}
}

func TestClassifyComplexGetsLargeTier(t *testing.T) {
	c := New(tokenizer.Approximate{})
	res := c.Classify(longComplexPrompt())

	if res.Complexity != ComplexityComplex {
		t.Fatalf("expected complex complexity, got %s", res.Complexity)
	}
	if res.RecommendedTier != TierLarge {
		t.Fatalf("expected large tier, got %s", res.RecommendedTier)
	}
}

func TestClassifyArabicCode(t *testing.T) {
	c := New(tokenizer.Approximate{})
	res := c.Classify("لدي خطأ في الكود، هل يمكنك مساعدتي في تصحيح الأخطاء في هذه الدالة؟")

	if res.Domain != DomainCode {
		t.Fatalf("expected code domain, got %s", res.Domain)
	}
	if res.Intent != "debug" {
		t.Fatalf("expected debug intent, got %s", res.Intent)
	}
}

func TestClassifyArabicSimpleChatGetsLocalTier(t *testing.T) {
	c := New(tokenizer.Approximate{})
	res := c.Classify("مرحبا كيف حالك؟")

	if res.Complexity != ComplexitySimple {
		t.Fatalf("expected simple complexity, got %s", res.Complexity)
	}
	if res.RecommendedTier != TierLocal {
		t.Fatalf("expected local tier, got %s", res.RecommendedTier)
	}
}

func longComplexPrompt() string {
	var b strings.Builder
	b.WriteString("Step 1: explain this. Step 2: refactor this. ")
	for i := 0; i < 40; i++ {
		b.WriteString("This is sentence number filler to pad out the prompt length so it crosses the token threshold. ")
	}
	b.WriteString("```go\nfunc a() {}\n```\n```go\nfunc b() {}\n```\n")
	b.WriteString("Why does this fail? What should I change? How can I fix it?")
	return b.String()
}
