// Package model builds cognitive maps of how each user understands your software.
// Tracks click patterns, hesitations, error encounters, and help-seeking to
// reconstruct what users think your app does vs what it actually does.
package model

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/prism/store"
)

// UserEvent is a behavioral signal from a user interaction.
type UserEvent struct {
	UserID    string    `json:"user_id"`
	EventType string   `json:"event_type"` // click, navigate, error, search, hesitate, rage_click, backtrack, help
	Path      string    `json:"path"`
	Element   string    `json:"element,omitempty"`
	Duration  int       `json:"duration_ms,omitempty"` // time spent on page/action
	Timestamp time.Time `json:"timestamp"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// CognitiveMap represents one user's mental model of your application.
type CognitiveMap struct {
	UserID          string                `json:"user_id"`
	EventCount      int                   `json:"event_count"`
	PathFrequency   map[string]int        `json:"path_frequency"`    // pages visited and how often
	Confusions      []Confusion           `json:"confusions"`        // moments of confusion
	Misunderstandings []Misunderstanding   `json:"misunderstandings"` // inferred wrong mental models
	Expertise       float64               `json:"expertise"`         // 0-1 estimated expertise
	CommonFlows     [][]string            `json:"common_flows"`      // typical navigation sequences
	AvoidedPaths    []string              `json:"avoided_paths"`     // pages they never visit
	HelpSeeking     int                   `json:"help_seeking"`      // times they looked for help
	RageClicks      int                   `json:"rage_clicks"`
	Backtracking    int                   `json:"backtracking"`      // navigating back after going somewhere
	LastSeen        time.Time             `json:"last_seen"`
}

// Confusion is a detected moment where a user was lost.
type Confusion struct {
	Path      string    `json:"path"`
	Type      string    `json:"type"` // hesitation, backtrack, rage_click, repeated_error
	Detail    string    `json:"detail"`
	Count     int       `json:"count"`
	Timestamp time.Time `json:"timestamp"`
}

// Misunderstanding is an inferred incorrect mental model.
type Misunderstanding struct {
	What     string `json:"what"`      // what the user seems to think
	Reality  string `json:"reality"`   // what actually happens
	Evidence string `json:"evidence"`  // behavioral evidence
	Severity string `json:"severity"`  // high, medium, low
}

// Engine builds and maintains cognitive maps for all users.
type Engine struct {
	mu     sync.RWMutex
	events map[string][]UserEvent // userID -> events
	maps   map[string]*CognitiveMap
	allPaths map[string]int // all known paths in the app
	db     *store.DB // optional persistence
}

// New creates a cognitive model engine. If db is non-nil, events are persisted
// to SQLite and restored on startup.
func New(db *store.DB) *Engine {
	e := &Engine{
		events:   map[string][]UserEvent{},
		maps:     map[string]*CognitiveMap{},
		allPaths: map[string]int{},
		db:       db,
	}
	if db != nil {
		e.loadFromStore()
	}
	return e
}

// loadFromStore restores path counts and per-user events from the database.
func (e *Engine) loadFromStore() {
	counts, err := e.db.GetAllPathCounts()
	if err != nil {
		log.Printf("prism: failed to load path counts: %v", err)
		return
	}
	e.allPaths = counts

	// Discover users by checking which user_ids have events.
	// We iterate known paths (a proxy) — but we need user IDs.
	// Instead, load events for each user. We'll find users via a count query.
	// Since there's no ListUsers method, we add one or scan user_events.
	userIDs, err := e.db.ListUserIDs()
	if err != nil {
		log.Printf("prism: failed to list users: %v", err)
		return
	}
	for _, uid := range userIDs {
		evts, err := e.db.GetUserEvents(uid, 5000)
		if err != nil {
			log.Printf("prism: failed to load events for user %s: %v", uid, err)
			continue
		}
		for _, se := range evts {
			e.events[uid] = append(e.events[uid], UserEvent{
				UserID:    se.UserID,
				EventType: se.EventType,
				Path:      se.Path,
				Element:   se.Element,
				Duration:  se.Duration,
				Timestamp: se.Timestamp,
				Meta:      se.Meta,
			})
		}
		e.rebuildMap(uid)
	}
	log.Printf("prism: loaded %d users, %d path entries from store", len(userIDs), len(counts))
}

// IngestEvent records a user behavioral event.
func (e *Engine) IngestEvent(ev UserEvent) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}

	// Persist to SQLite if available.
	if e.db != nil {
		se := store.UserEvent{
			UserID:    ev.UserID,
			EventType: ev.EventType,
			Path:      ev.Path,
			Element:   ev.Element,
			Duration:  ev.Duration,
			Timestamp: ev.Timestamp,
			Meta:      ev.Meta,
		}
		if err := e.db.SaveEvent(ev.UserID, se); err != nil {
			log.Printf("prism: failed to persist event: %v", err)
		}
	}

	e.mu.Lock()
	e.events[ev.UserID] = append(e.events[ev.UserID], ev)
	if len(e.events[ev.UserID]) > 5000 {
		e.events[ev.UserID] = e.events[ev.UserID][len(e.events[ev.UserID])-5000:]
	}
	e.allPaths[ev.Path]++
	e.mu.Unlock()

	// Rebuild map for this user
	e.rebuildMap(ev.UserID)
}

func (e *Engine) rebuildMap(userID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	events := e.events[userID]
	if len(events) == 0 {
		return
	}

	cm := &CognitiveMap{
		UserID:        userID,
		EventCount:    len(events),
		PathFrequency: map[string]int{},
		LastSeen:      events[len(events)-1].Timestamp,
	}

	// Path frequency
	for _, ev := range events {
		cm.PathFrequency[ev.Path]++
		switch ev.EventType {
		case "rage_click":
			cm.RageClicks++
			cm.Confusions = append(cm.Confusions, Confusion{Path: ev.Path, Type: "rage_click", Detail: ev.Element, Count: 1, Timestamp: ev.Timestamp})
		case "backtrack":
			cm.Backtracking++
			cm.Confusions = append(cm.Confusions, Confusion{Path: ev.Path, Type: "backtrack", Detail: "navigated away then came back", Count: 1, Timestamp: ev.Timestamp})
		case "help":
			cm.HelpSeeking++
		case "hesitate":
			if ev.Duration > 10000 { // >10 seconds on a page = hesitation
				cm.Confusions = append(cm.Confusions, Confusion{Path: ev.Path, Type: "hesitation", Detail: fmt.Sprintf("spent %ds", ev.Duration/1000), Count: 1, Timestamp: ev.Timestamp})
			}
		case "error":
			cm.Confusions = append(cm.Confusions, Confusion{Path: ev.Path, Type: "repeated_error", Detail: ev.Element, Count: 1, Timestamp: ev.Timestamp})
		}
	}

	// Find avoided paths — paths in the app the user never visits
	for path := range e.allPaths {
		if cm.PathFrequency[path] == 0 {
			cm.AvoidedPaths = append(cm.AvoidedPaths, path)
		}
	}

	// Estimate expertise from behavioral signals
	cm.Expertise = estimateExpertise(cm)

	// Infer misunderstandings from patterns
	cm.Misunderstandings = inferMisunderstandings(cm, events)

	// Deduplicate confusions by path+type
	cm.Confusions = dedupeConfusions(cm.Confusions)

	e.maps[userID] = cm
}

// GetMap returns the cognitive map for a user.
func (e *Engine) GetMap(userID string) *CognitiveMap {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if m, ok := e.maps[userID]; ok {
		cp := *m
		return &cp
	}
	return nil
}

// AllMaps returns cognitive maps for all users.
func (e *Engine) AllMaps() []CognitiveMap {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []CognitiveMap
	for _, m := range e.maps {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Expertise < out[j].Expertise })
	return out
}

// Stats returns aggregate statistics.
type Stats struct {
	Users        int     `json:"users"`
	TotalEvents  int     `json:"total_events"`
	AvgExpertise float64 `json:"avg_expertise"`
	TopConfusion string  `json:"top_confusion_path"`
}

func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := Stats{Users: len(e.maps)}
	confusionPaths := map[string]int{}
	for _, m := range e.maps {
		s.TotalEvents += m.EventCount
		s.AvgExpertise += m.Expertise
		for _, c := range m.Confusions {
			confusionPaths[c.Path]++
		}
	}
	if s.Users > 0 { s.AvgExpertise /= float64(s.Users) }
	topCount := 0
	for p, c := range confusionPaths {
		if c > topCount { topCount = c; s.TopConfusion = p }
	}
	return s
}

func estimateExpertise(cm *CognitiveMap) float64 {
	score := 0.5 // baseline

	// More pages visited = more experienced
	if len(cm.PathFrequency) > 10 { score += 0.1 }
	if len(cm.PathFrequency) > 20 { score += 0.1 }

	// Less help-seeking = more experienced
	if cm.HelpSeeking == 0 { score += 0.1 }
	if cm.HelpSeeking > 5 { score -= 0.15 }

	// Fewer rage clicks = more experienced
	if cm.RageClicks == 0 { score += 0.1 }
	if cm.RageClicks > 3 { score -= 0.2 }

	// Less backtracking = more experienced
	if cm.Backtracking == 0 { score += 0.05 }
	if cm.Backtracking > 5 { score -= 0.1 }

	if score < 0 { score = 0 }
	if score > 1 { score = 1 }
	return score
}

func inferMisunderstandings(cm *CognitiveMap, events []UserEvent) []Misunderstanding {
	var mis []Misunderstanding

	// If user keeps going to wrong page for a task, they misunderstand navigation
	if cm.Backtracking > 3 {
		mis = append(mis, Misunderstanding{
			What:     "User expects the feature to be somewhere it isn't",
			Reality:  "Feature is located in a different section",
			Evidence: fmt.Sprintf("%d backtracking events detected", cm.Backtracking),
			Severity: "medium",
		})
	}

	// If user rage-clicks something, they think it should do something it doesn't
	if cm.RageClicks > 2 {
		mis = append(mis, Misunderstanding{
			What:     "User expects a UI element to be interactive",
			Reality:  "The element may be disabled, loading, or non-interactive",
			Evidence: fmt.Sprintf("%d rage click events", cm.RageClicks),
			Severity: "high",
		})
	}

	return mis
}

func dedupeConfusions(confusions []Confusion) []Confusion {
	seen := map[string]*Confusion{}
	for i := range confusions {
		key := confusions[i].Path + ":" + confusions[i].Type
		if existing, ok := seen[key]; ok {
			existing.Count++
		} else {
			cp := confusions[i]
			seen[key] = &cp
		}
	}
	var out []Confusion
	for _, c := range seen {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}
