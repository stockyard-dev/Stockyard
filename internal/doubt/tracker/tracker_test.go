package tracker

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNew_NilDB(t *testing.T) {
	s := New(nil)
	if s == nil {
		t.Fatal("New(nil) returned nil")
	}
	if s.TrackedCount() != 0 {
		t.Errorf("TrackedCount = %d, want 0", s.TrackedCount())
	}
}

func TestRecord_Request(t *testing.T) {
	s := New(nil)

	s.Record(Signal{
		UnitID:    "unit-1",
		Type:      "request",
		Value:     150.0,
		Timestamp: time.Now(),
	})

	stats := s.GetStats("unit-1")
	if stats == nil {
		t.Fatal("expected stats for unit-1")
	}
	if stats.Requests != 1 {
		t.Errorf("Requests = %d, want 1", stats.Requests)
	}
	if stats.AvgLatencyMs != 150.0 {
		t.Errorf("AvgLatencyMs = %v, want 150.0", stats.AvgLatencyMs)
	}
}

func TestRecord_Error(t *testing.T) {
	s := New(nil)

	s.Record(Signal{UnitID: "unit-2", Type: "error", Timestamp: time.Now()})

	stats := s.GetStats("unit-2")
	if stats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats.Errors)
	}
	if stats.Requests != 1 {
		t.Errorf("Requests = %d, want 1 (errors are also requests)", stats.Requests)
	}
}

func TestRecord_Panic(t *testing.T) {
	s := New(nil)

	s.Record(Signal{UnitID: "unit-3", Type: "panic", Timestamp: time.Now()})

	stats := s.GetStats("unit-3")
	if stats.Panics != 1 {
		t.Errorf("Panics = %d, want 1", stats.Panics)
	}
	if stats.Requests != 1 {
		t.Errorf("Requests = %d, want 1", stats.Requests)
	}
}

func TestRecord_Timeout(t *testing.T) {
	s := New(nil)

	s.Record(Signal{UnitID: "unit-4", Type: "timeout", Timestamp: time.Now()})

	stats := s.GetStats("unit-4")
	if stats.Timeouts != 1 {
		t.Errorf("Timeouts = %d, want 1", stats.Timeouts)
	}
	if stats.Requests != 1 {
		t.Errorf("Requests = %d, want 1", stats.Requests)
	}
}

func TestRecord_Latency(t *testing.T) {
	s := New(nil)

	s.Record(Signal{UnitID: "unit-5", Type: "latency", Value: 200.0, Timestamp: time.Now()})

	stats := s.GetStats("unit-5")
	if stats.AvgLatencyMs != 200.0 {
		t.Errorf("AvgLatencyMs = %v, want 200.0", stats.AvgLatencyMs)
	}
	// Latency signals don't increment requests
	if stats.Requests != 0 {
		t.Errorf("Requests = %d, want 0 (latency is not a request)", stats.Requests)
	}
}

func TestRecord_LatencyCapped(t *testing.T) {
	s := New(nil)

	for i := 0; i < 1100; i++ {
		s.Record(Signal{UnitID: "capped", Type: "request", Value: float64(i), Timestamp: time.Now()})
	}

	s.mu.RLock()
	ud := s.units["capped"]
	latLen := len(ud.latencies)
	s.mu.RUnlock()

	if latLen > 1000 {
		t.Errorf("latencies len = %d, should be capped at 1000", latLen)
	}
}

func TestGetStats_NotFound(t *testing.T) {
	s := New(nil)
	stats := s.GetStats("nonexistent")
	if stats != nil {
		t.Error("expected nil for nonexistent unit")
	}
}

func TestGetStats_ErrorRate(t *testing.T) {
	s := New(nil)
	now := time.Now()

	// 80 requests + 20 errors = 100 total, error rate = 20%
	for i := 0; i < 80; i++ {
		s.Record(Signal{UnitID: "rated", Type: "request", Value: 100, Timestamp: now})
	}
	for i := 0; i < 20; i++ {
		s.Record(Signal{UnitID: "rated", Type: "error", Timestamp: now})
	}

	stats := s.GetStats("rated")
	if stats.Requests != 100 {
		t.Errorf("Requests = %d, want 100", stats.Requests)
	}
	if stats.ErrorRate != 20.0 {
		t.Errorf("ErrorRate = %v, want 20.0", stats.ErrorRate)
	}
}

func TestGetStats_P95Latency(t *testing.T) {
	s := New(nil)
	now := time.Now()

	// Record latencies 1-100
	for i := 1; i <= 100; i++ {
		s.Record(Signal{UnitID: "p95", Type: "request", Value: float64(i), Timestamp: now})
	}

	stats := s.GetStats("p95")
	// Sorted 1..100, p95 idx = 95
	if stats.P95LatencyMs < 90 || stats.P95LatencyMs > 100 {
		t.Errorf("P95LatencyMs = %v, expected ~95", stats.P95LatencyMs)
	}
}

