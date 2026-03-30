// Database connectors for Seance — SQL and Redis sources.
package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// =========================================================================
// SQL Source — queries relational databases (Postgres, MySQL, SQLite)
// =========================================================================

// SQLSourceConfig holds configuration for a SQL data source.
type SQLSourceConfig struct {
	DSN     string            `json:"dsn"`
	Driver  string            `json:"driver"` // postgres, mysql, sqlite
	Queries map[string]string `json:"queries"` // name -> read-only SQL query
}

// SQLSource fetches data from a relational database using named read-only queries.
type SQLSource struct {
	name   string
	config SQLSourceConfig
}

// NewSQLSource creates a SQL database connector.
func NewSQLSource(name string, config SQLSourceConfig) *SQLSource {
	return &SQLSource{name: name, config: config}
}

func (s *SQLSource) Name() string { return s.name }
func (s *SQLSource) Type() string { return "sql" }

func (s *SQLSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	start := time.Now()
	snap := &Snapshot{
		Source:    s.name,
		Type:     "sql",
		FetchedAt: time.Now(),
	}

	driverName := s.config.Driver
	if driverName == "postgres" {
		driverName = "pgx"
	}

	db, err := sql.Open(driverName, s.config.DSN)
	if err != nil {
		snap.Error = fmt.Sprintf("connect: %v", err)
		snap.LatencyMs = int(time.Since(start).Milliseconds())
		return snap, err
	}
	defer db.Close()

	db.SetMaxOpenConns(2)
	db.SetConnMaxLifetime(30 * time.Second)

	if err := db.PingContext(ctx); err != nil {
		snap.Error = fmt.Sprintf("ping: %v", err)
		snap.LatencyMs = int(time.Since(start).Milliseconds())
		return snap, err
	}

	var sb strings.Builder
	raw := map[string]any{}
	queryErrors := 0

	for qname, query := range s.config.Queries {
		qResult, err := s.runReadOnlyQuery(ctx, db, qname, query)
		if err != nil {
			sb.WriteString(fmt.Sprintf("--- %s ---\nERROR: %v\n\n", qname, err))
			raw[qname] = map[string]any{"error": err.Error()}
			queryErrors++
			continue
		}
		sb.WriteString(qResult.formatted)
		sb.WriteString("\n")
		raw[qname] = qResult.rows
	}

	snap.Data = sb.String()
	snap.Raw = raw
	snap.LatencyMs = int(time.Since(start).Milliseconds())

	if queryErrors > 0 && queryErrors == len(s.config.Queries) {
		snap.Error = "all queries failed"
	}

	return snap, nil
}

type queryResult struct {
	formatted string
	rows      []map[string]any
}

func (s *SQLSource) runReadOnlyQuery(ctx context.Context, db *sql.DB, name, query string) (*queryResult, error) {
	// Enforce read-only via transaction
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin read-only tx: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	var results []map[string]any
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- %s ---\n", name))

	// Header row
	sb.WriteString("| ")
	colWidths := make([]int, len(cols))
	for i, col := range cols {
		colWidths[i] = len(col)
		if colWidths[i] < 10 {
			colWidths[i] = 10
		}
		sb.WriteString(fmt.Sprintf("%-*s | ", colWidths[i], col))
	}
	sb.WriteString("\n")

	// Separator
	sb.WriteString("|")
	for _, w := range colWidths {
		sb.WriteString(strings.Repeat("-", w+2))
		sb.WriteString("|")
	}
	sb.WriteString("\n")

	rowCount := 0
	maxRows := 100

	for rows.Next() {
		if rowCount >= maxRows {
			sb.WriteString(fmt.Sprintf("... (%d+ rows, truncated)\n", maxRows))
			break
		}

		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}

		row := map[string]any{}
		sb.WriteString("| ")
		for i, col := range cols {
			val := formatSQLValue(values[i])
			row[col] = values[i]
			sb.WriteString(fmt.Sprintf("%-*s | ", colWidths[i], truncateStr(val, colWidths[i])))
		}
		sb.WriteString("\n")
		results = append(results, row)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	if rowCount == 0 {
		sb.WriteString("(no rows)\n")
	} else {
		sb.WriteString(fmt.Sprintf("(%d rows)\n", rowCount))
	}

	return &queryResult{formatted: sb.String(), rows: results}, nil
}

func formatSQLValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", val)
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-1] + "~"
	}
	return s
}

// =========================================================================
// Redis Source — scans keys matching patterns and fetches values
// =========================================================================

// RedisSourceConfig holds configuration for a Redis data source.
type RedisSourceConfig struct {
	Addr     string   `json:"addr"`     // host:port
	Password string   `json:"password"` // optional
	DB       int      `json:"db"`       // database number
	Patterns []string `json:"patterns"` // key glob patterns to scan
}

// RedisSource fetches key-value summaries from Redis using the RESP protocol.
type RedisSource struct {
	name   string
	config RedisSourceConfig
}

// NewRedisSource creates a Redis connector.
func NewRedisSource(name string, config RedisSourceConfig) *RedisSource {
	if config.Addr == "" {
		config.Addr = "localhost:6379"
	}
	if len(config.Patterns) == 0 {
		config.Patterns = []string{"*"}
	}
	return &RedisSource{name: name, config: config}
}

