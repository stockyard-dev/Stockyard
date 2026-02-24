package features

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// StreamCapture holds a captured stream with metadata.
type StreamCapture struct {
	ID            string    `json:"id"`
	RequestID     string    `json:"request_id"`
	Model         string    `json:"model"`
	Provider      string    `json:"provider"`
	Project       string    `json:"project"`
	Timestamp     time.Time `json:"timestamp"`
	TTFT          time.Duration `json:"ttft_ms"`          // time to first token
	TotalDuration time.Duration `json:"total_duration_ms"`
	TokensOut     int       `json:"tokens_out"`
	TPS           float64   `json:"tokens_per_second"`
	Complete      bool      `json:"complete"` // false if stream was interrupted
	FullResponse  string    `json:"full_response"`
	ChunkCount    int       `json:"chunk_count"`
}

// StreamSnapper captures and analyzes SSE streams.
type StreamSnapper struct {
	mu       sync.RWMutex
	captures map[string]*StreamCapture // id → capture
	cfg      config.StreamSnapConfig

	totalStreams    atomic.Int64
	totalCaptures  atomic.Int64
	totalTokens    atomic.Int64
	interrupted    atomic.Int64
	avgTTFT        atomic.Int64 // stored as microseconds
}

// NewStreamSnapper creates a new stream capture engine.
func NewStreamSnapper(cfg config.StreamSnapConfig) *StreamSnapper {
	return &StreamSnapper{
		captures: make(map[string]*StreamCapture),
		cfg:      cfg,
	}
}

// CaptureNonStream records a non-streaming response as if it were a captured stream.
// This allows StreamSnap to provide unified metrics for both streaming and non-streaming requests.
func (ss *StreamSnapper) CaptureNonStream(req *provider.Request, resp *provider.Response) {
	if !ss.cfg.Enabled {
		return
	}

	ss.totalStreams.Add(1)
	ss.totalCaptures.Add(1)

	id := fmt.Sprintf("snap-%d", time.Now().UnixNano())
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	capture := &StreamCapture{
		ID:            id,
		RequestID:     resp.ID,
		Model:         req.Model,
		Provider:      resp.Provider,
		Project:       req.Project,
		Timestamp:     time.Now(),
		TTFT:          resp.Latency,
		TotalDuration: resp.Latency,
		TokensOut:     resp.Usage.CompletionTokens,
		Complete:      true,
		FullResponse:  content,
		ChunkCount:    1,
	}

	if capture.TotalDuration > 0 && capture.TokensOut > 0 {
		capture.TPS = float64(capture.TokensOut) / capture.TotalDuration.Seconds()
	}

	ss.totalTokens.Add(int64(capture.TokensOut))

	// Store capture (with eviction of old entries)
	ss.store(capture)
}

// RecordStreamStart is called when a streaming request begins.
func (ss *StreamSnapper) RecordStreamStart(requestID, model, prov, project string) *StreamCaptureSession {
	if !ss.cfg.Enabled {
		return nil
	}

	ss.totalStreams.Add(1)
	id := fmt.Sprintf("snap-%d", time.Now().UnixNano())

	return &StreamCaptureSession{
		snapper:   ss,
		startTime: time.Now(),
		capture: &StreamCapture{
			ID:        id,
			RequestID: requestID,
			Model:     model,
			Provider:  prov,
			Project:   project,
			Timestamp: time.Now(),
		},
	}
}

// store saves a capture with size limit enforcement.
func (ss *StreamSnapper) store(cap *StreamCapture) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.captures[cap.ID] = cap

	// Enforce max entries (keep last 10000)
	maxEntries := 10000
	if len(ss.captures) > maxEntries {
		// Find oldest and remove
		var oldestID string
		oldestTime := time.Now()
		for id, c := range ss.captures {
			if c.Timestamp.Before(oldestTime) {
				oldestTime = c.Timestamp
				oldestID = id
			}
		}
		if oldestID != "" {
			delete(ss.captures, oldestID)
		}
	}
}

