// Stockyard Checkout — stockyard.dev/js/checkout.js
// Calls the sy-api backend to create a Stripe Checkout session.
(function() {
  'use strict';

  var API = window.STOCKYARD_API || 'https://api.stockyard.dev';

  window.stockyardCheckout = function(product, tier, btnEl) {
    if (!btnEl) btnEl = (typeof event !== 'undefined' && event.target) ? event.target : null;
    var originalText = btnEl ? btnEl.textContent : '';
    if (btnEl) { btnEl.textContent = 'Redirecting...'; btnEl.disabled = true; }

    fetch(API + '/api/checkout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ product: product, tier: tier })
    })
    .then(function(res) { return res.json(); })
    .then(function(data) {
      if (data.url) {
        window.location.href = data.url;
      } else {
        throw new Error(data.error || 'No checkout URL');
      }
    })
    .catch(function(e) {
      console.error('Stockyard checkout:', e);
      if (btnEl) {
        btnEl.textContent = 'Error \u2014 try again';
        btnEl.disabled = false;
        setTimeout(function() { btnEl.textContent = originalText; }, 3000);
      }
    });
  };
})();
