/**
 * Interactive login script for ROKS clusters with passkey/MFA authentication.
 *
 * Usage:
 *   CONSOLE_URL=$(oc whoami --show-console) npx ts-node e2e/live/login.ts
 */

import { chromium } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const AUTH_DIR = path.resolve(__dirname, '.auth');
const AUTH_STATE_PATH = path.join(AUTH_DIR, 'state.json');
const CHROME_PROFILE_DIR = path.join(AUTH_DIR, 'chrome-profile');

async function main() {
  const consoleURL = process.env.CONSOLE_URL;
  if (!consoleURL) {
    console.error('CONSOLE_URL env var is required.');
    console.error('  CONSOLE_URL=$(oc whoami --show-console) npx ts-node e2e/live/login.ts');
    process.exit(1);
  }

  fs.mkdirSync(AUTH_DIR, { recursive: true });

  console.log(`Opening Chrome to ${consoleURL}...`);
  console.log('Please complete the login (W3 SSO + IBM Verify).');
  console.log('The browser will close automatically once login is detected.\n');

  const context = await chromium.launchPersistentContext(CHROME_PROFILE_DIR, {
    headless: false,
    channel: 'chrome',
    ignoreHTTPSErrors: true,
  });

  const page = context.pages()[0] || await context.newPage();

  // Log navigations for debugging
  page.on('framenavigated', (frame) => {
    if (frame === page.mainFrame()) {
      const url = frame.url();
      if (url.startsWith('http')) {
        console.log(`  navigated → ${url.substring(0, 120)}...`);
      }
    }
  });

  await page.goto(consoleURL);

  // Wait for the console to load after login — try multiple selectors
  // ROKS console may not have data-test="user-dropdown"
  try {
    console.log('\nWaiting for console to load after login...');
    await page.waitForFunction(
      (baseURL) => {
        // Detect: we're on the console domain AND the page has loaded content
        const onConsole = window.location.href.startsWith(baseURL!);
        const hasNav = document.querySelector('#page-sidebar, nav, [data-test="user-dropdown"], .pf-v5-c-page__sidebar, .co-masthead');
        return onConsole && hasNav;
      },
      consoleURL,
      { timeout: 300_000 },
    );
    console.log('\nConsole detected! Saving session state...');
  } catch (err) {
    console.error('\nFailed to detect console after login.');
    console.error('Current URL:', page.url());
    // Take a screenshot for debugging
    const screenshotPath = path.join(AUTH_DIR, 'login-debug.png');
    await page.screenshot({ path: screenshotPath }).catch(() => {});
    console.error(`Debug screenshot saved to ${screenshotPath}`);
    await context.close();
    process.exit(1);
  }

  // Give the console a moment to fully initialize
  await page.waitForTimeout(2000);

  await context.storageState({ path: AUTH_STATE_PATH });
  await context.close();

  console.log(`\nSession state saved to ${AUTH_STATE_PATH}`);
  console.log('You can now run: CONSOLE_URL=$(oc whoami --show-console) npm run test:e2e:live');
}

main().catch((err) => {
  console.error('Login script error:', err.message);
  process.exit(1);
});
