package store

import (
	"path/filepath"
	"testing"
)

func TestFeralStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	c := &Campaign{ID: "c-1", Name: "test", TargetURL: "http://localhost", AttackTypes: "[]"}
	if err := db.CreateCampaign(c); err != nil { t.Fatal(err) }
	got, err := db.GetCampaign("c-1")
	if err != nil { t.Fatal(err) }
	if got.Name != "test" { t.Errorf("got %s", got.Name) }
}
