package apiserver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSqliteDB_CustomerLifecycle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Upsert customer
	c, err := db.UpsertCustomer("cus_test123", "test@example.com", "Test User")
	if err != nil {
		t.Fatalf("UpsertCustomer: %v", err)
	}
	if c.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", c.Email)
	}

	// Get by Stripe ID
	c2, err := db.GetCustomerByStripeID("cus_test123")
	if err != nil {
		t.Fatalf("GetCustomerByStripeID: %v", err)
	}
	if c2.ID != c.ID {
		t.Fatalf("expected same ID")
	}

	// Get by email
	c3, err := db.GetCustomerByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetCustomerByEmail: %v", err)
	}
	if c3.StripeCustomerID != "cus_test123" {
		t.Fatalf("expected cus_test123, got %s", c3.StripeCustomerID)
	}

	// Upsert same customer (update name)
	c4, err := db.UpsertCustomer("cus_test123", "test@example.com", "Updated Name")
	if err != nil {
		t.Fatalf("UpsertCustomer update: %v", err)
	}
	if c4.Name != "Updated Name" {
		t.Fatalf("expected Updated Name, got %s", c4.Name)
	}
}

func TestSqliteDB_LicenseLifecycle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	db.UpsertCustomer("cus_lic1", "lic@example.com", "")

	rec := &LicenseRecord{
		CustomerID:           1,
		StripeCustomerID:     "cus_lic1",
		StripeSubscriptionID: "sub_test1",
		Product:              "stockyard",
		Tier:                 "pro",
		LicenseKey:           "SY-TEST-KEY-123",
		Status:               "active",
		Email:                "lic@example.com",
	}
	if err := db.CreateLicense(rec); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}
	if rec.ID == 0 {
		t.Fatal("expected non-zero license ID")
	}

	// Get by key
	l, err := db.GetLicenseByKey("SY-TEST-KEY-123")
	if err != nil {
		t.Fatalf("GetLicenseByKey: %v", err)
	}
	if l.Product != "stockyard" {
		t.Fatalf("expected stockyard, got %s", l.Product)
	}

	// Get by subscription
	ls, _ := db.GetLicensesBySubscription("sub_test1")
	if len(ls) != 1 {
		t.Fatalf("expected 1 license, got %d", len(ls))
	}

	// Update status
	db.UpdateLicenseStatus("sub_test1", "canceled")
	l2, _ := db.GetLicenseByKey("SY-TEST-KEY-123")
	if l2.Status != "canceled" {
		t.Fatalf("expected canceled, got %s", l2.Status)
	}

	// Webhook idempotency
	if db.IsWebhookProcessed("evt_123") {
		t.Fatal("should not be processed yet")
	}
	db.MarkWebhookProcessed("evt_123", "checkout.session.completed")
	if !db.IsWebhookProcessed("evt_123") {
		t.Fatal("should be processed")
	}

	// Stats
	stats := db.Stats()
	if stats["total_licenses"].(int64) != 1 {
		t.Fatalf("expected 1 total license, got %v", stats["total_licenses"])
	}
}

