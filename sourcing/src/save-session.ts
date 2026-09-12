import { firefox } from 'playwright-core';

const AUTH_FILE = 'auth.json';

// One-time script: launches a fresh Firefox instance (separate from your
// personal browser/profile), lets you log in manually, then dumps the
// resulting cookies + localStorage into a JSON file so later runs (see
// main.ts) can load that session back into a headless browser.
async function main() {
    const browser = await firefox.launch({ headless: false });
    const context = await browser.newContext();
    const page = await context.newPage();

    await page.goto('https://www.facebook.com/login');

    console.log('Log in manually in the opened window, then press Enter here to save the session...');
    await new Promise((resolve) => process.stdin.once('data', resolve));

    await context.storageState({ path: AUTH_FILE });
    console.log(`Saved session to ${AUTH_FILE}`);

    await browser.close();
}

main()
    .then(() => console.log('ok'))
    .catch((err) => console.error(err));
