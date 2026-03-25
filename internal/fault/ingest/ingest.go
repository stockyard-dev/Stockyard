// Package ingest provides multi-source error ingestion for Stockyard Fault.
// It can tail log files, read structured JSON from stdin/readers, and parse
// common log formats (Go, Python, JSON structured logs) into fault errors.
package ingest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
)

// LogIngester reads errors from multiple sources and emits them as detect.Error values.
type LogIngester struct {
	monitor *detect.Monitor
}

// New creates a LogIngester that feeds errors into the given monitor.
func New(monitor *detect.Monitor) *LogIngester {
	return &LogIngester{monitor: monitor}
}

// WatchFile tails a log file and emits parsed errors on the returned channel.
// It reads from the current end of file forward (like tail -f).
func (li *LogIngester) WatchFile(path string) (<-chan detect.Error, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	// Seek to end so we only get new lines
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		return nil, fmt.Errorf("seek %s: %w", path, err)
	}

	ch := make(chan detect.Error, 64)
	go func() {
		defer f.Close()
		defer close(ch)
		scanner := bufio.NewScanner(f)
		for {
			for scanner.Scan() {
				line := scanner.Text()
				if e, ok := parseLine(line); ok {
					li.monitor.RecordError(e)
					ch <- e
				}
			}
			// No more data — wait and retry (tail -f behavior)
			time.Sleep(250 * time.Millisecond)
			// Reset scanner to pick up new data
			scanner = bufio.NewScanner(f)
		}
	}()
	return ch, nil
}

// ReadJSON reads a stream of newline-delimited JSON error objects from r.
func (li *LogIngester) ReadJSON(r io.Reader) (<-chan detect.Error, error) {
	ch := make(chan detect.Error, 64)
	go func() {
		defer close(ch)
		dec := json.NewDecoder(r)
		for {
			var e detect.Error
			if err := dec.Decode(&e); err != nil {
				if err == io.EOF {
					return
				}
				// Skip malformed lines
				continue
			}
			if e.Type == "" {
				e.Type = "ingested"
			}
			if e.Timestamp.IsZero() {
				e.Timestamp = time.Now()
			}
			li.monitor.RecordError(e)
			ch <- e
		}
	}()
	return ch, nil
}

// ReadBatch parses a JSON array of error objects from r (e.g. HTTP POST body).
func (li *LogIngester) ReadBatch(r io.Reader) ([]detect.Error, error) {
	var errors []detect.Error
	if err := json.NewDecoder(r).Decode(&errors); err != nil {
		return nil, fmt.Errorf("invalid JSON array: %w", err)
	}
	for i := range errors {
		if errors[i].Type == "" {
			errors[i].Type = "ingested"
		}
		if errors[i].Timestamp.IsZero() {
			errors[i].Timestamp = time.Now()
		}
		li.monitor.RecordError(errors[i])
	}
	return errors, nil
}

// parseLine attempts to parse a single log line into an error.
// Supports: JSON structured logs, Go log.Fatal/log.Panic, Python tracebacks.
func parseLine(line string) (detect.Error, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return detect.Error{}, false
	}

	// Try JSON structured log first
	if strings.HasPrefix(line, "{") {
		if e, ok := parseJSONLog(line); ok {
			return e, true
		}
	}

	// Go fatal/panic patterns
	if e, ok := parseGoLog(line); ok {
		return e, true
	}

	// Python traceback patterns
	if e, ok := parsePythonLog(line); ok {
		return e, true
	}

	return detect.Error{}, false
}

// parseJSONLog handles structured JSON logs with level/error fields.
func parseJSONLog(line string) (detect.Error, bool) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return detect.Error{}, false
	}

	// Check if it's an error-level log
	level, _ := obj["level"].(string)
	level = strings.ToLower(level)
	if level != "error" && level != "fatal" && level != "panic" && level != "critical" {
		// Also check for "severity" or "status"
		sev, _ := obj["severity"].(string)
		sev = strings.ToLower(sev)
		if sev != "error" && sev != "fatal" && sev != "panic" && sev != "critical" {
			return detect.Error{}, false
		}
	}

	msg, _ := obj["message"].(string)
	if msg == "" {
		msg, _ = obj["msg"].(string)
	}
	if msg == "" {
		msg, _ = obj["error"].(string)
	}

	errType := "log_error"
	if level == "fatal" || level == "panic" {
		errType = level
	}

	stack, _ := obj["stack_trace"].(string)
	if stack == "" {
		stack, _ = obj["stacktrace"].(string)
	}

	return detect.Error{
		ID:         fmt.Sprintf("ingest-%d", time.Now().UnixNano()),
		Timestamp:  time.Now(),
		Type:       errType,
		Message:    msg,
		StackTrace: stack,
	}, true
}

var (
	goFatalRe = regexp.MustCompile(`(?i)^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})?\s*(fatal|panic|log\.Fatal|log\.Panic)\b[:\s]*(.+)`)
	goGoroutineRe = regexp.MustCompile(`^goroutine \d+ \[`)
)

// parseGoLog detects Go log.Fatal, log.Panic and runtime panic lines.
func parseGoLog(line string) (detect.Error, bool) {
	if m := goFatalRe.FindStringSubmatch(line); m != nil {
		errType := "panic"
		if strings.Contains(strings.ToLower(m[2]), "fatal") {
			errType = "fatal"
		}
		return detect.Error{
			ID:        fmt.Sprintf("ingest-%d", time.Now().UnixNano()),
			Timestamp: time.Now(),
			Type:      errType,
			Message:   strings.TrimSpace(m[3]),
		}, true
	}
	if goGoroutineRe.MatchString(line) {
		return detect.Error{
			ID:         fmt.Sprintf("ingest-%d", time.Now().UnixNano()),
			Timestamp:  time.Now(),
			Type:       "panic",
			Message:    line,
			StackTrace: line,
		}, true
	}
	return detect.Error{}, false
}

var (
	pyTracebackRe = regexp.MustCompile(`^Traceback \(most recent call last\)`)
	pyErrorRe     = regexp.MustCompile(`^(\w+Error|\w+Exception):?\s*(.*)`)
)

// parsePythonLog detects Python traceback and exception lines.
func parsePythonLog(line string) (detect.Error, bool) {
	if pyTracebackRe.MatchString(line) {
		return detect.Error{
			ID:         fmt.Sprintf("ingest-%d", time.Now().UnixNano()),
			Timestamp:  time.Now(),
			Type:       "python_traceback",
			Message:    line,
			StackTrace: line,
		}, true
	}
	if m := pyErrorRe.FindStringSubmatch(line); m != nil {
		return detect.Error{
			ID:        fmt.Sprintf("ingest-%d", time.Now().UnixNano()),
			Timestamp: time.Now(),
			Type:      "python_error",
			Message:   line,
		}, true
	}
	return detect.Error{}, false
}
