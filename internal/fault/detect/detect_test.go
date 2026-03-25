package detect

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFingerprint(t *testing.T) {
	tests := []struct {
		name string
		a, b Error
		same bool
	}{
		{
			name: "same error same fingerprint",
			a:    Error{Type: "http_500", Method: "GET", Path: "/api/users", StatusCode: 500},
			b:    Error{Type: "http_500", Method: "GET", Path: "/api/users", StatusCode: 500},
			same: true,
		},
		{
			name: "different path different fingerprint",
			a:    Error{Type: "http_500", Method: "GET", Path: "/api/users", StatusCode: 500},
			b:    Error{Type: "http_500", Method: "GET", Path: "/api/items", StatusCode: 500},
			same: false,
		},
		{
			name: "different method different fingerprint",
			a:    Error{Type: "http_500", Method: "GET", Path: "/api/users", StatusCode: 500},
			b:    Error{Type: "http_500", Method: "POST", Path: "/api/users", StatusCode: 500},
			same: false,
		},
		{
			name: "different type different fingerprint",
			a:    Error{Type: "panic", Method: "GET", Path: "/api/users", StatusCode: 0},
			b:    Error{Type: "http_500", Method: "GET", Path: "/api/users", StatusCode: 500},
			same: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fpA := fingerprint(tt.a)
			fpB := fingerprint(tt.b)
			if (fpA == fpB) != tt.same {
				t.Errorf("fingerprint(%+v)=%q, fingerprint(%+v)=%q, same=%v want=%v",
					tt.a, fpA, tt.b, fpB, fpA == fpB, tt.same)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"short", 10, "short"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"exact", 5, "exact"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestRecordError_Deduplication(t *testing.T) {
	m := New()

	e := Error{
		ID:         "test-1",
		Type:       "http_500",
		Message:    "Internal Server Error",
		Path:       "/api/users",
		Method:     "GET",
		StatusCode: 500,
		Timestamp:  time.Now(),
	}

	// Record same error 5 times
	for i := 0; i < 5; i++ {
		m.RecordError(e)
	}

	// Should only have 1 unique error
	if m.ErrorCount() != 1 {
		t.Errorf("expected 1 unique error, got %d", m.ErrorCount())
	}

	// But the count on the deduped error should be incremented
	fp := fingerprint(e)
	m.mu.RLock()
	existing := m.seen[fp]
	m.mu.RUnlock()

	if existing == nil {
		t.Fatal("expected error to exist in seen map")
	}
	if existing.Count != 5 {
		t.Errorf("expected count=5, got %d", existing.Count)
	}
}

func TestRecordError_DifferentErrors(t *testing.T) {
	m := New()

	m.RecordError(Error{ID: "1", Type: "http_500", Method: "GET", Path: "/a", StatusCode: 500, Timestamp: time.Now()})
	m.RecordError(Error{ID: "2", Type: "http_500", Method: "GET", Path: "/b", StatusCode: 500, Timestamp: time.Now()})
	m.RecordError(Error{ID: "3", Type: "panic", Method: "POST", Path: "/c", StatusCode: 0, Timestamp: time.Now()})

	if m.ErrorCount() != 3 {
		t.Errorf("expected 3 unique errors, got %d", m.ErrorCount())
	}
}

func TestRecordError_AutoFillsFields(t *testing.T) {
	m := New()

	m.RecordError(Error{
		Type:    "http_500",
		Message: "test",
		Path:    "/x",
		Method:  "GET",
	})

	errs := m.RecentErrors(1)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	e := errs[0]
	if e.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if e.Timestamp.IsZero() {
		t.Error("expected auto-generated timestamp")
	}
	if e.FirstSeen.IsZero() {
		t.Error("expected auto-generated FirstSeen")
	}
	if e.Count != 1 {
		t.Errorf("expected count=1, got %d", e.Count)
	}
}

func TestRecentErrors(t *testing.T) {
	m := New()

	for i := 0; i < 10; i++ {
		m.RecordError(Error{
			Type:      "http_500",
			Method:    "GET",
			Path:      "/" + string(rune('a'+i)),
			StatusCode: 500,
			Timestamp: time.Now(),
		})
	}

	recent := m.RecentErrors(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent errors, got %d", len(recent))
	}

	// Ask for more than available
	all := m.RecentErrors(100)
	if len(all) != 10 {
		t.Errorf("expected 10 errors, got %d", len(all))
	}
}

func TestMaxErrors(t *testing.T) {
	m := New()
	m.maxErrs = 5

	for i := 0; i < 10; i++ {
		m.RecordError(Error{
			Type:      "http_500",
			Method:    "GET",
			Path:      "/" + string(rune('a'+i)),
			StatusCode: 500,
			Timestamp: time.Now(),
		})
	}

	if len(m.errors) > 5 {
		t.Errorf("expected at most 5 errors retained, got %d", len(m.errors))
	}
}

func TestOnError_HandlerCalled(t *testing.T) {
	m := New()

	var called int
	m.OnError(func(e Error) {
		called++
	})

	m.RecordError(Error{Type: "http_500", Method: "GET", Path: "/a", StatusCode: 500, Timestamp: time.Now()})
	// Second time is a duplicate — handler should NOT be called again
	m.RecordError(Error{Type: "http_500", Method: "GET", Path: "/a", StatusCode: 500, Timestamp: time.Now()})
	// Different error — handler should be called
	m.RecordError(Error{Type: "panic", Method: "POST", Path: "/b", StatusCode: 0, Timestamp: time.Now()})

	if called != 2 {
		t.Errorf("expected handler called 2 times (new errors only), got %d", called)
	}
}

func TestMiddleware_500Detection(t *testing.T) {
	m := New()

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))

	req := httptest.NewRequest("GET", "/api/fail", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if m.ErrorCount() != 1 {
		t.Errorf("expected 1 error detected, got %d", m.ErrorCount())
	}

	errs := m.RecentErrors(1)
	if errs[0].Type != "http_500" {
		t.Errorf("expected type http_500, got %s", errs[0].Type)
	}
	if errs[0].Path != "/api/fail" {
		t.Errorf("expected path /api/fail, got %s", errs[0].Path)
	}
	if errs[0].StatusCode != 500 {
		t.Errorf("expected status 500, got %d", errs[0].StatusCode)
	}
}

func TestMiddleware_200NoError(t *testing.T) {
	m := New()

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/ok", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if m.ErrorCount() != 0 {
		t.Errorf("expected 0 errors for 200 response, got %d", m.ErrorCount())
	}
}

func TestMiddleware_PanicRecovery(t *testing.T) {
	m := New()

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something bad happened")
	}))

	req := httptest.NewRequest("GET", "/api/panic", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != 500 {
		t.Errorf("expected 500 response after panic, got %d", rr.Code)
	}

	if m.ErrorCount() != 1 {
		t.Errorf("expected 1 panic error, got %d", m.ErrorCount())
	}

	errs := m.RecentErrors(1)
	if errs[0].Type != "panic" {
		t.Errorf("expected type panic, got %s", errs[0].Type)
	}
	if !strings.Contains(errs[0].Message, "something bad happened") {
		t.Errorf("expected panic message in error, got %s", errs[0].Message)
	}
}

