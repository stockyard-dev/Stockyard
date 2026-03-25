package store

import (
	"database/sql"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%q): %v", dir, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeSpec(id, name, content, format string) *Spec {
	now := time.Now().UTC().Truncate(time.Second)
	return &Spec{
		ID:        id,
		Name:      name,
		Content:   content,
		Format:    format,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func makeBuild(id, specID, status string) *Build {
	now := time.Now().UTC().Truncate(time.Second)
	return &Build{
		ID:        id,
		SpecID:    specID,
		Status:    status,
		OutputDir: "/tmp/out",
		FilesJSON: "[]",
		StartedAt: now,
	}
}

// ---------------------------------------------------------------------------
// Initialization / Migration
// ---------------------------------------------------------------------------

func TestOpen_CreatesDatabase(t *testing.T) {
	s := newTestStore(t)
	if s.DataDir() == "" {
		t.Error("DataDir should not be empty")
	}
	// Tables should exist
	if _, err := s.ListSpecs(); err != nil {
		t.Fatalf("ListSpecs on fresh db: %v", err)
	}
	if _, err := s.ListBuilds(); err != nil {
		t.Fatalf("ListBuilds on fresh db: %v", err)
	}
}

func TestOpen_MigrateIdempotent(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open(dir)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	s2.Close()
}

// ---------------------------------------------------------------------------
// Specs CRUD
// ---------------------------------------------------------------------------

func TestCreateSpec_and_GetSpec(t *testing.T) {
	s := newTestStore(t)
	spec := makeSpec("spec-1", "UserAPI", "openapi: 3.0.0", "yaml")

	if err := s.CreateSpec(spec); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	got, err := s.GetSpec("spec-1")
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if got == nil {
		t.Fatal("GetSpec returned nil")
	}
	if got.Name != "UserAPI" || got.Content != "openapi: 3.0.0" || got.Format != "yaml" {
		t.Errorf("unexpected spec: %+v", got)
	}
}

func TestGetSpec_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetSpec("nonexistent")
	if err != nil {
		t.Fatalf("GetSpec error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent spec, got %+v", got)
	}
}

func TestCreateSpec_DuplicateID(t *testing.T) {
	s := newTestStore(t)
	spec := makeSpec("spec-1", "A", "content", "yaml")
	if err := s.CreateSpec(spec); err != nil {
		t.Fatalf("first CreateSpec: %v", err)
	}
	// Same ID should fail (PRIMARY KEY constraint)
	err := s.CreateSpec(spec)
	if err == nil {
		t.Error("expected error on duplicate spec ID")
	}
}

func TestListSpecs_Empty(t *testing.T) {
	s := newTestStore(t)
	specs, err := s.ListSpecs()
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 specs, got %d", len(specs))
	}
}

func TestListSpecs_OrderedByUpdatedAtDesc(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		spec := &Spec{
			ID:        string(rune('a' + i)),
			Name:      string(rune('A' + i)),
			Content:   "c",
			Format:    "yaml",
			CreatedAt: base,
			UpdatedAt: base.Add(time.Duration(i) * time.Hour),
		}
		s.CreateSpec(spec)
	}

	specs, _ := s.ListSpecs()
	if len(specs) != 3 {
		t.Fatalf("expected 3 specs, got %d", len(specs))
	}
	if specs[0].ID != "c" || specs[2].ID != "a" {
		t.Errorf("unexpected order: %s, %s, %s", specs[0].ID, specs[1].ID, specs[2].ID)
	}
}

func TestDeleteSpec(t *testing.T) {
	s := newTestStore(t)
	spec := makeSpec("spec-1", "A", "c", "yaml")
	s.CreateSpec(spec)

	if err := s.DeleteSpec("spec-1"); err != nil {
		t.Fatalf("DeleteSpec: %v", err)
	}

	got, _ := s.GetSpec("spec-1")
	if got != nil {
		t.Error("spec should be deleted")
	}
}

func TestDeleteSpec_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.DeleteSpec("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestDeleteSpec_CascadesBuilds(t *testing.T) {
	s := newTestStore(t)
	spec := makeSpec("spec-1", "A", "c", "yaml")
	s.CreateSpec(spec)
	b := makeBuild("build-1", "spec-1", "success")
	s.CreateBuild(b)

	s.DeleteSpec("spec-1")

	builds, _ := s.ListBuildsBySpec("spec-1")
	if len(builds) != 0 {
		t.Errorf("expected cascade delete of builds, got %d", len(builds))
	}
}

