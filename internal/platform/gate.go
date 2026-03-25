package platform

import (
	"encoding/json"
	"net/http"
)

// Gate returns an HTTP handler that blocks all requests with a 403
// containing upgrade information. Used when a product's tier requirement
// exceeds the active license tier.
func Gate(activeTier Tier, productID string) http.Handler {
	requiredTier := ProductTiers[productID]
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"error":    "upgrade required",
			"product":  productID,
			"required": TierName(requiredTier),
			"current":  TierName(activeTier),
			"upgrade":  "https://stockyard.dev/pricing",
		})
	})
}

// GateMiddleware wraps an existing handler with tier checking.
// If the active tier is sufficient, requests pass through to next.
// Otherwise, a 403 with upgrade info is returned.
func GateMiddleware(activeTier Tier, productID string, next http.Handler) http.Handler {
	requiredTier := ProductTiers[productID]
	if activeTier >= requiredTier {
		return next // tier sufficient, no gate needed
	}
	return Gate(activeTier, productID)
}
