# Stockyard × SillyTavern

> Save 30-60% on RP API costs with automatic response caching.

SillyTavern RP users typically spend $200-500/month on API calls. Stockyard's caching, smart routing, and cost caps bring that down dramatically.

## Install

1. Copy `index.js` and `manifest.json` to:  
   `SillyTavern/public/scripts/extensions/third-party/stockyard/`
2. Start Stockyard: `npx @stockyard/stockyard`
3. Restart SillyTavern → Enable Stockyard in Extensions

## Features

- **Cost display** in every response (today's spend, budget remaining)
- **Response caching** — repeated scenarios/greetings return instantly
- **Rate limiting** — prevent accidental API hammering
- **Budget caps** — auto-stop when daily limit is hit
- **Smart routing** — simple continuations use cheap models

## License

MIT
