package platform

import (
	"encoding/json"
	"net/http"
)

// HealthSummary is the response for /api/platform/health.
type HealthSummary struct {
	Status     string         `json:"status"`      // ok, degraded, down
	Tier       string         `json:"tier"`
	Products   int            `json:"products"`
	Active     int            `json:"active"`
	Locked     int            `json:"locked"`
	Healthy    int            `json:"healthy"`
	Degraded   int            `json:"degraded"`
	Down       int            `json:"down"`
	ByCategory map[string]int `json:"by_category"`
}

// HandleHealth returns an HTTP handler for GET /api/platform/health.
func HandleHealth(reg *ProductRegistry, activeTier Tier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all := reg.All()

		summary := HealthSummary{
			Status:     "ok",
			Tier:       TierName(activeTier),
			Products:   len(all),
			ByCategory: make(map[string]int),
		}

		for _, ps := range all {
			summary.ByCategory[ps.Category]++
			if !ps.Active {
				summary.Locked++
				continue
			}
			summary.Active++
			switch ps.Status {
			case "healthy":
				summary.Healthy++
			case "degraded":
				summary.Degraded++
				summary.Status = "degraded"
			case "down":
				summary.Down++
				summary.Status = "degraded"
			}
		}

		if summary.Down > summary.Active/2 {
			summary.Status = "down"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	}
}
