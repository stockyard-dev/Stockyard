# site/desktop/ — desktop auto-update release assets

Files in this directory become public at `https://stockyard.dev/desktop/…`
via handlers in `internal/site/site.go`. Currently wired:

- `GET /desktop/updates.json`     → `updates.json`     (release manifest)
- `GET /desktop/updates.json.sig` → `updates.json.sig` (Ed25519 signature)

Both 404 until release assets land here. The desktop client handles
that cleanly — auto-update simply stays off.

## Release cut flow

Canonical docs: `stockyard-desktop/docs/RELEASES.md`. Short form:

1. In the `stockyard-desktop` repo, build the per-platform binaries.
2. Generate `updates.json` with schema=1, per-platform URL + sha256.
3. Sign: `stockyard-signer sign -key <priv.key> updates.json`
4. Copy `updates.json` and `updates.json.sig` to `site/desktop/` here.
5. `make site-sync` (the sync rule mirrors `.json` and `.sig`).
6. Commit + push. Railway auto-deploys.

## Notes

- The binaries themselves (`stockyard-desktop-<goos>-<goarch>`) do NOT
  live here. Host them on a separate CDN / release store and reference
  them by URL in `updates.json`. This dir is just manifest + signature.
- `updates.json` is signed over its exact bytes. Re-serialize ≠ same
  signature. Never hand-edit after signing.
