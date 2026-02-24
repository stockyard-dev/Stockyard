package license

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

func TestGenerateAndValidateKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Set the production key to our test keypair
	ProductionPublicKey = kp.PublicKeyB64()
	defer func() { ProductionPublicKey = "" }()

	key, err := kp.Issue(IssueRequest{
		Product:    "costcap",
		Tier:       TierPro,
		CustomerID: "cus_test123",
		Email:      "dev@example.com",
		Duration:   365 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(key, "SY-") {
		t.Errorf("key should start with SY-, got %s", key[:10])
	}
	if !strings.Contains(key, ".") {
		t.Error("key should contain . separator")
	}

	// Validate
	lic := Validate(key)
	if !lic.Valid {
		t.Fatal("key should be valid")
	}
	if lic.Payload.Product != "costcap" {
		t.Errorf("product = %s, want costcap", lic.Payload.Product)
	}
	if lic.Payload.Tier != TierPro {
		t.Errorf("tier = %s, want pro", lic.Payload.Tier)
	}
	if lic.Payload.CustomerID != "cus_test123" {
		t.Errorf("customer = %s, want cus_test123", lic.Payload.CustomerID)
	}
	if lic.Payload.Email != "dev@example.com" {
		t.Errorf("email = %s, want dev@example.com", lic.Payload.Email)
	}
	if lic.IsExpired() {
		t.Error("key should not be expired")
	}
}

func TestExpiredKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	ProductionPublicKey = kp.PublicKeyB64()
	defer func() { ProductionPublicKey = "" }()

	// Issue a key that expired 1 hour ago by issuing with 1ns duration,
	// then manually backdating. We test via direct Payload manipulation.
	lic := &License{
		Valid: true,
		Payload: Payload{
			Product:    "costcap",
			Tier:       TierStarter,
			CustomerID: "cus_expired",
			ExpiresAt:  time.Now().Add(-1 * time.Hour).Unix(),
		},
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if !lic.IsExpired() {
		t.Error("key should be expired")
	}

	// Also test that enforcer treats expired license as free tier
	e := NewEnforcer(lic)
	if e.Tier() != TierFree {
		t.Errorf("expired license tier should be free, got %s", e.Tier())
	}
}

func TestInvalidKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"no prefix", "eyJwIjoiY29zdGNhcCJ9.abc123"},
		{"no separator", "SY-eyJwIjoiY29zdGNhcCJ9"},
		{"bad base64 payload", "SY-!!!invalid!!!.abc123"},
		{"bad base64 sig", "SY-eyJwIjoiY29zdGNhcCJ9.!!!invalid!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lic := Validate(tt.key)
			if lic.Valid {
				t.Error("should not be valid")
			}
		})
	}
}

func TestWrongSignature(t *testing.T) {
	kp1, _ := GenerateKeyPair()
	kp2, _ := GenerateKeyPair()

	// Set production key to kp1
	ProductionPublicKey = kp1.PublicKeyB64()
	defer func() { ProductionPublicKey = "" }()

	// Sign with kp2
	key, _ := kp2.Issue(IssueRequest{
		Product: "costcap", Tier: TierPro, CustomerID: "cus_wrong",
	})

	lic := Validate(key)
	if lic.Valid {
		t.Error("key signed with wrong key should not validate")
	}
}

func TestCoversProduct(t *testing.T) {
	tests := []struct {
		keyProduct string
		checkProd  string
		want       bool
	}{
		{"costcap", "costcap", true},
		{"costcap", "llmcache", false},
		{"stockyard", "costcap", true},
		{"stockyard", "anything", true},
		{"*", "costcap", true},
		{"*", "anything", true},
	}

	for _, tt := range tests {
		lic := &License{Valid: true, Payload: Payload{Product: tt.keyProduct}}
		got := lic.CoversProduct(tt.checkProd)
		if got != tt.want {
			t.Errorf("CoversProduct(%q, %q) = %v, want %v", tt.keyProduct, tt.checkProd, got, tt.want)
		}
	}
}

