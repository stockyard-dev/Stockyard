# Stockyard Launch Checklist

Last updated: end of session at commit `3e5cb55`. This is the single-screen
go/no-go reference. Skim before pushing the launch button.

---

## 🔴 BLOCKERS that need manual action before launch

### 1. Cloudflare HTTP → HTTPS redirect is broken
**Symptom:** `curl -I http://stockyard.dev/` returns `HTTP/1.1 403 Forbidden`,
NOT a 301/302 to HTTPS. Cloudflare is not proxying HTTP traffic.
**Impact:** Browsers that don't auto-upgrade hit a 403. Any link sent in a
chat/email without explicit `https://` may fail. Per prior user memories,
this previously blocked all GSC indexing.
**Fix (manual, ~2 minutes):** In the Cloudflare dashboard for stockyard.dev:
  1. DNS tab — confirm the A/CNAME record for `stockyard.dev` and `www`
     has the orange cloud icon (proxied), not gray (DNS-only).
  2. SSL/TLS → Edge Certificates — toggle "Always Use HTTPS" to ON.
  3. Verify with `curl -I http://stockyard.dev/` → expect `301` with
     `location: https://stockyard.dev/`.

---

## ✅ Functional verification (all green at end of session)

| Path | Expected | Verified live |
|---|---|---|
| `https://stockyard.dev/` | 200, homepage with Windows hint × 2 | ✅ |
| `https://stockyard.dev/pricing/` | 200, pricing page renders | ✅ |
| `https://stockyard.dev/dossier/` | 200, tool product page | ✅ |
| `https://stockyard.dev/sitemap.xml` | 200, sitemap accessible | ✅ |
| `POST /api/checkout` `{"plan":"individual","interval":"monthly"}` | `{"url":"https://checkout.stripe.com/c/pay/cs_live_..."}` | ✅ |
| `POST /api/checkout` `{"plan":"dossier-pro","interval":"monthly"}` | `{"url":"cs_live_..."}` | ✅ |
| `POST /api/checkout` `{"bundle":"brewery"}` | `{"url":"cs_live_..."}` | ✅ |
| `POST /api/recommend` with novel description | JSON with `tools[]`, `slug`, `cached:false` | ✅ (8 tools returned) |
| Static assets (`/site-assets/...`) | `cf-cache-status: HIT`, edge-cached | ✅ |
| Apibridge keypair load | Persistent platform keypair (no fallback warning) | ✅ |

## 🔧 Production environment status

**Railway service:** `becc02a7-e7e8-4185-a965-37b1967a6862` (project `cab0be2e-...`)
**Last deploy:** commit `3e5cb55226`, status SUCCESS, single replica, /data volume mounted
**Stripe:** LIVE keys (`sk_liv*`). Real card charges. Test card 4242 will NOT work.
**License keypairs:**
- `STOCKYARD_PUBLIC_KEY` / `STOCKYARD_SIGNING_KEY` — set, persistent (loads via the multi-base64-variant decoder shipped in `3e5cb55`)
- `STOCKYARD_TOOLS_PRIVATE_KEY` — set, hex-encoded 64-byte ed25519
**Mailer:** `RESEND_API_KEY` set, real email delivery
**Webhook:** `STRIPE_WEBHOOK_SECRET` set, signature verification active
**Per-tool prices configured:** brand, bundle, complete, corral, fence, gate, trough (~7 tools)
**Per-tier prices configured:** stockyard individual / pro / team / enterprise × monthly / annual

## 📋 Smoke-test script (run before pushing the launch button)

```bash
# 1. Apibridge healthy
curl -s https://stockyard.dev/health
# expect: {"status":"ok",...}

# 2. Checkout works (suite tier)
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"plan":"individual","interval":"monthly"}' \
  https://stockyard.dev/api/checkout | head -c 200
# expect: {"url":"https://checkout.stripe.com/c/pay/cs_live_..."}

# 3. Recommend works (novel query)
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"description":"$(date +%s) launch smoke test"}' \
  https://stockyard.dev/api/recommend | head -c 300
# expect: JSON with "tools": [...], cached:false

# 4. HTTPS redirect (currently BROKEN — see Blockers)
curl -sI http://stockyard.dev/ | head -3
# expect: HTTP/1.1 301 ... location: https://stockyard.dev/
# CURRENT: HTTP/1.1 403 — needs Cloudflare dashboard fix

# 5. Sitemap accessible
curl -sI https://stockyard.dev/sitemap.xml | head -1
# expect: HTTP/2 200
```

## 🎯 Launch-day rollback levers

| If this breaks | Lever |
|---|---|
| Checkout returns 404 again | Check apibridge logs for keypair errors. The fallback in `654982f` keeps things mounted even if keys break. |
| Checkout returns Stripe coupon error | The retry-without-coupon shim in `973192e` handles this gracefully. If still failing, unset `STRIPE_FIRST_MONTH_COUPON` in Railway. |
| Recommend returns 503 for everything | Semaphore is full or Anthropic is down. Bump `MaxConcurrentLLMCalls` from 5 → 10 in `internal/site/recommend.go` and redeploy. |
| Cache too aggressive | Cloudflare is `cf-cache-status: DYNAMIC` for HTML — fixes propagate immediately. No purge needed. |
| Need to purge anyway | Cloudflare dashboard → Caching → Configuration → Purge Cache button. Or `POST https://api.cloudflare.com/client/v4/zones/{zone_id}/purge_cache` with `{"purge_everything":true}`. |
| Single replica overloaded | Bump `numReplicas` in railway.toml (currently 1) — Railway Pro plan supports it. |

## 🚀 Tier 3 close-out

| Item | Status |
|---|---|
| Stripe checkout reachable in production | ✅ |
| Stripe webhook handler traced and verified | ✅ |
| Persistent platform keypair (not ephemeral) | ✅ |
| Windows SmartScreen UX hint | ✅ |
| Recommend concurrency cap | ✅ |
| Cloudflare cache audit | ✅ |
| Cloudflare HTTPS redirect | 🔴 needs manual dashboard fix |
| `/launch.mp4` Range header strip | ⏳ deferred (cosmetic) |
| Real test-mode end-to-end Stripe walkthrough | ⏳ optional (would need parallel Railway env with sk_test keys) |

## 🧯 The one thing left

**Toggle "Always Use HTTPS" in the Cloudflare dashboard.** That's it. Everything
else works. Once HTTPS is fixed at the edge, the launch is technically green
across every dimension that has been verified.