func TestSqliteDB_CloudTenantLifecycle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Signup
	tenant, err := db.CreateTenant("cloud@example.com", "Cloud User")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if tenant.Plan != "free" {
		t.Fatalf("expected free, got %s", tenant.Plan)
	}
	if tenant.APIKey == "" {
		t.Fatal("expected non-empty API key")
	}
	if tenant.DailyRequestLimit != 1000 {
		t.Fatalf("expected 1000 limit, got %d", tenant.DailyRequestLimit)
	}

	// Duplicate email
	_, err = db.CreateTenant("cloud@example.com", "Dupe")
	if err == nil {
		t.Fatal("expected duplicate email error")
	}

	// Get by API key
	t2, err := db.GetTenantByAPIKey(tenant.APIKey)
	if err != nil {
		t.Fatalf("GetTenantByAPIKey: %v", err)
	}
	if t2.Email != "cloud@example.com" {
		t.Fatalf("expected cloud@example.com, got %s", t2.Email)
	}

	// Update provider keys
	db.UpdateProviderKeys(tenant.APIKey, map[string]string{"openai": "sk-test123"})
	t3, _ := db.GetTenantByAPIKey(tenant.APIKey)
	if t3.ProviderKeys["openai"] != "sk-test123" {
		t.Fatalf("expected sk-test123, got %s", t3.ProviderKeys["openai"])
	}

	// Update proxy config
	db.UpdateProxyConfig(tenant.APIKey, map[string]any{"costcap": map[string]any{"daily_limit_usd": 50}})
	t4, _ := db.GetTenantByAPIKey(tenant.APIKey)
	if t4.ProxyConfig["costcap"] == nil {
		t.Fatal("expected costcap config")
	}

	// Upgrade
	db.UpgradeToPro(tenant.APIKey, "cus_cloud1", "sub_cloud1")
	t5, _ := db.GetTenantByAPIKey(tenant.APIKey)
	if t5.Plan != "pro" {
		t.Fatalf("expected pro, got %s", t5.Plan)
	}
	if t5.DailyRequestLimit != 0 {
		t.Fatalf("expected unlimited (0), got %d", t5.DailyRequestLimit)
	}

	// Downgrade
	db.DowngradeToFree("sub_cloud1")
	t6, _ := db.GetTenantByAPIKey(tenant.APIKey)
	if t6.Plan != "free" {
		t.Fatalf("expected free, got %s", t6.Plan)
	}

	// Usage tracking
	db.IncrementUsage(tenant.ID, 100, 50, 0.01, false, false)
	db.IncrementUsage(tenant.ID, 200, 100, 0.02, true, false)
	db.IncrementUsage(tenant.ID, 50, 25, 0.005, false, true)

	usage := db.GetUsageToday(tenant.ID)
	if usage.Requests != 3 {
		t.Fatalf("expected 3 requests, got %d", usage.Requests)
	}
	if usage.TokensIn != 350 {
		t.Fatalf("expected 350 tokens_in, got %d", usage.TokensIn)
	}
	if usage.CacheHits != 1 {
		t.Fatalf("expected 1 cache hit, got %d", usage.CacheHits)
	}
	if usage.Errors != 1 {
		t.Fatalf("expected 1 error, got %d", usage.Errors)
	}

	// Rate limit check
	if db.CheckRateLimit(tenant.ID, 0) {
		t.Fatal("unlimited should never be rate limited")
	}
	if db.CheckRateLimit(tenant.ID, 10) {
		t.Fatal("under limit should not be rate limited")
	}
	if !db.CheckRateLimit(tenant.ID, 3) {
		t.Fatal("at limit should be rate limited")
	}

	// List + stats
	tenants := db.ListTenants()
	if len(tenants) != 1 {
		t.Fatalf("expected 1 tenant, got %d", len(tenants))
	}
	stats := db.CloudStats()
	if stats["total_tenants"].(int) != 1 {
		t.Fatalf("expected 1 total, got %v", stats["total_tenants"])
	}
}

func TestSqliteDB_ExchangeLifecycle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	item := &ExchangeItem{
		Slug:        "test-config",
		Type:        "config",
		Title:       "Test Config",
		Description: "A test configuration",
		AuthorEmail: "author@example.com",
		AuthorName:  "Author",
		Content:     "listen: :4000\n",
		Tags:        []string{"test", "starter"},
		Products:    []string{"costcap"},
		Providers:   []string{"openai"},
		Status:      "featured",
	}
	if err := db.CreateExchangeItem(item); err != nil {
		t.Fatalf("CreateExchangeItem: %v", err)
	}
	if item.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// Get
	got, err := db.GetExchangeItem("test-config")
	if err != nil {
		t.Fatalf("GetExchangeItem: %v", err)
	}
	if got.Title != "Test Config" {
		t.Fatalf("expected Test Config, got %s", got.Title)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(got.Tags))
	}

	// List
	items := db.ListExchangeItems("", "", "", 10, 0)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	// Filter by type
	items = db.ListExchangeItems("chain", "", "", 10, 0)
	if len(items) != 0 {
		t.Fatalf("expected 0 items for type=chain, got %d", len(items))
	}

	// Featured
	featured := db.FeaturedExchangeItems()
	if len(featured) != 1 {
		t.Fatalf("expected 1 featured, got %d", len(featured))
	}

	// Downloads
	db.IncrementExchangeDownloads("test-config")
	db.IncrementExchangeDownloads("test-config")
	got2, _ := db.GetExchangeItem("test-config")
	if got2.Downloads != 2 {
		t.Fatalf("expected 2 downloads, got %d", got2.Downloads)
	}

	// Stars
	stars, starred, _ := db.ToggleExchangeStar("test-config", "user@example.com")
	if !starred {
		t.Fatal("expected starred=true")
	}
	if stars != 1 {
		t.Fatalf("expected 1 star, got %d", stars)
	}

	// Unstar
	stars2, starred2, _ := db.ToggleExchangeStar("test-config", "user@example.com")
	if starred2 {
		t.Fatal("expected starred=false (unstar)")
	}
	if stars2 != 0 {
		t.Fatalf("expected 0 stars, got %d", stars2)
	}

	// Fork
	fork, err := db.ForkExchangeItem("test-config", "my-fork", "forker@example.com", "Forker")
	if err != nil {
		t.Fatalf("ForkExchangeItem: %v", err)
	}
	if fork.Title != "Test Config (fork)" {
		t.Fatalf("expected fork title, got %s", fork.Title)
	}

	// Check fork count on original
	orig, _ := db.GetExchangeItem("test-config")
	if orig.Forks != 1 {
		t.Fatalf("expected 1 fork, got %d", orig.Forks)
	}

	// Stats
	stats := db.ExchangeStats()
	if stats["total_items"].(int) != 2 {
		t.Fatalf("expected 2 total items, got %v", stats["total_items"])
	}

	// Count
	if db.ExchangeItemCount() != 2 {
		t.Fatalf("expected count 2, got %d", db.ExchangeItemCount())
	}
}

