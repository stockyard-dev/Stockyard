package store

import (
	"path/filepath"
	"testing"
)

func TestCrucibleStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	s := &Score{ID: "s-1", TraceID: "t-1", CompoundScore: 0.85, Components: "{}", DegradationFactors: "[]"}
	if err := db.CreateScore(s); err != nil { t.Fatal(err) }
	scores, _ := db.ListScores(10, 0)
	if len(scores) != 1 { t.Errorf("got %d scores", len(scores)) }
}
