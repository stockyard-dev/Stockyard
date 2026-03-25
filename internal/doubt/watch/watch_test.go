package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	dir := t.TempDir()
	w := New(Config{Dir: dir})

	if w.dir != dir {
		t.Errorf("dir = %q, want %q", w.dir, dir)
	}
	if w.interval != 5*time.Second {
		t.Errorf("interval = %v, want 5s", w.interval)
	}
	if w.handler == nil {
		t.Error("handler should not be nil (default noop)")
	}
	if w.modTimes == nil {
		t.Error("modTimes should be initialized")
	}
	if w.stopped {
		t.Error("should not be stopped initially")
	}
}

func TestNew_CustomInterval(t *testing.T) {
	w := New(Config{
		Dir:      t.TempDir(),
		Interval: 10 * time.Second,
	})
	if w.interval != 10*time.Second {
		t.Errorf("interval = %v, want 10s", w.interval)
	}
}

func TestNew_CustomHandler(t *testing.T) {
	called := false
	w := New(Config{
		Dir:     t.TempDir(),
		Handler: func(Event) { called = true },
	})
	w.handler(Event{})
	if !called {
		t.Error("custom handler should have been called")
	}
}

func TestStop(t *testing.T) {
	w := New(Config{Dir: t.TempDir()})
	w.Stop()
	if !w.stopped {
		t.Error("should be stopped after Stop()")
	}
	// Double stop should not panic
	w.Stop()
}

func TestReport_NilByDefault(t *testing.T) {
	w := New(Config{Dir: t.TempDir()})
	if w.Report() != nil {
		t.Error("Report should be nil before Start")
	}
}

func TestUnits_EmptyByDefault(t *testing.T) {
	w := New(Config{Dir: t.TempDir()})
	if len(w.Units()) != 0 {
		t.Errorf("Units should be empty before Start, got %d", len(w.Units()))
	}
}

func TestWatchedExtensions(t *testing.T) {
	exts := WatchedExtensions()
	expected := map[string]bool{".go": true, ".py": true, ".ts": true, ".js": true}

	if len(exts) != len(expected) {
		t.Errorf("WatchedExtensions() = %v, want %v extensions", exts, len(expected))
	}
	for _, ext := range exts {
		if !expected[ext] {
			t.Errorf("unexpected extension %q", ext)
		}
	}
}

func TestIsWatchableFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"app.py", true},
		{"index.ts", true},
		{"script.js", true},
		{"component.tsx", true},
		{"component.jsx", true},
		{"readme.md", false},
		{"data.json", false},
		{"Makefile", false},
		{"image.png", false},
		{"MAIN.GO", true},
		{"APP.PY", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsWatchableFile(tt.path)
			if got != tt.want {
				t.Errorf("IsWatchableFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFormatEvent(t *testing.T) {
	ts := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name  string
		event Event
		want  string
	}{
		{
			name:  "file changed",
			event: Event{Type: "file_changed", File: "main.go", Timestamp: ts},
			want:  "[14:30:45] changed: main.go",
		},
		{
			name:  "scan complete",
			event: Event{Type: "scan_complete", Units: 42, Timestamp: ts},
			want:  "[14:30:45] scan: 42 units",
		},
		{
			name:  "score update",
			event: Event{Type: "score_update", Message: "Scored 10 units", Timestamp: ts},
			want:  "[14:30:45] Scored 10 units",
		},
		{
			name:  "unknown type",
			event: Event{Type: "custom", Message: "something happened", Timestamp: ts},
			want:  "[14:30:45] custom: something happened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatEvent(tt.event)
			if got != tt.want {
				t.Errorf("FormatEvent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildModTimeMap(t *testing.T) {
	dir := t.TempDir()

	// Create watchable files
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "app.py"), []byte("pass"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme"), 0644)

	w := New(Config{Dir: dir})
	w.buildModTimeMap()

	w.mu.RLock()
	defer w.mu.RUnlock()

	goPath := filepath.Join(dir, "main.go")
	if _, ok := w.modTimes[goPath]; !ok {
		t.Error("main.go should be in modTimes")
	}

	pyPath := filepath.Join(dir, "app.py")
	if _, ok := w.modTimes[pyPath]; !ok {
		t.Error("app.py should be in modTimes")
	}

	mdPath := filepath.Join(dir, "readme.md")
	if _, ok := w.modTimes[mdPath]; ok {
		t.Error("readme.md should NOT be in modTimes")
	}
}

func TestDetectChanges_NewFile(t *testing.T) {
	dir := t.TempDir()

	w := New(Config{Dir: dir})
	w.buildModTimeMap()

	// Create a new file after initial map
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main"), 0644)

	changed := w.detectChanges()
	if len(changed) != 1 {
		t.Errorf("got %d changes, want 1", len(changed))
	}
}

func TestDetectChanges_DeletedFile(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "delete_me.go")
	os.WriteFile(path, []byte("package main"), 0644)

	w := New(Config{Dir: dir})
	w.buildModTimeMap()

	// Delete the file
	os.Remove(path)

	changed := w.detectChanges()
	found := false
	for _, c := range changed {
		if c == "delete_me.go (deleted)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected deleted file in changes, got %v", changed)
	}
}

func TestDetectChanges_NoChanges(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "stable.go"), []byte("package main"), 0644)

	w := New(Config{Dir: dir})
	w.buildModTimeMap()

	// Detect changes immediately - should be none
	// Need to first populate modTimes via detectChanges (which replaces)
	// So call detectChanges twice: first sets up, second finds no changes
	w.detectChanges()
	changed := w.detectChanges()
	if len(changed) != 0 {
		t.Errorf("got %d changes, want 0", len(changed))
	}
}

func TestDetectChanges_SkipsDirs(t *testing.T) {
	dir := t.TempDir()

	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "config.go"), []byte("package git"), 0644)

	w := New(Config{Dir: dir})
	w.buildModTimeMap()

	w.mu.RLock()
	for path := range w.modTimes {
		if filepath.Dir(path) == gitDir {
			w.mu.RUnlock()
			t.Error(".git files should not be tracked")
			return
		}
	}
	w.mu.RUnlock()
}
