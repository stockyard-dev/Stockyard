package store

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "verdikt_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_MigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verdikt.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db2.Close()
}

// ── Evaluations ───────────────────────────────────────────────────

func TestSaveAndListEvaluations(t *testing.T) {
	db := newTestDB(t)

	e := Evaluation{
		RequestID: "req-1", Domain: "chat", Score: 0.85,
		Relevance: 0.9, Accuracy: 0.8, Helpfulness: 0.85, Safety: 1.0, Tone: 0.9,
		Verdict: "good", Issues: []string{"minor tone"}, LatencyMs: 150,
	}
	if err := db.SaveEvaluation(e); err != nil {
		t.Fatal(err)
	}

	evals, err := db.ListEvaluations(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(evals) != 1 {
		t.Fatalf("expected 1, got %d", len(evals))
	}
	if evals[0].RequestID != "req-1" {
		t.Errorf("expected req-1, got %s", evals[0].RequestID)
	}
	if evals[0].Score != 0.85 {
		t.Errorf("expected 0.85, got %f", evals[0].Score)
	}
	if len(evals[0].Issues) != 1 || evals[0].Issues[0] != "minor tone" {
		t.Errorf("unexpected issues: %v", evals[0].Issues)
	}
}

func TestListEvaluations_Empty(t *testing.T) {
	db := newTestDB(t)
	evals, err := db.ListEvaluations(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(evals) != 0 {
		t.Fatalf("expected 0, got %d", len(evals))
	}
}

func TestListEvaluations_Limit(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 5; i++ {
		db.SaveEvaluation(Evaluation{RequestID: "r", Domain: "d", Score: float64(i)})
	}
	evals, _ := db.ListEvaluations(2)
	if len(evals) != 2 {
		t.Fatalf("expected 2, got %d", len(evals))
	}
}

func TestSaveEvaluation_NilIssues(t *testing.T) {
	db := newTestDB(t)
	e := Evaluation{RequestID: "r", Domain: "d", Score: 0.5, Issues: nil}
	if err := db.SaveEvaluation(e); err != nil {
		t.Fatal(err)
	}
	evals, _ := db.ListEvaluations(1)
	if evals[0].Issues == nil {
		// json.Unmarshal of "null" yields nil -- that's acceptable
	}
}

// ── Feedback ──────────────────────────────────────────────────────

func TestSaveFeedback(t *testing.T) {
	db := newTestDB(t)
	if err := db.SaveFeedback("req-1", "thumbs_up", "great answer", 0.9); err != nil {
		t.Fatal(err)
	}
	// Verify row was inserted by querying directly.
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM feedback").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 feedback row, got %d", count)
	}
}

// ── Baselines ─────────────────────────────────────────────────────

func TestBaselines_SetGetList(t *testing.T) {
	db := newTestDB(t)

	if err := db.SetBaseline("chat", 0.8); err != nil {
		t.Fatal(err)
	}
	if err := db.SetBaseline("code", 0.7); err != nil {
		t.Fatal(err)
	}

	score, err := db.GetBaseline("chat")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.8 {
		t.Errorf("expected 0.8, got %f", score)
	}

	all, err := db.ListBaselines()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
}

func TestGetBaseline_Missing(t *testing.T) {
	db := newTestDB(t)
	score, err := db.GetBaseline("missing")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Errorf("expected 0, got %f", score)
	}
}

func TestSetBaseline_Upsert(t *testing.T) {
	db := newTestDB(t)
	db.SetBaseline("chat", 0.7)
	db.SetBaseline("chat", 0.9)
	score, _ := db.GetBaseline("chat")
	if score != 0.9 {
		t.Errorf("expected 0.9, got %f", score)
	}
}

// ── Calibration ───────────────────────────────────────────────────

func TestCalibration_SetAndGet(t *testing.T) {
	db := newTestDB(t)
	if err := db.SetCalibration("weight_safety", 1.5); err != nil {
		t.Fatal(err)
	}
	val, err := db.GetCalibration("weight_safety")
	if err != nil {
		t.Fatal(err)
	}
	if val != 1.5 {
		t.Errorf("expected 1.5, got %f", val)
	}
}

