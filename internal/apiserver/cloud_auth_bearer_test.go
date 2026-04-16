package apiserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestAuthByBearer_HappyPath: a valid cloud-single license key in the
// Authorization header resolves to the correct cloud account.
func TestAuthByBearer_HappyPath(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Seed an account + license.
	email := "customer@example.com"
	if _, err := db.CreateCloudAccount(email, "cloud-single", "cus_A", "sub_A"); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	licenseKey := "SY-testpayload.testsig"
	rec := &LicenseRecord{
		StripeCustomerID: "cus_A",
		Product:          "stockyard-desktop",
		Tier:             "cloud-single",
		LicenseKey:       licenseKey,
		Status:           "active",
		Email:            email,
		ExpiresAt:        time.Now().Add(10 * 365 * 24 * time.Hour),
	}
	if err := db.CreateLicense(rec); err != nil {
		t.Fatalf("seed license: %v", err)
	}

	svc := NewCloudService(db, &LogMailer{}, nil, "http://localhost", false)
	r := httptest.NewRequest("GET", "/api/cloud/desktop/me", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)

	acct := svc.authByBearer(r)
	if acct == nil {
		t.Fatalf("expected account, got nil")
	}
	if acct.Email != email {
		t.Fatalf("email mismatch: got %q want %q", acct.Email, email)
	}
	if acct.Tier != "cloud-single" {
		t.Fatalf("tier mismatch: got %q want cloud-single", acct.Tier)
	}
}

// TestAuthByBearer_RejectsLocalTier: Local-tier licenses must not grant
// Cloud access. Local customers don't have cloud_accounts rows.
func TestAuthByBearer_RejectsLocalTier(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	licenseKey := "SY-localkey.sig"
	rec := &LicenseRecord{
		StripeCustomerID: "cus_local",
		Product:          "stockyard-desktop",
		Tier:             "local",
		LicenseKey:       licenseKey,
		Status:           "active",
		Email:            "local@example.com",
	}
	if err := db.CreateLicense(rec); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewCloudService(db, &LogMailer{}, nil, "http://localhost", false)
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)

	if acct := svc.authByBearer(r); acct != nil {
		t.Fatalf("expected nil for local tier, got %+v", acct)
	}
}

// TestAuthByBearer_RejectsCanceled: canceled licenses must not grant
// access. Customer has to renew before uploads resume.
func TestAuthByBearer_RejectsCanceled(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	licenseKey := "SY-canceled.sig"
	rec := &LicenseRecord{
		StripeCustomerID: "cus_C",
		Product:          "stockyard-desktop",
		Tier:             "cloud-multi",
		LicenseKey:       licenseKey,
		Status:           "canceled",
		Email:            "x@example.com",
	}
	if err := db.CreateLicense(rec); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewCloudService(db, &LogMailer{}, nil, "http://localhost", false)
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)

	if acct := svc.authByBearer(r); acct != nil {
		t.Fatalf("expected nil for canceled license, got %+v", acct)
	}
}

// TestAuthByBearer_NoHeader: missing Authorization header returns nil
// without error (caller falls through to cookie auth).
func TestAuthByBearer_NoHeader(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	svc := NewCloudService(db, &LogMailer{}, nil, "http://localhost", false)
	r := httptest.NewRequest("GET", "/x", nil)
	if acct := svc.authByBearer(r); acct != nil {
		t.Fatalf("expected nil with no header, got %+v", acct)
	}
}

// TestAuthByBearer_UnknownKey: a bearer string that isn't in the
// licenses table returns nil.
func TestAuthByBearer_UnknownKey(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	svc := NewCloudService(db, &LogMailer{}, nil, "http://localhost", false)
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("Authorization", "Bearer SY-unknown.sig")
	if acct := svc.authByBearer(r); acct != nil {
		t.Fatalf("expected nil for unknown key, got %+v", acct)
	}
}

// newTestDB opens a fresh SQLite DB in a temp dir and runs migrations.
// Returned cleanup closes and removes the DB file.
func newTestDB(t *testing.T) (*SqliteDB, func()) {
	t.Helper()
	dir := t.TempDir()
	db, err := OpenSqliteDB(dir + "/test.db")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db, func() {
		_ = db.conn.Close()
	}
}

// ensure http import is retained even if the above grows.
var _ = http.MethodGet
