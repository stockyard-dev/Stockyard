// Package apiserver — SQLite-backed persistence for the Stockyard API backend.
//
// This replaces the in-memory + JSON file stores (customers.go DB, CloudStore,
// ExchangeStore) with a single SQLite database. All existing method signatures
// are preserved for drop-in compatibility with server.go and handlers.
//
// Migration: On first open, if legacy JSON files exist (stockyard-api.db as JSON,
// cloud.json, exchange.json), they are imported into SQLite and the originals
// renamed to *.json.migrated.
package apiserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SqliteDB is the unified SQLite-backed store for the API backend.
// It replaces DB (customers/licenses), CloudStore, and ExchangeStore.
type SqliteDB struct {
	conn *sql.DB
	path string
}

// OpenSqliteDB opens (or creates) the SQLite database and runs migrations.
func OpenSqliteDB(path string) (*SqliteDB, error) {
	dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Single writer, multiple readers
	conn.SetMaxOpenConns(1)

	db := &SqliteDB{conn: conn, path: path}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *SqliteDB) Close() error {
	return db.conn.Close()
}

// ---------------------------------------------------------------------------
// Schema migrations
// ---------------------------------------------------------------------------

func (db *SqliteDB) migrate() error {
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT (datetime('now'))
		)
	`); err != nil {
		return err
	}

	migrations := []string{apiMigrationV1}

	for i, m := range migrations {
		v := i + 1
		var count int
		db.conn.QueryRow("SELECT COUNT(*) FROM schema_version WHERE version = ?", v).Scan(&count)
		if count > 0 {
			continue
		}
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration v%d: %w", v, err)
		}
		db.conn.Exec("INSERT INTO schema_version (version) VALUES (?)", v)
		log.Printf("db: applied migration v%d", v)
	}
	return nil
}

const apiMigrationV1 = `
-- Customers (Stripe)
CREATE TABLE IF NOT EXISTS customers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stripe_customer_id TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    name TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email);

-- Licenses
CREATE TABLE IF NOT EXISTS licenses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    customer_id INTEGER REFERENCES customers(id),
    stripe_customer_id TEXT NOT NULL,
    stripe_subscription_id TEXT DEFAULT '',
    product TEXT NOT NULL,
    tier TEXT NOT NULL,
    license_key TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    email TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_licenses_key ON licenses(license_key);
CREATE INDEX IF NOT EXISTS idx_licenses_sub ON licenses(stripe_subscription_id);
CREATE INDEX IF NOT EXISTS idx_licenses_customer ON licenses(stripe_customer_id);

-- Webhook idempotency
CREATE TABLE IF NOT EXISTS processed_webhooks (
    event_id TEXT PRIMARY KEY,
    event_type TEXT,
    processed_at TEXT DEFAULT (datetime('now'))
);

-- Cloud tenants
CREATE TABLE IF NOT EXISTS cloud_tenants (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT DEFAULT '',
    api_key TEXT UNIQUE NOT NULL,
    plan TEXT NOT NULL DEFAULT 'free',
    daily_request_limit INTEGER NOT NULL DEFAULT 1000,
    stripe_customer_id TEXT DEFAULT '',
    stripe_subscription_id TEXT DEFAULT '',
    provider_keys_json TEXT DEFAULT '{}',
    proxy_config_json TEXT DEFAULT '{}',
    enabled_products_json TEXT DEFAULT '["costcap"]',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_cloud_apikey ON cloud_tenants(api_key);

-- Cloud usage (daily rollups per tenant)
CREATE TABLE IF NOT EXISTS cloud_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL REFERENCES cloud_tenants(id),
    date TEXT NOT NULL,
    requests INTEGER DEFAULT 0,
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    cache_hits INTEGER DEFAULT 0,
    errors INTEGER DEFAULT 0,
    UNIQUE(tenant_id, date)
);
CREATE INDEX IF NOT EXISTS idx_usage_tenant_date ON cloud_usage(tenant_id, date);

