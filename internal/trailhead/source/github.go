// Package source provides connectors for indexing external data sources.
package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHub fetches data from GitHub repositories.
type GitHub struct {
	token   string
	client  *http.Client
	baseURL string
}

// NewGitHub creates a GitHub source connector.
func NewGitHub(token string) *GitHub {
	return &GitHub{
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.github.com",
	}
}

// RepoInfo holds basic repository information.
type RepoInfo struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stargazers_count"`
}

// Commit represents a Git commit.
type Commit struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Email   string    `json:"email"`
	Date    time.Time `json:"date"`
	URL     string    `json:"url"`
	Files   []string  `json:"files,omitempty"`
}

// PullRequest represents a GitHub PR.
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`
	URL       string    `json:"url"`
	Labels    []string  `json:"labels"`
	Comments  []Comment `json:"comments,omitempty"`
}

// Issue represents a GitHub issue.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"url"`
	Labels    []string  `json:"labels"`
	Comments  []Comment `json:"comments,omitempty"`
}

// Comment represents a discussion comment.
type Comment struct {
	Author string    `json:"author"`
	Body   string    `json:"body"`
	Date   time.Time `json:"date"`
}

// CodeFile represents a file in the repository.
type CodeFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	SHA     string `json:"sha"`
	URL     string `json:"url"`
}

// FetchRepo gets repository information.
func (g *GitHub) FetchRepo(ctx context.Context, owner, repo string) (*RepoInfo, error) {
	var info RepoInfo
	err := g.get(ctx, fmt.Sprintf("/repos/%s/%s", owner, repo), &info)
	if err != nil {
		return nil, err
	}
	info.Owner = owner
	info.Name = repo
	return &info, nil
}

// FetchCommits fetches recent commits.
func (g *GitHub) FetchCommits(ctx context.Context, owner, repo string, limit int) ([]Commit, error) {
	if limit <= 0 {
		limit = 100
	}

	var raw []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		HTMLURL string `json:"html_url"`
	}

	err := g.get(ctx, fmt.Sprintf("/repos/%s/%s/commits?per_page=%d", owner, repo, min(limit, 100)), &raw)
	if err != nil {
		return nil, err
	}

	commits := make([]Commit, len(raw))
	for i, r := range raw {
		commits[i] = Commit{
			SHA:     r.SHA,
			Message: r.Commit.Message,
			Author:  r.Commit.Author.Name,
			Email:   r.Commit.Author.Email,
			Date:    r.Commit.Author.Date,
			URL:     r.HTMLURL,
		}
	}
	return commits, nil
}

// FetchPullRequests fetches PRs (open and closed).
func (g *GitHub) FetchPullRequests(ctx context.Context, owner, repo string, limit int) ([]PullRequest, error) {
	if limit <= 0 {
		limit = 50
	}

	var raw []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
		MergedAt  *time.Time `json:"merged_at"`
		HTMLURL   string    `json:"html_url"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}

	err := g.get(ctx, fmt.Sprintf("/repos/%s/%s/pulls?state=all&per_page=%d&sort=updated&direction=desc", owner, repo, min(limit, 100)), &raw)
	if err != nil {
		return nil, err
	}

	prs := make([]PullRequest, len(raw))
	for i, r := range raw {
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		prs[i] = PullRequest{
			Number:    r.Number,
			Title:     r.Title,
			Body:      r.Body,
			Author:    r.User.Login,
			State:     r.State,
			CreatedAt: r.CreatedAt,
			MergedAt:  r.MergedAt,
			URL:       r.HTMLURL,
			Labels:    labels,
		}
	}
	return prs, nil
}

// FetchIssues fetches issues.
func (g *GitHub) FetchIssues(ctx context.Context, owner, repo string, limit int) ([]Issue, error) {
	if limit <= 0 {
		limit = 50
	}

	var raw []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
		HTMLURL   string    `json:"html_url"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		PullRequest *struct{} `json:"pull_request"` // Non-nil if this is a PR
	}

	err := g.get(ctx, fmt.Sprintf("/repos/%s/%s/issues?state=all&per_page=%d&sort=updated&direction=desc", owner, repo, min(limit, 100)), &raw)
	if err != nil {
		return nil, err
	}

	var issues []Issue
	for _, r := range raw {
		if r.PullRequest != nil {
			continue // Skip PRs (they show up in the issues endpoint too)
		}
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		issues = append(issues, Issue{
			Number:    r.Number,
			Title:     r.Title,
			Body:      r.Body,
			Author:    r.User.Login,
			State:     r.State,
			CreatedAt: r.CreatedAt,
			URL:       r.HTMLURL,
			Labels:    labels,
		})
	}
	return issues, nil
}

// FetchFiles fetches code files from the repo tree (non-binary, < 100KB).
func (g *GitHub) FetchFiles(ctx context.Context, owner, repo string, extensions []string) ([]CodeFile, error) {
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Size int    `json:"size"`
			SHA  string `json:"sha"`
			URL  string `json:"url"`
		} `json:"tree"`
	}

	err := g.get(ctx, fmt.Sprintf("/repos/%s/%s/git/trees/HEAD?recursive=1", owner, repo), &tree)
	if err != nil {
		return nil, err
	}

	extSet := map[string]bool{}
	for _, e := range extensions {
		extSet[strings.ToLower(e)] = true
	}
	if len(extSet) == 0 {
		// Default extensions
		for _, e := range []string{".go", ".py", ".js", ".ts", ".rs", ".md", ".yaml", ".yml", ".toml", ".json", ".sql", ".sh", ".dockerfile"} {
			extSet[e] = true
		}
	}

	var files []CodeFile
	for _, f := range tree.Tree {
		if f.Type != "blob" || f.Size > 100000 {
			continue
		}
		ext := ""
		if idx := strings.LastIndex(f.Path, "."); idx >= 0 {
			ext = strings.ToLower(f.Path[idx:])
		}
		if !extSet[ext] && ext != "" {
			continue
		}

		files = append(files, CodeFile{
			Path: f.Path,
			Size: f.Size,
			SHA:  f.SHA,
			URL:  fmt.Sprintf("https://github.com/%s/%s/blob/HEAD/%s", owner, repo, f.Path),
		})
	}
	return files, nil
}

// FetchFileContent downloads a single file's content.
func (g *GitHub) FetchFileContent(ctx context.Context, owner, repo, path string) (string, error) {
	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	err := g.get(ctx, fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path), &file)
	if err != nil {
		return "", err
	}

	if file.Encoding == "base64" {
		content := strings.ReplaceAll(file.Content, "\n", "")
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return "", fmt.Errorf("decoding base64: %w", err)
		}
		return string(decoded), nil
	}
	return file.Content, nil
}

// get makes an authenticated GET request to the GitHub API.
func (g *GitHub) get(ctx context.Context, path string, target any) error {
	url := g.baseURL + path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading GitHub response: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	return json.Unmarshal(body, target)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