func TestGetCalibration_Missing(t *testing.T) {
	db := newTestDB(t)
	val, err := db.GetCalibration("missing")
	if err != nil {
		t.Fatal(err)
	}
	if val != 0 {
		t.Errorf("expected 0, got %f", val)
	}
}

func TestCalibration_Upsert(t *testing.T) {
	db := newTestDB(t)
	db.SetCalibration("k", 1.0)
	db.SetCalibration("k", 2.0)
	val, _ := db.GetCalibration("k")
	if val != 2.0 {
		t.Errorf("expected 2.0, got %f", val)
	}
}

// ── Batch Evaluations ─────────────────────────────────────────────

func TestSaveBatchEvaluation(t *testing.T) {
	db := newTestDB(t)
	evals := []Evaluation{
		{RequestID: "r1", Domain: "chat", Score: 0.9, Issues: []string{"a"}},
		{RequestID: "r2", Domain: "code", Score: 0.7, Issues: nil},
	}
	if err := db.SaveBatchEvaluation("batch-1", evals); err != nil {
		t.Fatal(err)
	}
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM batch_evaluations WHERE batch_id = 'batch-1'").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestSaveBatchEvaluation_Empty(t *testing.T) {
	db := newTestDB(t)
	if err := db.SaveBatchEvaluation("batch-empty", nil); err != nil {
		t.Fatal(err)
	}
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM batch_evaluations WHERE batch_id = 'batch-empty'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// ── Gates ─────────────────────────────────────────────────────────

func TestGates_SaveListDelete(t *testing.T) {
	db := newTestDB(t)

	g := Gate{ID: "g-1", Name: "safety-gate", Dimension: "safety", Threshold: 0.8, Window: 50, Enabled: true}
	if err := db.SaveGate(g); err != nil {
		t.Fatal(err)
	}

	gates, err := db.ListGates()
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 1 {
		t.Fatalf("expected 1, got %d", len(gates))
	}
	if gates[0].Name != "safety-gate" {
		t.Errorf("expected safety-gate, got %s", gates[0].Name)
	}

	if err := db.DeleteGate("g-1"); err != nil {
		t.Fatal(err)
	}
	gates, _ = db.ListGates()
	if len(gates) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(gates))
	}
}

func TestSaveGate_Upsert(t *testing.T) {
	db := newTestDB(t)
	g := Gate{ID: "g-1", Name: "v1", Dimension: "safety", Threshold: 0.5, Window: 10, Enabled: true}
	db.SaveGate(g)
	g.Name = "v2"
	g.Threshold = 0.9
	db.SaveGate(g)

	gates, _ := db.ListGates()
	if len(gates) != 1 {
		t.Fatalf("expected 1, got %d", len(gates))
	}
	if gates[0].Name != "v2" || gates[0].Threshold != 0.9 {
		t.Errorf("unexpected gate after upsert: %+v", gates[0])
	}
}

func TestListGates_Empty(t *testing.T) {
	db := newTestDB(t)
	gates, err := db.ListGates()
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 0 {
		t.Fatalf("expected 0, got %d", len(gates))
	}
}

// ── Alerts ────────────────────────────────────────────────────────

func TestSaveAndListAlerts(t *testing.T) {
	db := newTestDB(t)

	if err := db.SaveAlert("g-1", "safety-gate", "safety", 0.8, 0.65); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveAlert("g-2", "tone-gate", "tone", 0.7, 0.55); err != nil {
		t.Fatal(err)
	}

	alerts, err := db.ListAlerts(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2, got %d", len(alerts))
	}
	// Newest first
	if alerts[0].GateName != "tone-gate" {
		t.Errorf("expected newest first, got %s", alerts[0].GateName)
	}
}

func TestListAlerts_Empty(t *testing.T) {
	db := newTestDB(t)
	alerts, err := db.ListAlerts(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("expected 0, got %d", len(alerts))
	}
}

func TestListAlerts_Limit(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 5; i++ {
		db.SaveAlert("g", "gate", "dim", 0.5, 0.3)
	}
	alerts, _ := db.ListAlerts(2)
	if len(alerts) != 2 {
		t.Fatalf("expected 2, got %d", len(alerts))
	}
}
