// Package apiserver — Cloud backend schema (v5).
//
// Adds the tables that back the Cloud Single / Cloud Multi desktop tiers:
//   cloud_accounts         — one row per paying Cloud customer (separate
//                            from cloud_tenants which was for the legacy
//                            LLM-proxy product).
//   cloud_magic_tokens     — one-shot email login tokens, 15-minute TTL.
//   cloud_sessions         — browser/desktop session cookies after magic-
//                            link exchange, 30-day TTL.
//   cloud_sites            — per-account named sites (Cloud Multi creates
//                            >1; Cloud Single is locked to 1).
//   cloud_backup_blobs     — metadata for each uploaded encrypted blob.
//                            Body storage is pluggable (see blobstore.go).
//
// NOT covered here: real-time sync, conflict resolution, restore UX.
// Those land in Step 8 of ROADMAP.md. This skeleton gets Cloud
// customers a working account + backup-upload path on launch.
package apiserver

// apiMigrationV5Cloud adds the Cloud backend tables.
const apiMigrationV5Cloud = `
-- Cloud accounts: one per paying Cloud customer. Keyed by email; the
-- desktop license file carries the customer's email, so the desktop
-- app can resolve which account to log into from the license payload
-- alone.
CREATE TABLE IF NOT EXISTS cloud_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT UNIQUE NOT NULL,
    tier TEXT NOT NULL,                      -- 'cloud-single' or 'cloud-multi'
    stripe_customer_id TEXT DEFAULT '',
    stripe_subscription_id TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',   -- 'active', 'past_due', 'canceled'
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_cloud_accounts_email ON cloud_accounts(email);
CREATE INDEX IF NOT EXISTS idx_cloud_accounts_stripe_customer ON cloud_accounts(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_cloud_accounts_stripe_sub ON cloud_accounts(stripe_subscription_id);

-- Magic-link login tokens. Short-lived, single-use. We store the
-- SHA-256 hash (not the token itself) so a DB leak doesn't grant
-- login access. The token sent via email is the pre-hash string.
CREATE TABLE IF NOT EXISTS cloud_magic_tokens (
    token_hash TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    consumed_at TEXT DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_cloud_magic_email ON cloud_magic_tokens(email);
CREATE INDEX IF NOT EXISTS idx_cloud_magic_expires ON cloud_magic_tokens(expires_at);

-- Cloud sessions: long-lived auth token stored client-side as a
-- cookie. Same hash-not-token pattern as magic tokens.
CREATE TABLE IF NOT EXISTS cloud_sessions (
    token_hash TEXT PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES cloud_accounts(id) ON DELETE CASCADE,
    created_at TEXT DEFAULT (datetime('now')),
    last_seen_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    user_agent TEXT DEFAULT '',
    revoked_at TEXT DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_cloud_sessions_account ON cloud_sessions(account_id);
CREATE INDEX IF NOT EXISTS idx_cloud_sessions_expires ON cloud_sessions(expires_at);

-- Cloud sites: named location namespaces owned by an account.
-- Cloud Single accounts have exactly one row here (enforced at write
-- time in handlers). Cloud Multi accounts can create unlimited.
CREATE TABLE IF NOT EXISTS cloud_sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES cloud_accounts(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,                     -- URL-safe identifier, unique per account
    display_name TEXT NOT NULL,             -- 'Downtown Studio'
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(account_id, slug)
);
CREATE INDEX IF NOT EXISTS idx_cloud_sites_account ON cloud_sites(account_id);

-- Cloud backup blobs: one row per encrypted snapshot uploaded from
-- the desktop app. Body is stored out-of-band (see BlobStore
-- interface); this table holds only metadata + a reference (key) the
-- BlobStore can resolve back to the bytes.
--
-- A single snapshot may span multiple tools; the desktop sends them
-- in one blob (tarball of per-tool SQLite databases, encrypted with
-- the account's key). We don't split per tool here — keeps restore
-- atomic and conflict logic simple for v0.
CREATE TABLE IF NOT EXISTS cloud_backup_blobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES cloud_accounts(id) ON DELETE CASCADE,
    site_id INTEGER REFERENCES cloud_sites(id) ON DELETE CASCADE,
    blob_key TEXT NOT NULL,                 -- opaque storage key (filename on disk, S3 key, etc.)
    size_bytes INTEGER NOT NULL,
    sha256_hex TEXT NOT NULL,               -- content hash for dedup + integrity
    client_version TEXT DEFAULT '',         -- desktop app version that uploaded this
    uploaded_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_cloud_blobs_account ON cloud_backup_blobs(account_id);
CREATE INDEX IF NOT EXISTS idx_cloud_blobs_site ON cloud_backup_blobs(site_id);
CREATE INDEX IF NOT EXISTS idx_cloud_blobs_uploaded ON cloud_backup_blobs(uploaded_at);
`
