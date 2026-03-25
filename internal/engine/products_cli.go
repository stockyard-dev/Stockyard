package engine

import (
	"fmt"
	"sort"

	"github.com/stockyard-dev/stockyard/internal/license"
	"github.com/stockyard-dev/stockyard/internal/platform"
)

// printProducts displays all platform products with tier info.
// Called by `stockyard products`.
func printProducts() {
	lic := license.FromEnv()
	activeTier := platform.ResolveTier(lic)

	fmt.Println()
	fmt.Printf("  Stockyard Platform Products\n")
	fmt.Printf("  Active tier: %s\n", platform.TierName(activeTier))
	fmt.Println()

	type entry struct {
		id, name, desc, category, tierName string
		tier                               platform.Tier
		active                             bool
	}

	defs := []entry{
		{"bid", "Bid", "Real-time LLM model exchange", "runtime", "Individual", platform.TierIndividual, false},
		{"replay", "Replay", "Time-travel debugging", "infra", "Individual", platform.TierIndividual, false},
		{"doubt", "Doubt", "Confidence scores for code", "quality", "Individual", platform.TierIndividual, false},
		{"verdikt", "Verdikt", "AI output quality judge", "lifecycle", "Individual", platform.TierIndividual, false},
		{"stampede", "Stampede", "AI synthetic user testing", "lifecycle", "Pro", platform.TierPro, false},
		{"fault", "Fault", "Self-healing errors", "quality", "Pro", platform.TierPro, false},
		{"morph", "Morph", "Universal API translator", "runtime", "Pro", platform.TierPro, false},
		{"spine", "Spine", "Self-operating infrastructure", "infra", "Pro", platform.TierPro, false},
		{"hollow", "Hollow", "Negative space analysis", "quality", "Pro", platform.TierPro, false},
		{"seance", "Séance", "Talk to production", "insight", "Team", platform.TierTeam, false},
		{"grain", "Grain", "Decisions as first-class objects", "runtime", "Team", platform.TierTeam, false},
		{"echo", "Echo", "Evolutionary code memory", "insight", "Team", platform.TierTeam, false},
		{"tide", "Tide", "Metabolic feature control", "infra", "Team", platform.TierTeam, false},
		{"fossil", "Fossil", "Dead code archaeology", "insight", "Team", platform.TierTeam, false},
		{"prism", "Prism", "User cognitive mapping", "insight", "Team", platform.TierTeam, false},
		{"trailhead", "Trailhead", "Codebase knowledge engine", "insight", "Team", platform.TierTeam, false},
		{"iron", "Iron", "Burns specs into working code", "lifecycle", "Enterprise", platform.TierEnterprise, false},
		{"orchestrator", "Orchestrator", "Autonomous coordination brain", "lifecycle", "Enterprise", platform.TierEnterprise, false},

		// ── New Products ──
		{"relic", "Relic", "Content provenance chain", "quality", "Individual", platform.TierIndividual, false},
		{"breed", "Breed", "Genetic prompt evolution", "lifecycle", "Pro", platform.TierPro, false},
		{"fossilrec", "Fossil Record", "Temporal model archaeology", "quality", "Pro", platform.TierPro, false},
		{"phantom", "Phantom", "Persistent AI canaries", "quality", "Team", platform.TierTeam, false},
		{"feral", "Feral", "Adversarial AI hunter", "quality", "Team", platform.TierTeam, false},
		{"tidepool", "Tide Pool", "Emergent behavior detection", "insight", "Team", platform.TierTeam, false},
		{"crucible", "Crucible", "System-level confidence scoring", "quality", "Team", platform.TierTeam, false},
		{"cortex", "Cortex", "Shared AI memory substrate", "insight", "Enterprise", platform.TierEnterprise, false},
		{"mycelium", "Mycelium", "Cross-instance intelligence", "infra", "Enterprise", platform.TierEnterprise, false},
		{"spore", "Spore", "Self-replicating patterns", "infra", "Enterprise", platform.TierEnterprise, false},
		{"molt", "Molt", "Automatic architecture shedding", "infra", "Enterprise", platform.TierEnterprise, false},
	}

	// Mark active products
	for i := range defs {
		defs[i].active = activeTier >= defs[i].tier
	}

	// Sort by tier, then name
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].tier != defs[j].tier {
			return defs[i].tier < defs[j].tier
		}
		return defs[i].id < defs[j].id
	})

	// Group by tier
	currentTier := platform.Tier(-1)
	for _, d := range defs {
		if d.tier != currentTier {
			currentTier = d.tier
			price := platform.TierPrice(d.tier)
			fmt.Printf("  ── %s (%s) ────────────────────────────────\n", d.tierName, price)
		}
		marker := "✓"
		if !d.active {
			marker = "✗"
		}
		fmt.Printf("    %s  %-14s %-35s %s\n", marker, d.name, d.desc, d.category)
	}

	fmt.Println()
	active := 0
	for _, d := range defs {
		if d.active {
			active++
		}
	}
	fmt.Printf("  %d/%d products active\n", active, len(defs))

	if activeTier < platform.TierEnterprise {
		fmt.Printf("  Upgrade at https://stockyard.dev/pricing\n")
	}
	fmt.Println()
}
