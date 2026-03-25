package store

import (
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { db.Close() })
	return db
}

func TestBreedStore(t *testing.T) {
	db := testDB(t)
	p := &Population{ID: "pop-1", Name: "test", TargetEndpoint: "/api/chat", PopulationSize: 10, MutationRate: 0.1, CrossoverRate: 0.7, FitnessMetric: "quality"}
	if err := db.CreatePopulation(p); err != nil { t.Fatal(err) }
	got, err := db.GetPopulation("pop-1")
	if err != nil { t.Fatal(err) }
	if got.Name != "test" { t.Errorf("got %s", got.Name) }
}
