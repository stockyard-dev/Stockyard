package store

import (
	"path/filepath"
	"testing"
)

func TestMoltStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	a := &Analysis{ID: "a-1", ComponentType: "middleware", ComponentID: "rate-limiter", Recommendation: "shed", Reason: "no activity in 30 days"}
	if err := db.CreateAnalysis(a); err != nil { t.Fatal(err) }
	analyses, _ := db.ListAnalysis("")
	if len(analyses) != 1 { t.Errorf("got %d", len(analyses)) }
}
