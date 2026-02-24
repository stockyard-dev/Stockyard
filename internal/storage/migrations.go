package storage

// migrate runs all database migrations.
func (db *DB) migrate() error {
	migrations := []string{
		migrationV1,
		migrationV2,
		migrationV3,
	}

	// Create migrations tracking table
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT (datetime('now'))
		)
	`); err != nil {
		return err
	}

	for i, m := range migrations {
		version := i + 1
		var count int
		db.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
		if count > 0 {
			continue
		}

		if _, err := db.conn.Exec(m); err != nil {
			return err
		}
		if _, err := db.conn.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return err
		}
	}

	return nil
}

const migrationV1 = `
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL,
    project TEXT NOT NULL DEFAULT 'default',
    user_id TEXT,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    tokens_in INTEGER NOT NULL,
    tokens_out INTEGER NOT NULL,
    cost_usd REAL NOT NULL,
    latency_ms INTEGER NOT NULL,
    status INTEGER NOT NULL,
    cache_hit BOOLEAN DEFAULT FALSE,
    validation_pass BOOLEAN,
    failover_used BOOLEAN DEFAULT FALSE,
    request_body TEXT,
    response_body TEXT,
    error TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp);
CREATE INDEX IF NOT EXISTS idx_requests_project ON requests(project);
CREATE INDEX IF NOT EXISTS idx_requests_user ON requests(user_id);

CREATE TABLE IF NOT EXISTS spend_rollups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    date TEXT NOT NULL,
    total_cost REAL NOT NULL DEFAULT 0,
    total_requests INTEGER NOT NULL DEFAULT 0,
    total_tokens_in INTEGER NOT NULL DEFAULT 0,
    total_tokens_out INTEGER NOT NULL DEFAULT 0,
    UNIQUE(project, date)
);

CREATE TABLE IF NOT EXISTS cache_entries (
    key TEXT PRIMARY KEY,
    response TEXT NOT NULL,
    model TEXT NOT NULL,
    tokens_saved INTEGER NOT NULL,
    cost_saved REAL NOT NULL,
    hits INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_entries(expires_at);

CREATE TABLE IF NOT EXISTS config_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
`

const migrationV2 = `
CREATE TABLE IF NOT EXISTS rate_limit_state (
    key TEXT PRIMARY KEY,
    request_count INTEGER DEFAULT 0,
    window_start TEXT NOT NULL,
    blocked_until TEXT
);
`

const migrationV3 = `
CREATE TABLE IF NOT EXISTS keypool_usage (
    key_name TEXT NOT NULL,
    date TEXT NOT NULL,
    requests INTEGER DEFAULT 0,
    tokens INTEGER DEFAULT 0,
    errors INTEGER DEFAULT 0,
    rate_limits INTEGER DEFAULT 0,
    UNIQUE(key_name, date)
);

CREATE TABLE IF NOT EXISTS pii_redactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    request_id TEXT,
    pattern TEXT NOT NULL,
    placeholder TEXT NOT NULL,
    message_role TEXT
);

CREATE INDEX IF NOT EXISTS idx_pii_timestamp ON pii_redactions(timestamp);

CREATE TABLE IF NOT EXISTS model_route_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    request_id TEXT,
    original_model TEXT NOT NULL,
    routed_model TEXT NOT NULL,
    rule_name TEXT NOT NULL,
    tokens_in INTEGER,
    cost_usd REAL
);

CREATE INDEX IF NOT EXISTS idx_route_timestamp ON model_route_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_route_rule ON model_route_log(rule_name);

CREATE TABLE IF NOT EXISTS eval_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    request_id TEXT,
    passed BOOLEAN NOT NULL,
    attempt INTEGER NOT NULL,
    validators TEXT,
    failures TEXT
);

CREATE INDEX IF NOT EXISTS idx_eval_timestamp ON eval_results(timestamp);

CREATE TABLE IF NOT EXISTS usage_metering (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    dimension TEXT NOT NULL,
    dimension_key TEXT NOT NULL,
    requests INTEGER DEFAULT 0,
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    date TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_dimension ON usage_metering(dimension, dimension_key);
CREATE INDEX IF NOT EXISTS idx_usage_date ON usage_metering(date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_unique ON usage_metering(dimension, dimension_key, date);
`