func (s *RedisSource) Name() string { return s.name }
func (s *RedisSource) Type() string { return "redis" }

func (s *RedisSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	start := time.Now()
	snap := &Snapshot{
		Source:    s.name,
		Type:     "redis",
		FetchedAt: time.Now(),
	}

	conn, err := s.connect(ctx)
	if err != nil {
		snap.Error = fmt.Sprintf("connect: %v", err)
		snap.LatencyMs = int(time.Since(start).Milliseconds())
		return snap, err
	}
	defer conn.Close()

	// AUTH if password provided
	if s.config.Password != "" {
		if err := s.sendCommand(conn, "AUTH", s.config.Password); err != nil {
			snap.Error = fmt.Sprintf("auth: %v", err)
			snap.LatencyMs = int(time.Since(start).Milliseconds())
			return snap, err
		}
		if _, err := s.readReply(conn); err != nil {
			snap.Error = fmt.Sprintf("auth reply: %v", err)
			snap.LatencyMs = int(time.Since(start).Milliseconds())
			return snap, err
		}
	}

	// SELECT db if non-zero
	if s.config.DB != 0 {
		if err := s.sendCommand(conn, "SELECT", fmt.Sprintf("%d", s.config.DB)); err != nil {
			snap.Error = fmt.Sprintf("select db: %v", err)
			snap.LatencyMs = int(time.Since(start).Milliseconds())
			return snap, err
		}
		if _, err := s.readReply(conn); err != nil {
			snap.Error = fmt.Sprintf("select db reply: %v", err)
			snap.LatencyMs = int(time.Since(start).Milliseconds())
			return snap, err
		}
	}

	var sb strings.Builder
	raw := map[string]any{}
	totalKeys := 0
	maxKeysPerPattern := 50

	for _, pattern := range s.config.Patterns {
		keys, err := s.scanKeys(conn, pattern, maxKeysPerPattern)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Pattern %q: ERROR %v\n", pattern, err))
			continue
		}

		sb.WriteString(fmt.Sprintf("--- Pattern: %s (%d keys) ---\n", pattern, len(keys)))

		for _, key := range keys {
			val, err := s.getKey(conn, key)
			if err != nil {
				sb.WriteString(fmt.Sprintf("  %s: ERROR %v\n", key, err))
				continue
			}
			// Truncate large values
			display := val
			if len(display) > 200 {
				display = display[:200] + "...[truncated]"
			}
			sb.WriteString(fmt.Sprintf("  %s: %s\n", key, display))
			raw[key] = val
			totalKeys++
		}
		sb.WriteString("\n")
	}

	snap.Data = sb.String()
	snap.Raw = raw
	snap.LatencyMs = int(time.Since(start).Milliseconds())

	if totalKeys == 0 && len(s.config.Patterns) > 0 {
		snap.Data = "No matching keys found"
	}

	return snap, nil
}

// connect establishes a TCP connection to Redis.
func (s *RedisSource) connect(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.config.Addr)
	if err != nil {
		return nil, err
	}
	// Set deadline from context
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(10 * time.Second))
	}
	return conn, nil
}

// sendCommand writes a RESP command to the connection.
func (s *RedisSource) sendCommand(conn net.Conn, args ...string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}
	_, err := conn.Write([]byte(sb.String()))
	return err
}

// readReply reads a single RESP reply line.
func (s *RedisSource) readReply(conn net.Conn) (string, error) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	reply := string(buf[:n])

	// Check for errors
	if len(reply) > 0 && reply[0] == '-' {
		return "", fmt.Errorf("redis: %s", strings.TrimSpace(reply[1:]))
	}

	return reply, nil
}

// scanKeys uses KEYS command to find matching keys (SCAN would be better for
// production but KEYS is simpler for our read-only use case with limits).
func (s *RedisSource) scanKeys(conn net.Conn, pattern string, maxKeys int) ([]string, error) {
	if err := s.sendCommand(conn, "KEYS", pattern); err != nil {
		return nil, err
	}

	reply, err := s.readReply(conn)
	if err != nil {
		return nil, err
	}

	return s.parseArrayReply(reply, maxKeys), nil
}

// getKey fetches the value for a single key.
func (s *RedisSource) getKey(conn net.Conn, key string) (string, error) {
	if err := s.sendCommand(conn, "GET", key); err != nil {
		return "", err
	}

	reply, err := s.readReply(conn)
	if err != nil {
		return "", err
	}

	return s.parseBulkReply(reply), nil
}

// parseArrayReply extracts strings from a RESP array response.
func (s *RedisSource) parseArrayReply(reply string, maxItems int) []string {
	lines := strings.Split(reply, "\r\n")
	var results []string
	for _, line := range lines {
		if len(line) == 0 || line[0] == '*' || line[0] == '$' {
			continue
		}
		results = append(results, line)
		if len(results) >= maxItems {
			break
		}
	}
	return results
}

// parseBulkReply extracts the value from a RESP bulk string reply.
func (s *RedisSource) parseBulkReply(reply string) string {
	lines := strings.Split(reply, "\r\n")
	if len(lines) < 2 {
		return reply
	}
	// First line is $N (length), second is the value
	if lines[0] == "$-1" {
		return "(nil)"
	}
	return lines[1]
}
