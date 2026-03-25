package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/seance/connector"
)

// fakeSource implements connector.Source for testing.
type fakeSource struct {
	name string
	typ  string
}

func (f fakeSource) Name() string { return f.name }
func (f fakeSource) Type() string { return f.typ }
func (f fakeSource) Fetch(_ context.Context, _ []string) (*connector.Snapshot, error) {
	return nil, nil
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name     string
		question string
		want     []string
		exclude  []string
	}{
		{
			name:     "removes stop words",
			question: "Why is the database slow?",
			want:     []string{"database", "slow"},
			exclude:  []string{"why", "is", "the"},
		},
		{
			name:     "lowercases and trims punctuation",
			question: "What about Redis? Is it healthy!",
			want:     []string{"redis", "healthy"},
			exclude:  []string{"what", "about", "is", "it"},
		},
		{
			name:     "filters short words",
			question: "a b cd ef gh",
			want:     []string{"cd", "ef", "gh"},
			exclude:  []string{"a", "b"},
		},
		{
			name:     "empty question",
			question: "",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKeywords(tt.question)
			for _, w := range tt.want {
				if !contains(got, w) {
					t.Errorf("expected keyword %q in %v", w, got)
				}
			}
			for _, w := range tt.exclude {
				if contains(got, w) {
					t.Errorf("did not expect stop word %q in %v", w, got)
				}
			}
		})
	}
}

func TestScoreSource_NameMatch(t *testing.T) {
	src := fakeSource{name: "redis", typ: "redis"}
	score, reason := scoreSource("why is redis slow", []string{"redis", "slow"}, src)

	// Should get name match (0.3) + keyword (0.15) + type affinity + baseline (0.1)
	if score < 0.5 {
		t.Errorf("expected score >= 0.5 for direct name match, got %f", score)
	}
	if !strings.Contains(reason, "name match") {
		t.Errorf("expected reason to contain 'name match', got %q", reason)
	}
}

func TestScoreSource_BaselineOnly(t *testing.T) {
	src := fakeSource{name: "unrelated-service", typ: "static"}
	score, reason := scoreSource("why is redis slow", []string{"redis", "slow"}, src)

	// Should only get baseline 0.1
	if score != 0.1 {
		t.Errorf("expected baseline score 0.1, got %f", score)
	}
	if reason != "baseline" {
		t.Errorf("expected reason 'baseline', got %q", reason)
	}
}

func TestScoreSource_TypeAffinity(t *testing.T) {
	src := fakeSource{name: "app-metrics", typ: "prometheus"}
	score, _ := scoreSource("why is latency high", []string{"latency", "high"}, src)

	// latency -> prometheus is a primary match (0.3), plus baseline (0.1)
	if score < 0.4 {
		t.Errorf("expected score >= 0.4 for type affinity match, got %f", score)
	}
}

func TestScoreSource_ClampToOne(t *testing.T) {
	// A source that matches on many dimensions
	src := fakeSource{name: "error log", typ: "log"}
	score, _ := scoreSource("error log crash", []string{"error", "log", "crash"}, src)

	if score > 1.0 {
		t.Errorf("score should be clamped to 1.0, got %f", score)
	}
}

func TestRankSources_SortOrder(t *testing.T) {
	sources := []connector.Source{
		fakeSource{name: "unrelated", typ: "static"},
		fakeSource{name: "redis-cache", typ: "redis"},
		fakeSource{name: "app-logs", typ: "log"},
	}

	ranked := RankSources("why is redis cache slow", sources)

	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked sources, got %d", len(ranked))
	}

	// redis-cache should be ranked first
	if ranked[0].Source.Name() != "redis-cache" {
		t.Errorf("expected redis-cache ranked first, got %s", ranked[0].Source.Name())
	}

	// Scores should be descending
	for i := 1; i < len(ranked); i++ {
		if ranked[i].Score > ranked[i-1].Score {
			t.Errorf("scores not descending: [%d]=%f > [%d]=%f",
				i, ranked[i].Score, i-1, ranked[i-1].Score)
		}
	}
}

