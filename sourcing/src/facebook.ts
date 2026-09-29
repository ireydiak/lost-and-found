import { chromium } from "playwright-core";
import type { Page } from "playwright-core";
import { writeFileSync, mkdirSync } from "node:fs";

const AUTH_FILE = "auth.json";
const OUTPUT_DIR = "raw-chromium-test"; // separate from raw/ (Lightpanda) for a clean A/B comparison
const SCROLL_ITERATIONS = 15;
const WAIT_AFTER_SCROLL_MS = 5000;

async function socialSourcing() {
    // Experiment: was connectOverCDP("ws://127.0.0.1:9222"), which turned out
    // to be Lightpanda (a lightweight, non-Chromium browser engine), not real
    // Chrome. Launching real Playwright-managed Chromium directly here to
    // test whether Lightpanda's rendering was the cause of posts with no
    // discoverable timestamp. auth.json is browser-agnostic (just cookies/
    // localStorage), so it loads fine into a fresh Chromium context.
    const browser = await chromium.launch({ headless: true });

    const context = await browser.newContext({
        storageState: AUTH_FILE
    });
    const page = await context.newPage();

    await page.goto("https://www.facebook.com/groups/velovolemtl");

    mkdirSync(OUTPUT_DIR, { recursive: true });

    // One snapshot per scroll iteration — Facebook can unmount off-screen posts
    // as more content loads in, so capturing only the final state risks losing
    // early posts. The extractor (a separate Go tool) dedupes across all files.
    for (let i = 0; i < SCROLL_ITERATIONS; i++) {
        await saveSnapshot(page);
        await scrollToLoadMore(page);
    }
    await saveSnapshot(page);

    await page.close();
    await context.close();
    await browser.close();
}

async function saveSnapshot(page: Page): Promise<void> {
    const html = await page.content();
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    const filePath = `${OUTPUT_DIR}/feed-${timestamp}.html`;
    writeFileSync(filePath, html);
    console.log(`Saved snapshot: ${filePath}`);
}

async function scrollToLoadMore(page: Page): Promise<void> {
    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
    await page.waitForTimeout(WAIT_AFTER_SCROLL_MS);
}

socialSourcing()
    .then(() => console.log("ok"))
    .catch((err) => console.error(err))