func TestSqliteDB_LegacyJSONImport(t *testing.T) {
	dir := t.TempDir()

	// Write a legacy cloud.json
	cloudJSON := `{
		"tenants": {
			"ct_abc123": {
				"id": "ct_abc123",
				"email": "legacy@example.com",
				"name": "Legacy User",
				"api_key": "sk_sy_legacy123",
				"plan": "pro",
				"created_at": "2025-01-15T10:00:00Z",
				"daily_request_limit": 0,
				"provider_keys": {"openai": "sk-legacykey"},
				"proxy_config": {},
				"enabled_products": ["*"]
			}
		},
		"usage": {
			"ct_abc123:2025-01-15": {
				"tenant_id": "ct_abc123",
				"date": "2025-01-15",
				"requests": 42,
				"tokens_in": 1000,
				"tokens_out": 500,
				"cost_usd": 0.15,
				"cache_hits": 10,
				"errors": 1
			}
		}
	}`
	os.WriteFile(filepath.Join(dir, "cloud.json"), []byte(cloudJSON), 0644)

	// Open SQLite and import
	dbPath := filepath.Join(dir, "test-import.sqlite")
	db, err := OpenSqliteDB(dbPath)
	if err != nil {
		t.Fatalf("OpenSqliteDB: %v", err)
	}
	defer db.Close()

	if err := db.ImportLegacyJSON(dir); err != nil {
		t.Fatalf("ImportLegacyJSON: %v", err)
	}

	// Verify tenant imported
	tenant, err := db.GetTenantByAPIKey("sk_sy_legacy123")
	if err != nil {
		t.Fatalf("GetTenantByAPIKey after import: %v", err)
	}
	if tenant.Email != "legacy@example.com" {
		t.Fatalf("expected legacy@example.com, got %s", tenant.Email)
	}
	if tenant.Plan != "pro" {
		t.Fatalf("expected pro, got %s", tenant.Plan)
	}
	if tenant.ProviderKeys["openai"] != "sk-legacykey" {
		t.Fatalf("expected sk-legacykey, got %s", tenant.ProviderKeys["openai"])
	}

	// Verify usage imported
	usage := db.GetUsageRange("ct_abc123", "2025-01-15", "2025-01-15")
	if len(usage) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(usage))
	}
	if usage[0].Requests != 42 {
		t.Fatalf("expected 42 requests, got %d", usage[0].Requests)
	}

	// Verify JSON file renamed
	if _, err := os.Stat(filepath.Join(dir, "cloud.json.migrated")); err != nil {
		t.Fatal("expected cloud.json.migrated to exist")
	}

	// Second import should be a no-op
	os.WriteFile(filepath.Join(dir, "cloud.json"), []byte(cloudJSON), 0644)
	db.ImportLegacyJSON(dir) // should skip because data exists
	tenants := db.ListTenants()
	if len(tenants) != 1 {
		t.Fatalf("expected 1 tenant after second import (no-op), got %d", len(tenants))
	}
}

func openTestDB(t *testing.T) *SqliteDB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sqlite")
	db, err := OpenSqliteDB(path)
	if err != nil {
		t.Fatalf("OpenSqliteDB: %v", err)
	}
	return db
}
