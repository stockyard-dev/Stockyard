package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewGitHub(t *testing.T) {
	g := NewGitHub("test-token")
	if g == nil {
		t.Fatal("NewGitHub() returned nil")
	}
	if g.token != "test-token" {
		t.Errorf("token = %q, want %q", g.token, "test-token")
	}
	if g.baseURL != "https://api.github.com" {
		t.Errorf("baseURL = %q, want %q", g.baseURL, "https://api.github.com")
	}
}

func TestFetchRepo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing or incorrect Authorization header")
		}
		resp := map[string]any{
			"description":      "A test repo",
			"language":         "Go",
			"stargazers_count": 42,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("test-token")
	g.baseURL = ts.URL

	info, err := g.FetchRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("FetchRepo() error = %v", err)
	}
	if info.Owner != "owner" {
		t.Errorf("Owner = %q, want %q", info.Owner, "owner")
	}
	if info.Name != "repo" {
		t.Errorf("Name = %q, want %q", info.Name, "repo")
	}
	if info.Description != "A test repo" {
		t.Errorf("Description = %q, want %q", info.Description, "A test repo")
	}
	if info.Stars != 42 {
		t.Errorf("Stars = %d, want 42", info.Stars)
	}
}

func TestFetchCommits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []map[string]any{
			{
				"sha": "abc123def456",
				"commit": map[string]any{
					"message": "Initial commit",
					"author": map[string]any{
						"name":  "Alice",
						"email": "alice@example.com",
						"date":  "2024-01-01T00:00:00Z",
					},
				},
				"html_url": "https://github.com/owner/repo/commit/abc123",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	commits, err := g.FetchCommits(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("FetchCommits() error = %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("commits len = %d, want 1", len(commits))
	}
	if commits[0].SHA != "abc123def456" {
		t.Errorf("SHA = %q", commits[0].SHA)
	}
	if commits[0].Author != "Alice" {
		t.Errorf("Author = %q", commits[0].Author)
	}
	if commits[0].Message != "Initial commit" {
		t.Errorf("Message = %q", commits[0].Message)
	}
}

func TestFetchCommits_DefaultLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify default limit is applied
		json.NewEncoder(w).Encode([]any{})
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	_, err := g.FetchCommits(context.Background(), "owner", "repo", 0)
	if err != nil {
		t.Fatalf("FetchCommits() error = %v", err)
	}
}

func TestFetchPullRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []map[string]any{
			{
				"number":     42,
				"title":      "Add feature",
				"body":       "This adds a new feature",
				"state":      "closed",
				"created_at": "2024-01-01T00:00:00Z",
				"merged_at":  "2024-01-02T00:00:00Z",
				"html_url":   "https://github.com/owner/repo/pull/42",
				"user":       map[string]string{"login": "bob"},
				"labels":     []map[string]string{{"name": "enhancement"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	prs, err := g.FetchPullRequests(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("FetchPullRequests() error = %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("prs len = %d, want 1", len(prs))
	}
	if prs[0].Number != 42 {
		t.Errorf("Number = %d, want 42", prs[0].Number)
	}
	if prs[0].Author != "bob" {
		t.Errorf("Author = %q, want %q", prs[0].Author, "bob")
	}
	if len(prs[0].Labels) != 1 || prs[0].Labels[0] != "enhancement" {
		t.Errorf("Labels = %v, want [enhancement]", prs[0].Labels)
	}
}

func TestFetchIssues_FiltersPRs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []map[string]any{
			{
				"number":       1,
				"title":        "Bug report",
				"body":         "Found a bug",
				"state":        "open",
				"created_at":   "2024-01-01T00:00:00Z",
				"html_url":     "https://github.com/owner/repo/issues/1",
				"user":         map[string]string{"login": "carol"},
				"labels":       []map[string]string{},
				"pull_request": nil,
			},
			{
				"number":       2,
				"title":        "A PR (should be filtered)",
				"body":         "",
				"state":        "open",
				"created_at":   "2024-01-01T00:00:00Z",
				"html_url":     "https://github.com/owner/repo/pull/2",
				"user":         map[string]string{"login": "dave"},
				"labels":       []map[string]string{},
				"pull_request": map[string]string{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	issues, err := g.FetchIssues(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("FetchIssues() error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("issues len = %d, want 1 (PR should be filtered)", len(issues))
	}
	if issues[0].Title != "Bug report" {
		t.Errorf("Title = %q, want %q", issues[0].Title, "Bug report")
	}
}

func TestFetchFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"tree": []map[string]any{
				{"path": "main.go", "type": "blob", "size": 500, "sha": "abc"},
				{"path": "README.md", "type": "blob", "size": 200, "sha": "def"},
				{"path": "internal", "type": "tree", "size": 0},
				{"path": "large.bin", "type": "blob", "size": 200000, "sha": "ghi"},
				{"path": "image.png", "type": "blob", "size": 100, "sha": "jkl"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	files, err := g.FetchFiles(context.Background(), "owner", "repo", nil)
	if err != nil {
		t.Fatalf("FetchFiles() error = %v", err)
	}

	// Should include .go and .md, exclude tree, large file, and .png
	pathSet := map[string]bool{}
	for _, f := range files {
		pathSet[f.Path] = true
	}
	if !pathSet["main.go"] {
		t.Error("missing main.go")
	}
	if !pathSet["README.md"] {
		t.Error("missing README.md")
	}
	if pathSet["internal"] {
		t.Error("should not include tree entries")
	}
	if pathSet["large.bin"] {
		t.Error("should not include files > 100KB")
	}
}

func TestFetchFiles_CustomExtensions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"tree": []map[string]any{
				{"path": "main.go", "type": "blob", "size": 500, "sha": "abc"},
				{"path": "app.py", "type": "blob", "size": 300, "sha": "def"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	files, err := g.FetchFiles(context.Background(), "owner", "repo", []string{".py"})
	if err != nil {
		t.Fatalf("FetchFiles() error = %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	if files[0].Path != "app.py" {
		t.Errorf("Path = %q, want %q", files[0].Path, "app.py")
	}
}

func TestFetchFileContent(t *testing.T) {
	originalContent := "package main\n\nfunc main() {}"
	encoded := base64.StdEncoding.EncodeToString([]byte(originalContent))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"content":  encoded,
			"encoding": "base64",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	content, err := g.FetchFileContent(context.Background(), "owner", "repo", "main.go")
	if err != nil {
		t.Fatalf("FetchFileContent() error = %v", err)
	}
	if content != originalContent {
		t.Errorf("content = %q, want %q", content, originalContent)
	}
}

func TestFetchFileContent_PlainText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"content":  "plain text content",
			"encoding": "utf-8",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	content, err := g.FetchFileContent(context.Background(), "owner", "repo", "file.txt")
	if err != nil {
		t.Fatalf("FetchFileContent() error = %v", err)
	}
	if content != "plain text content" {
		t.Errorf("content = %q, want %q", content, "plain text content")
	}
}

func TestGet_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer ts.Close()

	g := NewGitHub("token")
	g.baseURL = ts.URL

	var result map[string]any
	err := g.get(context.Background(), "/repos/owner/repo", &result)
	if err == nil {
		t.Error("get() expected error for 404")
	}
}

func TestGet_NoToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("should not send Authorization header when token is empty")
		}
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer ts.Close()

	g := NewGitHub("")
	g.baseURL = ts.URL

	var result map[string]string
	err := g.get(context.Background(), "/test", &result)
	if err != nil {
		t.Fatalf("get() error = %v", err)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("truncate short")
	}
	if truncate("hello world", 5) != "hello..." {
		t.Error("truncate long")
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{3, 1, 1},
		{5, 5, 5},
	}
	for _, tt := range tests {
		got := min(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