func TestTierLimits(t *testing.T) {
	free := Limits(TierFree)
	if free.MaxRequestsPerDay != 1000 {
		t.Errorf("free daily limit = %d, want 1000", free.MaxRequestsPerDay)
	}
	if free.WhiteLabel {
		t.Error("free should not have whitelabel")
	}

	starter := Limits(TierStarter)
	if starter.MaxRequestsPerDay != 10_000 {
		t.Errorf("starter daily limit = %d, want 10000", starter.MaxRequestsPerDay)
	}

	pro := Limits(TierPro)
	if pro.MaxRequestsPerDay != 0 {
		t.Error("pro should have unlimited daily requests")
	}
	if !pro.ExportAccess {
		t.Error("pro should have export access")
	}

	team := Limits(TierTeam)
	if !team.MultiInstance {
		t.Error("team should have multi-instance")
	}
	if !team.WhiteLabel {
		t.Error("team should have whitelabel")
	}
}

func TestEnforcerDailyLimit(t *testing.T) {
	lic := &License{
		Valid:   true,
		Payload: Payload{Product: "costcap", Tier: TierFree, CustomerID: "test"},
	}
	e := NewEnforcer(lic)

	// Free tier = 1000/day. Burn through them.
	for i := int64(0); i < 1000; i++ {
		if err := e.Check(); err != nil {
			t.Fatalf("request %d should be allowed: %v", i, err)
		}
	}

	// Next one should be blocked
	err := e.Check()
	if err == nil {
		t.Fatal("request 1001 should be blocked")
	}
	if !strings.Contains(err.Error(), "daily request limit") {
		t.Errorf("error should mention daily limit, got: %v", err)
	}
	if !strings.Contains(err.Error(), "stockyard.dev/pricing") {
		t.Error("error should include upgrade URL")
	}
}

func TestEnforcerProUnlimited(t *testing.T) {
	lic := &License{
		Valid:   true,
		Payload: Payload{Product: "costcap", Tier: TierPro, CustomerID: "test"},
	}
	e := NewEnforcer(lic)

	// Pro tier = unlimited. 10K requests should all pass.
	for i := 0; i < 10_000; i++ {
		if err := e.Check(); err != nil {
			t.Fatalf("pro request %d should be allowed: %v", i, err)
		}
	}
}

func TestEnforcerStats(t *testing.T) {
	lic := &License{
		Valid:   true,
		Payload: Payload{Product: "costcap", Tier: TierStarter, CustomerID: "cus_123"},
	}
	e := NewEnforcer(lic)

	for i := 0; i < 50; i++ {
		e.Check()
	}

	stats := e.Stats()
	if stats["tier"] != "starter" {
		t.Errorf("tier = %v, want starter", stats["tier"])
	}
	if stats["requests_today"].(int64) != 50 {
		t.Errorf("requests_today = %v, want 50", stats["requests_today"])
	}
	if stats["daily_limit"].(int64) != 10_000 {
		t.Errorf("daily_limit = %v, want 10000", stats["daily_limit"])
	}
}

func TestMiddlewareBlocks(t *testing.T) {
	lic := &License{
		Valid:   true,
		Payload: Payload{Product: "costcap", Tier: TierFree, CustomerID: "test"},
	}
	e := NewEnforcer(lic)

	called := 0
	handler := e.Middleware()(func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		called++
		return &provider.Response{ID: "ok"}, nil
	})

	// Exhaust free tier
	for i := 0; i < 1000; i++ {
		handler(context.Background(), &provider.Request{Model: "gpt-4o"})
	}
	if called != 1000 {
		t.Errorf("expected 1000 calls, got %d", called)
	}

	// Next should be blocked
	_, err := handler(context.Background(), &provider.Request{Model: "gpt-4o"})
	if err == nil {
		t.Fatal("should be blocked")
	}
	if !IsLicenseError(err) {
		t.Errorf("should be LicenseError, got %T", err)
	}
	if called != 1000 {
		t.Error("handler should not have been called again")
	}
}

