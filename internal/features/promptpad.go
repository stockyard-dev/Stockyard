package features

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// TemplateEntry is a loaded prompt template with compiled variants.
type TemplateEntry struct {
	Name     string
	Version  string
	Template string
	Variants []variantEntry
}

type variantEntry struct {
	Name     string
	Weight   int
	Override string
	Hits     atomic.Int64
}

// PromptPadManager manages prompt templates and A/B testing.
type PromptPadManager struct {
	mu        sync.RWMutex
	templates map[string]*TemplateEntry
	hitCount  atomic.Int64
}

// NewPromptPad creates a prompt pad from config.
func NewPromptPad(cfg config.PromptPadConfig) *PromptPadManager {
	pp := &PromptPadManager{
		templates: make(map[string]*TemplateEntry),
	}

	for _, tmpl := range cfg.Templates {
		entry := &TemplateEntry{
			Name:     tmpl.Name,
			Version:  tmpl.Version,
			Template: tmpl.Template,
		}
		for _, v := range tmpl.Variants {
			w := v.Weight
			if w <= 0 {
				w = 50
			}
			entry.Variants = append(entry.Variants, variantEntry{
				Name:     v.Name,
				Weight:   w,
				Override: v.Override,
			})
		}
		pp.templates[tmpl.Name] = entry
	}

	return pp
}

// Resolve looks up a template by name, selects a variant via weighted random,
// and interpolates variables. Returns the final prompt text and variant name.
func (pp *PromptPadManager) Resolve(name string, vars map[string]string) (string, string, error) {
	pp.mu.RLock()
	entry, ok := pp.templates[name]
	pp.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("promptpad: template %q not found", name)
	}

	pp.hitCount.Add(1)
	base := entry.Template

	// Select variant if available
	variantName := "default"
	if len(entry.Variants) > 0 {
		v := selectVariant(entry.Variants)
		variantName = v.Name
		v.Hits.Add(1)
		if v.Override != "" {
			base = base + "\n" + v.Override
		}
	}

	// Interpolate variables: {{var_name}} → value
	result := base
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		result = strings.ReplaceAll(result, placeholder, v)
	}

	return result, variantName, nil
}

// GetTemplate returns a template by name.
func (pp *PromptPadManager) GetTemplate(name string) *TemplateEntry {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	return pp.templates[name]
}

// ListTemplates returns all template names.
func (pp *PromptPadManager) ListTemplates() []string {
	pp.mu.RLock()
	defer pp.mu.RUnlock()
	names := make([]string, 0, len(pp.templates))
	for name := range pp.templates {
		names = append(names, name)
	}
	return names
}

// Stats returns template usage statistics.
func (pp *PromptPadManager) Stats() map[string]any {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	tmplStats := make([]map[string]any, 0)
	for _, entry := range pp.templates {
		varStats := make([]map[string]any, 0)
		for i := range entry.Variants {
			varStats = append(varStats, map[string]any{
				"name": entry.Variants[i].Name,
				"hits": entry.Variants[i].Hits.Load(),
			})
		}
		tmplStats = append(tmplStats, map[string]any{
			"name":     entry.Name,
			"version":  entry.Version,
			"variants": varStats,
		})
	}

	return map[string]any{
		"total_hits": pp.hitCount.Load(),
		"templates":  tmplStats,
	}
}

func selectVariant(variants []variantEntry) *variantEntry {
	total := 0
	for _, v := range variants {
		total += v.Weight
	}
	r := rand.Intn(total)
	for i := range variants {
		r -= variants[i].Weight
		if r < 0 {
			return &variants[i]
		}
	}
	return &variants[0]
}

// PromptPadMiddleware returns middleware that resolves prompt templates.
// Templates are selected via X-Template header with vars in X-Template-Vars.
func PromptPadMiddleware(pad *PromptPadManager) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Check for template reference in Extra (set from X-Template header)
			tmplName, _ := req.Extra["_template"].(string)
			if tmplName == "" {
				return next(ctx, req)
			}

			// Parse variables from X-Template-Vars: "key1=val1,key2=val2"
			vars := make(map[string]string)
			if varsStr, ok := req.Extra["_template_vars"].(string); ok && varsStr != "" {
				for _, pair := range strings.Split(varsStr, ",") {
					parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
					if len(parts) == 2 {
						vars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}

			resolved, variant, err := pad.Resolve(tmplName, vars)
			if err != nil {
				log.Printf("promptpad: %v", err)
				return next(ctx, req)
			}

			// Inject resolved template as system message
			sysMsg := provider.Message{Role: "system", Content: resolved}
			req.Messages = append([]provider.Message{sysMsg}, req.Messages...)
			req.Extra["_template_variant"] = variant

			log.Printf("promptpad: resolved template %q variant %q", tmplName, variant)
			return next(ctx, req)
		}
	}
}
