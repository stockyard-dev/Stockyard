package apiserver

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/license"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func testKeyPair(t *testing.T) *license.KeyPair {
	t.Helper()
	kp, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	license.ProductionPublicKey = kp.PublicKeyB64()
	t.Cleanup(func() { license.ProductionPublicKey = "" })
	return kp
}

func testServer(t *testing.T) (*Server, *DB, *license.KeyPair) {
	t.Helper()
	db := testDB(t)
	kp := testKeyPair(t)
	stripe := NewStripeClient(StripeConfig{SecretKey: "sk_test_fake"})
	mailer := &LogMailer{}

	srv := NewServer(
		ServerConfig{Port: 0, AdminKey: "test-admin-key"},
		db, stripe, kp, mailer,
	)
	return srv, db, kp
}

// --- Database tests ---

func TestDBCustomerUpsert(t *testing.T) {
	db := testDB(t)

	cust, err := db.UpsertCustomer("cus_123", "test@example.com", "Test User")
	if err != nil {
		t.Fatal(err)
	}
	if cust.StripeCustomerID != "cus_123" {
		t.Errorf("stripe_id = %s, want cus_123", cust.StripeCustomerID)
	}
	if cust.Email != "test@example.com" {
		t.Errorf("email = %s, want test@example.com", cust.Email)
	}

	// Upsert same customer with updated email
	cust2, err := db.UpsertCustomer("cus_123", "new@example.com", "Test User")
	if err != nil {
		t.Fatal(err)
	}
	if cust2.Email != "new@example.com" {
		t.Errorf("updated email = %s, want new@example.com", cust2.Email)
	}
	if cust2.ID != cust.ID {
		t.Error("upsert should preserve ID")
	}
}

func TestDBLicenseCRUD(t *testing.T) {
	db := testDB(t)

	cust, _ := db.UpsertCustomer("cus_abc", "dev@test.com", "")

	rec := &LicenseRecord{
		CustomerID:           cust.ID,
		StripeCustomerID:     "cus_abc",
		StripeSubscriptionID: "sub_xyz",
		Product:              "costcap",
		Tier:                 "pro",
		LicenseKey:           "SY-test-key-123",
		Status:               "active",
		Email:                "dev@test.com",
		ExpiresAt:            time.Now().Add(365 * 24 * time.Hour),
	}
	if err := db.CreateLicense(rec); err != nil {
		t.Fatal(err)
	}
	if rec.ID == 0 {
		t.Error("license ID should be set")
	}

	// Get by key
	found, err := db.GetLicenseByKey("SY-test-key-123")
	if err != nil {
		t.Fatal(err)
	}
	if found.Product != "costcap" {
		t.Errorf("product = %s, want costcap", found.Product)
	}

	// Get by subscription
	bySub, err := db.GetLicensesBySubscription("sub_xyz")
	if err != nil {
		t.Fatal(err)
	}
	if len(bySub) != 1 {
		t.Fatalf("expected 1 license by sub, got %d", len(bySub))
	}

	// Get by customer
	byCust, err := db.GetLicensesByCustomer("cus_abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(byCust) != 1 {
		t.Fatalf("expected 1 license by customer, got %d", len(byCust))
	}

	// Update status
	db.UpdateLicenseStatus("sub_xyz", "canceled")
	updated, _ := db.GetLicenseByKey("SY-test-key-123")
	if updated.Status != "canceled" {
		t.Errorf("status = %s, want canceled", updated.Status)
	}

	// Update tier
	db.UpdateLicenseStatus("sub_xyz", "active")
	db.UpdateLicenseTier("sub_xyz", "team")
	upgraded, _ := db.GetLicenseByKey("SY-test-key-123")
	if upgraded.Tier != "team" {
		t.Errorf("tier = %s, want team", upgraded.Tier)
	}
}

