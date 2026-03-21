package features

import (
	"context"
	"log"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// PromptCompressMiddleware reduces token usage by stripping whitespace,
// removing filler words, and deduplicating repeated instructions.
func PromptCompressMiddleware() proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			var origChars, compChars int
			for i, msg := range req.Messages {
				orig := msg.Content
				origChars += len(orig)

				compressed := compressText(orig)
				compChars += len(compressed)
				req.Messages[i].Content = compressed
			}

			if origChars > 0 {
				ratio := float64(compChars) / float64(origChars) * 100
				log.Printf("[promptcompress] %d → %d chars (%.0f%%)", origChars, compChars, ratio)
			}

			return next(ctx, req)
		}
	}
}

func compressText(s string) string {
	// Collapse multiple spaces/newlines
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else if r == '\n' {
			if !prevSpace {
				b.WriteRune('\n')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	result := b.String()

	// Remove common filler phrases
	fillers := []string{
		"please note that ", "it is important to note that ",
		"as mentioned earlier, ", "as previously stated, ",
		"in other words, ", "to put it simply, ",
	}
	lower := strings.ToLower(result)
	for _, f := range fillers {
		for {
			idx := strings.Index(strings.ToLower(result), f)
			if idx < 0 {
				break
			}
			result = result[:idx] + result[idx+len(f):]
		}
		_ = lower
	}

	return strings.TrimSpace(result)
}
