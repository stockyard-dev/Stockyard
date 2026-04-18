# Stockyard Design System

**Status:** v1, written Apr 17 2026.
**Scope:** All Stockyard product surfaces — the marketing site (stockyard.dev), the desktop app, and the admin dashboard.
**Source of truth:** This document. If a CSS file or HTML page disagrees with this doc, this doc wins and the file gets updated.

This is a working design system for a solo-founder product. It is opinionated, not exhaustive. It exists so that any designer (human or AI) producing Stockyard UI can produce something consistent with what already exists, without inventing tokens, components, or voice from scratch.

---

## Brand premise

Stockyard is *frontier general store software*. Warm browns and rust, leather and cream, sharp corners, serif headlines, monospace data. The visual reference is a 1890s livestock market or a present-day ranch supply store — the kind of place that has been in business for a century and intends to be in business for another century. **Not a tech-bro startup.** Not Notion. Not Linear. Not gradient purple anywhere.

The product is honest software for people who run small businesses with their own hands. The design should feel that way: substantial, not flashy; readable, not decorative; built to last, not built to impress investors.

---

## Tokens

These are the canonical values. Any surface that uses different values is wrong and needs to update.

> **Current drift (Apr 17 2026):** The marketing site at `site/index.html` uses slightly different (cooler, less saturated) values for 11 of the 13 color tokens — `--bg: #1a1410` vs canonical `#191410`, `--rust: #c45d2c` vs canonical `#e8763a`, etc. The site values were established earlier and the desktop values were tuned more recently. The desktop values are canonical. Reconciliation of the site to match is pending an eyes-on review in daylight; see TODO at the bottom of this doc. Until reconciled, treat the desktop CSS at `stockyard-desktop/frontend/src/style.css` as the source of truth for color.

### Color

```css
:root {
  /* Backgrounds, darkest to lightest */
  --bg:               #191410;  /* page / canvas */
  --bg2:              #1e1812;  /* panels, cards, inputs */
  --bg3:              #241d15;  /* elevated surfaces, hover states, borders */

  /* Text, brightest to dimmest */
  --cream:            #f5e9d3;  /* primary text */
  --cream-dim:        #c9b896;  /* secondary text, sub-headlines */
  --cream-muted:      #8a7a60;  /* tertiary text, labels, placeholders */

  /* Accents */
  --rust:             #e8763a;  /* primary accent, CTAs, focus rings */
  --rust-light:       #f0a16e;  /* hover state for rust elements, links */
  --leather:          #c6a778;  /* secondary accent, brand mark */
  --leather-light:    #d9c198;  /* hover state for leather elements */
  --gold:             #d4a84b;  /* highlight numbers (savings, counts), rare */
  --green:            #4a9e5c;  /* success state ONLY */
  --green-light:      #6bb87f;  /* success hover */
}
```

### Color usage rules

The palette is small on purpose. Three rules:

**Rust is the action color.** Buttons, links, focus rings, the bar that says "your toolkit is ready." If the user is supposed to do something with it, it's rust. Never use rust for decoration or for static illustration; the eye learns "rust = click."

**Gold is a special-occasion color.** A big number — a savings figure, a toolkit count, a price — can be gold. Anything else cannot. If you find yourself reaching for gold to make something feel "premium," stop; use rust instead. Gold loses its specialness if it's everywhere.

**Green is for confirmed success only.** A running tool, a paid invoice, a successful backup. Never use green for "this is fine" or "this is the recommended option" — that's rust. Green appears only when something happened correctly.

### Light mode

The site has a `prefers-color-scheme: light` block. **Light mode is a fallback for accessibility, not a designed mode.** It exists so that visitors with system-forced light mode don't see broken contrast. It is not a first-class brand mode and we do not put effort into making it look as good as dark.

Current light values (carry forward from `site/index.html`, do not iterate on these):

