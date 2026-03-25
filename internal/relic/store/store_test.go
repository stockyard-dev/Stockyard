package store

import (
	"os"
	"path/filepath"
	"fmt"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := os.Stat(filepath.Join(dir, "test.db")); os.IsNotExist(err) {
		t.Fatal("database file not created")
	}
}

func TestCreateAndGet(t *testing.T) {
	db := testDB(t)
	c := &Certificate{
		ID: "cert-1", TraceID: "tr-1", ContentHash: "abc123",
		Model: "gpt-4", Provider: "openai", PromptHash: "ph1",
		ChainHash: "ch1", Confidence: 0.95,
		GuardrailsActive: "[]", Metadata: "{}",
	}
	if err := db.CreateCertificate(c); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetCertificate("cert-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "gpt-4" {
		t.Errorf("got model %s, want gpt-4", got.Model)
	}
}

func TestListAndTrace(t *testing.T) {
	db := testDB(t)
	for i := 0; i < 3; i++ {
		c := &Certificate{
			ID: fmt.Sprintf("cert-%d", i), TraceID: "tr-1", ContentHash: fmt.Sprintf("h%d", i),
			Model: "gpt-4", Provider: "openai", PromptHash: "ph1",
			ChainHash: fmt.Sprintf("ch%d", i), GuardrailsActive: "[]", Metadata: "{}",
		}
		db.CreateCertificate(c)
	}
	certs, err := db.ListCertificates(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(certs) != 3 {
		t.Errorf("got %d certs, want 3", len(certs))
	}
	trace, err := db.GetByTrace("tr-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(trace) != 3 {
		t.Errorf("got %d trace certs, want 3", len(trace))
	}
}
