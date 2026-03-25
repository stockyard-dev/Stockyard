package model

import (
	"testing"
	"time"
)

func TestBuildSessions_Empty(t *testing.T) {
	sessions := buildSessions("user1", nil)
	if sessions != nil {
		t.Errorf("expected nil for empty events, got %v", sessions)
	}
}

func TestBuildSessions_SingleEvent(t *testing.T) {
	now := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []UserEvent{
		{UserID: "u1", EventType: "click", Path: "/home", Timestamp: now},
	}
	sessions := buildSessions("u1", events)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].EventCount != 1 {
		t.Errorf("expected 1 event, got %d", sessions[0].EventCount)
	}
	if sessions[0].ID != "u1-s0" {
		t.Errorf("expected ID u1-s0, got %s", sessions[0].ID)
	}
}

func TestBuildSessions_SplitByGap(t *testing.T) {
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []UserEvent{
		{UserID: "u1", Path: "/home", Timestamp: base},
		{UserID: "u1", Path: "/about", Timestamp: base.Add(5 * time.Minute)},
		// 31 minute gap -> new session
		{UserID: "u1", Path: "/settings", Timestamp: base.Add(36 * time.Minute)},
		{UserID: "u1", Path: "/profile", Timestamp: base.Add(38 * time.Minute)},
	}
	sessions := buildSessions("u1", events)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].EventCount != 2 {
		t.Errorf("session 0: expected 2 events, got %d", sessions[0].EventCount)
	}
	if sessions[1].EventCount != 2 {
		t.Errorf("session 1: expected 2 events, got %d", sessions[1].EventCount)
	}
	if sessions[0].ID != "u1-s0" || sessions[1].ID != "u1-s1" {
		t.Errorf("unexpected session IDs: %s, %s", sessions[0].ID, sessions[1].ID)
	}
}

func TestBuildSessions_ExactlyAtGapBoundary(t *testing.T) {
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []UserEvent{
		{UserID: "u1", Path: "/a", Timestamp: base},
		// Exactly 30 minutes - should NOT split (gap > sessionGap, not >=)
		{UserID: "u1", Path: "/b", Timestamp: base.Add(30 * time.Minute)},
	}
	sessions := buildSessions("u1", events)
	if len(sessions) != 1 {
		t.Errorf("expected 1 session at exactly 30 min gap, got %d", len(sessions))
	}
}

func TestBuildSessions_DedupesPaths(t *testing.T) {
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []UserEvent{
		{UserID: "u1", Path: "/home", Timestamp: base},
		{UserID: "u1", Path: "/about", Timestamp: base.Add(1 * time.Minute)},
		{UserID: "u1", Path: "/home", Timestamp: base.Add(2 * time.Minute)},
	}
	sessions := buildSessions("u1", events)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Paths should be deduped
	if len(sessions[0].Paths) != 2 {
		t.Errorf("expected 2 unique paths, got %d: %v", len(sessions[0].Paths), sessions[0].Paths)
	}
}

func TestDedupePaths(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"empty", nil, nil},
		{"no dupes", []string{"/a", "/b"}, []string{"/a", "/b"}},
		{"with dupes", []string{"/a", "/b", "/a", "/c", "/b"}, []string{"/a", "/b", "/c"}},
		{"all same", []string{"/x", "/x", "/x"}, []string{"/x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupePaths(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("dedupePaths(%v) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("dedupePaths(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestEstimateExpertise(t *testing.T) {
	tests := []struct {
		name string
		cm   CognitiveMap
		min  float64
		max  float64
	}{
		{
			name: "baseline user",
			cm:   CognitiveMap{PathFrequency: map[string]int{"/a": 1}},
			min:  0.4,
			max:  0.6,
		},
		{
			name: "expert user - many paths, no issues",
			cm: CognitiveMap{
				PathFrequency: func() map[string]int {
					m := map[string]int{}
					for i := 0; i < 25; i++ {
						m["/page"+string(rune('a'+i))] = 1
					}
					return m
				}(),
				HelpSeeking: 0,
				RageClicks:  0,
				Backtracking: 0,
			},
			min: 0.8,
			max: 1.0,
		},
		{
			name: "novice user - lots of issues",
			cm: CognitiveMap{
				PathFrequency: map[string]int{"/a": 1},
				HelpSeeking:   10,
				RageClicks:    5,
				Backtracking:  8,
			},
			min: 0.0,
			max: 0.2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := estimateExpertise(&tt.cm)
			if score < tt.min || score > tt.max {
				t.Errorf("estimateExpertise: got %.2f, want [%.2f, %.2f]", score, tt.min, tt.max)
			}
		})
	}
}

func TestEstimateExpertise_ClampedToRange(t *testing.T) {
	// Extreme novice: should not go below 0
	cm := &CognitiveMap{
		PathFrequency: map[string]int{"/a": 1},
		HelpSeeking:   100,
		RageClicks:    100,
		Backtracking:  100,
	}
	score := estimateExpertise(cm)
	if score < 0 || score > 1 {
		t.Errorf("expertise out of [0,1]: got %.2f", score)
	}
}

func TestInferMisunderstandings(t *testing.T) {
	tests := []struct {
		name     string
		cm       CognitiveMap
		wantMiss int
	}{
		{
			name:     "no misunderstandings",
			cm:       CognitiveMap{Backtracking: 1, RageClicks: 1},
			wantMiss: 0,
		},
		{
			name:     "backtracking misunderstanding",
			cm:       CognitiveMap{Backtracking: 4, RageClicks: 0},
			wantMiss: 1,
		},
		{
			name:     "rage click misunderstanding",
			cm:       CognitiveMap{Backtracking: 0, RageClicks: 3},
			wantMiss: 1,
		},
		{
			name:     "both misunderstandings",
			cm:       CognitiveMap{Backtracking: 5, RageClicks: 5},
			wantMiss: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mis := inferMisunderstandings(&tt.cm, nil)
			if len(mis) != tt.wantMiss {
				t.Errorf("got %d misunderstandings, want %d", len(mis), tt.wantMiss)
			}
		})
	}
}

