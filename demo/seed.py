#!/usr/bin/env python3
"""Seed demo instance with realistic data for 5 Stockyard tools."""

import json
import time
import random
import urllib.request

def post(url, data):
    req = urllib.request.Request(url, json.dumps(data).encode(), {"Content-Type": "application/json"}, method="POST")
    try:
        return json.loads(urllib.request.urlopen(req, timeout=5).read())
    except Exception as e:
        print(f"  WARN: {url} — {e}")
        return {}

def get(url):
    try:
        return json.loads(urllib.request.urlopen(url, timeout=5).read())
    except:
        return {}

def seed_bounty(port=9320):
    """Bounty — bug tracker with projects, issues, milestones, comments."""
    print("Seeding Bounty...")
    base = f"http://localhost:{port}"

    proj = post(f"{base}/api/projects", {"name": "Stockyard Platform", "description": "Core platform development"})
    pid = proj.get("id", "")
    if not pid:
        print("  Bounty not responding, skipping")
        return

    proj2 = post(f"{base}/api/projects", {"name": "Marketing Site", "description": "stockyard.dev website and landing pages"})
    pid2 = proj2.get("id", "")

    # Milestones
    post(f"{base}/api/milestones", {"project_id": pid, "title": "v2.0 Release", "description": "Major platform update", "due_date": "2026-04-15"})
    post(f"{base}/api/milestones", {"project_id": pid, "title": "v2.1 Polish", "description": "Bug fixes and performance", "due_date": "2026-05-01"})

    issues = [
        {"project_id": pid, "title": "WebSocket connections drop after 30min idle", "body": "Users report WS connections dying silently. Need keepalive ping or reconnect logic.", "priority": "high", "labels": ["bug", "networking"], "assignee": "michael"},
        {"project_id": pid, "title": "Add rate limiting per API key", "body": "Currently no per-key rate limits. Need sliding window counter with configurable thresholds.", "priority": "high", "labels": ["feature", "security"], "assignee": "michael"},
        {"project_id": pid, "title": "SQLite WAL checkpoint runs during peak hours", "body": "Checkpoint causes latency spikes. Should schedule during low-traffic windows.", "priority": "medium", "labels": ["bug", "performance"]},
        {"project_id": pid, "title": "Dashboard dark mode contrast too low", "body": "Several users reported the cream-on-dark text is hard to read on cheap monitors.", "priority": "low", "labels": ["ui", "accessibility"]},
        {"project_id": pid, "title": "Support Ed25519 SSH keys for Git operations", "body": "Currently only RSA keys work. Ed25519 is the modern default.", "priority": "medium", "labels": ["feature"]},
        {"project_id": pid, "title": "Memory leak in health poller goroutine", "body": "After 72h uptime, Hub process grows to 500MB. Likely unclosed HTTP response bodies in the poller.", "priority": "critical", "labels": ["bug", "memory"], "assignee": "michael"},
        {"project_id": pid, "title": "Add CSV export for issues", "body": "Users want to export issue lists for reporting. CSV with filters applied.", "priority": "low", "labels": ["feature"]},
        {"project_id": pid, "title": "Webhook delivery retry with exponential backoff", "body": "Failed webhook deliveries currently retry 3x with fixed 5s delay. Need exponential backoff.", "priority": "medium", "labels": ["feature", "reliability"]},
        {"project_id": pid2, "title": "Update comparison pages for Q2 pricing changes", "body": "Sentry and LaunchDarkly both changed pricing. Need to update 8 comparison pages.", "priority": "medium", "labels": ["content"], "assignee": "michael"},
        {"project_id": pid2, "title": "Add testimonials section to homepage", "body": "We have 3 user quotes now. Add a testimonials strip below the catalog preview.", "priority": "low", "labels": ["content", "conversion"]},
        {"project_id": pid2, "title": "Blog post: self-hosting cost analysis", "body": "Deep dive into VPS costs vs SaaS costs for a typical 5-person team.", "priority": "medium", "labels": ["content"]},
        {"project_id": pid2, "title": "Fix mobile nav overlay z-index on Safari", "body": "Nav dropdown renders behind hero section on iOS Safari 17.", "priority": "high", "labels": ["bug", "mobile"]},
    ]

    created_ids = []
    for issue in issues:
        r = post(f"{base}/api/issues", issue)
        if r.get("id"):
            created_ids.append(r["id"])

    # Set some statuses
    if len(created_ids) >= 6:
        post(f"{base}/api/issues/{created_ids[0]}/status", {"status": "in_progress"})
        post(f"{base}/api/issues/{created_ids[5]}/status", {"status": "in_progress"})
        post(f"{base}/api/issues/{created_ids[3]}/close", {})
        post(f"{base}/api/issues/{created_ids[6]}/close", {})

    # Comments
    comments = [
        (0, "michael", "Reproduced this locally. The issue is in the WS upgrade handler not setting a read deadline."),
        (0, "michael", "Fix is in PR #47. Sets a 45s read deadline with 30s ping interval."),
        (5, "michael", "Found the leak — response bodies from failed health checks weren't being closed. One-line fix."),
        (2, "michael", "This is a SQLite config issue. Setting wal_autocheckpoint to a higher value should fix it."),
    ]
    for idx, author, body in comments:
        if idx < len(created_ids):
            post(f"{base}/api/issues/{created_ids[idx]}/comments", {"author": author, "body": body})

    print(f"  ✓ {len(created_ids)} issues, 2 projects, 2 milestones, {len(comments)} comments")


