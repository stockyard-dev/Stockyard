#!/usr/bin/env python3
"""Take Product Hunt gallery screenshots from the live site."""
import asyncio
from playwright.async_api import async_playwright

PAGES = [
    ("01-hero.png", "https://stockyard.dev", "#main", None),
    ("02-observe.png", "https://stockyard.dev/products/observe/", ".product-demo", None),
    ("03-trust.png", "https://stockyard.dev/products/trust/", ".product-demo", None),
    ("04-studio.png", "https://stockyard.dev/products/studio/", ".product-demo", None),
    ("05-forge.png", "https://stockyard.dev/products/forge/", ".product-demo", None),
]

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        page = await browser.new_page(viewport={"width": 1270, "height": 760})

        for filename, url, selector, scroll_to in PAGES:
            print(f"Capturing {filename} from {url}...")
            await page.goto(url, wait_until="networkidle", timeout=30000)
            await page.wait_for_timeout(1000)  # let animations settle
            
            if scroll_to:
                await page.evaluate(f"document.querySelector('{scroll_to}').scrollIntoView()")
                await page.wait_for_timeout(500)
            
            await page.screenshot(
                path=f"marketing/toolkit/cards/product-hunt-gallery/{filename}",
                clip={"x": 0, "y": 0, "width": 1270, "height": 760}
            )
            print(f"  Saved {filename}")

        await browser.close()
        print("Done!")

asyncio.run(main())
