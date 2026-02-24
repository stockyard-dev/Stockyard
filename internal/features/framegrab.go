package features

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type FrameGrabEvent struct { Timestamp time.Time `json:"timestamp"`; Frames int `json:"frames"`; Model string `json:"model"` }
type FrameGrabState struct {
	mu sync.Mutex; cfg config.FrameGrabConfig; recentEvents []FrameGrabEvent
	videosProcessed atomic.Int64; framesExtracted atomic.Int64
}

func NewFrameGrab(cfg config.FrameGrabConfig) *FrameGrabState { return &FrameGrabState{cfg: cfg, recentEvents: make([]FrameGrabEvent, 0, 200)} }
func (f *FrameGrabState) Stats() map[string]any {
	f.mu.Lock(); events := make([]FrameGrabEvent, len(f.recentEvents)); copy(events, f.recentEvents); f.mu.Unlock()
	return map[string]any{"videos": f.videosProcessed.Load(), "frames": f.framesExtracted.Load(), "recent_events": events}
}
func FrameGrabMiddleware(f *FrameGrabState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { f.videosProcessed.Add(1); return next(ctx, req) }
	}
}
