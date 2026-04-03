(function(){
  // === CONVERSION EVENT TRACKING ===
  // Track install button clicks
  document.addEventListener('click',function(e){
    var a=e.target.closest('a[href*="#install"],a[href*="install.sh"]');
    if(a){
      var tool=window.location.pathname.split('/')[1]||'stockyard';
      gtag('event','generate_lead',{event_category:'install',event_label:tool});
      gtag('event','conversion',{send_to:'AW-18046975504/install'});
    }
    // Track any checkout/pricing CTA clicks on content pages
    var cta=e.target.closest('a[href="/complete/"],a[href="/pricing/"],a[href="/cloud/"]');
    if(cta){
      gtag('event','select_content',{content_type:'cta',item_id:cta.getAttribute('href'),event_label:window.location.pathname});
    }
  });

  // Track pricing page view
  if(window.location.pathname==='/pricing/'||window.location.pathname==='/pricing'){
    gtag('event','view_item_list',{item_list_name:'pricing_page'});
  }

  // Track scroll depth (25%, 50%, 75%, 100%)
  var scrollMarks={25:false,50:false,75:false,100:false};
  window.addEventListener('scroll',function(){
    var pct=Math.round(100*window.scrollY/(document.body.scrollHeight-window.innerHeight));
    [25,50,75,100].forEach(function(m){
      if(pct>=m&&!scrollMarks[m]){
        scrollMarks[m]=true;
        gtag('event','scroll',{percent_scrolled:m,page_path:window.location.pathname});
      }
    });
  },{passive:true});

  // === STICKY CTA BAR ===
  var skip=['/pricing/','/billing/','/complete/','/cloud/','/affiliate/'];
  var path=window.location.pathname;
  if(path==='/'||path==='/index.html')return;
  for(var i=0;i<skip.length;i++){if(path.indexOf(skip[i])===0)return;}

  var bar=document.createElement('div');
  bar.id='sticky-cta';
  bar.innerHTML='<div style="display:flex;align-items:center;justify-content:center;gap:1rem;flex-wrap:wrap">'
    +'<span style="font-family:\'JetBrains Mono\',monospace;font-size:0.78rem;color:#f0e6d3">All 150 tools. One binary each. <strong style="color:#d4a843">$29/mo</strong></span>'
    +'<a href="/complete/" style="font-family:\'JetBrains Mono\',monospace;font-size:0.75rem;padding:0.45rem 1.2rem;background:#c45d2c;color:#f0e6d3;border:none;text-decoration:none;white-space:nowrap">Get Complete &rarr;</a>'
    +'<a href="/tools/" style="font-family:\'JetBrains Mono\',monospace;font-size:0.68rem;color:#bfb5a3;text-decoration:none;white-space:nowrap">Browse tools</a>'
    +'<button onclick="this.parentElement.parentElement.style.display=\'none\'" style="background:none;border:none;color:#7a7060;cursor:pointer;font-size:1.1rem;padding:0 0.3rem;line-height:1" aria-label="Close">&times;</button>'
    +'</div>';
  bar.style.cssText='position:fixed;bottom:0;left:0;right:0;background:#241e18;border-top:1px solid #2e261e;padding:0.7rem 1.5rem;z-index:900;transform:translateY(100%);transition:transform 0.3s ease;box-shadow:0 -4px 12px rgba(0,0,0,0.3)';

  if(window.matchMedia&&window.matchMedia('(prefers-color-scheme:light)').matches){
    bar.style.background='#f0ebe3';
    bar.style.borderTopColor='#e0d9ce';
    bar.querySelector('span').style.color='#1a1410';
    bar.querySelector('a').style.background='#b04a1e';
  }

  document.body.appendChild(bar);

  var shown=false;
  window.addEventListener('scroll',function(){
    var pct=window.scrollY/(document.body.scrollHeight-window.innerHeight);
    if(pct>0.25&&!shown){
      bar.style.transform='translateY(0)';
      shown=true;
      gtag('event','view_promotion',{promotion_name:'sticky_cta',creative_slot:'bottom_bar'});
    }
  },{passive:true});

  bar.querySelector('a[href="/complete/"]').addEventListener('click',function(){
    gtag('event','select_promotion',{promotion_name:'sticky_cta',creative_slot:'bottom_bar'});
  });

  // === BLOG MID-ARTICLE CTA ===
  if(path.indexOf('/blog/')===0&&path!=='/blog/'){
    var paras=document.querySelectorAll('section p, article p, .content p, main p');
    if(paras.length>5){
      var box=document.createElement('div');
      box.style.cssText='margin:2rem 0;padding:1.5rem;border:1px solid #c45d2c;background:#241e18;text-align:center';
      box.innerHTML='<div style="font-family:\'JetBrains Mono\',monospace;font-size:0.65rem;letter-spacing:2px;text-transform:uppercase;color:#c45d2c;margin-bottom:0.6rem">Stockyard Complete</div>'
        +'<div style="font-size:0.95rem;color:#f0e6d3;margin-bottom:0.5rem;font-weight:700">150 self-hosted tools. $29/mo.</div>'
        +'<div style="font-size:0.82rem;color:#bfb5a3;font-style:italic;margin-bottom:1rem">Single binary each. SQLite storage. No dependencies.</div>'
        +'<a href="/complete/" style="font-family:\'JetBrains Mono\',monospace;font-size:0.78rem;padding:0.5rem 1.5rem;background:#c45d2c;color:#f0e6d3;text-decoration:none;display:inline-block">See what\'s included &rarr;</a>';
      paras[4].parentNode.insertBefore(box,paras[4].nextSibling);
    }
  }
})();
