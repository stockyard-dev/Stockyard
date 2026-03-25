package store

import (
	"path/filepath"
	"testing"
)

func TestTidepoolStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	stats, _ := db.Stats()
	if stats["observations"].(int) != 0 { t.Error("expected 0 observations") }
}
