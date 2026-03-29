# Per-Team API Key Isolation — Implementation Plan

## Problem

Stockyard has users and API keys, but no way to group keys by team/project for isolated logs, metrics, and spend tracking. A company running Stockyard wants to give their frontend team and backend team separate keys with separate visibility.

## Codebase Survey Findings

### What exists
- `users` table + `api_keys` table (per-user, no team concept)
- `team_members` table in `internal/apps/team/` (flat member list with roles, invites — NOT connected to API keys)
- `requests` table has `user_id` + `project` columns
- `observe_traces` has NO `user_id` or `team_id` column
- Auth middleware validates `sk-sy-` keys → injects `User` + `APIKey` into context
- `provider.Request` has `UserID`, `Project` fields

### Critical gap
Auth middleware injects user into context, but **proxy handler never reads it**. `req.UserID` only gets set from `X-User-Id` header (requires `STOCKYARD_TRUST_PROXY`). So even with auth enabled, requests aren't attributed to the authenticated user in logs/traces. This must be fixed as part of team isolation.

## Design

### Concept: "Team" = an API key namespace
- A team is a named group that owns API keys
- Keys belong to either a user (existing) or a team (new)
- Requests made with a team key are tagged with `team_id` in logs/traces/spend
- Teams are created/managed via admin API
- No full RBAC — admin key manages everything (matches existing pattern)

### Data Model Changes

**1. New `teams` table:**
```sql
CREATE TABLE IF NOT EXISTS teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    created_by INTEGER REFERENCES users(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**2. Add `team_id` to `api_keys`:**
```sql
ALTER TABLE api_keys ADD COLUMN team_id INTEGER REFERENCES teams(id);
CREATE INDEX IF NOT EXISTS idx_api_keys_team ON api_keys(team_id);
```
- Nullable — existing user keys keep working unchanged
- Key has team_id OR user_id (team keys still require a user_id for ownership)

**3. Add `team_id` to `requests`:**
```sql
ALTER TABLE requests ADD COLUMN team_id TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_requests_team ON requests(team_id);
```

**4. Add `user_id` + `team_id` to `observe_traces`:**
```sql
ALTER TABLE observe_traces ADD COLUMN user_id TEXT DEFAULT '';
ALTER TABLE observe_traces ADD COLUMN team_id TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_traces_user ON observe_traces(user_id);
CREATE INDEX IF NOT EXISTS idx_traces_team ON observe_traces(team_id);
```

**5. New `team_spend_rollups` table:**
```sql
CREATE TABLE IF NOT EXISTS team_spend_rollups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    team_id TEXT NOT NULL,
    date TEXT NOT NULL,
    total_cost REAL DEFAULT 0,
    total_requests INTEGER DEFAULT 0,
    total_tokens_in INTEGER DEFAULT 0,
    total_tokens_out INTEGER DEFAULT 0,
    UNIQUE(team_id, date)
);
CREATE INDEX IF NOT EXISTS idx_team_spend ON team_spend_rollups(team_id);
```

### Code Changes

**File 1: `internal/auth/auth.go`**
- Add `Team` struct
- Add team CRUD methods on Store: `CreateTeam`, `GetTeam`, `ListTeams`, `DeleteTeam`
- Add `GenerateTeamKey(teamID, userID, name)` — creates key with team_id set
- Add `ListTeamKeys(teamID)`, `RevokeTeamKey(teamID, keyID)`
- Modify `ValidateKey` to also return team info when key has team_id
- Add `TeamID` field to `APIKey` struct
- Register team API routes: `/api/teams`, `/api/teams/{id}/keys`

**File 2: `internal/auth/middleware.go`**
- Add `Team` context key + `WithTeam`/`TeamFromContext` helpers
- Update `ProxyAuthMiddleware`: when validated key has team_id, inject team into context
- Add `TeamID` to provider.Request struct

**File 3: `internal/provider/provider.go`**
- Add `TeamID string` field to Request struct

**File 4: `internal/proxy/handler.go`**
- After auth middleware runs, bridge context → request fields:
  - If `UserFromContext` returns user, set `req.UserID = fmt.Sprintf("%d", user.ID)`
  - If `TeamFromContext` returns team, set `req.TeamID = fmt.Sprintf("%d", team.ID)`
- This fixes the existing gap where authenticated users aren't attributed

**File 5: `internal/engine/hooks.go`**
- Pass `req.UserID` and `req.TeamID` to `recordObserveTrace`
- Write them to `observe_traces` table

**File 6: `internal/storage/requests.go`**
- Add `TeamID` field to `RequestLog`
- Include in INSERT/SELECT queries

**File 7: `internal/storage/spend.go`**
- Add `RecordTeamSpend` function (mirrors `RecordUserSpend`)

**File 8: `internal/storage/migrations.go`**
- Add ALTERs for existing tables (team_id columns)
- Add teams table, team_spend_rollups table

**File 9: `internal/features/logging.go`**
- Bridge team context into RequestLog.TeamID before insert

**File 10: `internal/api/routes.go`**
- Add team-filtered log endpoint: `GET /api/teams/{id}/logs`
- Add team spend endpoint: `GET /api/teams/{id}/spend`

**File 11: `internal/dashboard/src/10-settings.js` (or new `12-teams.js`)**
- Team management tab: create/list/delete teams
- Per-team key management: generate, list, revoke
- Per-team usage stats

### API Shape

```
# Team CRUD (admin key required)
POST   /api/teams                    { name, description }
GET    /api/teams                    → { teams: [...], count }
GET    /api/teams/{id}               → { team, keys: [...] }
PUT    /api/teams/{id}               { name?, description? }
DELETE /api/teams/{id}               → { status: "deleted" }

