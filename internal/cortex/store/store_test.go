package store

import (
	"path/filepath"
	"testing"
)

func TestCortexStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	m := &Memory{ID: "m-1", Category: "fact", Subject: "gpt-4", Predicate: "latency", Value: "200ms", Confidence: 0.9, DecayRate: 0.01}
	if err := db.CreateMemory(m); err != nil { t.Fatal(err) }
	got, err := db.GetMemory("m-1")
	if err != nil { t.Fatal(err) }
	if got.Subject != "gpt-4" { t.Errorf("got %s", got.Subject) }
}
