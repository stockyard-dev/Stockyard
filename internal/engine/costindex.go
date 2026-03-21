package engine

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

const costIndexSchema = `
CREATE TABLE IF NOT EXISTS cost_index_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    input_per_1m REAL NOT NULL,
    output_per_1m REAL NOT NULL,
    UNIQUE(date, model)
);
CREATE INDEX IF NOT EXISTS idx_cost_index_date ON cost_index_history(date);
`

// RegisterCostIndexRoutes mounts the cost index API.
func RegisterCostIndexRoutes(mux *http.ServeMux, conn *sql.DB) {
	conn.Exec(costIndexSchema)

	mux.HandleFunc("GET /api/cost-index", func(w http.ResponseWriter, r *http.Request) {
		pricing := provider.ListPricing()

		type modelEntry struct {
			Model    string  `json:"model"`
			Provider string  `json:"provider"`
			Input    float64 `json:"input_per_1m_tokens"`
			Output   float64 `json:"output_per_1m_tokens"`
		}

		var entries []modelEntry
		for model, p := range pricing {
			entries = append(entries, modelEntry{
				Model:    model,
				Provider: p.Provider,
				Input:    p.Input,
				Output:   p.Output,
			})
		}

		// Sort by provider then model.
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Provider != entries[j].Provider {
				return entries[i].Provider < entries[j].Provider
			}
			return entries[i].Input < entries[j].Input
		})

		// Group by provider.
		byProvider := make(map[string][]modelEntry)
		for _, e := range entries {
			byProvider[e.Provider] = append(byProvider[e.Provider], e)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"models":      entries,
			"by_provider": byProvider,
			"count":       len(entries),
		})
	})

	log.Printf("[costindex] routes registered")
}