func TestDevModeAcceptsAnyKey(t *testing.T) {
	// With no ProductionPublicKey set, dev mode should accept well-formed keys
	ProductionPublicKey = ""

	kp, _ := GenerateKeyPair()
	key, _ := kp.Issue(IssueRequest{
		Product: "costcap", Tier: TierPro, CustomerID: "dev_test",
	})

	lic := Validate(key)
	if !lic.Valid {
		t.Error("dev mode should accept any well-formed key")
	}
}

func TestFromEnvNoKey(t *testing.T) {
	lic := FromEnv()
	if !lic.Valid {
		t.Error("no key should return valid free license")
	}
	if lic.Payload.Tier != TierFree {
		t.Errorf("no key tier = %s, want free", lic.Payload.Tier)
	}
}

func TestHelperMethods(t *testing.T) {
	kp, _ := GenerateKeyPair()

	key, err := kp.IssueStarter("costcap", "cus_1", "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	lic := Validate(key)
	if lic.Payload.Tier != TierStarter {
		t.Error("IssueStarter should produce starter tier")
	}

	key, _ = kp.IssueSuitePro("cus_2", "b@c.com")
	lic = Validate(key)
	if lic.Payload.Product != "stockyard" {
		t.Error("IssueSuitePro should produce stockyard product")
	}
	if lic.Payload.Tier != TierPro {
		t.Error("IssueSuitePro should produce pro tier")
	}

	key, _ = kp.IssueTeam("costcap", "cus_3", "c@d.com", 10)
	lic = Validate(key)
	if lic.Payload.MaxSeats != 10 {
		t.Errorf("IssueTeam seats = %d, want 10", lic.Payload.MaxSeats)
	}
}

func TestUpgradeNudge(t *testing.T) {
	lic := &License{
		Valid:   true,
		Payload: Payload{Tier: TierFree, CustomerID: "test"},
	}
	e := NewEnforcer(lic)
	nudge := NewUpgradeNudge(e)

	// First 99 requests: no nudge
	for i := 0; i < 99; i++ {
		e.Check()
		if should, _ := nudge.ShouldNudge(); should {
			t.Errorf("should not nudge at request %d", i+1)
		}
	}

	// 100th request: nudge
	e.Check()
	if should, msg := nudge.ShouldNudge(); !should {
		t.Error("should nudge at request 100")
	} else if !strings.Contains(msg, "stockyard.dev/pricing") {
		t.Error("nudge should include pricing URL")
	}
}

func TestKeyPairB64Roundtrip(t *testing.T) {
	kp1, _ := GenerateKeyPair()

	pub64 := kp1.PublicKeyB64()
	priv64 := kp1.PrivateKeyB64()

	kp2, err := LoadKeyPair(pub64, priv64)
	if err != nil {
		t.Fatal(err)
	}

	// Keys signed by kp1 should validate with kp2 (same keypair)
	ProductionPublicKey = kp2.PublicKeyB64()
	defer func() { ProductionPublicKey = "" }()

	key, _ := kp1.Issue(IssueRequest{
		Product: "test", Tier: TierPro, CustomerID: "roundtrip",
	})
	lic := Validate(key)
	if !lic.Valid {
		t.Error("roundtrip keypair should validate")
	}
}

func TestTierFromString(t *testing.T) {
	tests := []struct{ input string; want Tier }{
		{"free", TierFree},
		{"starter", TierStarter},
		{"Starter", TierStarter},
		{"pro", TierPro},
		{"PRO", TierPro},
		{"team", TierTeam},
		{"enterprise", TierEnterprise},
		{"unknown", TierFree},
		{"", TierFree},
	}
	for _, tt := range tests {
		if got := TierFromString(tt.input); got != tt.want {
			t.Errorf("TierFromString(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
