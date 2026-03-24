// Package model builds cognitive maps of how each user understands your software.
// Tracks click patterns, hesitations, error encounters, and help-seeking to
// reconstruct what users think your app does vs what it actually does.
package model

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/prism/store"
)

const sessionGap = 30 * time.Minute

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

// Session represents a contiguous period of user activity separated by
// 30-minute inactivity gaps.
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	EventCount int       `json:"event_count"`
	Paths      []string  `json:"paths"`
}

// HeatmapEntry represents confusion signals aggregated for a single path
// across all users.
type HeatmapEntry struct {
	Path           string  `json:"path"`
	ConfusionScore int     `json:"confusion_score"`
	RageClicks     int     `json:"rage_clicks"`
	Backtracks     int     `json:"backtracks"`
	Hesitations    int     `json:"hesitations"`
	Errors         int     `json:"errors"`
}

// CohortStats describes a group of users clustered by expertise level.
type CohortStats struct {
	Count        int     `json:"count"`
	AvgConfusion float64 `json:"avg_confusion"`
	AvgExpertise float64 `json:"avg_expertise"`
	AvgEvents    float64 `json:"avg_events"`
	UserIDs      []string `json:"user_ids"`
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

	// Extract common navigation flows
	cm.CommonFlows = extractCommonFlows(events)

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

// AllMapsSince returns cognitive maps for users active since the given time.
func (e *Engine) AllMapsSince(since, until time.Time) []CognitiveMap {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []CognitiveMap
	for _, m := range e.maps {
		if !since.IsZero() && m.LastSeen.Before(since) {
			continue
		}
		if !until.IsZero() && m.LastSeen.After(until) {
			continue
		}
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

// StatsSince returns aggregate statistics filtered by time window.
func (e *Engine) StatsSince(since, until time.Time) Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := Stats{}
	confusionPaths := map[string]int{}
	for _, m := range e.maps {
		if !since.IsZero() && m.LastSeen.Before(since) {
			continue
		}
		if !until.IsZero() && m.LastSeen.After(until) {
			continue
		}
		s.Users++
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

// GetUserSessions groups a user's events into sessions separated by 30-minute
// inactivity gaps.
func (e *Engine) GetUserSessions(userID string) []Session {
	e.mu.RLock()
	events := e.events[userID]
	e.mu.RUnlock()

	return buildSessions(userID, events)
}

// buildSessions creates sessions from a chronologically ordered list of events.
func buildSessions(userID string, events []UserEvent) []Session {
	if len(events) == 0 {
		return nil
	}

	var sessions []Session
	sessionIdx := 0
	cur := Session{
		UserID:    userID,
		StartedAt: events[0].Timestamp,
		EndedAt:   events[0].Timestamp,
		Paths:     []string{events[0].Path},
	}
	cur.EventCount = 1

	for i := 1; i < len(events); i++ {
		gap := events[i].Timestamp.Sub(events[i-1].Timestamp)
		if gap > sessionGap {
			// Close current session and start new one.
			cur.ID = fmt.Sprintf("%s-s%d", userID, sessionIdx)
			cur.Paths = dedupePaths(cur.Paths)
			sessions = append(sessions, cur)
			sessionIdx++
			cur = Session{
				UserID:    userID,
				StartedAt: events[i].Timestamp,
				EndedAt:   events[i].Timestamp,
				Paths:     []string{events[i].Path},
			}
			cur.EventCount = 1
		} else {
			cur.EndedAt = events[i].Timestamp
			cur.EventCount++
			cur.Paths = append(cur.Paths, events[i].Path)
		}
	}

	// Close last session.
	cur.ID = fmt.Sprintf("%s-s%d", userID, sessionIdx)
	cur.Paths = dedupePaths(cur.Paths)
	sessions = append(sessions, cur)

	return sessions
}

// dedupePaths returns unique paths preserving first-seen order.
func dedupePaths(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// extractCommonFlows finds the most common 2-3 step navigation sequences
// (n-grams) from the user's events grouped into sessions.
func extractCommonFlows(events []UserEvent) [][]string {
	if len(events) < 2 {
		return nil
	}

	// Build sessions first
	sessions := buildSessions("", events)

	// Count n-grams (2 and 3) across sessions
	ngrams := map[string]int{}
	for _, sess := range sessions {
		// Use full path sequence (not deduped) for this session
		var paths []string
		for _, p := range events {
			if !p.Timestamp.Before(sess.StartedAt) && !p.Timestamp.After(sess.EndedAt) {
				paths = append(paths, p.Path)
			}
		}

		// 2-grams
		for i := 0; i+1 < len(paths); i++ {
			key := paths[i] + " -> " + paths[i+1]
			ngrams[key]++
		}
		// 3-grams
		for i := 0; i+2 < len(paths); i++ {
			key := paths[i] + " -> " + paths[i+1] + " -> " + paths[i+2]
			ngrams[key]++
		}
	}

	// Sort by frequency, take top 5
	type kv struct {
		key   string
		count int
	}
	var sorted []kv
	for k, v := range ngrams {
		if v >= 2 { // only include flows seen at least twice
			sorted = append(sorted, kv{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	topN := 5
	if len(sorted) < topN {
		topN = len(sorted)
	}

	var flows [][]string
	for i := 0; i < topN; i++ {
		parts := strings.Split(sorted[i].key, " -> ")
		flows = append(flows, parts)
	}
	return flows
}

// ConfusionHeatmap aggregates confusion signals across all users, returning
// a ranked list of paths by confusion score.
func (e *Engine) ConfusionHeatmap() []HeatmapEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pathData := map[string]*HeatmapEntry{}

	for _, events := range e.events {
		for _, ev := range events {
			entry, ok := pathData[ev.Path]
			if !ok {
				entry = &HeatmapEntry{Path: ev.Path}
				pathData[ev.Path] = entry
			}
			switch ev.EventType {
			case "rage_click":
				entry.RageClicks++
				entry.ConfusionScore += 3
			case "backtrack":
				entry.Backtracks++
				entry.ConfusionScore += 2
			case "hesitate":
				if ev.Duration > 10000 {
					entry.Hesitations++
					entry.ConfusionScore += 2
				}
			case "error":
				entry.Errors++
				entry.ConfusionScore += 3
			}
		}
	}

	var result []HeatmapEntry
	for _, entry := range pathData {
		if entry.ConfusionScore > 0 {
			result = append(result, *entry)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ConfusionScore > result[j].ConfusionScore })
	return result
}

// Cohorts clusters users into novice, intermediate, and expert groups.
func (e *Engine) Cohorts() map[string]*CohortStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cohorts := map[string]*CohortStats{
		"novice":       {},
		"intermediate": {},
		"expert":       {},
	}

	for _, m := range e.maps {
		var cohort string
		switch {
		case m.Expertise < 0.3:
			cohort = "novice"
		case m.Expertise <= 0.7:
			cohort = "intermediate"
		default:
			cohort = "expert"
		}
		c := cohorts[cohort]
		c.Count++
		c.AvgConfusion += float64(len(m.Confusions))
		c.AvgExpertise += m.Expertise
		c.AvgEvents += float64(m.EventCount)
		c.UserIDs = append(c.UserIDs, m.UserID)
	}

	for _, c := range cohorts {
		if c.Count > 0 {
			c.AvgConfusion /= float64(c.Count)
			c.AvgExpertise /= float64(c.Count)
			c.AvgEvents /= float64(c.Count)
		}
	}

	return cohorts
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