```css
@media (prefers-color-scheme: light) {
  :root {
    --bg:#faf7f2; --bg2:#f0ebe3; --bg3:#e0d9ce;
    --rust:#b04a1e; --rust-light:#c45d2c;
    --leather:#7a6544; --leather-light:#8b7355;
    --cream:#1a1410; --cream-dim:#4a4035; --cream-muted:#8a806e;
    --gold:#b08a28; --green:#3a7a4a; --green-light:#4a9050;
  }
}
```

If a designer asks "should I redesign the light mode," the answer is no.

### Type

```css
--font-serif: 'Libre Baskerville', Georgia, serif;
--font-mono:  'JetBrains Mono', ui-monospace, monospace;
```

**Serif is for prose.** Headlines, body copy, hero text, anything the reader is meant to *read*. Default font for `body`.

**Monospace is for data and structure.** Tool names, prices, status labels, button text, navigation links, counts, timestamps, any text that conveys a discrete value or category rather than narrative.

There is no third font. There is no sans-serif anywhere in Stockyard UI.

### Type scale

```
Hero h1:       clamp(1.9rem, 4.2vw, 2.8rem)  /  serif  /  line-height 1.25
Section h2:    1.6rem                          /  serif  /  line-height 1.3
Subsection h3: 1.15rem                         /  serif  /  line-height 1.4
Body:          1rem                            /  serif  /  line-height 1.7
Body small:    0.85rem                         /  serif  /  line-height 1.6
Mono label:    0.7rem  letter-spacing 2-3px  /  mono /  uppercase
Mono small:    0.75rem  letter-spacing 0.5-1px /  mono
Mono data:     0.85rem                          /  mono
Mono button:   0.78rem  letter-spacing 1.5px   /  mono /  uppercase
```

Sizes scale with the page, not with the component. If a component needs a different size than what's listed here, that probably means the component is wrong, not the scale.

### Spacing

No formal spacing scale — use `rem` values directly with these guidelines:

- `0.4rem` — internal padding inside small elements (chips, status badges)
- `0.75rem` — gap between related items in a grid or row
- `1rem` — gap between unrelated items, padding inside cards
- `1.5rem` — gap between sections within a card
- `2rem` — gap between major page sections
- `4rem` — top padding of hero areas

### Radii

```css
border-radius: 2px;   /* universal — buttons, cards, inputs, badges */
```

There is one corner radius and it is 2px. Pill buttons (radius >12px) are forbidden. Square corners (radius 0) are forbidden. The 2px sharpness is part of the brand — it reads as "built, not extruded."

### Borders

```css
border: 1px solid var(--bg3);  /* default — cards, panels, inputs */
border: 1px solid var(--rust); /* focused / active state */
border: 1px solid var(--leather); /* hover state for high-affordance items */
```

Solid only. No dashed. No double. No gradients-as-borders.

### Shadows

Shadows are rare in this design system. The brand reads as "flat with intention," not "layered with depth." When a shadow is genuinely needed (a hovering element clearly above another, like a tool card on hover), use:

```css
box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
```

Focus rings are the exception — they exist on every interactive element:

```css
box-shadow: 0 0 0 3px rgba(232, 118, 58, 0.25);  /* rust focus ring */
```

### Motion

Transitions are short and quiet. No bouncing, no spring physics, no theatrical reveals.

```css
transition: background 0.15s, border-color 0.15s, color 0.15s;
```

For state changes that need to communicate (the "thinking" pulse during LLM generation), use a calm opacity oscillation:

```css
@keyframes gen-pulse {
  0%, 100% { opacity: 0.7; }
  50%      { opacity: 1; }
}
animation: gen-pulse 1.6s ease-in-out infinite;
```

No bounce. No scale-in. No `cubic-bezier` curves with overshoot. The product is not trying to delight you with animation; it's trying to do its job.

---

## Components

These are the atoms. Every component listed here exists somewhere in the live product. The reference column points to the canonical implementation; copy from there, don't reinvent.

