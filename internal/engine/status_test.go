package engine

import (
	"testing"
	"time"
)

func TestStatusCollectorRecordRequest(t *testing.T) {
	sc := &StatusCollector{
		startTime:  time.Now(),
		lastChecks: make(map[string]healthCheck),
	}

	sc.RecordRequest(5*time.Millisecond, false)
	sc.RecordRequest(10*time.Millisecond, true)
	sc.RecordRequest(3*time.Millisecond, false)

	if sc.requestCount.Load() != 3 {
		t.Errorf("requestCount = %d, want 3", sc.requestCount.Load())
	}
	if sc.errorCount.Load() != 1 {
		t.Errorf("errorCount = %d, want 1", sc.errorCount.Load())
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Minute, "5m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{25*time.Hour + 15*time.Minute, "1d 1h 15m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := formatInt(tt.n)
		if got != tt.want {
			t.Errorf("formatInt(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
