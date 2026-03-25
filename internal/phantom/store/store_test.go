package store

import (
	"path/filepath"
	"testing"
)

func TestPhantomStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	p := &Persona{ID: "ph-1", Name: "Tester", TargetURL: "http://localhost", BehaviorProfile: "{}", Memory: "[]", Schedule: "hourly"}
	if err := db.CreatePersona(p); err != nil { t.Fatal(err) }
	got, err := db.GetPersona("ph-1")
	if err != nil { t.Fatal(err) }
	if got.Name != "Tester" { t.Errorf("got %s", got.Name) }
}
