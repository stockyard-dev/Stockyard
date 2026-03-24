// Package ingest processes raw content into chunks for embedding and indexing.
package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/trailhead/cstore"
	"github.com/stockyard-dev/stockyard/internal/trailhead/source"
)

// Config holds ingestion settings.
type Config struct {
	MaxChunkTokens    int // Target chunk size in tokens (default 512)
	OverlapTokens     int // Overlap between chunks (default 64)
	MaxFileSize       int // Skip files larger than this (default 100KB)
	IndexCode         bool
	IndexCommits      bool
	IndexPRs          bool
	IndexIssues       bool
	CommitLimit       int
	PRLimit           int
	IssueLimit        int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxChunkTokens: 512,
		OverlapTokens:  64,
		MaxFileSize:    100000,
		IndexCode:      true,
		IndexCommits:   true,
		IndexPRs:       true,
		IndexIssues:    true,
		CommitLimit:    200,
		PRLimit:        100,
		IssueLimit:     100,
	}
}

// Progress reports indexing progress.
type Progress struct {
	Phase     string
	Current   int
	Total     int
	Detail    string
}

// IndexGitHubRepo indexes a GitHub repository into the knowledge base.
func IndexGitHubRepo(ctx context.Context, gh *source.GitHub, owner, repo string, db *cstore.DB, cfg Config, onProgress func(Progress)) error {
	sourceID := fmt.Sprintf("github:%s/%s", owner, repo)

	// Save source
	db.SaveSource(cstore.Source{
		ID:   sourceID,
		Type: "github",
		Name: fmt.Sprintf("%s/%s", owner, repo),
	})

	if onProgress == nil {
		onProgress = func(Progress) {}
	}

	totalDocs := 0

	// Index commits
	if cfg.IndexCommits {
		onProgress(Progress{Phase: "commits", Detail: "fetching..."})
		commits, err := gh.FetchCommits(ctx, owner, repo, cfg.CommitLimit)
		if err != nil {
			return fmt.Errorf("fetching commits: %w", err)
		}

		for i, c := range commits {
			onProgress(Progress{Phase: "commits", Current: i + 1, Total: len(commits), Detail: truncate(c.Message, 60)})

			content := fmt.Sprintf("Commit %s by %s\n\n%s", c.SHA[:8], c.Author, c.Message)
			hash := contentHash(content)
			if db.DocumentExists(hash) {
				continue
			}

			doc := cstore.Document{
				ID:          fmt.Sprintf("%s:commit:%s", sourceID, c.SHA[:8]),
				SourceID:    sourceID,
				Type:        "commit",
				Title:       firstLine(c.Message),
				Content:     content,
				URL:         c.URL,
				Author:      c.Author,
				CreatedDate: c.Date,
				ContentHash: hash,
				Metadata:    map[string]string{"sha": c.SHA},
			}
			db.SaveDocument(doc)

			// Extract entities
			db.UpsertEntity(cstore.Entity{ID: "person:" + c.Author, Type: "person", Name: c.Author})
			db.AddRelationship(cstore.Relationship{
				FromEntity: "person:" + c.Author,
				ToEntity:   doc.ID,
				Type:       "authored",
				Weight:     1.0,
				Context:    firstLine(c.Message),
			}, doc.ID)

			totalDocs++
		}
	}

	// Index PRs
	if cfg.IndexPRs {
		onProgress(Progress{Phase: "pull_requests", Detail: "fetching..."})
		prs, err := gh.FetchPullRequests(ctx, owner, repo, cfg.PRLimit)
		if err != nil {
			return fmt.Errorf("fetching PRs: %w", err)
		}

		for i, pr := range prs {
			onProgress(Progress{Phase: "pull_requests", Current: i + 1, Total: len(prs), Detail: pr.Title})

			content := fmt.Sprintf("Pull Request #%d: %s\nAuthor: %s\nState: %s\n\n%s",
				pr.Number, pr.Title, pr.Author, pr.State, pr.Body)
			hash := contentHash(content)
			if db.DocumentExists(hash) {
				continue
			}

			doc := cstore.Document{
				ID:          fmt.Sprintf("%s:pr:%d", sourceID, pr.Number),
				SourceID:    sourceID,
				Type:        "pull_request",
				Title:       fmt.Sprintf("PR #%d: %s", pr.Number, pr.Title),
				Content:     content,
				URL:         pr.URL,
				Author:      pr.Author,
				CreatedDate: pr.CreatedAt,
				ContentHash: hash,
				Metadata: map[string]string{
					"number": fmt.Sprintf("%d", pr.Number),
					"state":  pr.State,
					"labels": strings.Join(pr.Labels, ","),
				},
			}
			db.SaveDocument(doc)

			// Entities
			db.UpsertEntity(cstore.Entity{ID: "person:" + pr.Author, Type: "person", Name: pr.Author})
			db.AddRelationship(cstore.Relationship{
				FromEntity: "person:" + pr.Author,
				ToEntity:   doc.ID,
				Type:       "authored",
				Weight:     1.5,
				Context:    pr.Title,
			}, doc.ID)

			totalDocs++
		}
	}

	// Index issues
	if cfg.IndexIssues {
		onProgress(Progress{Phase: "issues", Detail: "fetching..."})
		issues, err := gh.FetchIssues(ctx, owner, repo, cfg.IssueLimit)
		if err != nil {
			return fmt.Errorf("fetching issues: %w", err)
		}

		for i, iss := range issues {
			onProgress(Progress{Phase: "issues", Current: i + 1, Total: len(issues), Detail: iss.Title})

			content := fmt.Sprintf("Issue #%d: %s\nAuthor: %s\nState: %s\n\n%s",
				iss.Number, iss.Title, iss.Author, iss.State, iss.Body)
			hash := contentHash(content)
			if db.DocumentExists(hash) {
				continue
			}

			doc := cstore.Document{
				ID:          fmt.Sprintf("%s:issue:%d", sourceID, iss.Number),
				SourceID:    sourceID,
				Type:        "issue",
				Title:       fmt.Sprintf("Issue #%d: %s", iss.Number, iss.Title),
				Content:     content,
				URL:         iss.URL,
				Author:      iss.Author,
				CreatedDate: iss.CreatedAt,
				ContentHash: hash,
				Metadata: map[string]string{
					"number": fmt.Sprintf("%d", iss.Number),
					"state":  iss.State,
					"labels": strings.Join(iss.Labels, ","),
				},
			}
			db.SaveDocument(doc)

			db.UpsertEntity(cstore.Entity{ID: "person:" + iss.Author, Type: "person", Name: iss.Author})
			totalDocs++
		}
	}

	// Index code files
	if cfg.IndexCode {
		onProgress(Progress{Phase: "code_files", Detail: "fetching tree..."})
		files, err := gh.FetchFiles(ctx, owner, repo, nil)
		if err != nil {
			return fmt.Errorf("fetching files: %w", err)
		}

		for i, f := range files {
			if f.Size > cfg.MaxFileSize {
				continue
			}
			onProgress(Progress{Phase: "code_files", Current: i + 1, Total: len(files), Detail: f.Path})

			content, err := gh.FetchFileContent(ctx, owner, repo, f.Path)
			if err != nil {
				continue // Skip files we can't read
			}

			hash := contentHash(content)
			if db.DocumentExists(hash) {
				continue
			}

			doc := cstore.Document{
				ID:          fmt.Sprintf("%s:file:%s", sourceID, f.Path),
				SourceID:    sourceID,
				Type:        "code_file",
				Title:       f.Path,
				Content:     fmt.Sprintf("File: %s\n\n%s", f.Path, content),
				URL:         f.URL,
				CreatedDate: time.Now(),
				ContentHash: hash,
				Metadata:    map[string]string{"path": f.Path, "sha": f.SHA},
			}
			db.SaveDocument(doc)

			// File entity
			db.UpsertEntity(cstore.Entity{ID: "file:" + f.Path, Type: "file", Name: f.Path})
			totalDocs++
		}
	}

	db.UpdateSourceSync(sourceID)
	onProgress(Progress{Phase: "complete", Total: totalDocs, Detail: fmt.Sprintf("%d documents indexed", totalDocs)})

	return nil
}

