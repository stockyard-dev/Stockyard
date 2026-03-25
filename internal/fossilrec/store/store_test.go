package store

import (
	"path/filepath"
	"testing"
)

func TestFossilRecStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	f := &FingerprintRecord{ID: "fp-1", Model: "gpt-4", Provider: "openai", SampleCount: 100, Signature: "abc123"}
	if err := db.CreateFingerprint(f); err != nil { t.Fatal(err) }
	fps, err := db.ListFingerprints("gpt-4", 10)
	if err != nil { t.Fatal(err) }
	if len(fps) != 1 { t.Errorf("got %d fps", len(fps)) }
}
