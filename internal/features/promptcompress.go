package features

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

var compressWhitespaceRe = regexp.MustCompile(`[ \t]+`)
var compressNewlineRe = regexp.MustCompile(`\n{3,}`)

// PromptCompressMiddleware strips excess whitespace and removes repeated
// instructions from request messages before passing them to the next handler.
// The compression ratio is tracked in resp.Meta["X-Stockyard-Compression-Ratio"].
func PromptCompressMiddleware() proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Measure original size
			originalSize := 0
			for _, msg := range req.Messages {
				originalSize += len(msg.Content)
			}

			// Compress messages in place
			seen := make(map[string]bool)
			for i := range req.Messages {
				content := req.Messages[i].Content

				// Strip excess whitespace: collapse runs of spaces/tabs to single space
				content = compressWhitespaceRe.ReplaceAllString(content, " ")

				// Collapse excessive newlines to double newline
				content = compressNewlineRe.ReplaceAllString(content, "\n\n")

				// Trim leading/trailing whitespace
				content = strings.TrimSpace(content)

				req.Messages[i].Content = content
			}

			// Remove repeated instructions: deduplicate identical system messages
			deduped := make([]provider.Message, 0, len(req.Messages))
			for _, msg := range req.Messages {
				if msg.Role == "system" {
					key := msg.Role + ":" + msg.Content
					if seen[key] {
						continue
					}
					seen[key] = true
				}
				deduped = append(deduped, msg)
			}
			req.Messages = deduped

			// Measure compressed size
			compressedSize := 0
			for _, msg := range req.Messages {
				compressedSize += len(msg.Content)
			}

			// Call next handler
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Track compression ratio in response metadata
			ratio := 1.0
			if originalSize > 0 {
				ratio = float64(compressedSize) / float64(originalSize)
			}
			if resp.Meta == nil {
				resp.Meta = make(map[string]string)
			}
			resp.Meta["X-Stockyard-Compression-Ratio"] = fmt.Sprintf("%.2f", ratio)

			return resp, nil
		}
	}
}