// GetCapture returns a captured stream by ID.
func (ss *StreamSnapper) GetCapture(id string) (*StreamCapture, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	cap, ok := ss.captures[id]
	return cap, ok
}

// RecentCaptures returns the most recent N captures.
func (ss *StreamSnapper) RecentCaptures(n int) []*StreamCapture {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	all := make([]*StreamCapture, 0, len(ss.captures))
	for _, c := range ss.captures {
		all = append(all, c)
	}

	// Sort by timestamp descending
	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Timestamp.After(all[i].Timestamp) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// Stats returns aggregated stream metrics.
func (ss *StreamSnapper) Stats() map[string]any {
	return map[string]any{
		"total_streams":   ss.totalStreams.Load(),
		"total_captures":  ss.totalCaptures.Load(),
		"total_tokens":    ss.totalTokens.Load(),
		"interrupted":     ss.interrupted.Load(),
		"avg_ttft_us":     ss.avgTTFT.Load(),
		"stored_captures": len(ss.captures),
	}
}

// StreamCaptureSession tracks a single streaming request.
type StreamCaptureSession struct {
	snapper      *StreamSnapper
	startTime    time.Time
	firstToken   time.Time
	capture      *StreamCapture
	contentBuf   []byte
	chunkCount   int
	tokenCount   int
	gotFirstTok  bool
}

// OnChunk records a stream chunk.
func (s *StreamCaptureSession) OnChunk(data []byte, tokensSoFar int) {
	if s == nil {
		return
	}

	s.chunkCount++

	if !s.gotFirstTok && len(data) > 0 {
		s.firstToken = time.Now()
		s.gotFirstTok = true
	}

	// Extract content delta from SSE data
	content := extractContentDelta(data)
	if content != "" {
		s.contentBuf = append(s.contentBuf, content...)
	}

	s.tokenCount = tokensSoFar
}

// Finish completes the capture and stores it.
func (s *StreamCaptureSession) Finish(complete bool) {
	if s == nil {
		return
	}

	s.capture.TotalDuration = time.Since(s.startTime)
	s.capture.TokensOut = s.tokenCount
	s.capture.ChunkCount = s.chunkCount
	s.capture.Complete = complete
	s.capture.FullResponse = string(s.contentBuf)

	if s.gotFirstTok {
		s.capture.TTFT = s.firstToken.Sub(s.startTime)
		// Update rolling average TTFT
		s.snapper.avgTTFT.Store(int64(s.capture.TTFT / time.Microsecond))
	}

	if s.capture.TotalDuration > 0 && s.capture.TokensOut > 0 {
		s.capture.TPS = float64(s.capture.TokensOut) / s.capture.TotalDuration.Seconds()
	}

	if !complete {
		s.snapper.interrupted.Add(1)
	}

	s.snapper.totalCaptures.Add(1)
	s.snapper.totalTokens.Add(int64(s.capture.TokensOut))
	s.snapper.store(s.capture)

	log.Printf("streamsnap: id=%s model=%s ttft=%v tps=%.1f tokens=%d complete=%v",
		s.capture.ID, s.capture.Model, s.capture.TTFT, s.capture.TPS, s.capture.TokensOut, complete)
}

// extractContentDelta tries to pull content from an SSE data line.
func extractContentDelta(data []byte) string {
	// SSE data format: data: {"choices":[{"delta":{"content":"..."}}]}
	type sseChunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}

	// Strip "data: " prefix if present
	raw := data
	if len(raw) > 6 && string(raw[:6]) == "data: " {
		raw = raw[6:]
	}

	var chunk sseChunk
	if err := json.Unmarshal(raw, &chunk); err == nil {
		if len(chunk.Choices) > 0 {
			return chunk.Choices[0].Delta.Content
		}
	}
	return ""
}

// StreamSnapMiddleware returns middleware that captures non-streaming responses.
// Streaming capture requires integration at the proxy level (in streaming.go).
func StreamSnapMiddleware(ss *StreamSnapper) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Capture non-streaming responses
			if !req.Stream {
				ss.CaptureNonStream(req, resp)
			}

			return resp, nil
		}
	}
}
