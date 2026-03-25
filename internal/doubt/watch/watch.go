// Package watch provides filesystem polling for Stockyard Doubt.
// It detects file changes and triggers re-scans and re-scores.
package watch

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
	"github.com/stockyard-dev/stockyard/internal/doubt/scorer"
	"github.com/stockyard-dev/stockyard/internal/doubt/tracker"
)

// Event represents a change detected by the watcher.
type Event struct {
	Type      string    `json:"type"`       // file_changed, scan_complete, score_update
	File      string    `json:"file,omitempty"`
	Units     int       `json:"units,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// EventHandler is called when a watch event occurs.
type EventHandler func(Event)

// Watcher polls the filesystem for changes and triggers re-scans.
type Watcher struct {
	dir      string
	interval time.Duration
	tracker  *tracker.Store
	handler  EventHandler

	mu         sync.RWMutex
	modTimes   map[string]time.Time
	lastReport *scorer.Report
	units      []scanner.Unit
	stopCh     chan struct{}
	stopped    bool
}

// Config holds watcher settings.
type Config struct {
	Dir      string
	Interval time.Duration // polling interval, default 5s
	Tracker  *tracker.Store
	Handler  EventHandler
}

// New creates a new file watcher.
func New(cfg Config) *Watcher {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Handler == nil {
		cfg.Handler = func(Event) {}
	}

	return &Watcher{
		dir:      cfg.Dir,
		interval: cfg.Interval,
		tracker:  cfg.Tracker,
		handler:  cfg.Handler,
		modTimes: map[string]time.Time{},
		stopCh:   make(chan struct{}),
	}
}

// Start begins the watch loop. It performs an initial scan, then polls for changes.
func (w *Watcher) Start() error {
	// Initial scan
	if err := w.fullScan(); err != nil {
		return fmt.Errorf("initial scan: %w", err)
	}

	// Build initial modtime map
	w.buildModTimeMap()

	go w.pollLoop()
	return nil
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.stopped {
		w.stopped = true
		close(w.stopCh)
	}
}

// Report returns the latest scored report.
func (w *Watcher) Report() *scorer.Report {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastReport
}

// Units returns the latest scanned units.
func (w *Watcher) Units() []scanner.Unit {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.units
}

func (w *Watcher) pollLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			changed := w.detectChanges()
			if len(changed) > 0 {
				for _, f := range changed {
					w.handler(Event{
						Type:      "file_changed",
						File:      f,
						Timestamp: time.Now(),
						Message:   fmt.Sprintf("File changed: %s", f),
					})
				}

				if err := w.fullScan(); err != nil {
					log.Printf("doubt/watch: re-scan error: %v", err)
					continue
				}

				w.handler(Event{
					Type:      "scan_complete",
					Units:     len(w.units),
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Re-scanned: %d units", len(w.units)),
				})
			}
		}
	}
}

func (w *Watcher) fullScan() error {
	result, err := scanner.ScanDir(w.dir)
	if err != nil {
		return err
	}

	report := scorer.Compute(result.Units, w.tracker)

	w.mu.Lock()
	w.units = result.Units
	w.lastReport = report
	w.mu.Unlock()

	w.handler(Event{
		Type:      "score_update",
		Units:     len(result.Units),
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Scored %d units, avg confidence %.0f%%", report.Summary.TotalUnits, report.Summary.AvgConfidence*100),
	})

	return nil
}

func (w *Watcher) buildModTimeMap() {
	w.mu.Lock()
	defer w.mu.Unlock()

	filepath.Walk(w.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				base := filepath.Base(path)
				if base == ".git" || base == "node_modules" || base == "vendor" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".go" || ext == ".py" || ext == ".ts" || ext == ".js" {
			w.modTimes[path] = info.ModTime()
		}
		return nil
	})
}

func (w *Watcher) detectChanges() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	var changed []string
	newModTimes := map[string]time.Time{}

	filepath.Walk(w.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				base := filepath.Base(path)
				if base == ".git" || base == "node_modules" || base == "vendor" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".py" && ext != ".ts" && ext != ".js" {
			return nil
		}

		newModTimes[path] = info.ModTime()

		// Check if file is new or modified
		oldTime, exists := w.modTimes[path]
		if !exists || info.ModTime().After(oldTime) {
			relPath, _ := filepath.Rel(w.dir, path)
			if relPath == "" {
				relPath = path
			}
			changed = append(changed, relPath)
		}
		return nil
	})

	// Check for deleted files
	for path := range w.modTimes {
		if _, exists := newModTimes[path]; !exists {
			relPath, _ := filepath.Rel(w.dir, path)
			if relPath == "" {
				relPath = path
			}
			changed = append(changed, relPath+" (deleted)")
		}
	}

	w.modTimes = newModTimes
	return changed
}

// WatchedExtensions returns the file extensions being watched.
func WatchedExtensions() []string {
	return []string{".go", ".py", ".ts", ".js"}
}

// FormatEvent returns a human-readable string for a watch event.
func FormatEvent(e Event) string {
	switch e.Type {
	case "file_changed":
		return fmt.Sprintf("[%s] changed: %s", e.Timestamp.Format("15:04:05"), e.File)
	case "scan_complete":
		return fmt.Sprintf("[%s] scan: %d units", e.Timestamp.Format("15:04:05"), e.Units)
	case "score_update":
		return fmt.Sprintf("[%s] %s", e.Timestamp.Format("15:04:05"), e.Message)
	default:
		return fmt.Sprintf("[%s] %s: %s", e.Timestamp.Format("15:04:05"), e.Type, e.Message)
	}
}

// IsWatchableFile checks if a file extension is one we should watch.
func IsWatchableFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".ts", ".js", ".tsx", ".jsx":
		return true
	}
	return false
}
