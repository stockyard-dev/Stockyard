package docs

import (
	"fmt"
	"strings"
)

// Page represents a documentation page.
type Page struct {
	Title       string
	Path        string // e.g. "install/index.html"
	Section     string // sidebar section
	Content     string // HTML content
	Description string // meta description
}

// SidebarSection groups pages for navigation.
type SidebarSection struct {
	Title string
	Items []SidebarItem
}

// SidebarItem is a single sidebar link.
type SidebarItem struct {
	Title  string
	Path   string
	Active bool
}

// Render wraps page content in the full docs layout.
func Render(p Page, sidebar []SidebarSection) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>` + esc(p.Title) + ` | Stockyard Docs</title>
<meta name="description" content="` + esc(p.Description) + `">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>
`)
	sb.WriteString(docsCSS())
	sb.WriteString(`
</style>
</head>
<body>
`)
	sb.WriteString(renderNav())
	sb.WriteString(`<div class="docs-layout">`)
	sb.WriteString(renderSidebar(sidebar, p.Path))
	sb.WriteString(`<main class="docs-main"><article class="docs-content">`)
	sb.WriteString(p.Content)
	sb.WriteString(`</article></main>`)
	sb.WriteString(`</div>`)
	sb.WriteString(renderFooter())
	sb.WriteString(docsJS())
	sb.WriteString(`
</body>
</html>`)

	return sb.String()
}

func renderNav() string {
	return `<nav class="nav">
  <div class="nav-left">
    <a href="/" class="nav-brand">Stockyard</a>
    <span class="nav-sep">/</span>
    <a href="/docs/" class="nav-section">Docs</a>
  </div>
  <div class="nav-right">
    <div class="search-box">
      <input type="text" id="docs-search" placeholder="Search docs..." autocomplete="off">
    </div>
    <div class="nav-links">
      <a href="/products/">Products</a>
      <a href="/pricing/">Pricing</a>
      <a href="https://github.com/stockyard-dev/stockyard">GitHub</a>
    </div>
  </div>
</nav>
`
}

func renderSidebar(sections []SidebarSection, activePath string) string {
	var sb strings.Builder
	sb.WriteString(`<aside class="docs-sidebar"><nav class="sidebar-nav">`)

	for _, sec := range sections {
		sb.WriteString(`<div class="sidebar-section">`)
		sb.WriteString(`<div class="sidebar-heading">` + esc(sec.Title) + `</div>`)
		sb.WriteString(`<ul>`)
		for _, item := range sec.Items {
			cls := ""
			if item.Path == activePath || strings.TrimSuffix(item.Path, "index.html") == strings.TrimSuffix(activePath, "index.html") {
				cls = ` class="active"`
			}
			sb.WriteString(fmt.Sprintf(`<li%s><a href="/docs/%s">%s</a></li>`, cls, item.Path, esc(item.Title)))
		}
		sb.WriteString(`</ul></div>`)
	}

	sb.WriteString(`</nav></aside>`)
	return sb.String()
}

func renderFooter() string {
	return `<footer class="docs-footer">
  <div class="footer-inner">
    <span class="footer-brand">Stockyard</span>
    <span class="footer-sep">Where LLM traffic gets sorted.</span>
  </div>
</footer>
`
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func docsCSS() string {
	return `*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;--bg4:#382f25;
  --rust:#c45d2c;--rust-light:#e8753a;--rust-dark:#8b3d1a;
  --leather:#a0845c;--leather-light:#c4a87a;
  --cream:#f0e6d3;--cream-dim:#bfb5a3;
  --gold:#d4a843;
  --green:#5a9a5a;--red:#c45050;--blue:#5a8ab5;
  --font-serif:'Libre Baskerville',Georgia,serif;
  --font-mono:'JetBrains Mono',monospace;
  --sidebar-w:260px;
}
body{background:var(--bg);color:var(--cream);font-family:var(--font-serif);line-height:1.7;overflow-x:hidden}
a{color:var(--rust-light);text-decoration:none}
a:hover{color:var(--gold)}

