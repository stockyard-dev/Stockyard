package store

import (
	"path/filepath"
	"testing"
)

func TestMyceliumStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	i := &Insight{ID: "i-1", InsightType: "model_behavior", Model: "gpt-4", Summary: "test", Data: "{}", Confidence: 0.8, SampleSize: 100}
	if err := db.CreateInsight(i); err != nil { t.Fatal(err) }
	insights, _ := db.ListInsights("", "", 10)
	if len(insights) != 1 { t.Errorf("got %d", len(insights)) }
}
