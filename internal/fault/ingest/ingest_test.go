package ingest

import (
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
)

func TestParseLine_Empty(t *testing.T) {
	_, ok := parseLine("")
	if ok {
		t.Error("expected empty line to not parse")
	}
	_, ok = parseLine("   ")
	if ok {
		t.Error("expected whitespace line to not parse")
	}
}

func TestParseJSONLog(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantType string
	}{
		{
			name:    "error level with message",
			line:    `{"level":"error","message":"connection refused"}`,
			wantOK:  true,
			wantType: "log_error",
		},
		{
			name:    "fatal level",
			line:    `{"level":"fatal","message":"out of memory"}`,
			wantOK:  true,
			wantType: "fatal",
		},
		{
			name:    "panic level",
			line:    `{"level":"panic","msg":"nil pointer"}`,
			wantOK:  true,
			wantType: "panic",
		},
		{
			name:    "critical severity",
			line:    `{"severity":"critical","message":"disk full"}`,
			wantOK:  true,
			wantType: "log_error",
		},
		{
			name:    "info level ignored",
			line:    `{"level":"info","message":"started server"}`,
			wantOK:  false,
		},
		{
			name:    "debug level ignored",
			line:    `{"level":"debug","message":"processing request"}`,
			wantOK:  false,
		},
		{
			name:    "error field as message",
			line:    `{"level":"error","error":"timeout exceeded"}`,
			wantOK:  true,
			wantType: "log_error",
		},
		{
			name:    "with stack trace",
			line:    `{"level":"error","message":"crash","stack_trace":"goroutine 1"}`,
			wantOK:  true,
			wantType: "log_error",
		},
		{
			name:    "with stacktrace key",
			line:    `{"level":"error","message":"crash","stacktrace":"main.go:42"}`,
			wantOK:  true,
			wantType: "log_error",
		},
		{
			name:   "invalid JSON",
			line:   `{not valid json}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := parseJSONLog(tt.line)
			if ok != tt.wantOK {
				t.Errorf("parseJSONLog() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if ok && e.Type != tt.wantType {
				t.Errorf("type = %q, want %q", e.Type, tt.wantType)
			}
		})
	}
}

func TestParseGoLog(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOK   bool
		wantType string
	}{
		{
			name:     "fatal with timestamp",
			line:     "2024/01/15 10:30:00 fatal: database connection lost",
			wantOK:   true,
			wantType: "fatal",
		},
		{
			name:     "panic without timestamp",
			line:     "panic: runtime error: index out of range",
			wantOK:   true,
			wantType: "panic",
		},
		{
			name:     "log.Fatal",
			line:     "log.Fatal: cannot bind to port",
			wantOK:   true,
			wantType: "fatal",
		},
		{
			name:     "log.Panic",
			line:     "log.Panic: assertion failed",
			wantOK:   true,
			wantType: "panic",
		},
		{
			name:     "goroutine line",
			line:     "goroutine 1 [running]:",
			wantOK:   true,
			wantType: "panic",
		},
		{
			name:   "normal log line",
			line:   "2024/01/15 10:30:00 info: server started",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := parseGoLog(tt.line)
			if ok != tt.wantOK {
				t.Errorf("parseGoLog() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if ok && e.Type != tt.wantType {
				t.Errorf("type = %q, want %q", e.Type, tt.wantType)
			}
		})
	}
}

func TestParsePythonLog(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOK   bool
		wantType string
	}{
		{
			name:     "traceback header",
			line:     "Traceback (most recent call last):",
			wantOK:   true,
			wantType: "python_traceback",
		},
		{
			name:     "ValueError",
			line:     "ValueError: invalid literal for int()",
			wantOK:   true,
			wantType: "python_error",
		},
		{
			name:     "KeyError",
			line:     "KeyError: 'missing_key'",
			wantOK:   true,
			wantType: "python_error",
		},
		{
			name:     "RuntimeError",
			line:     "RuntimeError: maximum recursion depth exceeded",
			wantOK:   true,
			wantType: "python_error",
		},
		{
			name:     "custom exception",
			line:     "MyCustomException: something went wrong",
			wantOK:   true,
			wantType: "python_error",
		},
		{
			name:   "normal text",
			line:   "Everything is fine",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := parsePythonLog(tt.line)
			if ok != tt.wantOK {
				t.Errorf("parsePythonLog() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if ok && e.Type != tt.wantType {
				t.Errorf("type = %q, want %q", e.Type, tt.wantType)
			}
		})
	}
}

func TestParseLine_JSONPreferred(t *testing.T) {
	// JSON structured log should be parsed as JSON, not as Go/Python
	line := `{"level":"error","message":"test error"}`
	e, ok := parseLine(line)
	if !ok {
		t.Fatal("expected line to parse")
	}
	if e.Type != "log_error" {
		t.Errorf("expected log_error type, got %q", e.Type)
	}
}

func TestReadJSON(t *testing.T) {
	monitor := detect.New()
	li := New(monitor)

	input := `{"id":"e1","type":"panic","message":"crash"}
{"id":"e2","message":"another error"}
`
	ch, err := li.ReadJSON(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	var errors []detect.Error
	for e := range ch {
		errors = append(errors, e)
	}

	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}

	if errors[0].Type != "panic" {
		t.Errorf("first error type = %q, want panic", errors[0].Type)
	}
	// Second error has no type, should get default "ingested"
	if errors[1].Type != "ingested" {
		t.Errorf("second error type = %q, want ingested", errors[1].Type)
	}
}

func TestReadJSON_Empty(t *testing.T) {
	monitor := detect.New()
	li := New(monitor)

	// Empty reader should close channel immediately
	ch, err := li.ReadJSON(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}

	var errors []detect.Error
	for e := range ch {
		errors = append(errors, e)
	}

	if len(errors) != 0 {
		t.Errorf("expected 0 errors from empty input, got %d", len(errors))
	}
}

func TestReadBatch(t *testing.T) {
	monitor := detect.New()
	li := New(monitor)

	input := `[
		{"id":"e1","type":"panic","message":"crash"},
		{"id":"e2","message":"another error"}
	]`

	errors, err := li.ReadBatch(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}
	// Second error should get default type
	if errors[1].Type != "ingested" {
		t.Errorf("expected default type 'ingested', got %q", errors[1].Type)
	}
}

func TestReadBatch_InvalidJSON(t *testing.T) {
	monitor := detect.New()
	li := New(monitor)

	_, err := li.ReadBatch(strings.NewReader("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNew(t *testing.T) {
	monitor := detect.New()
	li := New(monitor)
	if li == nil {
		t.Fatal("New returned nil")
	}
	if li.monitor != monitor {
		t.Error("monitor not set correctly")
	}
}