func TestDBWebhookIdempotency(t *testing.T) {
	db := testDB(t)

	if db.IsWebhookProcessed("evt_123") {
		t.Error("event should not be processed yet")
	}

	db.MarkWebhookProcessed("evt_123", "checkout.session.completed")

	if !db.IsWebhookProcessed("evt_123") {
		t.Error("event should be processed")
	}
}

func TestDBStats(t *testing.T) {
	db := testDB(t)

	cust, _ := db.UpsertCustomer("cus_1", "a@b.com", "")
	db.CreateLicense(&LicenseRecord{
		CustomerID: cust.ID, StripeCustomerID: "cus_1", Product: "costcap",
		Tier: "pro", LicenseKey: "k1", Status: "active", Email: "a@b.com",
	})
	db.CreateLicense(&LicenseRecord{
		CustomerID: cust.ID, StripeCustomerID: "cus_1", Product: "stockyard",
		Tier: "team", LicenseKey: "k2", Status: "active", Email: "a@b.com",
	})

	stats := db.Stats()
	if stats["customers"].(int64) != 1 {
		t.Errorf("customers = %v, want 1", stats["customers"])
	}
	if stats["active_licenses"].(int64) != 2 {
		t.Errorf("active_licenses = %v, want 2", stats["active_licenses"])
	}
}

// --- Product catalog tests ---

func TestCatalog(t *testing.T) {
	products := Catalog()
	if len(products) != 6 {
		t.Errorf("catalog has %d apps, expected 6", len(products))
	}

	// Check apps exist
	for _, slug := range []string{"proxy", "observe", "trust", "studio", "forge", "exchange"} {
		if ProductBySlug(slug) == nil {
			t.Errorf("app %s not found in catalog", slug)
		}
	}

	// Check unknown product
	if ProductBySlug("nonexistent") != nil {
		t.Error("nonexistent product should return nil")
	}

	// Check plans
	plans := Plans()
	if len(plans) != 3 {
		t.Errorf("expected 3 plans, got %d", len(plans))
	}
	cloud := PlanBySlug("cloud")
	if cloud == nil {
		t.Fatal("cloud plan not found")
	}
	if cloud.PriceCents != 2900 {
		t.Errorf("cloud price = %d, want 2900", cloud.PriceCents)
	}
}

// --- HTTP endpoint tests ---

func TestHealthEndpoint(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
}

func TestProductsEndpoint(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/products", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	count := resp["count"].(float64)
	if count < 120 {
		t.Errorf("product count = %v, expected 120+", count)
	}
}

func TestProductsFilterByCategory(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/products?category=safety", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	count := resp["count"].(float64)
	if count < 3 {
		t.Errorf("safety products = %v, expected at least 3", count)
	}
}

func TestProductBySlugEndpoint(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/products/costcap", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["slug"] != "costcap" {
		t.Errorf("slug = %v, want costcap", resp["slug"])
	}
}