func TestAllocateTokenBudget(t *testing.T) {
	tests := []struct {
		name   string
		ranked []RankedSource
		budget int
		checks func(t *testing.T, alloc map[string]int)
	}{
		{
			name:   "empty sources",
			ranked: nil,
			budget: 1000,
			checks: func(t *testing.T, alloc map[string]int) {
				if len(alloc) != 0 {
					t.Errorf("expected empty allocations, got %v", alloc)
				}
			},
		},
		{
			name:   "zero budget uses default",
			ranked: []RankedSource{{Source: fakeSource{name: "a"}, Score: 1.0}},
			budget: 0,
			checks: func(t *testing.T, alloc map[string]int) {
				if alloc["a"] != DefaultTokenBudget {
					t.Errorf("expected %d tokens, got %d", DefaultTokenBudget, alloc["a"])
				}
			},
		},
		{
			name: "proportional allocation",
			ranked: []RankedSource{
				{Source: fakeSource{name: "high"}, Score: 0.8},
				{Source: fakeSource{name: "low"}, Score: 0.2},
			},
			budget: 1000,
			checks: func(t *testing.T, alloc map[string]int) {
				if alloc["high"] <= alloc["low"] {
					t.Errorf("high-score source should get more tokens: high=%d, low=%d",
						alloc["high"], alloc["low"])
				}
				total := alloc["high"] + alloc["low"]
				if total > 1000 {
					t.Errorf("total allocation %d exceeds budget 1000", total)
				}
			},
		},
		{
			name: "minimum floor enforced",
			ranked: []RankedSource{
				{Source: fakeSource{name: "big"}, Score: 0.99},
				{Source: fakeSource{name: "tiny"}, Score: 0.01},
			},
			budget: 1000,
			checks: func(t *testing.T, alloc map[string]int) {
				if alloc["tiny"] < 100 {
					t.Errorf("expected minimum 100 tokens for tiny, got %d", alloc["tiny"])
				}
			},
		},
		{
			name: "budget exhaustion stops allocation",
			ranked: func() []RankedSource {
				var rs []RankedSource
				for i := 0; i < 20; i++ {
					rs = append(rs, RankedSource{
						Source: fakeSource{name: strings.Repeat("x", i+1)},
						Score:  0.5,
					})
				}
				return rs
			}(),
			budget: 500,
			checks: func(t *testing.T, alloc map[string]int) {
				total := 0
				for _, v := range alloc {
					total += v
				}
				if total > 500 {
					t.Errorf("total allocation %d exceeds budget 500", total)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc := AllocateTokenBudget(tt.ranked, tt.budget)
			tt.checks(t, alloc)
		})
	}
}

func TestTruncateToFit_ShortContent(t *testing.T) {
	data := "short text"
	result := TruncateToFit(data, 1000)
	if result != data {
		t.Errorf("short content should not be truncated, got %q", result)
	}
}

func TestTruncateToFit_PreservesErrorLines(t *testing.T) {
	var lines []string
	lines = append(lines, "header line")
	for i := 0; i < 50; i++ {
		lines = append(lines, "normal filler content line that is not important")
	}
	lines = append(lines, "CRITICAL ERROR: database connection failed")
	for i := 0; i < 50; i++ {
		lines = append(lines, "more normal filler content line padding")
	}
	lines = append(lines, "final line")

	data := strings.Join(lines, "\n")

	// Use a small budget to force truncation
	result := TruncateToFit(data, 200)

	if !strings.Contains(result, "CRITICAL ERROR") {
		t.Error("truncated output should preserve error lines")
	}
	if !strings.Contains(result, "[truncated") {
		t.Error("truncated output should contain truncation marker")
	}
	if len(result) < len(data) {
		// It was truncated, good
	} else {
		t.Error("expected content to actually be truncated")
	}
}

func TestTruncateToFit_FewLines(t *testing.T) {
	data := strings.Repeat("a", 5000)
	result := TruncateToFit(data, 100)
	// maxChars = 100/0.25 = 400
	if !strings.Contains(result, "...[truncated]") {
		t.Error("few-line content should be hard-truncated with marker")
	}
}

func TestScoreLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		idx      int
		total    int
		minScore float64
	}{
		{"error line", "ERROR: something broke", 10, 20, 5.0},
		{"fatal line", "FATAL: process crashed", 10, 20, 5.0},
		{"warning line", "WARNING: timeout detected", 10, 20, 3.0},
		{"first line", "some header", 0, 20, 2.0},
		{"last line", "final summary", 19, 20, 2.0},
		{"empty line", "   ", 10, 20, -2.0},
		{"separator", "---", 10, 20, 3.0},
		{"line with number", "count: 42", 10, 20, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreLine(tt.line, tt.idx, tt.total)
			if score < tt.minScore {
				t.Errorf("scoreLine(%q) = %f, want >= %f", tt.line, score, tt.minScore)
			}
		})
	}
}

func TestMatchTypeToQuestion(t *testing.T) {
	tests := []struct {
		name     string
		question string
		keywords []string
		srcType  string
		wantPos  bool // expect positive score
	}{
		{"error->log primary", "why error", []string{"error"}, "log", true},
		{"latency->prometheus", "latency spike", []string{"latency"}, "prometheus", true},
		{"cache->redis", "cache miss rate", []string{"cache"}, "redis", true},
		{"no match", "unrelated question", []string{"unrelated", "question"}, "prometheus", false},
		{"pattern match p99", "p99 is high", []string{"p99", "high"}, "prometheus", true},
		{"pattern match exception", "java exception thrown", []string{"java", "exception"}, "log", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := matchTypeToQuestion(tt.question, tt.keywords, tt.srcType)
			if tt.wantPos && score <= 0 {
				t.Errorf("expected positive score, got %f", score)
			}
			if !tt.wantPos && score > 0 {
				t.Errorf("expected zero score, got %f", score)
			}
		})
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