/* Nav */
.nav{padding:0.8rem 1.5rem;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid var(--bg3);position:sticky;top:0;background:var(--bg);z-index:100}
.nav-left{display:flex;align-items:center;gap:0.5rem}
.nav-brand{font-family:var(--font-mono);font-size:0.85rem;color:var(--leather-light);letter-spacing:2px;text-transform:uppercase}
.nav-sep{color:var(--bg3);font-family:var(--font-mono)}
.nav-section{font-family:var(--font-mono);font-size:0.8rem;color:var(--cream-dim)}
.nav-right{display:flex;align-items:center;gap:1.5rem}
.nav-links{display:flex;gap:1.2rem;font-size:0.8rem;font-family:var(--font-mono)}
.nav-links a{color:var(--cream-dim)}.nav-links a:hover{color:var(--rust-light)}
.search-box input{background:var(--bg2);border:1px solid var(--bg3);color:var(--cream);font-family:var(--font-mono);font-size:0.8rem;padding:0.4rem 0.8rem;width:200px;outline:none}
.search-box input:focus{border-color:var(--rust);width:280px;transition:width 0.2s}
.search-box input::placeholder{color:var(--leather)}

/* Layout */
.docs-layout{display:flex;min-height:calc(100vh - 50px)}

/* Sidebar */
.docs-sidebar{width:var(--sidebar-w);flex-shrink:0;border-right:1px solid var(--bg3);padding:1.5rem 0;overflow-y:auto;position:sticky;top:50px;height:calc(100vh - 50px)}
.sidebar-nav{padding:0 1rem}
.sidebar-section{margin-bottom:1.5rem}
.sidebar-heading{font-family:var(--font-mono);font-size:0.65rem;text-transform:uppercase;letter-spacing:2px;color:var(--leather);margin-bottom:0.5rem;padding-left:0.5rem}
.sidebar-nav ul{list-style:none}
.sidebar-nav li{margin-bottom:1px}
.sidebar-nav li a{display:block;padding:0.3rem 0.5rem 0.3rem 0.8rem;font-size:0.82rem;color:var(--cream-dim);border-left:2px solid transparent;transition:all 0.15s}
.sidebar-nav li a:hover{color:var(--cream);background:var(--bg2)}
.sidebar-nav li.active a{color:var(--rust-light);border-left-color:var(--rust);background:var(--bg2)}

/* Main content */
.docs-main{flex:1;min-width:0;padding:2rem 3rem 4rem;max-width:900px}
.docs-content h1{font-size:2rem;margin-bottom:0.5rem;line-height:1.3}
.docs-content h2{font-size:1.4rem;margin:2.5rem 0 0.8rem;padding-bottom:0.4rem;border-bottom:1px solid var(--bg3)}
.docs-content h3{font-size:1.1rem;margin:1.8rem 0 0.6rem;color:var(--leather-light)}
.docs-content h4{font-family:var(--font-mono);font-size:0.85rem;margin:1.2rem 0 0.4rem;color:var(--gold);text-transform:uppercase;letter-spacing:1px}
.docs-content p{margin-bottom:1rem;color:var(--cream-dim);font-size:0.95rem}
.docs-content ul,.docs-content ol{margin:0 0 1rem 1.5rem;color:var(--cream-dim);font-size:0.95rem}
.docs-content li{margin-bottom:0.3rem}
.docs-content strong{color:var(--cream)}
.docs-content em{color:var(--leather-light)}
.docs-content code{font-family:var(--font-mono);font-size:0.82rem;background:var(--bg2);padding:0.15rem 0.4rem;color:var(--rust-light);border:1px solid var(--bg3)}
.docs-content a{color:var(--rust-light);border-bottom:1px solid var(--bg3)}
.docs-content a:hover{border-bottom-color:var(--rust-light)}

