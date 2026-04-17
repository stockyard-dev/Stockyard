// License verifier wiring for the LLM proxy.
//
// auth.ProxyAuthMiddleware accepts a LicenseVerifier callback so it
// can validate "Authorization: Bearer SY-..." desktop license keys
// without depending on apiserver (would create an import cycle).
// This file is the wiring point — a small closure factory that
// hides the apiserver dependency from the auth package.

package engine

import (
	"github.com/stockyard-dev/stockyard/internal/apiserver"
	"github.com/stockyard-dev/stockyard/internal/auth"
)

// makeLicenseVerifier returns an auth.LicenseVerifier that validates
// SY- desktop license keys against the given private key (the same
// key apiserver uses to SIGN them — verification uses the matching
// public key derived from it).
//
// privKeyHex empty (env var unset, e.g. local dev): returns a no-op
// verifier that always rejects. License-key auth simply doesn't work
// in that environment, but sk-sy- and BYOK paths still do.
//
// Verifier returns:
//
//	ok=true, tier="..."   — valid signature, paying tier, not expired
//	ok=false, err=...     — signature failure or DB-style error
//	ok=false, err=nil     — valid signature but tier doesn't grant proxy
//	                        or license has expired (auth middleware will
//	                        map this to 401 with a clear message)
func makeLicenseVerifier(privKeyHex string) auth.LicenseVerifier {
	if privKeyHex == "" {
		return func(key string) (bool, string, error) {
			return false, "", nil
		}
	}
	return func(key string) (bool, string, error) {
		claims, err := apiserver.VerifyDesktopLicenseKey(key, privKeyHex)
		if err != nil {
			return false, "", err
		}
		if claims.IsExpired() {
			// Treated as a non-error rejection: signature was valid,
			// but the key is past its expiry. Caller maps to 401.
			return false, claims.Tier, nil
		}
		if !apiserver.LicenseTierGrantsProxy(claims.Tier) {
			return false, claims.Tier, nil
		}
		return true, claims.Tier, nil
	}
}
