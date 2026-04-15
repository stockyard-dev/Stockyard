# site/desktop/ — desktop release assets

Files in this directory become public at `https://stockyard.dev/desktop/…`
via handlers in `internal/site/site.go`. Currently wired:

- `GET /desktop/updates.json`          → `updates.json`          (auto-update manifest)
- `GET /desktop/updates.json.sig`      → `updates.json.sig`      (Ed25519 signature)
- `GET /desktop/tools-index.json`      → `tools-index.json`      (tool catalog: per-platform URLs + sha256)
- `GET /desktop/tools-index.json.sig`  → `tools-index.json.sig`  (Ed25519 signature)

All four 404 until their assets land here. Clients handle that
cleanly — auto-update stays off, and tooldl falls back to the
bundled-tools-only path (which the installer populates anyway).

## Cutting an app release (updates.json)

Canonical docs: `stockyard-desktop/docs/AUTO-UPDATE.md`. Short form:

1. In `stockyard-desktop`, build the per-platform binaries.
2. Generate `updates.json` (schema=1, per-platform URL + sha256).
3. `stockyard-signer sign -key <priv.key> updates.json`
4. Copy `updates.json` and `updates.json.sig` into this directory.
5. `make site-sync` → commit → push. Railway auto-deploys.

## Cutting a tools-index (tools-index.json)

Canonical docs: `stockyard-desktop/docs/TOOLS-INDEX-FORMAT.md` and
`stockyard-desktop/docs/ADR-001-TOOL-DISTRIBUTION.md`. Short form:

1. In `stockyard-desktop`:
   `GITHUB_TOKEN=<pat> go run ./cmd/release-prep -out release/dist`
2. `stockyard-signer sign -key <priv.key> release/dist/tools-index.json`
3. Copy `tools-index.json` and `tools-index.json.sig` into this directory.
4. `make site-sync` → commit → push. Railway auto-deploys.

The two indexes are independent: you can re-generate tools-index.json
to pick up new tool releases without touching updates.json, and vice
versa. Both are signed by the same operator-held key (the one whose
public half lives at `stockyard-desktop/internal/updater/verify.go:
ReleasePublicKeyHex`).

## Notes

- The actual release binaries (`stockyard-<goos>-<goarch>`,
  `<tool>-<goos>-<goarch>`) do NOT live here. Host them on GitHub
  Releases (or another CDN) and reference by URL in the manifests.
  This dir is just the signed JSON bodies.
- Both manifests are signed over their exact served bytes.
  Re-serialize ≠ same signature. Never hand-edit after signing; if
  you must edit, re-sign.
