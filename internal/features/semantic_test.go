package features

import (
	"math"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

func TestCosineSimilarity_Identical(t *testing.T) {
	a := sparseVec{"foo": 0.5, "bar": 0.3, "baz": 0.2}
	sim := cosineSimilarity(a, a)
	if math.Abs(sim-1.0) > 0.001 {
		t.Errorf("identical vectors: similarity = %f, want 1.0", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := sparseVec{"foo": 1.0}
	b := sparseVec{"bar": 1.0}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("orthogonal vectors: similarity = %f, want 0", sim)
	}
}

func TestCosineSimilarity_Partial(t *testing.T) {
	a := sparseVec{"foo": 0.5, "bar": 0.5}
	b := sparseVec{"foo": 0.5, "baz": 0.5}
	sim := cosineSimilarity(a, b)
	// Should be > 0 but < 1
	if sim <= 0 || sim >= 1 {
		t.Errorf("partial overlap: similarity = %f, want (0, 1)", sim)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	a := sparseVec{}
	b := sparseVec{"foo": 1.0}
	if cosineSimilarity(a, b) != 0 {
		t.Error("empty vector should give 0 similarity")
	}
}

func TestVectorize_SameInput(t *testing.T) {
	msgs := []provider.Message{{Role: "user", Content: "What is the capital of France?"}}
	v1 := vectorize("gpt-4o", msgs)
	v2 := vectorize("gpt-4o", msgs)
	sim := cosineSimilarity(v1, v2)
	if math.Abs(sim-1.0) > 0.001 {
		t.Errorf("same input: similarity = %f, want 1.0", sim)
	}
}

func TestVectorize_SimilarInput(t *testing.T) {
	msgs1 := []provider.Message{{Role: "user", Content: "What is the capital of France?"}}
	msgs2 := []provider.Message{{Role: "user", Content: "What's the capital of France?"}}
	v1 := vectorize("gpt-4o", msgs1)
	v2 := vectorize("gpt-4o", msgs2)
	sim := cosineSimilarity(v1, v2)
	// These are very similar — should be high
	if sim < 0.8 {
		t.Errorf("similar inputs: similarity = %f, want >= 0.8", sim)
	}
	t.Logf("similar prompts similarity: %.4f", sim)
}

func TestVectorize_DifferentInput(t *testing.T) {
	msgs1 := []provider.Message{{Role: "user", Content: "What is the capital of France?"}}
	msgs2 := []provider.Message{{Role: "user", Content: "How do I bake chocolate chip cookies?"}}
	v1 := vectorize("gpt-4o", msgs1)
	v2 := vectorize("gpt-4o", msgs2)
	sim := cosineSimilarity(v1, v2)
	// These are very different — should be low
	if sim > 0.5 {
		t.Errorf("different inputs: similarity = %f, want < 0.5", sim)
	}
	t.Logf("different prompts similarity: %.4f", sim)
}

func TestVectorize_DifferentModel(t *testing.T) {
	msgs := []provider.Message{{Role: "user", Content: "Hello world"}}
	v1 := vectorize("gpt-4o", msgs)
	v2 := vectorize("claude-sonnet-4-5-20250929", msgs)
	sim := cosineSimilarity(v1, v2)
	// Same content but different model — should still be fairly similar
	// but not identical (model name is part of the vector)
	if sim > 0.99 || sim < 0.5 {
		t.Errorf("different model: similarity = %f, want (0.5, 0.99)", sim)
	}
}

func TestSemanticCache_FindSimilar(t *testing.T) {
	sc := NewSemanticCache(0.85, 100, 5*time.Minute)

	// Store a response
	msgs1 := []provider.Message{{Role: "user", Content: "What is the capital of France?"}}
	resp := &provider.Response{
		ID: "cached-1", Model: "gpt-4o",
		Choices: []provider.Choice{{
			Message: provider.Message{Role: "assistant", Content: "Paris"},
		}},
	}
	sc.Store("gpt-4o", msgs1, &CacheEntry{Response: resp})

	// Query with slightly different wording
	msgs2 := []provider.Message{{Role: "user", Content: "What's the capital of France?"}}
	found := sc.FindSimilar("gpt-4o", msgs2)
	if found == nil {
		t.Fatal("expected semantic cache hit for similar prompt")
	}
	if found.Response.Choices[0].Message.Content != "Paris" {
		t.Errorf("content = %q, want Paris", found.Response.Choices[0].Message.Content)
	}
}

func TestSemanticCache_RejectsDifferent(t *testing.T) {
	sc := NewSemanticCache(0.85, 100, 5*time.Minute)

	msgs1 := []provider.Message{{Role: "user", Content: "What is the capital of France?"}}
	resp := &provider.Response{ID: "cached-1", Model: "gpt-4o"}
	sc.Store("gpt-4o", msgs1, &CacheEntry{Response: resp})

	// Query with completely different content
	msgs2 := []provider.Message{{Role: "user", Content: "How do I bake chocolate chip cookies?"}}
	found := sc.FindSimilar("gpt-4o", msgs2)
	if found != nil {
		t.Error("should NOT match completely different prompt")
	}
}

func TestSemanticCache_Expiration(t *testing.T) {
	sc := NewSemanticCache(0.85, 100, 1*time.Millisecond)

	msgs := []provider.Message{{Role: "user", Content: "test prompt"}}
	sc.Store("gpt-4o", msgs, &CacheEntry{Response: &provider.Response{ID: "exp-1"}})

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	found := sc.FindSimilar("gpt-4o", msgs)
	if found != nil {
		t.Error("expired entry should not be returned")
	}
}

func TestSemanticCache_CapacityLimit(t *testing.T) {
	sc := NewSemanticCache(0.85, 2, 5*time.Minute)

	for i := 0; i < 3; i++ {
		msgs := []provider.Message{{Role: "user", Content: "prompt number " + string(rune('A'+i))}}
		sc.Store("gpt-4o", msgs, &CacheEntry{Response: &provider.Response{ID: "cap"}})
	}

	// Should only have 2 entries (capacity)
	stats := sc.Stats()
	if stats["entries"].(int) > 2 {
		t.Errorf("entries = %d, want <= 2", stats["entries"].(int))
	}
}

func TestCacheMiddleware_SemanticStrategy(t *testing.T) {
	callCount := 0
	mockHandler := func(_ interface{}, req *provider.Request) (*provider.Response, error) {
		callCount++
		return &provider.Response{
			ID: "resp-1", Model: req.Model,
			Choices: []provider.Choice{{
				Message: provider.Message{Role: "assistant", Content: "Paris is the capital."},
			}},
			Usage: provider.Usage{PromptTokens: 10, CompletionTokens: 5},
		}, nil
	}
	_ = mockHandler

	cache := NewCache(CacheConfig{
		Enabled:    true,
		Strategy:   "semantic",
		TTL:        5 * time.Minute,
		MaxEntries: 100,
	})

	if cache.semantic == nil {
		t.Fatal("semantic cache should be initialized when strategy is 'semantic'")
	}

	stats := cache.Stats()
	if stats["strategy"] != "semantic" {
		t.Errorf("strategy = %v", stats["strategy"])
	}
	if _, ok := stats["semantic"]; !ok {
		t.Error("stats should include semantic sub-stats")
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World!", "hello world "},
		{"café résumé", "café résumé"},
		{"test123", "test123"},
		{"  lots   of   spaces  ", "  lots   of   spaces  "},
	}
	for _, tt := range tests {
		got := normalize(tt.input)
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