func TestDedupeConfusions(t *testing.T) {
	now := time.Now()
	confusions := []Confusion{
		{Path: "/settings", Type: "rage_click", Count: 1, Timestamp: now},
		{Path: "/settings", Type: "rage_click", Count: 1, Timestamp: now},
		{Path: "/settings", Type: "rage_click", Count: 1, Timestamp: now},
		{Path: "/home", Type: "hesitation", Count: 1, Timestamp: now},
	}
	deduped := dedupeConfusions(confusions)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 unique confusions, got %d", len(deduped))
	}
	// The rage_click one should have count 3 (1 original + 2 increments)
	for _, c := range deduped {
		if c.Path == "/settings" && c.Type == "rage_click" {
			if c.Count != 3 {
				t.Errorf("expected merged count 3, got %d", c.Count)
			}
		}
	}
}

func TestEngine_IngestAndGetMap(t *testing.T) {
	engine := New(nil)
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	events := []UserEvent{
		{UserID: "u1", EventType: "click", Path: "/home", Timestamp: base},
		{UserID: "u1", EventType: "click", Path: "/settings", Timestamp: base.Add(1 * time.Minute)},
		{UserID: "u1", EventType: "rage_click", Path: "/settings", Element: "save-btn", Timestamp: base.Add(2 * time.Minute)},
		{UserID: "u1", EventType: "help", Path: "/settings", Timestamp: base.Add(3 * time.Minute)},
		{UserID: "u1", EventType: "backtrack", Path: "/home", Timestamp: base.Add(4 * time.Minute)},
		{UserID: "u1", EventType: "error", Path: "/settings", Element: "validation failed", Timestamp: base.Add(5 * time.Minute)},
		{UserID: "u1", EventType: "hesitate", Path: "/billing", Duration: 15000, Timestamp: base.Add(6 * time.Minute)},
	}

	for _, ev := range events {
		engine.IngestEvent(ev)
	}

	cm := engine.GetMap("u1")
	if cm == nil {
		t.Fatal("expected cognitive map for u1")
	}
	if cm.EventCount != len(events) {
		t.Errorf("expected %d events, got %d", len(events), cm.EventCount)
	}
	if cm.RageClicks != 1 {
		t.Errorf("expected 1 rage click, got %d", cm.RageClicks)
	}
	if cm.Backtracking != 1 {
		t.Errorf("expected 1 backtrack, got %d", cm.Backtracking)
	}
	if cm.HelpSeeking != 1 {
		t.Errorf("expected 1 help seeking, got %d", cm.HelpSeeking)
	}
	if len(cm.Confusions) == 0 {
		t.Error("expected confusions to be recorded")
	}
	if cm.PathFrequency["/settings"] != 3 {
		t.Errorf("expected /settings frequency 3, got %d", cm.PathFrequency["/settings"])
	}
}

func TestEngine_GetMap_NoUser(t *testing.T) {
	engine := New(nil)
	cm := engine.GetMap("nonexistent")
	if cm != nil {
		t.Error("expected nil for unknown user")
	}
}

