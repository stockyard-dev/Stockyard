#!/usr/bin/env bash
set -euo pipefail

# Stockyard Stripe Product Setup
# Creates 3 products (Pro, Team, Enterprise) with 6 prices (monthly + annual each)
# Free tier has no Stripe product — it's self-hosted with no checkout.
#
# Usage:
#   export STRIPE_SECRET_KEY=sk_live_...
#   bash setup-stripe-products.sh
#
# After running, set the printed env vars in Railway.

if [ -z "${STRIPE_SECRET_KEY:-}" ]; then
  echo "ERROR: Set STRIPE_SECRET_KEY first"
  echo "  export STRIPE_SECRET_KEY=sk_live_..."
  exit 1
fi

API="https://api.stripe.com/v1"
AUTH="-u ${STRIPE_SECRET_KEY}:"

echo "=== Creating Stockyard Stripe Products ==="
echo ""

# --- Pro ($29/mo, $290/yr) ---
echo "Creating Pro product..."
PRO_PROD=$(curl -s $AUTH "$API/products" \
  -d "name=Stockyard Pro" \
  -d "description=Cloud-managed Stockyard. Zero ops. All 16 apps, 58 modules, auto-scaling." \
  -d "metadata[tier]=pro" \
  -d "metadata[product]=stockyard" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Product: $PRO_PROD"

echo "  Creating Pro monthly price..."
PRO_MONTHLY=$(curl -s $AUTH "$API/prices" \
  -d "product=$PRO_PROD" \
  -d "unit_amount=2900" \
  -d "currency=usd" \
  -d "recurring[interval]=month" \
  -d "metadata[tier]=pro" \
  -d "metadata[interval]=monthly" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Monthly: $PRO_MONTHLY"

echo "  Creating Pro annual price..."
PRO_ANNUAL=$(curl -s $AUTH "$API/prices" \
  -d "product=$PRO_PROD" \
  -d "unit_amount=29000" \
  -d "currency=usd" \
  -d "recurring[interval]=year" \
  -d "metadata[tier]=pro" \
  -d "metadata[interval]=annual" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Annual:  $PRO_ANNUAL"
echo ""

# --- Team ($99/mo, $990/yr) ---
echo "Creating Team product..."
TEAM_PROD=$(curl -s $AUTH "$API/products" \
  -d "name=Stockyard Team" \
  -d "description=5 seats, RBAC, compliance export, shared configs, priority support." \
  -d "metadata[tier]=team" \
  -d "metadata[product]=stockyard" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Product: $TEAM_PROD"

echo "  Creating Team monthly price..."
TEAM_MONTHLY=$(curl -s $AUTH "$API/prices" \
  -d "product=$TEAM_PROD" \
  -d "unit_amount=9900" \
  -d "currency=usd" \
  -d "recurring[interval]=month" \
  -d "metadata[tier]=team" \
  -d "metadata[interval]=monthly" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Monthly: $TEAM_MONTHLY"

echo "  Creating Team annual price..."
TEAM_ANNUAL=$(curl -s $AUTH "$API/prices" \
  -d "product=$TEAM_PROD" \
  -d "unit_amount=99000" \
  -d "currency=usd" \
  -d "recurring[interval]=year" \
  -d "metadata[tier]=team" \
  -d "metadata[interval]=annual" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Annual:  $TEAM_ANNUAL"
echo ""

# --- Enterprise ($299/mo, $2990/yr) ---
echo "Creating Enterprise product..."
ENT_PROD=$(curl -s $AUTH "$API/products" \
  -d "name=Stockyard Enterprise" \
  -d "description=Unlimited seats, SSO/SAML, Trust federation, 99.9% SLA, dedicated support." \
  -d "metadata[tier]=enterprise" \
  -d "metadata[product]=stockyard" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Product: $ENT_PROD"

echo "  Creating Enterprise monthly price..."
ENT_MONTHLY=$(curl -s $AUTH "$API/prices" \
  -d "product=$ENT_PROD" \
  -d "unit_amount=29900" \
  -d "currency=usd" \
  -d "recurring[interval]=month" \
  -d "metadata[tier]=enterprise" \
  -d "metadata[interval]=monthly" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Monthly: $ENT_MONTHLY"

echo "  Creating Enterprise annual price..."
ENT_ANNUAL=$(curl -s $AUTH "$API/prices" \
  -d "product=$ENT_PROD" \
  -d "unit_amount=299000" \
  -d "currency=usd" \
  -d "recurring[interval]=year" \
  -d "metadata[tier]=enterprise" \
  -d "metadata[interval]=annual" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Annual:  $ENT_ANNUAL"
echo ""

# --- Team additional seat ($20/seat/mo) ---
echo "Creating Team additional seat price..."
SEAT_PROD=$(curl -s $AUTH "$API/products" \
  -d "name=Stockyard Team - Additional Seat" \
  -d "description=Additional team seat beyond the 5 included with Team plan." \
  -d "metadata[tier]=team" \
  -d "metadata[addon]=seat" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

SEAT_MONTHLY=$(curl -s $AUTH "$API/prices" \
  -d "product=$SEAT_PROD" \
  -d "unit_amount=2000" \
  -d "currency=usd" \
  -d "recurring[interval]=month" \
  -d "metadata[addon]=seat" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  Seat:    $SEAT_MONTHLY"
echo ""

# --- Summary ---
echo "============================================"
echo "Set these env vars in Railway:"
echo "============================================"
echo ""
echo "STRIPE_PRICE_STOCKYARD_PRO_MONTHLY=$PRO_MONTHLY"
echo "STRIPE_PRICE_STOCKYARD_PRO_ANNUAL=$PRO_ANNUAL"
echo "STRIPE_PRICE_STOCKYARD_TEAM_MONTHLY=$TEAM_MONTHLY"
echo "STRIPE_PRICE_STOCKYARD_TEAM_ANNUAL=$TEAM_ANNUAL"
echo "STRIPE_PRICE_STOCKYARD_ENTERPRISE_MONTHLY=$ENT_MONTHLY"
echo "STRIPE_PRICE_STOCKYARD_ENTERPRISE_ANNUAL=$ENT_ANNUAL"
echo "STRIPE_PRICE_STOCKYARD_TEAM_SEAT_MONTHLY=$SEAT_MONTHLY"
echo ""
echo "Done. 4 products, 7 prices created."
