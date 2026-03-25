package persona

import "testing"

func TestProfile(t *testing.T) {
	p := Profile{Habits: []string{"browse", "search"}, Preferences: map[string]string{"lang": "en"}}
	if len(p.Habits) != 2 { t.Errorf("expected 2 habits") }
}