func TestEngine_HesitationThreshold(t *testing.T) {
	engine := New(nil)
	now := time.Now()

	// Duration <= 10000ms should NOT create hesitation confusion
	engine.IngestEvent(UserEvent{
		UserID: "u1", EventType: "hesitate", Path: "/page",
		Duration: 9000, Timestamp: now,
	})
	cm := engine.GetMap("u1")
	for _, c := range cm.Confusions {
		if c.Type == "hesitation" {
			t.Error("9s hesitation should not create a confusion entry")
		}
	}

	// Duration > 10000ms should create hesitation confusion
	engine2 := New(nil)
	engine2.IngestEvent(UserEvent{
		UserID: "u2", EventType: "hesitate", Path: "/page",
		Duration: 11000, Timestamp: now,
	})
	cm2 := engine2.GetMap("u2")
	found := false
	for _, c := range cm2.Confusions {
		if c.Type == "hesitation" {
			found = true
		}
	}
	if !found {
		t.Error("11s hesitation should create a confusion entry")
	}
}

func TestEngine_Stats(t *testing.T) {
	engine := New(nil)
	now := time.Now()

	// Ingest events for two users
	engine.IngestEvent(UserEvent{UserID: "u1", EventType: "click", Path: "/home", Timestamp: now})
	engine.IngestEvent(UserEvent{UserID: "u1", EventType: "rage_click", Path: "/settings", Timestamp: now.Add(1 * time.Minute)})
	engine.IngestEvent(UserEvent{UserID: "u2", EventType: "click", Path: "/home", Timestamp: now})

	s := engine.Stats()
	if s.Users != 2 {
		t.Errorf("expected 2 users, got %d", s.Users)
	}
	if s.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", s.TotalEvents)
	}
}

func TestEngine_ConfusionHeatmap(t *testing.T) {
	engine := New(nil)
	now := time.Now()

	// Multiple rage clicks on /settings should score high
	for i := 0; i < 5; i++ {
		engine.IngestEvent(UserEvent{
			UserID: "u1", EventType: "rage_click", Path: "/settings",
			Timestamp: now.Add(time.Duration(i) * time.Minute),
		})
	}
	engine.IngestEvent(UserEvent{
		UserID: "u2", EventType: "error", Path: "/checkout",
		Timestamp: now,
	})

	heatmap := engine.ConfusionHeatmap()
	if len(heatmap) == 0 {
		t.Fatal("expected heatmap entries")
	}
	// /settings should be first (highest confusion)
	if heatmap[0].Path != "/settings" {
		t.Errorf("expected /settings at top of heatmap, got %s", heatmap[0].Path)
	}
	if heatmap[0].RageClicks != 5 {
		t.Errorf("expected 5 rage clicks, got %d", heatmap[0].RageClicks)
	}
	// confusion score for rage clicks = 3 each
	if heatmap[0].ConfusionScore != 15 {
		t.Errorf("expected confusion score 15 (5*3), got %d", heatmap[0].ConfusionScore)
	}
}

func TestEngine_Cohorts(t *testing.T) {
	engine := New(nil)
	now := time.Now()

	// Create a novice user (lots of issues)
	for i := 0; i < 10; i++ {
		engine.IngestEvent(UserEvent{
			UserID: "novice", EventType: "rage_click", Path: "/settings",
			Timestamp: now.Add(time.Duration(i) * time.Minute),
		})
	}
	for i := 0; i < 10; i++ {
		engine.IngestEvent(UserEvent{
			UserID: "novice", EventType: "help", Path: "/help",
			Timestamp: now.Add(time.Duration(10+i) * time.Minute),
		})
	}

	// Create an expert user (many paths, no issues)
	for i := 0; i < 25; i++ {
		engine.IngestEvent(UserEvent{
			UserID: "expert", EventType: "click", Path: "/" + string(rune('a'+i)),
			Timestamp: now.Add(time.Duration(i) * time.Minute),
		})
	}

	cohorts := engine.Cohorts()
	if cohorts["novice"].Count == 0 && cohorts["intermediate"].Count == 0 && cohorts["expert"].Count == 0 {
		t.Error("expected at least some users in cohorts")
	}
	// Total users across cohorts should equal 2
	total := cohorts["novice"].Count + cohorts["intermediate"].Count + cohorts["expert"].Count
	if total != 2 {
		t.Errorf("expected 2 users total in cohorts, got %d", total)
	}
}

func TestEngine_GetUserSessions(t *testing.T) {
	engine := New(nil)
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	events := []UserEvent{
		{UserID: "u1", EventType: "click", Path: "/home", Timestamp: base},
		{UserID: "u1", EventType: "click", Path: "/about", Timestamp: base.Add(5 * time.Minute)},
		// 35 minute gap
		{UserID: "u1", EventType: "click", Path: "/settings", Timestamp: base.Add(40 * time.Minute)},
	}
	for _, ev := range events {
		engine.IngestEvent(ev)
	}

	sessions := engine.GetUserSessions("u1")
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].EventCount != 2 {
		t.Errorf("first session: expected 2 events, got %d", sessions[0].EventCount)
	}
	if sessions[1].EventCount != 1 {
		t.Errorf("second session: expected 1 event, got %d", sessions[1].EventCount)
	}
}

