package ingest

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxChunkTokens != 512 {
		t.Errorf("MaxChunkTokens = %d, want 512", cfg.MaxChunkTokens)
	}
	if cfg.OverlapTokens != 64 {
		t.Errorf("OverlapTokens = %d, want 64", cfg.OverlapTokens)
	}
	if cfg.MaxFileSize != 100000 {
		t.Errorf("MaxFileSize = %d, want 100000", cfg.MaxFileSize)
	}
	if !cfg.IndexCode {
		t.Error("IndexCode should default to true")
	}
	if !cfg.IndexCommits {
		t.Error("IndexCommits should default to true")
	}
	if !cfg.IndexPRs {
		t.Error("IndexPRs should default to true")
	}
	if !cfg.IndexIssues {
		t.Error("IndexIssues should default to true")
	}
	if cfg.CommitLimit != 200 {
		t.Errorf("CommitLimit = %d, want 200", cfg.CommitLimit)
	}
	if cfg.PRLimit != 100 {
		t.Errorf("PRLimit = %d, want 100", cfg.PRLimit)
	}
	if cfg.IssueLimit != 100 {
		t.Errorf("IssueLimit = %d, want 100", cfg.IssueLimit)
	}
}

func TestChunkText_ShortText(t *testing.T) {
	text := "This is a short text."
	chunks := ChunkText(text, 512, 64)
	if len(chunks) != 1 {
		t.Errorf("ChunkText(short) returned %d chunks, want 1", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("chunk = %q, want %q", chunks[0], text)
	}
}

func TestChunkText_EmptyText(t *testing.T) {
	chunks := ChunkText("", 512, 64)
	if len(chunks) != 1 {
		t.Errorf("ChunkText(empty) returned %d chunks, want 1", len(chunks))
	}
}

func TestChunkText_LongText(t *testing.T) {
	// Create text that's longer than one chunk
	// 512 tokens * 4 chars = 2048 chars per chunk
	text := strings.Repeat("word ", 1000) // 5000 chars
	chunks := ChunkText(text, 512, 64)
	if len(chunks) < 2 {
		t.Errorf("ChunkText(long) returned %d chunks, want >= 2", len(chunks))
	}

	// Each chunk should not exceed maxChars (roughly)
	maxChars := 512 * 4
	for i, chunk := range chunks {
		if len(chunk) > maxChars+100 { // small tolerance for boundary finding
			t.Errorf("chunk %d length = %d, exceeds max %d", i, len(chunk), maxChars)
		}
	}
}

func TestChunkText_DefaultMaxTokens(t *testing.T) {
	text := strings.Repeat("a ", 1500) // 3000 chars
	chunks := ChunkText(text, 0, 64)   // should default to 512
	if len(chunks) < 2 {
		t.Errorf("ChunkText with default maxTokens returned %d chunks, want >= 2", len(chunks))
	}
}

func TestChunkText_ParagraphBoundary(t *testing.T) {
	// Create text with paragraph breaks
	para1 := strings.Repeat("First paragraph content. ", 100)
	para2 := strings.Repeat("Second paragraph content. ", 100)
	text := para1 + "\n\n" + para2

	chunks := ChunkText(text, 256, 32)
	// Should have multiple chunks
	if len(chunks) < 2 {
		t.Errorf("ChunkText(paragraphs) returned %d chunks, want >= 2", len(chunks))
	}

	// No chunk should be empty
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			t.Errorf("chunk %d is empty", i)
		}
	}
}

func TestContentHash(t *testing.T) {
	h1 := contentHash("hello world")
	h2 := contentHash("hello world")
	h3 := contentHash("different text")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 32 { // 16 bytes as hex = 32 chars
		t.Errorf("hash length = %d, want 32", len(h1))
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{"hello\nworld", "hello"},
		{"single line", "single line"},
		{"", ""},
		{"first\nsecond\nthird", "first"},
	}
	for _, tt := range tests {
		got := firstLine(tt.s)
		if got != tt.want {
			t.Errorf("firstLine(%q) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