### Button

**Primary** — the action you most want the user to take. Rust background, cream text, mono font, uppercase, 1.5px letter-spacing.

```html
<a href="..." class="btn">Install Stockyard</a>
```

**Secondary (outline)** — the alternative path. Transparent background, border, dimmer text. Use when offering the user a choice; never for the primary action.

```html
<a href="..." class="btn btn-outline">How it works</a>
```

**Disabled** — `opacity: 0.6; cursor: wait;`. Used during pending operations, not for actions the user cannot take (those should be hidden, not disabled).

There is no tertiary button. There is no ghost button. There is no text-link-styled-as-button. If a third action genuinely needs to exist on a screen, that's a sign the screen is doing too much.

Reference: `site/index.html:54-57` (`.btn`, `.btn-outline`).

### Input (text)

Rust focus border. Cream-muted italic placeholder. Serif font for content (the user is writing, not labeling). `bg2` background.

```html
<input type="text" placeholder="mobile dog groomer in Portland" />
```

Reference: `site/index.html:65-67` (`.gen-input`).

### Card

The fundamental container. `bg2` background, `bg3` border, 2px radius, padding `1rem` to `1.8rem` depending on density. Used for every grouped piece of content — tool descriptions, pricing tiers, result panels, dashboard rows.

```html
<div class="card">
  <h3>Card title</h3>
  <p>Card body content.</p>
</div>
```

Hover state for interactive cards: `border-color: var(--leather); transform: translateY(-1px); box-shadow: 0 4px 12px rgba(0,0,0,.3);`

Reference: `frontend/src/style.css` desktop `.card` and `.tool-card`.

### Chip

Small pill-adjacent (still 2px radius, not pill) action element. Used for example queries, category tags, filter selections. Mono font, transparent background, `bg3` border, `cream-dim` text. Hover: rust border + rust-light text.

```html
<button class="chip" data-chip="yoga studio">yoga studio</button>
```

Reference: `site/index.html:71-72` (`.gen-chip`), `frontend/src/style.css` desktop `.chip`.

### Status dot

A small circular indicator showing the state of a tool. CSS-painted, not emoji. Four states:

| State | Color | Meaning |
|---|---|---|
| `running` | green | Tool is up and serving requests |
| `failed` | rust | Tool crashed or failed to start |
| `downloading` | leather | Tool binary being fetched |
| `starting` | cream-muted | Default / transitional state |

```html
<span class="dot" data-state="running" aria-label="running"></span>
```

Reference: `frontend/src/main.js` desktop dashboard rendering.

### Badge / Label (mono uppercase eyebrow)

A small categorical label, used as section labels ("HOW IT WORKS"), eyebrow text above headlines ("FOR SMALL BUSINESS OWNERS"), or context labels in cards ("TOOLKIT:", "REPLACES:"). Mono, uppercase, `0.68-0.7rem`, 2-3px letter-spacing, `rust-light` or `cream-muted` color.

```html
<div class="section-label">How it works</div>
<div class="hero-eyebrow">For small business owners</div>
```

Never use this style for things the user clicks. It signals "this is a label," not "this is an action."

### Counter (large number + label)

A prominent display number — toolkit count, savings figure, price. Gold color, large mono, paired with a small uppercase mono label beneath.

```html
<div class="gen-counter">
  <div class="gen-counter-num">412</div>
  <div class="gen-counter-label">toolkits built so far</div>
  <div class="gen-counter-latest">last built 3 minutes ago</div>
</div>
```

Reference: `site/index.html:115-119` (`.gen-counter`).

### Banner (success / error)

A full-width strip at the top of a panel announcing a state. Two variants: ok (green-tinted bg, green text) and bad (rust-tinted bg, rust text). One sentence, no longer.

```html
<div class="banner banner-ok">Everything's up. 8 tools running.</div>
<div class="banner banner-bad">2 tools failed to start.</div>
```