func TestGetStats_AvgLatency(t *testing.T) {
	s := New(nil)
	now := time.Now()

	s.Record(Signal{UnitID: "avg", Type: "request", Value: 100, Timestamp: now})
	s.Record(Signal{UnitID: "avg", Type: "request", Value: 200, Timestamp: now})
	s.Record(Signal{UnitID: "avg", Type: "request", Value: 300, Timestamp: now})

	stats := s.GetStats("avg")
	if stats.AvgLatencyMs != 200.0 {
		t.Errorf("AvgLatencyMs = %v, want 200.0", stats.AvgLatencyMs)
	}
}

func TestAllStats(t *testing.T) {
	s := New(nil)
	now := time.Now()

	s.Record(Signal{UnitID: "a", Type: "request", Value: 10, Timestamp: now})
	s.Record(Signal{UnitID: "b", Type: "request", Value: 20, Timestamp: now})

	all := s.AllStats()
	if len(all) != 2 {
		t.Errorf("AllStats returned %d entries, want 2", len(all))
	}
	if all["a"] == nil {
		t.Error("expected entry for 'a'")
	}
	if all["b"] == nil {
		t.Error("expected entry for 'b'")
	}
}

func TestTrackedCount(t *testing.T) {
	s := New(nil)
	if s.TrackedCount() != 0 {
		t.Errorf("TrackedCount = %d, want 0", s.TrackedCount())
	}

	s.Record(Signal{UnitID: "a", Type: "request", Timestamp: time.Now()})
	s.Record(Signal{UnitID: "b", Type: "request", Timestamp: time.Now()})

	if s.TrackedCount() != 2 {
		t.Errorf("TrackedCount = %d, want 2", s.TrackedCount())
	}

	// Same unit again
	s.Record(Signal{UnitID: "a", Type: "error", Timestamp: time.Now()})
	if s.TrackedCount() != 2 {
		t.Errorf("TrackedCount = %d, want 2 (no new unit)", s.TrackedCount())
	}
}

func TestIngestJSON(t *testing.T) {
	s := New(nil)

	signals := []Signal{
		{UnitID: "json-1", Type: "request", Value: 100, Timestamp: time.Now()},
		{UnitID: "json-2", Type: "error", Timestamp: time.Now()},
		{UnitID: "json-1", Type: "request", Value: 200, Timestamp: time.Now()},
	}

	data, err := json.Marshal(signals)
	if err != nil {
		t.Fatal(err)
	}

	count, err := s.IngestJSON(data)
	if err != nil {
		t.Fatalf("IngestJSON error: %v", err)
	}
	if count != 3 {
		t.Errorf("IngestJSON count = %d, want 3", count)
	}
	if s.TrackedCount() != 2 {
		t.Errorf("TrackedCount = %d, want 2", s.TrackedCount())
	}
}

func TestIngestJSON_InvalidJSON(t *testing.T) {
	s := New(nil)
	_, err := s.IngestJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestIngestJSON_ZeroTimestamp(t *testing.T) {
	s := New(nil)

	// Signal with zero timestamp should get auto-filled
	data := `[{"unit_id":"ts-test","type":"request","value":100}]`
	count, err := s.IngestJSON([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	stats := s.GetStats("ts-test")
	if stats == nil {
		t.Fatal("expected stats")
	}
}

func TestRecord_Timestamps(t *testing.T) {
	s := New(nil)

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	s.Record(Signal{UnitID: "ts", Type: "request", Value: 100, Timestamp: t1})
	s.Record(Signal{UnitID: "ts", Type: "request", Value: 200, Timestamp: t2})

	stats := s.GetStats("ts")
	if !stats.FirstSeen.Equal(t1) {
		t.Errorf("FirstSeen = %v, want %v", stats.FirstSeen, t1)
	}
	if !stats.LastSeen.Equal(t2) {
		t.Errorf("LastSeen = %v, want %v", stats.LastSeen, t2)
	}
}

func TestBuildStats_NoPanics(t *testing.T) {
	ud := &unitData{
		requests: 100,
		errors:   5,
		panics:   2,
		latencies: []float64{10, 20, 30},
		firstSeen: time.Now().Add(-24 * time.Hour),
		lastSeen:  time.Now(),
	}

	stats := buildStats("test", ud)
	// ErrorRate = (5+2)/100 * 100 = 7%
	if diff := stats.ErrorRate - 7.0; diff > 0.001 || diff < -0.001 {
		t.Errorf("ErrorRate = %v, want ~7.0", stats.ErrorRate)
	}
	if stats.DaysSinceFirst < 0.9 {
		t.Errorf("DaysSinceFirst = %v, want ~1", stats.DaysSinceFirst)
	}
}

func TestSortFloats(t *testing.T) {
	tests := []struct {
		input []float64
		want  []float64
	}{
		{[]float64{3, 1, 2}, []float64{1, 2, 3}},
		{[]float64{1}, []float64{1}},
		{nil, nil},
	}
	for _, tt := range tests {
		s := make([]float64, len(tt.input))
		copy(s, tt.input)
		sortFloats(s)
		for i := range s {
			if s[i] != tt.want[i] {
				t.Errorf("sortFloats(%v) = %v, want %v", tt.input, s, tt.want)
				break
			}
		}
	}
}
