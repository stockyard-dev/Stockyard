package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubLicenseVerifier returns a LicenseVerifier with predetermined
// behavior for the test. Decouples middleware tests from the actual
// signature-verification machinery — that's covered by
// internal/apiserver/desktop_license_test.go.
func stubLicenseVerifier(ok bool, tier string, err error) LicenseVerifier {
	return func(key string) (bool, string, error) {
		return ok, tier, err
	}
}

// passthroughHandler captures whether the request reached the next
// handler in the chain, and what tier (if any) was injected.
type passthroughHandler struct {
	called bool
	tier   string
}

func (p *passthroughHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.called = true
	p.tier = LicenseTierFromContext(r.Context())
	w.WriteHeader(200)
}

// TestProxyAuth_LicenseKey_HappyPath: a valid SY- key with a paying
// tier is accepted, the request reaches the next handler, and the
// tier is injected into context for downstream billing.
func TestProxyAuth_LicenseKey_HappyPath(t *testing.T) {
	verifier := stubLicenseVerifier(true, "cloud-single", nil)
	next := &passthroughHandler{}
	mw := ProxyAuthMiddleware(nil, ProxyAuthRequired, verifier)
	handler := mw(next)

	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("Authorization", "Bearer SY-fake-but-verifier-says-ok")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !next.called {
		t.Fatal("expected next handler to be called")
	}
	if next.tier != "cloud-single" {
		t.Errorf("expected tier=cloud-single in context, got %q", next.tier)
	}
}

// TestProxyAuth_LicenseKey_Rejected: verifier returns ok=false (e.g.
// expired or trial tier) → middleware returns 401, next NOT called.
func TestProxyAuth_LicenseKey_Rejected(t *testing.T) {
	verifier := stubLicenseVerifier(false, "trial-cloud-single", nil)
	next := &passthroughHandler{}
	mw := ProxyAuthMiddleware(nil, ProxyAuthRequired, verifier)
	handler := mw(next)

	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("Authorization", "Bearer SY-trial-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if next.called {
		t.Fatal("next handler should not have been called for rejected license")
	}
}

// TestProxyAuth_LicenseKey_VerifierError: verifier returns an error
// (e.g. signature failure) → 500 with auth_error type.
func TestProxyAuth_LicenseKey_VerifierError(t *testing.T) {
	verifier := stubLicenseVerifier(false, "", errStub("forged signature"))
	next := &passthroughHandler{}
	mw := ProxyAuthMiddleware(nil, ProxyAuthRequired, verifier)
	handler := mw(next)

	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("Authorization", "Bearer SY-forged")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if next.called {
		t.Fatal("next should not be called on verifier error")
	}
}

// TestProxyAuth_LicenseKey_NoVerifier: SY- key but verifier is nil
// (env var unset). Falls through to the "non-Stockyard key, pass
// through" path. In Required mode and with the SY- prefix not
// matching sk-sy-, we hit the normal "API key required" branch
// because the key is unknown to the auth chain.
func TestProxyAuth_LicenseKey_NoVerifier(t *testing.T) {
	// nil verifier (real config when STOCKYARD_DESKTOP_PRIVATE_KEY unset)
	mw := ProxyAuthMiddleware(nil, ProxyAuthRequired, nil)
	next := &passthroughHandler{}
	handler := mw(next)

	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("Authorization", "Bearer SY-some-license")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// SY- without verifier and not sk-sy-: it's a non-Stockyard key.
	// In ProxyAuthRequired with a non-empty key, current behavior is
	// to pass through (caller is expected to BYOK). The license-key
	// branch only fires when verifier is wired.
	if !next.called {
		t.Fatal("expected pass-through when verifier is nil and key is non-empty")
	}
	if next.tier != "" {
		t.Errorf("expected no tier in context, got %q", next.tier)
	}
}

// TestProxyAuth_LicenseKey_OnlyForV1: a SY- key on a non-/v1/ route
// is ignored — middleware is scoped to proxy endpoints.
func TestProxyAuth_LicenseKey_OnlyForV1(t *testing.T) {
	verifier := stubLicenseVerifier(true, "cloud-multi", nil)
	next := &passthroughHandler{}
	mw := ProxyAuthMiddleware(nil, ProxyAuthRequired, verifier)
	handler := mw(next)

	// Hit a non-/v1/ path with a SY- bearer. Should pass through
	// without consulting the verifier or injecting tier.
	r := httptest.NewRequest("GET", "/api/something", nil)
	r.Header.Set("Authorization", "Bearer SY-irrelevant")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !next.called {
		t.Fatal("non-/v1/ should always pass through")
	}
	if next.tier != "" {
		t.Errorf("non-/v1/ should not inject tier, got %q", next.tier)
	}
}

// TestLicenseTierContext_RoundTrip: WithLicenseTier injects, then
// LicenseTierFromContext extracts. Empty string default when not set.
func TestLicenseTierContext_RoundTrip(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if got := LicenseTierFromContext(r.Context()); got != "" {
		t.Errorf("default should be empty, got %q", got)
	}
	ctx := WithLicenseTier(r.Context(), "cloud-multi")
	if got := LicenseTierFromContext(ctx); got != "cloud-multi" {
		t.Errorf("got %q, want cloud-multi", got)
	}
}

// errStub is a tiny error type for test verifier errors so we don't
// pull in fmt or errors-package boilerplate.
type errStub string

func (e errStub) Error() string { return string(e) }