// ChunkDocuments splits all un-chunked documents into chunks.
func ChunkDocuments(db *cstore.DB, maxTokens, overlap int) (int, error) {
	// This would normally query for documents without chunks,
	// but for simplicity we'll chunk at embedding time
	return 0, nil
}

// ChunkText splits text into overlapping chunks of approximately maxTokens.
func ChunkText(text string, maxTokens, overlap int) []string {
	if maxTokens <= 0 {
		maxTokens = 512
	}

	// Rough approximation: 1 token ≈ 4 characters
	maxChars := maxTokens * 4
	overlapChars := overlap * 4

	if len(text) <= maxChars {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + maxChars
		if end > len(text) {
			end = len(text)
		}

		// Try to break at a paragraph or sentence boundary
		if end < len(text) {
			// Look for paragraph break
			if idx := strings.LastIndex(text[start:end], "\n\n"); idx > maxChars/2 {
				end = start + idx
			} else if idx := strings.LastIndex(text[start:end], "\n"); idx > maxChars/2 {
				end = start + idx
			} else if idx := strings.LastIndex(text[start:end], ". "); idx > maxChars/2 {
				end = start + idx + 1
			}
		}

		chunk := strings.TrimSpace(text[start:end])
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		start = end - overlapChars
		if start <= 0 || end >= len(text) {
			break
		}
	}

	// Catch remaining text
	if start > 0 && start < len(text) {
		remaining := strings.TrimSpace(text[start:])
		if len(remaining) > 0 && (len(chunks) == 0 || remaining != chunks[len(chunks)-1]) {
			chunks = append(chunks, remaining)
		}
	}

	return chunks
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:16])
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
