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
    // Track individual tool checkout button clicks
    var checkBtn=e.target.closest('#checkout-btn-monthly,#checkout-btn-annual,[onclick*="startCheckout"]');
    if(checkBtn){
      var tool=window.location.pathname.split('/')[1]||'unknown';
      gtag('event','begin_checkout',{currency:'USD',value:29,items:[{item_name:'Stockyard '+tool+' Pro',quantity:1}]});
      gtag('event','conversion',{send_to:'AW-18046975504/begin_checkout'});
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
    +'<span style="font-family:\'JetBrains Mono\',monospace;font-size:0.78rem;color:#f0e6d3">150 tools. <strong style="color:#d4a843">$29/mo</strong> <span style="font-size:0.65rem;color:#d4a843">(early adopter price)</span></span>'
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

  // === TOOL PAGE COMPLETE BUNDLE BANNER ===
  var annualBtn=document.getElementById('checkout-btn-annual');
  if(annualBtn&&path.indexOf('/blog/')!==0&&path!=='/complete/'&&path!=='/pricing/'){
    var grid=annualBtn.closest('.pricing-grid')||annualBtn.closest('[class*="pricing"]')||annualBtn.parentElement.parentElement;
    if(grid){
      var banner=document.createElement('div');
      banner.style.cssText='margin-top:1.5rem;padding:1.2rem 1.5rem;border:2px solid #d4a843;background:rgba(212,168,67,0.06);text-align:center';
      banner.innerHTML='<div style="font-family:\'JetBrains Mono\',monospace;font-size:0.6rem;letter-spacing:2px;text-transform:uppercase;color:#d4a843;margin-bottom:0.5rem">Better deal</div>'
        +'<div style="font-size:0.95rem;color:#f0e6d3;margin-bottom:0.4rem">Get <strong>all 150 tools</strong> for <strong style="color:#d4a843">$29/mo</strong></div>'
        +'<div style="font-size:0.78rem;color:#bfb5a3;font-style:italic;margin-bottom:0.8rem">This tool plus 149 others. One license key. Cancel any time.</div>'
        +'<a href="/complete/" style="font-family:\'JetBrains Mono\',monospace;font-size:0.78rem;padding:0.5rem 1.5rem;background:#d4a843;color:#1a1410;text-decoration:none;display:inline-block;font-weight:600">Get Complete &rarr;</a>';
      grid.parentNode.insertBefore(banner,grid.nextSibling);
    }
  }

  // === COMPARISON PAGE VERDICT BOX ===
  if(path.indexOf('/vs-')!==-1||path.indexOf('/vs/')!==-1){
    var parts=path.split('/').filter(Boolean);
    var toolName=parts[0]||'Stockyard';
    toolName=toolName.charAt(0).toUpperCase()+toolName.slice(1);
    var sections=document.querySelectorAll('section, .section, .section-alt');
    var target=sections.length>2?sections[Math.floor(sections.length/2)]:null;
    if(target){
      var verdict=document.createElement('div');
      verdict.style.cssText='max-width:700px;margin:0 auto;padding:1.5rem;border-left:3px solid #c45d2c;background:#241e18;margin-bottom:2rem';
      verdict.innerHTML='<div style="font-family:\'JetBrains Mono\',monospace;font-size:0.6rem;letter-spacing:2px;text-transform:uppercase;color:#c45d2c;margin-bottom:0.5rem">Quick take</div>'
        +'<div style="font-size:0.88rem;color:#f0e6d3;margin-bottom:0.5rem">'+toolName+' is a single binary with SQLite storage. No Docker, no external database, no cloud dependency. Free tier included.</div>'
        +'<div style="display:flex;gap:1rem;flex-wrap:wrap;margin-top:0.8rem">'
        +'<a href="/'+parts[0]+'/#install" style="font-family:\'JetBrains Mono\',monospace;font-size:0.75rem;padding:0.4rem 1rem;background:#c45d2c;color:#f0e6d3;text-decoration:none">Try '+toolName+' free &rarr;</a>'
        +'<a href="/complete/" style="font-family:\'JetBrains Mono\',monospace;font-size:0.72rem;color:#d4a843;text-decoration:none;padding:0.4rem 0">or all 150 tools for $29/mo</a>'
        +'</div>';
      target.parentNode.insertBefore(verdict,target);
    }
  }

  // === EMAIL CAPTURE ON CONTENT PAGES ===
  if(path.indexOf('/how-to-')===0||path.indexOf('/self-hosted-')===0||path.indexOf('/what-is-')===0){
    var ftrs=document.querySelectorAll('footer');
    if(ftrs.length>0){
      var emailBox=document.createElement('div');
      emailBox.style.cssText='max-width:500px;margin:0 auto;padding:2rem;text-align:center;border-top:1px solid #2e261e';
      emailBox.innerHTML='<div style="font-family:\'JetBrains Mono\',monospace;font-size:.6rem;letter-spacing:2px;text-transform:uppercase;color:#c45d2c;margin-bottom:.8rem">Get updates</div>'
        +'<p style="font-size:.88rem;color:#bfb5a3;font-style:italic;margin-bottom:1rem">New tools, guides, and self-hosting tips. No spam.</p>'
        +'<div id="content-email-form" style="display:flex;gap:.4rem;justify-content:center;flex-wrap:wrap">'
        +'<input id="content-email" type="email" placeholder="you@example.com" style="padding:.55rem .8rem;background:#241e18;border:1px solid #2e261e;color:#f0e6d3;font-family:\'JetBrains Mono\',monospace;font-size:.8rem;min-width:200px;outline:none">'
        +'<button onclick="contentSub()" style="padding:.55rem 1rem;background:#c45d2c;color:#f0e6d3;border:2px solid #c45d2c;font-family:\'JetBrains Mono\',monospace;font-size:.8rem;cursor:pointer">Subscribe</button>'
        +'</div>'
        +'<div id="content-email-ok" style="display:none;font-family:\'JetBrains Mono\',monospace;font-size:.8rem;color:#d4a843">&#10003; Subscribed</div>';
      ftrs[0].parentNode.insertBefore(emailBox,ftrs[0]);
      window.contentSub=function(){
        var e=document.getElementById('content-email').value;
        if(!e||!e.includes('@'))return;
        try{fetch('https://stockyard.dev/api/nurture',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:e,source:'content'})});}catch(x){}
        document.getElementById('content-email-form').style.display='none';
        document.getElementById('content-email-ok').style.display='block';
        gtag('event','generate_lead',{event_category:'email',event_label:'content_subscribe'});
      };
    }
  }
})();
