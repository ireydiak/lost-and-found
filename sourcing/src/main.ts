import { chromium } from 'playwright-core';

async function main() {
    const browser = await chromium.connectOverCDP('ws://127.0.0.1:9222');

    const context = await browser.newContext({});
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