/* Code blocks */
.docs-content pre{background:var(--bg2);border:1px solid var(--bg3);padding:1.2rem 1.5rem;margin:0 0 1.5rem;overflow-x:auto;line-height:1.6;position:relative}
.docs-content pre code{background:none;padding:0;border:none;font-size:0.8rem;color:var(--leather-light)}
.code-title{font-family:var(--font-mono);font-size:0.7rem;color:var(--leather);background:var(--bg3);padding:0.3rem 1rem;margin-bottom:-1px;border:1px solid var(--bg3);border-bottom:none;display:inline-block;text-transform:uppercase;letter-spacing:1px}
.comment{color:#5a5040}
.string{color:var(--green)}
.keyword{color:var(--rust-light)}
.flag{color:var(--gold)}
.url{color:var(--blue)}
.output{color:var(--leather)}

/* Tables */
.docs-content table{width:100%;border-collapse:collapse;margin:0 0 1.5rem;font-size:0.85rem}
.docs-content th{font-family:var(--font-mono);font-size:0.75rem;text-transform:uppercase;letter-spacing:1px;text-align:left;padding:0.6rem 1rem;background:var(--bg3);color:var(--leather-light);border:1px solid var(--bg3)}
.docs-content td{padding:0.5rem 1rem;border:1px solid var(--bg3);color:var(--cream-dim)}

/* Callout boxes */
.callout{padding:1rem 1.2rem;margin:0 0 1.5rem;border-left:3px solid var(--leather);background:var(--bg2);font-size:0.9rem}
.callout.tip{border-left-color:var(--green)}
.callout.warn{border-left-color:var(--gold)}
.callout.danger{border-left-color:var(--red)}
.callout-label{font-family:var(--font-mono);font-size:0.7rem;text-transform:uppercase;letter-spacing:2px;margin-bottom:0.3rem;display:block}
.callout.tip .callout-label{color:var(--green)}
.callout.warn .callout-label{color:var(--gold)}
.callout.danger .callout-label{color:var(--red)}

/* Subtitle / lead */
.lead{font-size:1.05rem;color:var(--cream-dim);font-style:italic;margin-bottom:2rem}

/* Tag/badge */
.tag{font-family:var(--font-mono);font-size:0.65rem;padding:0.15rem 0.5rem;border:1px solid var(--bg3);color:var(--leather-light);text-transform:uppercase;letter-spacing:1px;display:inline-block;margin-right:0.3rem}
.tag.new{border-color:var(--green);color:var(--green)}

/* Params table */
.params{font-size:0.85rem}
.params td:first-child{font-family:var(--font-mono);color:var(--rust-light);white-space:nowrap}
.params td:nth-child(2){font-family:var(--font-mono);color:var(--leather);font-size:0.8rem}

/* Footer */
.docs-footer{border-top:1px solid var(--bg3);padding:1.5rem 2rem;text-align:center}
.footer-inner{font-family:var(--font-mono);font-size:0.75rem;color:var(--leather)}
.footer-brand{color:var(--leather-light)}
.footer-sep{margin-left:0.5rem}

/* Responsive */
@media(max-width:900px){
  .docs-sidebar{display:none}
  .docs-main{padding:1.5rem}
  .nav-links{display:none}
  .search-box input{width:150px}
}

/* On-page TOC */
.page-toc{background:var(--bg2);border:1px solid var(--bg3);padding:1rem 1.5rem;margin:0 0 2rem;font-size:0.85rem}
.page-toc-title{font-family:var(--font-mono);font-size:0.7rem;text-transform:uppercase;letter-spacing:2px;color:var(--leather);margin-bottom:0.5rem}
.page-toc ul{list-style:none;margin:0;padding:0}
.page-toc li{margin-bottom:0.3rem;padding-left:0}
.page-toc li.indent{padding-left:1rem}
.page-toc a{color:var(--cream-dim);border-bottom:none}
.page-toc a:hover{color:var(--rust-light)}

/* API method badges */
.method{font-family:var(--font-mono);font-size:0.7rem;padding:0.15rem 0.5rem;letter-spacing:1px;font-weight:600;display:inline-block;margin-right:0.3rem}
.method.get{background:rgba(90,138,181,0.15);color:var(--blue);border:1px solid rgba(90,138,181,0.3)}
.method.post{background:rgba(90,154,90,0.15);color:var(--green);border:1px solid rgba(90,154,90,0.3)}
.method.delete{background:rgba(196,80,80,0.15);color:var(--red);border:1px solid rgba(196,80,80,0.3)}
`
}

func docsJS() string {
	return `<script>
// Minimal client-side search
document.addEventListener('DOMContentLoaded', function() {
  var input = document.getElementById('docs-search');
  if (!input) return;
  input.addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && this.value.trim()) {
      var q = this.value.trim().toLowerCase();
      // Simple: redirect to product page if matches a known slug
      window.location.href = '/docs/products/?q=' + encodeURIComponent(q);
    }
  });
  // Keyboard shortcut: / to focus search
  document.addEventListener('keydown', function(e) {
    if (e.key === '/' && document.activeElement.tagName !== 'INPUT') {
      e.preventDefault();
      input.focus();
    }
  });
});
</script>`
}
