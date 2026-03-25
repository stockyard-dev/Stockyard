// Package hunter provides the continuous hunting loop.
package hunter

// Config defines a hunting loop configuration.
type Config struct {
	CampaignID   string   `json:"campaign_id"`
	TargetURL    string   `json:"target_url"`
	AttackTypes  []string `json:"attack_types"`
	MaxGenerations int    `json:"max_generations"`
}

// Result represents the outcome of a hunting run.
type Result struct {
	Generation int  `json:"generation"`
	Attacks    int  `json:"attacks"`
	Successes  int  `json:"successes"`
	Done       bool `json:"done"`
}