// ---------------------------------------------------------------------------
// Builds CRUD
// ---------------------------------------------------------------------------

func TestCreateBuild_and_GetBuild(t *testing.T) {
	s := newTestStore(t)
	spec := makeSpec("spec-1", "A", "c", "yaml")
	s.CreateSpec(spec)

	b := makeBuild("build-1", "spec-1", "pending")
	if err := s.CreateBuild(b); err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	got, err := s.GetBuild("build-1")
	if err != nil {
		t.Fatalf("GetBuild: %v", err)
	}
	if got == nil {
		t.Fatal("GetBuild returned nil")
	}
	if got.SpecID != "spec-1" || got.Status != "pending" {
		t.Errorf("unexpected build: %+v", got)
	}
}

func TestGetBuild_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetBuild("nonexistent")
	if err != nil {
		t.Fatalf("GetBuild error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent build, got %+v", got)
	}
}

func TestUpdateBuild(t *testing.T) {
	s := newTestStore(t)
	spec := makeSpec("spec-1", "A", "c", "yaml")
	s.CreateSpec(spec)

	b := makeBuild("build-1", "spec-1", "pending")
	s.CreateBuild(b)

	b.Status = "success"
	b.CompletedAt = time.Now().UTC().Truncate(time.Second)
	b.FilesJSON = `[{"path":"main.go","content":"package main"}]`
	if err := s.UpdateBuild(b); err != nil {
		t.Fatalf("UpdateBuild: %v", err)
	}

	got, _ := s.GetBuild("build-1")
	if got.Status != "success" {
		t.Errorf("expected status=success, got %q", got.Status)
	}
	if got.FilesJSON != `[{"path":"main.go","content":"package main"}]` {
		t.Errorf("unexpected FilesJSON: %q", got.FilesJSON)
	}
}

func TestListBuilds_Empty(t *testing.T) {
	s := newTestStore(t)
	builds, err := s.ListBuilds()
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if len(builds) != 0 {
		t.Errorf("expected 0 builds, got %d", len(builds))
	}
}

func TestListBuildsBySpec(t *testing.T) {
	s := newTestStore(t)
	s.CreateSpec(makeSpec("spec-1", "A", "c", "yaml"))
	s.CreateSpec(makeSpec("spec-2", "B", "c", "yaml"))

	now := time.Now().UTC().Truncate(time.Second)
	s.CreateBuild(&Build{ID: "b1", SpecID: "spec-1", Status: "success", FilesJSON: "[]", StartedAt: now})
	s.CreateBuild(&Build{ID: "b2", SpecID: "spec-1", Status: "failed", FilesJSON: "[]", StartedAt: now.Add(time.Second)})
	s.CreateBuild(&Build{ID: "b3", SpecID: "spec-2", Status: "success", FilesJSON: "[]", StartedAt: now})

	builds, err := s.ListBuildsBySpec("spec-1")
	if err != nil {
		t.Fatalf("ListBuildsBySpec: %v", err)
	}
	if len(builds) != 2 {
		t.Errorf("expected 2 builds for spec-1, got %d", len(builds))
	}
	// Most recent first
	if builds[0].ID != "b2" {
		t.Errorf("expected b2 first, got %s", builds[0].ID)
	}
}

func TestListBuilds_OrderedByStartedAtDesc(t *testing.T) {
	s := newTestStore(t)
	s.CreateSpec(makeSpec("spec-1", "A", "c", "yaml"))
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		s.CreateBuild(&Build{
			ID:        string(rune('a' + i)),
			SpecID:    "spec-1",
			Status:    "success",
			FilesJSON: "[]",
			StartedAt: base.Add(time.Duration(i) * time.Hour),
		})
	}

	builds, _ := s.ListBuilds()
	if len(builds) != 3 {
		t.Fatalf("expected 3 builds, got %d", len(builds))
	}
	if builds[0].ID != "c" || builds[2].ID != "a" {
		t.Errorf("unexpected order: %s, %s, %s", builds[0].ID, builds[1].ID, builds[2].ID)
	}
}

// ---------------------------------------------------------------------------
// ParseBuildFiles
// ---------------------------------------------------------------------------

func TestParseBuildFiles(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{"empty string", "", 0, false},
		{"empty array", "[]", 0, false},
		{"one file", `[{"path":"a.go","content":"pkg"}]`, 1, false},
		{"invalid json", `{bad`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := ParseBuildFiles(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBuildFiles(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if len(files) != tt.wantCount {
				t.Errorf("expected %d files, got %d", tt.wantCount, len(files))
			}
		})
	}
}
