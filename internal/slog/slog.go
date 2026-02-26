// Package slog provides structured logging for Stockyard.
// JSON output, log levels, context fields, and request tracing.
//
// Usage:
//   slog.Init(slog.Config{Level: "info", Format: "json"})
//   slog.Info("server started", "port", 8080, "mode", "production")
//   slog.Error("request failed", "err", err, "trace_id", traceID)
package slog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLevel converts a string to a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error", "err":
		return LevelError
	default:
		return LevelInfo
	}
}

// Config holds logger configuration.
type Config struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json" or "text"
}

// Logger handles structured log output.
type Logger struct {
	mu     sync.Mutex
	w      io.Writer
	level  Level
	json   bool
	fields map[string]any // default fields added to every log line
}

// Entry represents a single log entry.
type Entry struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"msg"`
	Fields  map[string]any `json:"fields,omitempty"`
	Caller  string         `json:"caller,omitempty"`
}

var defaultLogger = &Logger{
	w:     os.Stderr,
	level: LevelInfo,
	json:  false,
}

// Init configures the global logger.
func Init(cfg Config) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	defaultLogger.level = ParseLevel(cfg.Level)
	defaultLogger.json = strings.ToLower(cfg.Format) == "json"
}

// SetOutput changes the output writer (for testing).
func SetOutput(w io.Writer) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.w = w
}

// WithFields returns a new logger with default fields.
func WithFields(fields map[string]any) *Logger {
	return &Logger{
		w:      defaultLogger.w,
		level:  defaultLogger.level,
		json:   defaultLogger.json,
		fields: fields,
	}
}

// Debug logs at debug level.
func Debug(msg string, keyvals ...any) { defaultLogger.log(LevelDebug, msg, keyvals...) }

// Info logs at info level.
func Info(msg string, keyvals ...any) { defaultLogger.log(LevelInfo, msg, keyvals...) }

// Warn logs at warn level.
func Warn(msg string, keyvals ...any) { defaultLogger.log(LevelWarn, msg, keyvals...) }

// Error logs at error level.
func Error(msg string, keyvals ...any) { defaultLogger.log(LevelError, msg, keyvals...) }

// Instance methods for sub-loggers
func (l *Logger) Debug(msg string, keyvals ...any) { l.log(LevelDebug, msg, keyvals...) }
func (l *Logger) Info(msg string, keyvals ...any)  { l.log(LevelInfo, msg, keyvals...) }
func (l *Logger) Warn(msg string, keyvals ...any)  { l.log(LevelWarn, msg, keyvals...) }
func (l *Logger) Error(msg string, keyvals ...any) { l.log(LevelError, msg, keyvals...) }

func (l *Logger) log(level Level, msg string, keyvals ...any) {
	if level < l.level {
		return
	}

	// Build fields map
	fields := make(map[string]any)

	// Default fields
	for k, v := range l.fields {
		fields[k] = v
	}

	// Key-value pairs
	for i := 0; i+1 < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprint(keyvals[i])
		}
		val := keyvals[i+1]
		// Convert errors to string
		if err, ok := val.(error); ok {
			val = err.Error()
		}
		fields[key] = val
	}
	// Odd trailing value
	if len(keyvals)%2 == 1 {
		fields["_extra"] = keyvals[len(keyvals)-1]
	}

	now := time.Now().UTC()

	if l.json {
		l.logJSON(level, msg, fields, now)
	} else {
		l.logText(level, msg, fields, now)
	}
}

func (l *Logger) logJSON(level Level, msg string, fields map[string]any, now time.Time) {
	entry := Entry{
		Time:    now.Format(time.RFC3339Nano),
		Level:   level.String(),
		Message: msg,
	}

	if len(fields) > 0 {
		entry.Fields = fields
	}

	// Add caller for errors
	if level >= LevelError {
		_, file, line, ok := runtime.Caller(3)
		if ok {
			// Trim to package/file.go:line
			if idx := strings.LastIndex(file, "/internal/"); idx != -1 {
				file = file[idx+1:]
			} else if idx := strings.LastIndex(file, "/"); idx != -1 {
				file = file[idx+1:]
			}
			entry.Caller = fmt.Sprintf("%s:%d", file, line)
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	json.NewEncoder(l.w).Encode(entry)
}

func (l *Logger) logText(level Level, msg string, fields map[string]any, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Format: 15:04:05 LEVEL msg key=value key=value
	ts := now.Format("15:04:05")
	lvl := strings.ToUpper(level.String())

	var buf strings.Builder
	fmt.Fprintf(&buf, "%s %s %s", ts, lvl, msg)

	for k, v := range fields {
		fmt.Fprintf(&buf, " %s=%v", k, v)
	}
	buf.WriteByte('\n')

	fmt.Fprint(l.w, buf.String())
}

// GetLevel returns the current log level.
func GetLevel() Level {
	return defaultLogger.level
}

// IsJSON returns whether JSON output is enabled.
func IsJSON() bool {
	return defaultLogger.json
}