-- Exchange items
CREATE TABLE IF NOT EXISTS exchange_items (
    id TEXT PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL DEFAULT 'config',
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    author_email TEXT DEFAULT '',
    author_name TEXT DEFAULT '',
    content TEXT NOT NULL,
    tags_json TEXT DEFAULT '[]',
    products_json TEXT DEFAULT '[]',
    providers_json TEXT DEFAULT '[]',
    downloads INTEGER DEFAULT 0,
    stars INTEGER DEFAULT 0,
    forks INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'published',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Exchange stars (user-item mapping)
CREATE TABLE IF NOT EXISTS exchange_stars (
    slug TEXT NOT NULL,
    email TEXT NOT NULL,
    PRIMARY KEY(slug, email)
);
`

// ---------------------------------------------------------------------------
// Legacy JSON import
// ---------------------------------------------------------------------------

// ImportLegacyJSON reads old JSON files and imports them into SQLite.
// Call after OpenSqliteDB. Safe to call multiple times (skips if data exists).
func (db *SqliteDB) ImportLegacyJSON(dataDir string) error {
	// 1. Import customers/licenses from the old "DB" JSON file
	if err := db.importCustomersJSON(dataDir); err != nil {
		log.Printf("db: import customers: %v (skipping)", err)
	}

	// 2. Import cloud tenants
	if err := db.importCloudJSON(dataDir); err != nil {
		log.Printf("db: import cloud: %v (skipping)", err)
	}

	// 3. Import exchange items
	if err := db.importExchangeJSON(dataDir); err != nil {
		log.Printf("db: import exchange: %v (skipping)", err)
	}

	return nil
}

func (db *SqliteDB) importCustomersJSON(dataDir string) error {
	// The old DB wrote to the path passed to OpenDB — typically stockyard-api.db
	// which was actually a JSON file despite the .db extension
	path := filepath.Join(dataDir, "stockyard-api.db")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // no legacy file
	}

	// Check if it's JSON (the old format) vs actual SQLite
	if len(data) > 0 && data[0] != '{' {
		return nil // already SQLite or binary, skip
	}

	var snap struct {
		Customers  map[string]*Customer `json:"customers"`
		Licenses   []*LicenseRecord     `json:"licenses"`
		NextCustID int64                `json:"next_cust_id"`
		NextLicID  int64                `json:"next_lic_id"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse customers JSON: %w", err)
	}

	// Check if already imported
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM customers").Scan(&count)
	if count > 0 {
		log.Printf("db: customers table not empty (%d rows), skipping JSON import", count)
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	imported := 0
	for _, c := range snap.Customers {
		_, err := tx.Exec(`INSERT OR IGNORE INTO customers (id, stripe_customer_id, email, name, created_at) VALUES (?, ?, ?, ?, ?)`,
			c.ID, c.StripeCustomerID, c.Email, c.Name, c.CreatedAt.Format(time.RFC3339))
		if err != nil {
			log.Printf("db: import customer %s: %v", c.Email, err)
			continue
		}
		imported++
	}

	for _, l := range snap.Licenses {
		expiresAt := ""
		if !l.ExpiresAt.IsZero() {
			expiresAt = l.ExpiresAt.Format(time.RFC3339)
		}
		_, err := tx.Exec(`INSERT OR IGNORE INTO licenses (id, customer_id, stripe_customer_id, stripe_subscription_id, product, tier, license_key, status, email, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			l.ID, l.CustomerID, l.StripeCustomerID, l.StripeSubscriptionID, l.Product, l.Tier, l.LicenseKey, l.Status, l.Email, l.CreatedAt.Format(time.RFC3339), expiresAt)
		if err != nil {
			log.Printf("db: import license %d: %v", l.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("db: imported %d customers + %d licenses from JSON", imported, len(snap.Licenses))
	os.Rename(path, path+".migrated")
	return nil
}

func (db *SqliteDB) importCloudJSON(dataDir string) error {
	path := filepath.Join(dataDir, "cloud.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var snap struct {
		Tenants map[string]*CloudTenant `json:"tenants"`
		Usage   map[string]*CloudUsage  `json:"usage"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse cloud JSON: %w", err)
	}

	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM cloud_tenants").Scan(&count)
	if count > 0 {
		log.Printf("db: cloud_tenants not empty (%d rows), skipping", count)
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range snap.Tenants {
		providerJSON, _ := json.Marshal(t.ProviderKeys)
		configJSON, _ := json.Marshal(t.ProxyConfig)
		productsJSON, _ := json.Marshal(t.EnabledProducts)

		_, err := tx.Exec(`INSERT OR IGNORE INTO cloud_tenants (id, email, name, api_key, plan, daily_request_limit, stripe_customer_id, stripe_subscription_id, provider_keys_json, proxy_config_json, enabled_products_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.Email, t.Name, t.APIKey, t.Plan, t.DailyRequestLimit,
			t.StripeCustomerID, t.StripeSubscriptionID,
			string(providerJSON), string(configJSON), string(productsJSON),
			t.CreatedAt.Format(time.RFC3339))
		if err != nil {
			log.Printf("db: import tenant %s: %v", t.Email, err)
		}
	}

	for _, u := range snap.Usage {
		_, err := tx.Exec(`INSERT OR IGNORE INTO cloud_usage (tenant_id, date, requests, tokens_in, tokens_out, cost_usd, cache_hits, errors) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			u.TenantID, u.Date, u.Requests, u.TokensIn, u.TokensOut, u.CostUSD, u.CacheHits, u.Errors)
		if err != nil {
			log.Printf("db: import usage: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("db: imported %d cloud tenants + %d usage records from JSON", len(snap.Tenants), len(snap.Usage))
	os.Rename(path, path+".migrated")
	return nil
}

func (db *SqliteDB) importExchangeJSON(dataDir string) error {
	path := filepath.Join(dataDir, "exchange.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var snap struct {
		Items map[string]*ExchangeItem `json:"items"`
		Stars map[string]bool          `json:"stars"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse exchange JSON: %w", err)
	}

	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM exchange_items").Scan(&count)
	if count > 0 {
		log.Printf("db: exchange_items not empty (%d rows), skipping", count)
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range snap.Items {
		tagsJSON, _ := json.Marshal(item.Tags)
		productsJSON, _ := json.Marshal(item.Products)
		providersJSON, _ := json.Marshal(item.Providers)

		_, err := tx.Exec(`INSERT OR IGNORE INTO exchange_items (id, slug, type, title, description, author_email, author_name, content, tags_json, products_json, providers_json, downloads, stars, forks, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.Slug, item.Type, item.Title, item.Description,
			item.AuthorEmail, item.AuthorName, item.Content,
			string(tagsJSON), string(productsJSON), string(providersJSON),
			item.Downloads, item.Stars, item.Forks, item.Status,
			item.CreatedAt.Format(time.RFC3339), item.UpdatedAt.Format(time.RFC3339))
		if err != nil {
			log.Printf("db: import exchange %s: %v", item.Slug, err)
		}
	}

	for key, starred := range snap.Stars {
		if !starred {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 {
			tx.Exec("INSERT OR IGNORE INTO exchange_stars (slug, email) VALUES (?, ?)", parts[0], parts[1])
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("db: imported %d exchange items from JSON", len(snap.Items))
	os.Rename(path, path+".migrated")
	return nil
}

// ---------------------------------------------------------------------------
// Customer / License methods (replaces customers.go DB)
// ---------------------------------------------------------------------------

func (db *SqliteDB) UpsertCustomer(stripeID, email, name string) (*Customer, error) {
	now := time.Now().Format(time.RFC3339)
	_, err := db.conn.Exec(`INSERT INTO customers (stripe_customer_id, email, name, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(stripe_customer_id) DO UPDATE SET email=excluded.email, name=CASE WHEN excluded.name='' THEN name ELSE excluded.name END`,
		stripeID, email, name, now)
	if err != nil {
		return nil, err
	}
	return db.GetCustomerByStripeID(stripeID)
}

func (db *SqliteDB) GetCustomerByStripeID(stripeID string) (*Customer, error) {
	c := &Customer{}
	var createdAt string
	err := db.conn.QueryRow("SELECT id, stripe_customer_id, email, name, created_at FROM customers WHERE stripe_customer_id = ?", stripeID).
		Scan(&c.ID, &c.StripeCustomerID, &c.Email, &c.Name, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %s", stripeID)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return c, nil
}

func (db *SqliteDB) GetCustomerByEmail(email string) (*Customer, error) {
	c := &Customer{}
	var createdAt string
	err := db.conn.QueryRow("SELECT id, stripe_customer_id, email, name, created_at FROM customers WHERE email = ? LIMIT 1", email).
		Scan(&c.ID, &c.StripeCustomerID, &c.Email, &c.Name, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %s", email)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return c, nil
}

func (db *SqliteDB) CreateLicense(rec *LicenseRecord) error {
	expiresAt := ""
	if !rec.ExpiresAt.IsZero() {
		expiresAt = rec.ExpiresAt.Format(time.RFC3339)
	}
	res, err := db.conn.Exec(`INSERT INTO licenses (customer_id, stripe_customer_id, stripe_subscription_id, product, tier, license_key, status, email, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.CustomerID, rec.StripeCustomerID, rec.StripeSubscriptionID, rec.Product, rec.Tier, rec.LicenseKey, rec.Status, rec.Email, expiresAt)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	rec.ID = id
	rec.CreatedAt = time.Now()
	return nil
}

func (db *SqliteDB) GetLicenseByKey(key string) (*LicenseRecord, error) {
	return db.scanLicenseRow(db.conn.QueryRow("SELECT id, customer_id, stripe_customer_id, stripe_subscription_id, product, tier, license_key, status, email, created_at, expires_at FROM licenses WHERE license_key = ?", key))
}

func (db *SqliteDB) GetLicensesBySubscription(subID string) ([]*LicenseRecord, error) {
	rows, err := db.conn.Query("SELECT id, customer_id, stripe_customer_id, stripe_subscription_id, product, tier, license_key, status, email, created_at, expires_at FROM licenses WHERE stripe_subscription_id = ?", subID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return db.scanLicenseRows(rows)
}

func (db *SqliteDB) GetLicensesByCustomer(stripeCustomerID string) ([]*LicenseRecord, error) {
	rows, err := db.conn.Query("SELECT id, customer_id, stripe_customer_id, stripe_subscription_id, product, tier, license_key, status, email, created_at, expires_at FROM licenses WHERE stripe_customer_id = ?", stripeCustomerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return db.scanLicenseRows(rows)
}

func (db *SqliteDB) UpdateLicenseStatus(subID, status string) error {
	_, err := db.conn.Exec("UPDATE licenses SET status = ? WHERE stripe_subscription_id = ?", status, subID)
	return err
}

func (db *SqliteDB) UpdateLicenseTier(subID, tier string) error {
	_, err := db.conn.Exec("UPDATE licenses SET tier = ? WHERE stripe_subscription_id = ?", tier, subID)
	return err
}

func (db *SqliteDB) UpdateLicenseStatusByID(id int64, status string) error {
	_, err := db.conn.Exec("UPDATE licenses SET status = ? WHERE id = ?", status, id)
	return err
}

func (db *SqliteDB) IsWebhookProcessed(eventID string) bool {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM processed_webhooks WHERE event_id = ?", eventID).Scan(&count)
	return count > 0
}

func (db *SqliteDB) MarkWebhookProcessed(eventID, eventType string) error {
	_, err := db.conn.Exec("INSERT OR IGNORE INTO processed_webhooks (event_id, event_type) VALUES (?, ?)", eventID, eventType)
	return err
}

func (db *SqliteDB) Stats() map[string]any {
	var custCount, activeCount, totalCount, canceledCount int64
	db.conn.QueryRow("SELECT COUNT(*) FROM customers").Scan(&custCount)
	db.conn.QueryRow("SELECT COUNT(*) FROM licenses WHERE status = 'active'").Scan(&activeCount)
	db.conn.QueryRow("SELECT COUNT(*) FROM licenses").Scan(&totalCount)
	db.conn.QueryRow("SELECT COUNT(*) FROM licenses WHERE status = 'canceled'").Scan(&canceledCount)

	tierCounts := map[string]int64{}
	rows, _ := db.conn.Query("SELECT tier, COUNT(*) FROM licenses WHERE status = 'active' GROUP BY tier")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tier string
			var c int64
			rows.Scan(&tier, &c)
			tierCounts[tier] = c
		}
	}

	return map[string]any{
		"customers":         custCount,
		"active_licenses":   activeCount,
		"total_licenses":    totalCount,
		"canceled_licenses": canceledCount,
		"by_tier":           tierCounts,
	}
}

func (db *SqliteDB) scanLicenseRow(row *sql.Row) (*LicenseRecord, error) {
	l := &LicenseRecord{}
	var createdAt, expiresAt string
	err := row.Scan(&l.ID, &l.CustomerID, &l.StripeCustomerID, &l.StripeSubscriptionID, &l.Product, &l.Tier, &l.LicenseKey, &l.Status, &l.Email, &createdAt, &expiresAt)
	if err != nil {
		return nil, fmt.Errorf("license not found")
	}
	l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	l.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	return l, nil
}

func (db *SqliteDB) scanLicenseRows(rows *sql.Rows) ([]*LicenseRecord, error) {
	var result []*LicenseRecord
	for rows.Next() {
		l := &LicenseRecord{}
		var createdAt, expiresAt string
		if err := rows.Scan(&l.ID, &l.CustomerID, &l.StripeCustomerID, &l.StripeSubscriptionID, &l.Product, &l.Tier, &l.LicenseKey, &l.Status, &l.Email, &createdAt, &expiresAt); err != nil {
			continue
		}
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		l.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		result = append(result, l)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Cloud tenant methods (replaces cloud.go CloudStore)
// ---------------------------------------------------------------------------

func (db *SqliteDB) CreateTenant(email, name string) (*CloudTenant, error) {
	// Check if email exists
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM cloud_tenants WHERE email = ?", email).Scan(&count)
	if count > 0 {
		return nil, fmt.Errorf("email already registered: %s", email)
	}

	tenant := &CloudTenant{
		ID:                generateTenantID(),
		Email:             email,
		Name:              name,
		APIKey:            generateAPIKey(),
		Plan:              "free",
		CreatedAt:         time.Now(),
		DailyRequestLimit: 1000,
		ProviderKeys:      make(map[string]string),
		ProxyConfig:       make(map[string]any),
		EnabledProducts:   []string{"costcap"},
	}

	_, err := db.conn.Exec(`INSERT INTO cloud_tenants (id, email, name, api_key, plan, daily_request_limit, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tenant.ID, tenant.Email, tenant.Name, tenant.APIKey, tenant.Plan, tenant.DailyRequestLimit, tenant.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return tenant, nil
}

func (db *SqliteDB) GetTenantByAPIKey(apiKey string) (*CloudTenant, error) {
	return db.scanTenant("SELECT id, email, name, api_key, plan, daily_request_limit, stripe_customer_id, stripe_subscription_id, provider_keys_json, proxy_config_json, enabled_products_json, created_at FROM cloud_tenants WHERE api_key = ?", apiKey)
}

func (db *SqliteDB) GetTenantByEmail(email string) (*CloudTenant, error) {
	return db.scanTenant("SELECT id, email, name, api_key, plan, daily_request_limit, stripe_customer_id, stripe_subscription_id, provider_keys_json, proxy_config_json, enabled_products_json, created_at FROM cloud_tenants WHERE email = ?", email)
}

func (db *SqliteDB) GetTenantByID(id string) (*CloudTenant, error) {
	return db.scanTenant("SELECT id, email, name, api_key, plan, daily_request_limit, stripe_customer_id, stripe_subscription_id, provider_keys_json, proxy_config_json, enabled_products_json, created_at FROM cloud_tenants WHERE id = ?", id)
}

func (db *SqliteDB) scanTenant(query string, args ...any) (*CloudTenant, error) {
	t := &CloudTenant{}
	var providerJSON, configJSON, productsJSON, createdAt, stripeCust, stripeSub string
	err := db.conn.QueryRow(query, args...).Scan(
		&t.ID, &t.Email, &t.Name, &t.APIKey, &t.Plan, &t.DailyRequestLimit,
		&stripeCust, &stripeSub, &providerJSON, &configJSON, &productsJSON, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}
	t.StripeCustomerID = stripeCust
	t.StripeSubscriptionID = stripeSub
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	json.Unmarshal([]byte(providerJSON), &t.ProviderKeys)
	json.Unmarshal([]byte(configJSON), &t.ProxyConfig)
	json.Unmarshal([]byte(productsJSON), &t.EnabledProducts)
	if t.ProviderKeys == nil {
		t.ProviderKeys = make(map[string]string)
	}
	if t.ProxyConfig == nil {
		t.ProxyConfig = make(map[string]any)
	}
	return t, nil
}

func (db *SqliteDB) UpdateProviderKeys(apiKey string, keys map[string]string) error {
	j, _ := json.Marshal(keys)
	_, err := db.conn.Exec("UPDATE cloud_tenants SET provider_keys_json = ? WHERE api_key = ?", string(j), apiKey)
	return err
}

func (db *SqliteDB) UpdateProxyConfig(apiKey string, config map[string]any) error {
	j, _ := json.Marshal(config)
	_, err := db.conn.Exec("UPDATE cloud_tenants SET proxy_config_json = ? WHERE api_key = ?", string(j), apiKey)
	return err
}

func (db *SqliteDB) UpgradeToPro(apiKey, stripeCustomerID, stripeSubID string) error {
	productsJSON, _ := json.Marshal([]string{"*"})
	_, err := db.conn.Exec("UPDATE cloud_tenants SET plan = 'pro', daily_request_limit = 0, enabled_products_json = ?, stripe_customer_id = ?, stripe_subscription_id = ? WHERE api_key = ?",
		string(productsJSON), stripeCustomerID, stripeSubID, apiKey)
	return err
}

func (db *SqliteDB) DowngradeToFree(stripeSubID string) error {
	productsJSON, _ := json.Marshal([]string{"costcap"})
	res, err := db.conn.Exec("UPDATE cloud_tenants SET plan = 'free', daily_request_limit = 1000, enabled_products_json = ?, stripe_subscription_id = '' WHERE stripe_subscription_id = ?",
		string(productsJSON), stripeSubID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subscription not found: %s", stripeSubID)
	}
	return nil
}

func (db *SqliteDB) IncrementUsage(tenantID string, tokensIn, tokensOut int64, costUSD float64, cacheHit bool, isError bool) {
	date := time.Now().UTC().Format("2006-01-02")
	cacheInc := int64(0)
	if cacheHit {
		cacheInc = 1
	}
	errorInc := int64(0)
	if isError {
		errorInc = 1
	}
	db.conn.Exec(`INSERT INTO cloud_usage (tenant_id, date, requests, tokens_in, tokens_out, cost_usd, cache_hits, errors)
		VALUES (?, ?, 1, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, date) DO UPDATE SET
			requests = requests + 1,
			tokens_in = tokens_in + excluded.tokens_in,
			tokens_out = tokens_out + excluded.tokens_out,
			cost_usd = cost_usd + excluded.cost_usd,
			cache_hits = cache_hits + excluded.cache_hits,
			errors = errors + excluded.errors`,
		tenantID, date, tokensIn, tokensOut, costUSD, cacheInc, errorInc)
}

func (db *SqliteDB) GetUsageToday(tenantID string) *CloudUsage {
	date := time.Now().UTC().Format("2006-01-02")
	u := &CloudUsage{TenantID: tenantID, Date: date}
	db.conn.QueryRow("SELECT requests, tokens_in, tokens_out, cost_usd, cache_hits, errors FROM cloud_usage WHERE tenant_id = ? AND date = ?", tenantID, date).
		Scan(&u.Requests, &u.TokensIn, &u.TokensOut, &u.CostUSD, &u.CacheHits, &u.Errors)
	return u
}

func (db *SqliteDB) GetUsageRange(tenantID, startDate, endDate string) []*CloudUsage {
	rows, err := db.conn.Query("SELECT tenant_id, date, requests, tokens_in, tokens_out, cost_usd, cache_hits, errors FROM cloud_usage WHERE tenant_id = ? AND date >= ? AND date <= ? ORDER BY date", tenantID, startDate, endDate)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []*CloudUsage
	for rows.Next() {
		u := &CloudUsage{}
		rows.Scan(&u.TenantID, &u.Date, &u.Requests, &u.TokensIn, &u.TokensOut, &u.CostUSD, &u.CacheHits, &u.Errors)
		results = append(results, u)
	}
	return results
}

func (db *SqliteDB) CheckRateLimit(tenantID string, limit int) bool {
	if limit <= 0 {
		return false
	}
	u := db.GetUsageToday(tenantID)
	return u.Requests >= int64(limit)
}

func (db *SqliteDB) ListTenants() []*CloudTenant {
	rows, err := db.conn.Query("SELECT id, email, name, api_key, plan, daily_request_limit, stripe_customer_id, stripe_subscription_id, provider_keys_json, proxy_config_json, enabled_products_json, created_at FROM cloud_tenants ORDER BY created_at DESC")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []*CloudTenant
	for rows.Next() {
		t := &CloudTenant{}
		var providerJSON, configJSON, productsJSON, createdAt, stripeCust, stripeSub string
		if err := rows.Scan(&t.ID, &t.Email, &t.Name, &t.APIKey, &t.Plan, &t.DailyRequestLimit, &stripeCust, &stripeSub, &providerJSON, &configJSON, &productsJSON, &createdAt); err != nil {
			continue
		}
		t.StripeCustomerID = stripeCust
		t.StripeSubscriptionID = stripeSub
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		json.Unmarshal([]byte(providerJSON), &t.ProviderKeys)
		json.Unmarshal([]byte(configJSON), &t.ProxyConfig)
		json.Unmarshal([]byte(productsJSON), &t.EnabledProducts)
		result = append(result, t)
	}
	return result
}

func (db *SqliteDB) CloudStats() map[string]any {
	var free, pro int
	db.conn.QueryRow("SELECT COUNT(*) FROM cloud_tenants WHERE plan = 'free'").Scan(&free)
	db.conn.QueryRow("SELECT COUNT(*) FROM cloud_tenants WHERE plan = 'pro'").Scan(&pro)
	return map[string]any{
		"total_tenants": free + pro,
		"free":          free,
		"pro":           pro,
	}
}

// ---------------------------------------------------------------------------
// Exchange methods (replaces exchange.go ExchangeStore)
// ---------------------------------------------------------------------------

func (db *SqliteDB) CreateExchangeItem(item *ExchangeItem) error {
	tagsJSON, _ := json.Marshal(item.Tags)
	productsJSON, _ := json.Marshal(item.Products)
	providersJSON, _ := json.Marshal(item.Providers)
	if item.ID == "" {
		item.ID = generateID("ex_", 12)
	}
	if item.Status == "" {
		item.Status = "published"
	}
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	_, err := db.conn.Exec(`INSERT INTO exchange_items (id, slug, type, title, description, author_email, author_name, content, tags_json, products_json, providers_json, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Slug, item.Type, item.Title, item.Description,
		item.AuthorEmail, item.AuthorName, item.Content,
		string(tagsJSON), string(productsJSON), string(providersJSON),
		item.Status, item.CreatedAt.Format(time.RFC3339), item.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *SqliteDB) GetExchangeItem(slug string) (*ExchangeItem, error) {
	item := &ExchangeItem{}
	var tagsJSON, productsJSON, providersJSON, createdAt, updatedAt string
	err := db.conn.QueryRow("SELECT id, slug, type, title, description, author_email, author_name, content, tags_json, products_json, providers_json, downloads, stars, forks, status, created_at, updated_at FROM exchange_items WHERE slug = ?", slug).
		Scan(&item.ID, &item.Slug, &item.Type, &item.Title, &item.Description,
			&item.AuthorEmail, &item.AuthorName, &item.Content,
			&tagsJSON, &productsJSON, &providersJSON,
			&item.Downloads, &item.Stars, &item.Forks, &item.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("not found: %s", slug)
	}
	json.Unmarshal([]byte(tagsJSON), &item.Tags)
	json.Unmarshal([]byte(productsJSON), &item.Products)
	json.Unmarshal([]byte(providersJSON), &item.Providers)
	item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return item, nil
}

func (db *SqliteDB) ListExchangeItems(itemType, tag, sortBy string, limit, offset int) []*ExchangeItem {
	if limit <= 0 {
		limit = 20
	}

	query := "SELECT id, slug, type, title, description, author_email, author_name, content, tags_json, products_json, providers_json, downloads, stars, forks, status, created_at, updated_at FROM exchange_items WHERE status IN ('published','featured')"
	var args []any

	if itemType != "" {
		query += " AND type = ?"
		args = append(args, itemType)
	}
	if tag != "" {
		query += " AND tags_json LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}

	switch sortBy {
	case "stars":
		query += " ORDER BY stars DESC"
	case "downloads":
		query += " ORDER BY downloads DESC"
	case "newest":
		query += " ORDER BY created_at DESC"
	default:
		query += " ORDER BY CASE WHEN status='featured' THEN 0 ELSE 1 END, stars DESC"
	}

	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return db.scanExchangeRows(rows)
}

func (db *SqliteDB) FeaturedExchangeItems() []*ExchangeItem {
	rows, err := db.conn.Query("SELECT id, slug, type, title, description, author_email, author_name, content, tags_json, products_json, providers_json, downloads, stars, forks, status, created_at, updated_at FROM exchange_items WHERE status = 'featured' ORDER BY stars DESC")
	if err != nil {
		return nil
	}
	defer rows.Close()
	return db.scanExchangeRows(rows)
}

func (db *SqliteDB) IncrementExchangeDownloads(slug string) error {
	res, err := db.conn.Exec("UPDATE exchange_items SET downloads = downloads + 1 WHERE slug = ?", slug)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("not found: %s", slug)
	}
	return nil
}

func (db *SqliteDB) ToggleExchangeStar(slug, email string) (int64, bool, error) {
	// Check current state
	var exists int
	db.conn.QueryRow("SELECT COUNT(*) FROM exchange_stars WHERE slug = ? AND email = ?", slug, email).Scan(&exists)

	if exists > 0 {
		db.conn.Exec("DELETE FROM exchange_stars WHERE slug = ? AND email = ?", slug, email)
		db.conn.Exec("UPDATE exchange_items SET stars = stars - 1 WHERE slug = ?", slug)
	} else {
		db.conn.Exec("INSERT OR IGNORE INTO exchange_stars (slug, email) VALUES (?, ?)", slug, email)
		db.conn.Exec("UPDATE exchange_items SET stars = stars + 1 WHERE slug = ?", slug)
	}

	var stars int64
	db.conn.QueryRow("SELECT stars FROM exchange_items WHERE slug = ?", slug).Scan(&stars)
	return stars, exists == 0, nil
}

func (db *SqliteDB) ForkExchangeItem(slug, newSlug, email, name string) (*ExchangeItem, error) {
	original, err := db.GetExchangeItem(slug)
	if err != nil {
		return nil, err
	}

	fork := &ExchangeItem{
		Slug:        newSlug,
		Type:        original.Type,
		Title:       original.Title + " (fork)",
		Description: original.Description,
		AuthorEmail: email,
		AuthorName:  name,
		Content:     original.Content,
		Tags:        original.Tags,
		Products:    original.Products,
		Providers:   original.Providers,
		Status:      "published",
	}
	if err := db.CreateExchangeItem(fork); err != nil {
		return nil, err
	}
	db.conn.Exec("UPDATE exchange_items SET forks = forks + 1 WHERE slug = ?", slug)
	return fork, nil
}

func (db *SqliteDB) ExchangeStats() map[string]any {
	typeCounts := map[string]int{}
	var totalDownloads, totalStars int64
	var totalItems int

	rows, _ := db.conn.Query("SELECT type, COUNT(*), SUM(downloads), SUM(stars) FROM exchange_items GROUP BY type")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var c int
			var d, s int64
			rows.Scan(&t, &c, &d, &s)
			typeCounts[t] = c
			totalDownloads += d
			totalStars += s
			totalItems += c
		}
	}

	return map[string]any{
		"total_items":     totalItems,
		"by_type":         typeCounts,
		"total_downloads": totalDownloads,
		"total_stars":     totalStars,
	}
}

func (db *SqliteDB) scanExchangeRows(rows *sql.Rows) []*ExchangeItem {
	var result []*ExchangeItem
	for rows.Next() {
		item := &ExchangeItem{}
		var tagsJSON, productsJSON, providersJSON, createdAt, updatedAt string
		if err := rows.Scan(&item.ID, &item.Slug, &item.Type, &item.Title, &item.Description,
			&item.AuthorEmail, &item.AuthorName, &item.Content,
			&tagsJSON, &productsJSON, &providersJSON,
			&item.Downloads, &item.Stars, &item.Forks, &item.Status, &createdAt, &updatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(tagsJSON), &item.Tags)
		json.Unmarshal([]byte(productsJSON), &item.Products)
		json.Unmarshal([]byte(providersJSON), &item.Providers)
		item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		result = append(result, item)
	}
	return result
}

// ExchangeItemCount returns total exchange items (used for seed check).
func (db *SqliteDB) ExchangeItemCount() int {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM exchange_items").Scan(&count)
	return count
}
