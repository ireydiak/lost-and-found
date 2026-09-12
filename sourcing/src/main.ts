import { chromium } from 'playwright-core';

const AUTH_FILE = 'auth.json';

async function main() {
    // Launches Playwright's own browser and loads the session saved by
    // save-session.ts, instead of requiring a live CDP connection to a
    // real, already-open Chrome window.
    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ storageState: AUTH_FILE });
    const page = await context.newPage();

    await page.goto('https://wikipedia.com/');

    const title = await page.locator('h1').textContent();
    console.log(title);

    await page.close();
    await context.close();
    await browser.close();
}

main()
    .then(() => console.log("ok"))
    .catch((err) => console.error(err))