func TestEngine_AvoidedPaths(t *testing.T) {
	engine := New(nil)
	now := time.Now()

	// u1 visits /home and /about
	engine.IngestEvent(UserEvent{UserID: "u1", EventType: "click", Path: "/home", Timestamp: now})
	engine.IngestEvent(UserEvent{UserID: "u1", EventType: "click", Path: "/about", Timestamp: now.Add(1 * time.Minute)})
	// u2 visits /home and /settings (creating /settings in allPaths)
	engine.IngestEvent(UserEvent{UserID: "u2", EventType: "click", Path: "/home", Timestamp: now})
	engine.IngestEvent(UserEvent{UserID: "u2", EventType: "click", Path: "/settings", Timestamp: now.Add(1 * time.Minute)})

	cm := engine.GetMap("u1")
	if cm == nil {
		t.Fatal("expected map for u1")
	}
	found := false
	for _, p := range cm.AvoidedPaths {
		if p == "/settings" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected /settings in avoided paths for u1, got %v", cm.AvoidedPaths)
	}
}

func TestEngine_EventCapAt5000(t *testing.T) {
	engine := New(nil)
	now := time.Now()

	// Ingest more than 5000 events
	for i := 0; i < 5010; i++ {
		engine.IngestEvent(UserEvent{
			UserID:    "u1",
			EventType: "click",
			Path:      "/page",
			Timestamp: now.Add(time.Duration(i) * time.Second),
		})
	}

	engine.mu.RLock()
	count := len(engine.events["u1"])
	engine.mu.RUnlock()

	if count > 5000 {
		t.Errorf("events should be capped at 5000, got %d", count)
	}
}

func TestEngine_TimestampAutoFill(t *testing.T) {
	engine := New(nil)
	engine.IngestEvent(UserEvent{
		UserID:    "u1",
		EventType: "click",
		Path:      "/home",
		// Timestamp is zero
	})

	cm := engine.GetMap("u1")
	if cm == nil {
		t.Fatal("expected map")
	}
	if cm.LastSeen.IsZero() {
		t.Error("expected non-zero LastSeen when timestamp auto-filled")
	}
}

func TestEngine_AllMaps(t *testing.T) {
	engine := New(nil)
	now := time.Now()

	engine.IngestEvent(UserEvent{UserID: "u1", EventType: "click", Path: "/home", Timestamp: now})
	engine.IngestEvent(UserEvent{UserID: "u2", EventType: "click", Path: "/home", Timestamp: now})

	maps := engine.AllMaps()
	if len(maps) != 2 {
		t.Errorf("expected 2 maps, got %d", len(maps))
	}
}

func TestEngine_AllMapsSince(t *testing.T) {
	engine := New(nil)
	t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 5, 10, 0, 0, 0, time.UTC)

	engine.IngestEvent(UserEvent{UserID: "old", EventType: "click", Path: "/home", Timestamp: t1})
	engine.IngestEvent(UserEvent{UserID: "new", EventType: "click", Path: "/home", Timestamp: t2})

	since := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	maps := engine.AllMapsSince(since, time.Time{})
	if len(maps) != 1 {
		t.Errorf("expected 1 map since cutoff, got %d", len(maps))
	}
	if len(maps) > 0 && maps[0].UserID != "new" {
		t.Errorf("expected user 'new', got %s", maps[0].UserID)
	}
}

func TestExtractCommonFlows(t *testing.T) {
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	var events []UserEvent
	// Repeat a flow 3 times: /home -> /products -> /cart
	for i := 0; i < 3; i++ {
		offset := time.Duration(i) * time.Hour
		events = append(events,
			UserEvent{Path: "/home", Timestamp: base.Add(offset)},
			UserEvent{Path: "/products", Timestamp: base.Add(offset + 1*time.Minute)},
			UserEvent{Path: "/cart", Timestamp: base.Add(offset + 2*time.Minute)},
		)
	}

	flows := extractCommonFlows(events)
	if len(flows) == 0 {
		t.Fatal("expected at least one common flow")
	}
	// The 2-gram /home -> /products should appear 3 times (>= 2 threshold)
	foundHomeProducts := false
	for _, f := range flows {
		if len(f) >= 2 && f[0] == "/home" && f[1] == "/products" {
			foundHomeProducts = true
		}
	}
	if !foundHomeProducts {
		t.Errorf("expected /home -> /products flow, got %v", flows)
	}
}