Reference: `frontend/src/main.js` desktop dashboard rendering.

### Tool row (dashboard)

The dashboard's primary content unit: status dot + tool name + meta cell (open link, error, or downloading state). Compact, scannable, one line tall by default; errors expand in a `<details>` element below.

```html
<div class="tool-row">
  <span class="dot" data-state="running"></span>
  <span class="tool-row-name">Billfold</span>
  <span class="tool-row-meta"><a href="http://localhost:7011/" target="_blank">open →</a></span>
</div>
```

Reference: `frontend/src/main.js` `renderDashboard`.

### Empty state

When there's nothing to show, never show "0 results" or a blank panel. Show a short title (one sentence), a guiding message, and an action.

```html
<div class="empty-state">
  <div class="empty-title">No match for that yet.</div>
  <div class="empty-msg">Try describing it differently, or <a>browse by category ↓</a></div>
</div>
```

Reference: `frontend/src/main.js` `renderEmpty`.

### Thinking state

LLM generation in progress. Pulsing border, italicized hint text echoing the user's input. Calm, not anxious.

```html
<div class="gen-thinking">
  Building a toolkit for a <strong>mobile dog groomer in Portland</strong>…
</div>
```

Reference: `site/index.html` `.gen-thinking`.

---

## Patterns

How components combine into recognizable Stockyard moments.

### The result card

The single most important pattern in the product. After the user submits a description, this is what they see. Composes:

1. A personalized title (serif, large, cream)
2. An audience description (one italic line, cream-dim, serif)
3. A grid of tool cards (each: name in mono leather-light + "replaces X ($N/mo)" in mono cream-muted)
4. A savings line (mono label "REPLACES" + gold dollar amount + primary CTA)

The eye should land on title → savings number → CTA, in roughly that order. Tool cards are scannable detail, not the focus.

### The hero with the working product in it

Stockyard's headline pattern: don't describe what the product does, *let the user do it.* The hero of stockyard.dev is a working bundle generator. The "after" should be visually adjacent to the "before" — typing happens here, result appears here, install button appears here, all on one screen.

This pattern is the brand's central design conviction. A future redesign that buries the input behind a "Try the demo →" button is wrong, even if it's prettier.

### The status banner + list

The desktop dashboard's pattern. A one-line banner tells the user the overall state (everything's fine / something is wrong); a list of rows lets them check details if they want. The banner does the work; the list is for the curious or the troubled.

### The progressive empty state

Empty states should suggest the next action, never just announce emptiness. "No bundles assembled" → "Type a description to build one →." "No backups" → "Backups appear here once Stockyard runs for the first time." The user always knows what to do next.

---

## Voice and copy

Copy is part of the design. These rules are non-negotiable.

### Hard rules

- **No em dashes in UI copy.** Use commas, periods, parentheses, or start a new sentence. Em dashes are fine in code comments and internal docs, but not in anything the user reads.
- **No emoji in UI.** Not in success states, not in errors, not in chips, not anywhere a user sees them. (Emoji in commit messages, PR descriptions, internal Slack — fine.)
- **No "seamlessly," "revolutionize," "unleash," "empower," "leverage," "frictionless," "intuitive," "robust," "powerful," "elegant," "beautiful."** These words read as marketing throat-clearing.
- **No "AI-powered" or "AI-first."** The LLM is part of the product but it's not the pitch. The pitch is what the user gets.
- **No "Get started for free" CTAs.** The product isn't free. CTAs say what the user is doing: "Install Stockyard," "See pricing," "Build my toolkit."
- **No fake testimonials and no "trusted by" logo rows.** When real ones exist, use them. Until then, the absence is more honest than the presence of made-up ones.

### Voice direction

The copy sounds like a person who runs a small business talking to another person who runs a small business. Conversational first-person and second-person. Comfortable with admitting tradeoffs. Specific over general.