# Team key management (admin key required)
POST   /api/teams/{id}/keys          { name }  → { key, key_prefix, ... }
GET    /api/teams/{id}/keys          → { keys: [...], count }
DELETE /api/teams/{id}/keys/{keyId}  → { status: "revoked" }
POST   /api/teams/{id}/keys/{keyId}/rotate → { new_key }

# Team observability (admin key required)
GET    /api/teams/{id}/spend         → { total, this_month, today }
GET    /api/teams/{id}/logs          → { logs: [...], total }
```

### Request Flow After Changes

```
1. Client sends: Authorization: Bearer sk-sy-XXXXX
2. ProxyAuthMiddleware validates key → gets (User, APIKey)
3. If APIKey.TeamID != 0, load Team → WithTeam(ctx, team)
4. Proxy handler bridges context → req.UserID, req.TeamID
5. Request executes through middleware chain
6. hooks.go records trace with user_id + team_id
7. logging.go records request with team_id
8. spend tracking rolls up per-team
```

### Edge Cases

1. **Deleting a team with active keys**: Revoke all keys first, then delete team. API enforces this.
2. **Key belongs to team AND user**: The user_id is the key creator/owner. team_id is the namespace. Both are recorded.
3. **Existing keys**: team_id = NULL, everything works as before.
4. **Team slug collision**: Slugify name, enforce uniqueness.
5. **Cache invalidation**: Team key validation goes through same ValidateKey path — cache works unchanged since we just add team_id to the returned APIKey.

### Build Order

1. Schema changes (migrations.go + auth.go schema)
2. Team CRUD on Store
3. APIKey.TeamID + ValidateKey changes
4. Context helpers (WithTeam/TeamFromContext)
5. Middleware updates
6. Proxy handler bridging
7. Hooks + logging + spend recording
8. API routes for teams
9. Dashboard UI
10. Build + push + verify

### Definition of Done

- [ ] `POST /api/teams` creates a team, returns it
- [ ] `POST /api/teams/{id}/keys` generates a team-scoped key
- [ ] Request with team key → `requests.team_id` populated
- [ ] Request with team key → `observe_traces.team_id` populated  
- [ ] `GET /api/teams/{id}/spend` returns team-scoped spend
- [ ] `GET /api/teams/{id}/logs` returns team-scoped logs
- [ ] Dashboard shows teams tab with key management + usage
- [ ] Existing user keys still work unchanged (no team_id)
- [ ] Build passes with CGO_ENABLED=0
- [ ] Live deploy succeeds
