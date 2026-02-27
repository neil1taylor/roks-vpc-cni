import { test as setup, expect } from '@playwright/test';
import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

const AUTH_STATE_PATH = 'e2e/live/.auth/state.json';
const STATE_MAX_AGE_MS = 2 * 60 * 60 * 1000; // 2 hours

setup('authenticate with OpenShift console', async ({ page, baseURL }) => {
  // Increase timeout for auth setup — IBM Cloud login can be slow
  setup.setTimeout(180_000);

  // Reuse existing auth state if it's recent enough
  const stateAbsPath = path.resolve(AUTH_STATE_PATH);
  if (fs.existsSync(stateAbsPath)) {
    const stat = fs.statSync(stateAbsPath);
    const ageMs = Date.now() - stat.mtimeMs;
    if (ageMs < STATE_MAX_AGE_MS) {
      console.log(`Reusing auth state (age: ${Math.round(ageMs / 1000)}s)`);
      const state = JSON.parse(fs.readFileSync(stateAbsPath, 'utf-8'));
      if (state.cookies?.length > 0) {
        await page.context().addCookies(state.cookies);
        await page.goto('/');
        try {
          await expect(
            page.locator('[data-test="user-dropdown"], .co-username, .pf-v5-c-masthead'),
          ).toBeVisible({ timeout: 15_000 });
          await page.context().storageState({ path: AUTH_STATE_PATH });
          return;
        } catch {
          console.warn('Saved auth state expired, re-authenticating...');
        }
      }
    }
  }

  // Tier 1: Try oc token (vanilla OpenShift or after `oc login --token`)
  let token: string | undefined = process.env.OC_TOKEN;
  if (!token) {
    try {
      token = execSync('oc whoami -t', { encoding: 'utf-8', timeout: 5000 }).trim();
    } catch {
      // not available
    }
  }

  if (token) {
    const url = new URL(baseURL!);
    await page.context().addCookies([
      {
        name: 'openshift-session-token',
        value: token,
        domain: url.hostname,
        path: '/',
        httpOnly: true,
        secure: true,
        sameSite: 'Lax',
      },
    ]);
    await page.goto('/');
    try {
      await expect(
        page.locator('[data-test="user-dropdown"], .co-username, .pf-v5-c-masthead'),
      ).toBeVisible({ timeout: 15_000 });
      await page.context().storageState({ path: AUTH_STATE_PATH });
      return;
    } catch {
      console.warn('Token cookie injection failed (likely ROKS cluster). Trying IBM Cloud login...');
    }
  }

  // Tier 2: IBM Cloud login form automation (ROKS clusters)
  // Navigate to console — it will redirect through the OAuth chain to login.ibm.com
  await page.goto('/');

  const ibmEmail = process.env.IBMCLOUD_EMAIL;
  const ibmPassword = process.env.IBMCLOUD_PASSWORD;

  if (ibmEmail && ibmPassword) {
    console.log('Automating IBM Cloud login form...');

    // Wait for the IBM login page to load
    const ibmIdField = page.locator('input[name="username"], input#username, input[placeholder*="IBMid"]').first();
    await expect(ibmIdField).toBeVisible({ timeout: 30_000 });

    // Step 1: Enter IBMid and click Continue
    await ibmIdField.fill(ibmEmail);
    await page.locator('button:has-text("Continue")').click();

    // Step 2: Wait for password field (may redirect to W3 SSO for IBM intranet users)
    const passwordField = page.locator('input[type="password"]').first();
    await expect(passwordField).toBeVisible({ timeout: 30_000 });
    await passwordField.fill(ibmPassword);

    // Click the sign-in / log-in button
    await page.locator('button[type="submit"], button:has-text("Log in"), button:has-text("Sign in")').first().click();

    // Wait for redirect back to the OpenShift console
    await expect(page.locator('[data-test="user-dropdown"]')).toBeVisible({ timeout: 60_000 });

    await page.context().storageState({ path: AUTH_STATE_PATH });
    return;
  }

  // Tier 3: Manual login — wait for user to complete login in the headed browser
  // Use page.pause() if PWDEBUG is set, otherwise wait with a long timeout
  if (!ibmEmail) {
    console.warn('');
    console.warn('=== Manual Login Required ===');
    console.warn('No automated auth method available. Please log in within the opened browser.');
    console.warn('');
    console.warn('To enable automation, set env vars:');
    console.warn('  IBMCLOUD_EMAIL=you@example.com IBMCLOUD_PASSWORD=... (ROKS)');
    console.warn('  OC_TOKEN=sha256~... (vanilla OpenShift)');
    console.warn('');
    console.warn('Waiting up to 120s for login to complete...');
    console.warn('');
  }

  // Wait for user to complete login — detect console loaded via multiple selectors
  await page.waitForFunction(
    (baseURL) => {
      const onConsole = window.location.href.startsWith(baseURL!);
      const hasNav = document.querySelector('#page-sidebar, nav, [data-test="user-dropdown"], .pf-v5-c-page__sidebar, .co-masthead');
      return onConsole && hasNav;
    },
    baseURL!,
    { timeout: 120_000 },
  );

  await page.waitForTimeout(2000);
  await page.context().storageState({ path: AUTH_STATE_PATH });
});
