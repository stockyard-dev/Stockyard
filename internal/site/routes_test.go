package site

import (
	"net/http"
	"testing"
)

// TestNoDuplicateRoutes verifies that Register() doesn't panic from duplicate
// route registration. Go 1.22's ServeMux panics on duplicate routes, which has
// caused production outages.
func TestNoDuplicateRoutes(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Register panicked — likely duplicate route registration: %v", r)
		}
	}()

	mux := http.NewServeMux()
	Register(mux, nil) // nil db is fine — we just check routes don't panic
}
