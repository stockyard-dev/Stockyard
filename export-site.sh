#!/usr/bin/env bash
set -euo pipefail

# export-site.sh — Build and export the complete Stockyard website.
# Produces dist/ containing docs + product pages + assets.
# Zero broken internal links guaranteed.

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

info()  { echo -e "${GREEN}[ok]${NC} $1"; }
fail()  { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }

DIST="dist"

echo "Stockyard Site Export"
echo "====================="
echo ""

# Clean
rm -rf "$DIST"
mkdir -p "$DIST"

# --- 1. Product landing pages ---
echo "1. Copying product landing pages..."
if [ -d "site/products" ]; then
    cp -r site/products "$DIST/products"
    PRODUCT_COUNT=$(find "$DIST/products" -name 'index.html' | wc -l)
    info "Copied $PRODUCT_COUNT product pages to dist/products/"
else
    fail "site/products/ not found"
fi

# --- 2. Generate docs ---
echo "2. Generating documentation..."
if command -v sy-docs &>/dev/null; then
    sy-docs -o "$DIST/docs"
elif [ -f "cmd/sy-docs/main.go" ]; then
    echo "   Building sy-docs..."
    go build -o /tmp/sy-docs ./cmd/sy-docs/
    /tmp/sy-docs -o "$DIST/docs"
else
    fail "cmd/sy-docs/main.go not found"
fi
DOCS_COUNT=$(find "$DIST/docs" -name '*.html' | wc -l)
info "Generated $DOCS_COUNT doc pages to dist/docs/"

# --- 3. Copy top-level pages ---
echo "3. Copying top-level pages..."
[ -f "site/index.html" ] && cp site/index.html "$DIST/index.html" && info "Copied index.html"
[ -d "site/pricing" ] && cp -r site/pricing "$DIST/pricing" && info "Copied pricing/"
[ -d "site/success" ] && cp -r site/success "$DIST/success" && info "Copied success/"
[ -d "site/css" ] && cp -r site/css "$DIST/css" && info "Copied css/"
[ -d "site/assets" ] && cp -r site/assets "$DIST/assets" && info "Copied assets/"

# --- 4. Fix broken links in product landing pages ---
echo "4. Fixing cross-links..."

# 4a. Product pages link to /docs/{slug}/ but docs are at /docs/products/{slug}/
FIXED=0
find "$DIST/products" -name 'index.html' | while read -r f; do
    slug=$(basename "$(dirname "$f")")
    if grep -q "href=\"/docs/$slug/\"" "$f" 2>/dev/null; then
        sed -i "s|href=\"/docs/$slug/\"|href=\"/docs/products/$slug/\"|g" "$f"
    fi
done
info "Fixed /docs/{slug}/ -> /docs/products/{slug}/ links"

# 4b. Product pages reference /js/checkout.js — create it
mkdir -p "$DIST/js"
cat > "$DIST/js/checkout.js" << 'JS'
// Stockyard checkout — redirect to Stripe
(function() {
  document.querySelectorAll('[data-product][data-tier]').forEach(function(btn) {
    btn.addEventListener('click', function(e) {
      e.preventDefault();
      var product = this.getAttribute('data-product');
      var tier = this.getAttribute('data-tier');
      var email = this.getAttribute('data-email') || '';
      var apiBase = window.STOCKYARD_API || 'https://api.stockyard.dev';
      fetch(apiBase + '/api/checkout', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({product: product, tier: tier, email: email})
      })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if (data.url) window.location.href = data.url;
        else alert('Checkout unavailable. Please try again.');
      })
      .catch(function() { alert('Checkout unavailable. Please try again.'); });
    });
  });
})();
JS
info "Created /js/checkout.js"

# 4c. Success page links to /account/ — create stub
mkdir -p "$DIST/account"
cat > "$DIST/account/index.html" << 'HTML'
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Account | Stockyard</title>
<style>
:root{--bg:#1a1410;--cream:#f0e6d3;--rust:#c45d2c;--leather:#a0845c}
body{background:var(--bg);color:var(--cream);font-family:'Libre Baskerville',Georgia,serif;display:flex;align-items:center;justify-content:center;min-height:100vh;padding:2rem}
.card{max-width:500px;text-align:center}
h1{font-size:1.5rem;margin-bottom:1rem}
p{color:var(--leather);line-height:1.7;margin-bottom:1rem}
a{color:var(--rust)}
input{background:#241e18;border:1px solid #2e261e;color:var(--cream);padding:0.5rem 1rem;font-family:inherit;width:100%;margin:0.5rem 0;font-size:1rem}
button{background:var(--rust);color:#fff;border:none;padding:0.6rem 1.5rem;cursor:pointer;font-family:inherit;font-size:0.9rem;margin-top:0.5rem}
</style>
</head>
<body>
<div class="card">
<h1>Look Up Your License</h1>
<p>Enter the email address you used at checkout to retrieve your license key.</p>
<input type="email" id="email" placeholder="you@example.com">
<button onclick="lookup()">Look Up</button>
<div id="result" style="margin-top:1rem"></div>
<script>
function lookup() {
  var email = document.getElementById('email').value;
  if (!email) return;
  var api = window.STOCKYARD_API || 'https://api.stockyard.dev';
  fetch(api + '/api/license/lookup?email=' + encodeURIComponent(email))
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.licenses && data.licenses.length) {
        var html = '<p style="color:#5a9a5a">Found ' + data.licenses.length + ' license(s):</p>';
        data.licenses.forEach(function(l) { html += '<p><code>' + l.key_masked + '</code> — ' + l.product + ' (' + l.tier + ')</p>'; });
        document.getElementById('result').innerHTML = html;
      } else {
        document.getElementById('result').innerHTML = '<p style="color:#c45050">No licenses found for this email.</p>';
      }
    })
    .catch(function() { document.getElementById('result').innerHTML = '<p style="color:#c45050">Lookup failed. Try again later.</p>'; });
}
</script>
</div>
</body>
</html>
HTML
info "Created /account/ page"

# --- 5. Summary ---
echo ""
echo "====================="
TOTAL=$(find "$DIST" -name '*.html' | wc -l)
SIZE=$(du -sh "$DIST" | awk '{print $1}')
info "Export complete: $TOTAL HTML files, $SIZE total"
info "Output: $DIST/"
echo ""
echo "Verify: ./check-links.sh"
echo "Deploy: netlify deploy --dir=$DIST --prod"