Good: *"You didn't start your business to pay six SaaS bills."*
Bad: "Save money on software."

Good: *"Works offline. That's the point. Which means you're the one backing things up, unless you pay for cloud."*
Bad: "Robust offline support with optional cloud sync."

Good: *"Built by a solo developer in Minnesota."*
Bad: "Crafted by a passionate team."

### Microcopy patterns

**Buttons:** verb-first, two to four words. "Install Stockyard," "See pricing," "Build my toolkit," "Open dashboard." Not "Click here," not "Learn more."

**Empty states:** name what's missing in five words or fewer, then point to the action. "No bundles assembled. Build one →"

**Errors:** plain English, no stack traces, no error codes the user can't act on. "Stockyard Cloud isn't reachable right now. Try again, or browse by category."

**Status:** declarative, present tense. "8 tools running." "Backup complete." "Cloud sync paused."

**Loading:** describe what's happening, not what *will* happen. "Building a toolkit for your mobile dog grooming business…" not "Please wait while we generate your recommendations."

---

## Anti-patterns (forbidden)

These appear nowhere in Stockyard.

- Gradient backgrounds (rust-to-gold, purple-to-blue, anything-to-anything)
- Glassmorphism, frosted-glass panels, backdrop-filter blur
- Pill buttons with radius >12px
- Sans-serif fonts of any kind
- Emoji as visual filler in UI
- Generic icon libraries (Lucide, Heroicons, Material Icons) used decoratively
- Animated number counters that count up on scroll
- Floating "chat with us" widgets in the corner
- Cookie banners that aren't strictly required
- "Trusted by" logo rows
- Fake testimonials
- Modal overlays for things that could be inline
- Tooltips that contain critical information
- Text in images
- Rounded avatar circles for things that aren't people
- Gradient text
- Drop shadows on text
- Skeumorphic effects (paper textures, leather textures, wood grain)
- Neon glow effects
- Full-bleed background videos
- Auto-playing anything

---

## How to use this document

**For Claude or another AI design agent:** paste this entire file into the prompt before asking for new UI. Tell the agent the design system is the source of truth and that any deviation needs to be called out explicitly with a reason.

**For a future human designer:** read it once, then keep it open while you work. When you find yourself reaching for a token, font, or component that isn't in here, ask yourself whether you're solving a real new problem or whether you're inventing inconsistency. If new, propose adding it to the doc in the same PR that uses it.

**For Michael:** when you find a component or pattern in the live product that contradicts this doc, that's a bug in either the product or the doc. Decide which, fix one, commit. The doc should never be stale relative to shipped UI.

---

## Open questions / future work

Things this v1 deliberately doesn't cover. Add later as they come up:

- **Reconcile site tokens to canonical (priority — pre-launch).** 11 of 13 color tokens drift between site and desktop. Desktop values are canonical per this doc. The site needs `find -path './site/index.html' -o -path './internal/site/static/index.html' | xargs sed -i` (or equivalent eyeballed pass) to align. Defer until a daylight eyes-on review confirms the warmer desktop palette doesn't wash out any specific homepage element (the "REPLACES" gold pill against the warmer rust, particularly).
- **Mobile-specific patterns.** The site is responsive but not mobile-first. The desktop app is webview but mostly used on full-size windows. A real "Stockyard on a phone" pass would need its own section.
- **Accessibility beyond focus rings.** ARIA patterns, screen reader text, keyboard-only navigation flows. The product has scattered correct practices (`role="status"`, `aria-live="polite"`) but no documented standard.
- **Iconography.** Currently Stockyard uses essentially no icons. If that changes, this doc needs an icon section.
- **Illustrations.** Same as icons; we don't have any. When we do, they need style rules.
- **Email and PDF templates.** License delivery emails, invoices, exported reports. Out of scope for v1.

---

*Last updated: Apr 17 2026.*
