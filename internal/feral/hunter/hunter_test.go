package hunter

import "testing"

func TestConfig(t *testing.T) {
	c := Config{CampaignID: "c1", TargetURL: "http://localhost", AttackTypes: []string{"injection"}}
	if c.CampaignID != "c1" { t.Error("unexpected") }
}