func TestMiddleware_FiltersAuthHeaders(t *testing.T) {
	m := New()

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))

	req := httptest.NewRequest("GET", "/api/secret", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=abc123")
	req.Header.Set("X-Custom", "safe-value")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	errs := m.RecentErrors(1)
	if len(errs) == 0 {
		t.Fatal("expected an error")
	}

	if _, ok := errs[0].Headers["Authorization"]; ok {
		t.Error("Authorization header should be filtered")
	}
	if _, ok := errs[0].Headers["Cookie"]; ok {
		t.Error("Cookie header should be filtered")
	}
	if errs[0].Headers["X-Custom"] != "safe-value" {
		t.Error("non-sensitive headers should be preserved")
	}
}

func TestConcurrentRecordError(t *testing.T) {
	m := New()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.RecordError(Error{
				Type:      "http_500",
				Method:    "GET",
				Path:      "/" + string(rune('a'+(i%26))),
				StatusCode: 500,
				Timestamp: time.Now(),
			})
		}(i)
	}

	wg.Wait()

	// Should not panic and error count should be <= 50 (dedup may reduce)
	if m.ErrorCount() == 0 {
		t.Error("expected some errors to be recorded")
	}
	if m.ErrorCount() > 50 {
		t.Errorf("expected at most 50 errors, got %d", m.ErrorCount())
	}
}
