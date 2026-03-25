package platform

import "github.com/stockyard-dev/stockyard/internal/license"

// Tier is a numeric tier level for comparison ordering.
// Higher values unlock more products.
type Tier int

const (
	TierCommunity  Tier = 0  // Free: proxy + 66 modules
	TierIndividual Tier = 1  // $29.99: proxy + 4 products
	TierPro        Tier = 2  // $99.99: proxy + 9 products
	TierTeam       Tier = 3  // $299.99: proxy + 16 products
	TierEnterprise Tier = 4  // Custom: everything + orchestrator brain + iron
	TierDev        Tier = 99 // Dev mode: everything unlocked
)

// ProductTiers maps each product ID to the minimum tier required.
var ProductTiers = map[string]Tier{
	// Individual tier ($29.99)
	"bid":     TierIndividual,
	"replay":  TierIndividual,
	"doubt":   TierIndividual,
	"verdikt": TierIndividual,

	// Pro tier ($99.99)
	"stampede": TierPro,
	"fault":    TierPro,
	"morph":    TierPro,
	"spine":    TierPro,
	"hollow":   TierPro,

	// Team tier ($299.99)
	"seance":    TierTeam,
	"grain":     TierTeam,
	"echo":      TierTeam,
	"tide":      TierTeam,
	"fossil":    TierTeam,
	"prism":     TierTeam,
	"trailhead": TierTeam,

	// Enterprise (custom)
	"iron":         TierEnterprise,
	"orchestrator": TierEnterprise,

	// ── New Products ──

	// Individual
	"relic": TierIndividual,

	// Pro
	"breed":     TierPro,
	"fossilrec": TierPro,

	// Team
	"phantom":  TierTeam,
	"feral":    TierTeam,
	"tidepool": TierTeam,
	"crucible": TierTeam,

	// Enterprise
	"cortex":   TierEnterprise,
	"mycelium": TierEnterprise,
	"spore":    TierEnterprise,
	"molt":     TierEnterprise,
}

// TierName returns the human-readable name for a tier.
func TierName(t Tier) string {
	switch t {
	case TierCommunity:
		return "Community"
	case TierIndividual:
		return "Individual"
	case TierPro:
		return "Pro"
	case TierTeam:
		return "Team"
	case TierEnterprise:
		return "Enterprise"
	case TierDev:
		return "Dev"
	default:
		return "Unknown"
	}
}

// TierPrice returns the price string for a tier.
func TierPrice(t Tier) string {
	switch t {
	case TierCommunity:
		return "Free"
	case TierIndividual:
		return "$29.99/mo"
	case TierPro:
		return "$99.99/mo"
	case TierTeam:
		return "$299.99/mo"
	case TierEnterprise:
		return "Custom"
	case TierDev:
		return "Dev"
	default:
		return ""
	}
}

// TierFromLicense converts the existing license.Tier string to our numeric Tier.
func TierFromLicense(lt license.Tier) Tier {
	switch lt {
	case license.TierIndividual:
		return TierIndividual
	case license.TierPro:
		return TierPro
	case license.TierTeam:
		return TierTeam
	case license.TierEnterprise:
		return TierEnterprise
	case license.TierCloud:
		return TierEnterprise // cloud maps to enterprise-level access
	default:
		return TierCommunity
	}
}

// ProductsForTier returns all product IDs available at a given tier.
func ProductsForTier(t Tier) []string {
	var products []string
	for id, required := range ProductTiers {
		if t >= required {
			products = append(products, id)
		}
	}
	return products
}

// AllProductIDs returns every known product ID.
func AllProductIDs() []string {
	ids := make([]string, 0, len(ProductTiers))
	for id := range ProductTiers {
		ids = append(ids, id)
	}
	return ids
}