def seed_corral(port=8760):
    """Corral — webhook capture with endpoints and events."""
    print("Seeding Corral...")
    base = f"http://localhost:{port}"

    health = get(f"{base}/api/health")
    if not health.get("status"):
        # Try /health instead
        health = get(f"{base}/health")
        if not health.get("status"):
            print("  Corral not responding, skipping")
            return

    # Create endpoints
    endpoints = [
        {"name": "stripe-webhooks", "description": "Stripe payment events"},
        {"name": "github-hooks", "description": "GitHub push and PR events"},
        {"name": "shopify-orders", "description": "Shopify order notifications"},
    ]
    for ep in endpoints:
        post(f"{base}/api/endpoints", ep)

    print(f"  ✓ 3 webhook endpoints created")


def seed_paddock(port=8750):
    """Paddock — status page with components and incidents."""
    print("Seeding Paddock...")
    base = f"http://localhost:{port}"

    health = get(f"{base}/api/health")
    if not health.get("status"):
        health = get(f"{base}/health")
        if not health.get("status"):
            print("  Paddock not responding, skipping")
            return

    components = [
        {"name": "API", "description": "Core REST API", "status": "operational"},
        {"name": "Dashboard", "description": "Web dashboard", "status": "operational"},
        {"name": "Webhook Delivery", "description": "Outbound webhook delivery", "status": "degraded_performance"},
        {"name": "Database", "description": "Primary database cluster", "status": "operational"},
    ]
    for c in components:
        post(f"{base}/api/components", c)

    incidents = [
        {"title": "Elevated webhook delivery latency", "status": "investigating", "body": "We are seeing increased latency on outbound webhook deliveries. Investigating the root cause."},
        {"title": "Dashboard login errors resolved", "status": "resolved", "body": "A configuration change caused intermittent 502 errors on the login page. This has been fixed."},
    ]
    for inc in incidents:
        post(f"{base}/api/incidents", inc)

    print(f"  ✓ 4 components, 2 incidents")


def seed_seismograph(port=9680):
    """Seismograph — error tracking with error groups."""
    print("Seeding Seismograph...")
    base = f"http://localhost:{port}"

    health = get(f"{base}/api/health")
    if not health.get("status"):
        health = get(f"{base}/health")
        if not health.get("status"):
            print("  Seismograph not responding, skipping")
            return

    errors = [
        {"name": "TypeError: Cannot read properties of undefined", "message": "Cannot read properties of undefined (reading 'map')", "level": "error", "source": "frontend", "stack": "at renderList (app.js:142)\nat Dashboard.render (app.js:89)\nat React.createElement (react.js:1024)"},
        {"name": "ConnectionRefusedError", "message": "Connection refused: localhost:5432", "level": "fatal", "source": "api-server", "stack": "at PostgresPool.connect (pool.js:45)\nat QueryRunner.execute (runner.js:112)"},
        {"name": "RateLimitExceeded", "message": "Rate limit exceeded for key sk_live_xxx", "level": "warning", "source": "api-gateway", "stack": "at RateLimiter.check (limiter.go:78)\nat Handler.ServeHTTP (handler.go:34)"},
        {"name": "TimeoutError: Request timed out", "message": "Request to /api/v2/completions timed out after 30000ms", "level": "error", "source": "llm-proxy", "stack": "at HttpClient.request (client.go:156)\nat Proxy.forward (proxy.go:89)"},
        {"name": "SQLITE_BUSY", "message": "database is locked (5)", "level": "warning", "source": "data-service", "stack": "at DB.Exec (store.go:234)\nat Handler.createRecord (handler.go:67)"},
    ]
    for err in errors:
        post(f"{base}/api/errors", err)

    print(f"  ✓ 5 error groups")


def seed_saltlick(port=8730):
    """Salt Lick — feature flags."""
    print("Seeding Salt Lick...")
    base = f"http://localhost:{port}"

    health = get(f"{base}/api/health")
    if not health.get("status"):
        health = get(f"{base}/health")
        if not health.get("status"):
            print("  Salt Lick not responding, skipping")
            return

    flags = [
        {"key": "new-dashboard", "name": "New Dashboard UI", "description": "Redesigned dashboard with dark mode", "enabled": True, "rollout_percentage": 25},
        {"key": "websocket-streaming", "name": "WebSocket Streaming", "description": "Real-time event streaming via WebSocket", "enabled": True, "rollout_percentage": 100},
        {"key": "csv-export", "name": "CSV Export", "description": "Export data as CSV from any table view", "enabled": False, "rollout_percentage": 0},
        {"key": "ai-summarize", "name": "AI Summarize", "description": "LLM-powered issue summarization", "enabled": True, "rollout_percentage": 10},
        {"key": "team-billing", "name": "Team Billing", "description": "Multi-seat billing and team management", "enabled": False, "rollout_percentage": 0},
        {"key": "api-v2", "name": "API v2 Endpoints", "description": "New API version with pagination and filtering", "enabled": True, "rollout_percentage": 50},
    ]
    for flag in flags:
        post(f"{base}/api/flags", flag)

    print(f"  ✓ 6 feature flags")


if __name__ == "__main__":
    print("\n  Stockyard Demo — Seeding realistic data\n")
    seed_bounty()
    seed_corral()
    seed_paddock()
    seed_seismograph()
    seed_saltlick()
    print("\n  Done. Demo data seeded.\n")
