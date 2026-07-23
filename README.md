# go-prompt-classiefier-compress-optm

Go module(s) for classifying, compressing, and optimizing LLM prompts —
built to slot into an AI gateway's request pipeline (classify → route →
compress → cache), the same shape as the multi-tier routing pipeline
described in [hugeai](https://github.com/hdahash/hugeai)'s `replit.md`.

## Packages

```
pkg/tokenizer   Approximate token-count estimation (no external tokenizer dependency)
pkg/classifier  Domain / complexity / intent classification -> recommended model tier
pkg/compressor  Prompt compression: local heuristic + remote model-backed client
pkg/optimizer   Ties classify + compress together against a token budget, plus cache keys
cmd/promptoptd  Stdlib-only HTTP server exposing the three as a sidecar service
```

Each package is a standalone, importable Go library with no dependency on
the others beyond `pkg/tokenizer`. `pkg/optimizer` is the only package that
composes the other two.

### `pkg/tokenizer`

```go
tok := tokenizer.Approximate{}
tok.CountTokens("some prompt text")
```

Blends a char-based and word-based (~1.3 tokens/word) heuristic. The char
estimate is script-aware, since most BPE vocabularies are English-centric
and fragment non-Latin scripts into more subword tokens per character:

| Script family                    | chars/token |
| --------------------------------- | ----------- |
| Latin (default)                   | 4.0         |
| Cyrillic (Russian, Ukrainian, ...) | 2.6         |
| Arabic, Hebrew                    | 2.2         |
| Devanagari (Hindi, Marathi, ...)  | 1.8         |
| Thai                               | 1.7         |
| CJK (Chinese, Japanese, Korean)   | 1.0         |

CJK and Thai are conventionally written without spaces between words, so
for text dominated by those scripts the word-based half of the estimate is
skipped (a whitespace-derived word count is meaningless there) and the
char-based estimate is used directly. `IsRTL(text)` is also exported: a
simplified "first strong character" heuristic (the same rule HTML's
`dir="auto"` uses) for right-to-left detection, useful for callers that want
to know before doing anything else with the text.

These are estimates, not a real BPE tokenizer — good enough to drive
compression-ratio and budget decisions without vendoring a model
vocabulary. Swap in a real tokenizer by implementing the one-method
`Tokenizer` interface.

### `pkg/classifier`

```go
c := classifier.New(nil) // nil -> tokenizer.Approximate
result := c.Classify(prompt)
// result.Domain:          code | math | creative | analysis | qa | chat
// result.Complexity:      simple | moderate | complex
// result.Intent:          debug | summarize | translate | extract | explain | generate | chat
// result.RecommendedTier: local | mid | large
```

Weighted keyword/regex rules assign a domain; length, code-block count,
question density, and step count drive a complexity score; domain +
complexity map to a recommended tier (e.g. simple chat → `local`/Ollama-class
model, complex code → `large`/frontier model). Pure rules, no network calls,
safe for concurrent use.

Domain and intent rules have Arabic and Hebrew keyword sets alongside the
English ones. Languages without dedicated keyword rules still classify —
they just fall back to `DomainChat` with complexity and tier decided by
script-agnostic structural signals (length, code-block count, question
density) rather than fine-grained domain/intent detection. Question
detection (both for `DomainQA` and the complexity score) recognizes `?`,
Arabic/Persian/Urdu `؟`, full-width CJK `？`, and Ethiopic `፧`. Adding a new
language's keywords is a matter of appending to the relevant rule in
`rules.go`; see the comment there.

### `pkg/compressor`

```go
type Compressor interface {
    Compress(ctx context.Context, prompt string, opts Options) (Result, error)
}
```

Two implementations, meant to be composed:

- **`Heuristic`** — dependency-free, extractive. Normalizes whitespace,
  strips filler phrasing ("kindly", "please", "just to clarify", ... and
  their Arabic/Hebrew equivalents), drops exact-duplicate sentences, then
  scores remaining sentences by term salience and keeps the highest-scoring
  ones until the target token budget is hit. Fenced code blocks (` ``` `)
  are pulled out first and always survive verbatim. `Options.Preserve`
  force-keeps any substring you pass. Word matching uses `\p{L}\p{N}` (any
  Unicode letter/number, not just `[A-Za-z0-9]`) so salience scoring works
  for any script, not just Latin. Sentence splitting recognizes terminators
  across scripts — `.`/`!`/`?`, Arabic/Persian/Urdu `؟`, full-width CJK
  `。！？`, Devanagari `।॥`, Ethiopic `።` — and the stopword list covers
  English, Arabic, and Hebrew function words. Scripts without clear
  word/sentence separators (Thai, Lao, Khmer) aren't specially handled:
  compression still runs and never corrupts the text, it's just less
  granular for those scripts (fewer, larger "sentences" to choose from).
- **`HTTPCompressor`** — a thin client for an external model-backed
  compression service (e.g. the LLMLingua-2-backed `optimizer-service` in
  `hugeai`), speaking the same `{prompt, target_token_ratio}` →
  `{compressed_prompt, original_tokens, compressed_tokens,
  compression_ratio, latency_ms}` contract that service already exposes.
- **`Fallback`** — wraps a `Primary`/`Secondary` pair so a remote,
  higher-quality compressor degrades to `Heuristic` instead of failing the
  request when unavailable.

### `pkg/optimizer`

```go
opt := optimizer.New(optimizer.Config{TokenBudget: 4000})
plan, err := opt.Optimize(ctx, prompt)
// plan.Classification, plan.Compression (nil if not needed),
// plan.FinalPrompt, plan.CacheKey, plan.FitsBudget
```

Classifies first; only compresses if the prompt exceeds `TokenBudget`, at a
ratio computed from how far over budget it is (clamped by `MinRatio` so it
never over-compresses). `CacheKey` hashes a whitespace/case-normalized
prompt plus the recommended tier, for exact-match semantic caching keyed by
which model tier would serve the request.

### `cmd/promptoptd`

A stdlib-only HTTP server (mirrors the conventions of `hugeai/go-proxy`:
atomic counters, `/health`, structured JSON errors) exposing the library as
a sidecar:

```
GET  /health
POST /classify  { "prompt": "..." }
POST /compress  { "prompt": "...", "target_token_ratio": 0.6 }
POST /optimize  { "prompt": "...", "token_budget": 4000 }
```

```bash
go run ./cmd/promptoptd                 # listens on :8003
PROMPTOPTD_PORT=9000 go run ./cmd/promptoptd
REMOTE_COMPRESSOR_URL=http://optimizer-service:8000/compress go run ./cmd/promptoptd
```

Setting `REMOTE_COMPRESSOR_URL` wires `/compress` and `/optimize` to try
that remote compressor first (matching the request/response shape of
`optimizer-service/main.py`) and fall back to the local heuristic on error.

## Design notes

- **No ML runtime in the hot path.** Classification and heuristic
  compression are pure Go, sub-millisecond, and need no model weights —
  suited to running inline on every request. Higher-quality, model-backed
  compression (LLMLingua-2 or similar) stays an optional, fallback-guarded
  remote call rather than a hard dependency.
- **Interfaces over concrete types.** `tokenizer.Tokenizer` and
  `compressor.Compressor` are one-method interfaces so a real BPE tokenizer
  or a different compression backend can be swapped in without touching
  `classifier` or `optimizer`.
- **Classification-first budgeting.** `optimizer.Optimize` only pays the
  cost of compression when the prompt is actually over budget, and the
  compression ratio is derived from the budget rather than hardcoded.
- **Script-aware, not just Arabic-aware.** hugeai's product targets both
  RTL (Arabic) and LTR (English) users, but the underlying mechanics —
  script-aware token estimation, Unicode-general word matching, cross-script
  sentence/question detection — are written to work for any script, not
  hardcoded to Arabic vs. Latin. Domain/intent keyword *rules* are
  necessarily per-language (English, Arabic, Hebrew today) since they're
  actual vocabulary, but everything structural (tokenization, complexity
  scoring, compression) degrades gracefully rather than silently breaking
  for a language with no dedicated rules yet — verified by
  `TestClassifyUnmappedLanguageFallsBackGracefully`. Note that Go's `regexp`
  `\b` word-boundary assertion is ASCII-only and doesn't fire around Arabic
  or Hebrew letters, so those rule patterns use plain substring matching
  instead.

## Test

```bash
go build ./...
go vet ./...
go test ./...
```