func TestProductNotFound(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/products/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestValidateLicenseEndpoint(t *testing.T) {
	srv, db, kp := testServer(t)

	// Issue a key
	key, _ := kp.Issue(license.IssueRequest{
		Product: "costcap", Tier: license.TierPro, CustomerID: "cus_test", Email: "t@t.com",
		Duration: 365 * 24 * time.Hour,
	})

	// Store in DB
	cust, _ := db.UpsertCustomer("cus_test", "t@t.com", "")
	db.CreateLicense(&LicenseRecord{
		CustomerID: cust.ID, StripeCustomerID: "cus_test", Product: "costcap",
		Tier: "pro", LicenseKey: key, Status: "active", Email: "t@t.com",
	})

	// Validate
	req := httptest.NewRequest("GET", "/api/license/validate?key="+key, nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["valid"] != true {
		t.Errorf("valid = %v, want true", resp["valid"])
	}
	if resp["product"] != "costcap" {
		t.Errorf("product = %v, want costcap", resp["product"])
	}
}

func TestValidateInvalidKey(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/license/validate?key=SY-garbage.trash", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["valid"] != false {
		t.Errorf("valid = %v, want false", resp["valid"])
	}
}

func TestLookupLicenseEndpoint(t *testing.T) {
	srv, db, _ := testServer(t)

	cust, _ := db.UpsertCustomer("cus_look", "look@test.com", "")
	db.CreateLicense(&LicenseRecord{
		CustomerID: cust.ID, StripeCustomerID: "cus_look", Product: "stockyard",
		Tier: "pro", LicenseKey: "SY-abcdefghijklmnopqrstuvwxyz.sig", Status: "active", Email: "look@test.com",
	})

	req := httptest.NewRequest("GET", "/api/license/lookup?email=look@test.com", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["found"] != true {
		t.Errorf("found = %v, want true", resp["found"])
	}

	licenses := resp["licenses"].([]any)
	if len(licenses) != 1 {
		t.Fatalf("expected 1 license, got %d", len(licenses))
	}

	lic := licenses[0].(map[string]any)
	if lic["product"] != "stockyard" {
		t.Errorf("product = %v, want stockyard", lic["product"])
	}
	// Key should be masked
	masked := lic["key_masked"].(string)
	if !strings.Contains(masked, "...") {
		t.Errorf("key should be masked, got %s", masked)
	}
}

// --- Admin endpoint tests ---

func TestAdminRequiresKey(t *testing.T) {
	srv, _, _ := testServer(t)

	// No admin key
	req := httptest.NewRequest("GET", "/api/admin/stats", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAdminStats(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/admin/stats", nil)
	req.Header.Set("X-Admin-Key", "test-admin-key")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["product_count"] == nil {
		t.Error("product_count missing from stats")
	}
}

func TestAdminIssueKey(t *testing.T) {
	srv, _, _ := testServer(t)

	body, _ := json.Marshal(map[string]any{
		"product": "costcap",
		"tier":    "pro",
		"email":   "admin-issue@test.com",
		"days":    90,
	})

	req := httptest.NewRequest("POST", "/api/admin/issue", bytes.NewReader(body))
	req.Header.Set("X-Admin-Key", "test-admin-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["license_key"] == nil {
		t.Error("license_key missing")
	}
	key := resp["license_key"].(string)
	if !strings.HasPrefix(key, "SY-") {
		t.Errorf("key should start with SY-, got %s", key[:10])
	}
}

func TestAdminRevoke(t *testing.T) {
	srv, db, kp := testServer(t)

	key, _ := kp.Issue(license.IssueRequest{
		Product: "costcap", Tier: license.TierPro, CustomerID: "cus_rev",
	})
	cust, _ := db.UpsertCustomer("cus_rev", "rev@test.com", "")
	db.CreateLicense(&LicenseRecord{
		CustomerID: cust.ID, StripeCustomerID: "cus_rev", Product: "costcap",
		Tier: "pro", LicenseKey: key, Status: "active", Email: "rev@test.com",
	})

	body, _ := json.Marshal(map[string]string{"key": key})
	req := httptest.NewRequest("POST", "/api/admin/revoke", bytes.NewReader(body))
	req.Header.Set("X-Admin-Key", "test-admin-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	// Verify revoked
	rec, _ := db.GetLicenseByKey(key)
	if rec.Status != "revoked" {
		t.Errorf("status = %s, want revoked", rec.Status)
	}
}

// --- Checkout tests (will fail without real Stripe key, but tests error handling) ---

func TestCheckoutMissingPriceID(t *testing.T) {
	srv, _, _ := testServer(t)

	body, _ := json.Marshal(map[string]string{
		"product": "costcap",
		"tier":    "pro",
	})

	req := httptest.NewRequest("POST", "/api/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Should fail because no STRIPE_PRICE_COSTCAP_PRO env var
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCheckoutUnknownProduct(t *testing.T) {
	srv, _, _ := testServer(t)

	body, _ := json.Marshal(map[string]string{"product": "fakeprod"})
	req := httptest.NewRequest("POST", "/api/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- Webhook signature tests ---

func TestWebhookSignatureVerification(t *testing.T) {
	secret := "whsec_test123"
	payload := []byte(`{"id":"evt_1","type":"test"}`)

	// Create valid signature
	ts := time.Now().Unix()
	signedPayload := fmt.Sprintf("%d.%s", ts, string(payload))

	sigHex := hmacSHA256([]byte(secret), []byte(signedPayload))
	sigHeader := fmt.Sprintf("t=%d,v1=%s", ts, sigHex)

	if !VerifyWebhookSignature(payload, sigHeader, secret) {
		t.Error("valid signature should verify")
	}

	// Wrong secret
	if VerifyWebhookSignature(payload, sigHeader, "wrong_secret") {
		t.Error("wrong secret should not verify")
	}

	// Tampered payload
	if VerifyWebhookSignature([]byte(`{"id":"evt_2"}`), sigHeader, secret) {
		t.Error("tampered payload should not verify")
	}

	// Empty
	if VerifyWebhookSignature(payload, "", secret) {
		t.Error("empty header should not verify")
	}
}

// --- Webhook handler tests ---

func TestWebhookCheckoutCompleted(t *testing.T) {
	srv, db, _ := testServer(t)

	event := map[string]any{
		"id":   "evt_test_checkout",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"customer":       "cus_webhook_test",
				"subscription":   "sub_webhook_test",
				"customer_email": "webhook@test.com",
				"metadata": map[string]any{
					"product": "costcap",
					"tier":    "pro",
				},
			},
		},
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest("POST", "/webhooks/stripe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// Verify license was created
	licenses, _ := db.GetLicensesByCustomer("cus_webhook_test")
	if len(licenses) != 1 {
		t.Fatalf("expected 1 license, got %d", len(licenses))
	}
	if licenses[0].Product != "costcap" {
		t.Errorf("product = %s, want costcap", licenses[0].Product)
	}
	if licenses[0].Tier != "pro" {
		t.Errorf("tier = %s, want pro", licenses[0].Tier)
	}
	if !strings.HasPrefix(licenses[0].LicenseKey, "SY-") {
		t.Error("license key should start with SY-")
	}

	// Verify idempotency — process same event again
	req2 := httptest.NewRequest("POST", "/webhooks/stripe", bytes.NewReader(body))
	w2 := httptest.NewRecorder()
	srv.mux.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Errorf("idempotent status = %d, want 200", w2.Code)
	}

	// Should still be just 1 license
	licenses2, _ := db.GetLicensesByCustomer("cus_webhook_test")
	if len(licenses2) != 1 {
		t.Errorf("expected 1 license after idempotent replay, got %d", len(licenses2))
	}
}

func TestWebhookSubscriptionDeleted(t *testing.T) {
	srv, db, _ := testServer(t)

	// First create a license via checkout
	checkout := map[string]any{
		"id": "evt_create", "type": "checkout.session.completed",
		"data": map[string]any{"object": map[string]any{
			"customer": "cus_del", "subscription": "sub_del",
			"customer_email": "del@test.com",
			"metadata":       map[string]any{"product": "costcap", "tier": "starter"},
		}},
	}
	body, _ := json.Marshal(checkout)
	req := httptest.NewRequest("POST", "/webhooks/stripe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Now delete subscription
	deleted := map[string]any{
		"id": "evt_delete", "type": "customer.subscription.deleted",
		"data": map[string]any{"object": map[string]any{"id": "sub_del"}},
	}
	body2, _ := json.Marshal(deleted)
	req2 := httptest.NewRequest("POST", "/webhooks/stripe", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	srv.mux.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("status = %d, want 200", w2.Code)
	}

	// License should be canceled
	licenses, _ := db.GetLicensesBySubscription("sub_del")
	if len(licenses) != 1 {
		t.Fatalf("expected 1 license, got %d", len(licenses))
	}
	if licenses[0].Status != "canceled" {
		t.Errorf("status = %s, want canceled", licenses[0].Status)
	}
}

// --- Helpers ---

func hmacSHA256(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
