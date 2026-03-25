package store

import (
	"path/filepath"
	"testing"
)

func TestSporeStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	p := &Pattern{ID: "p-1", Name: "retry-pattern", Description: "Auto retry on timeout", TriggerConditions: `{"error":"timeout"}`, Actions: `{"retry":true}`, Environments: "[]", Status: "dormant"}
	if err := db.CreatePattern(p); err != nil { t.Fatal(err) }
	got, _ := db.GetPattern("p-1")
	if got.Name != "retry-pattern" { t.Errorf("got %s", got.Name) }
}
