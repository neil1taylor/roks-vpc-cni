import { defineConfig, devices } from '@playwright/test';

const consoleURL = process.env.CONSOLE_URL;
if (!consoleURL) {
  throw new Error(
    'CONSOLE_URL env var is required. Set it to the OpenShift console URL, e.g.:\n' +
      '  CONSOLE_URL=$(oc whoami --show-console) npm run test:e2e:live',
  );
}

export default defineConfig({
  testDir: './e2e/live/tests',
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 15_000 },
  reporter: [['html', { outputFolder: 'playwright-report-live' }]],

  use: {
    baseURL: consoleURL,
    ignoreHTTPSErrors: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
    ...devices['Desktop Chrome'],
  },

  projects: [
    {
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
      testDir: './e2e/live',
      use: {
        // Use system Chrome for auth setup — Playwright's Chromium lacks passkey support
        channel: 'chrome',
      },
    },
    {
      name: 'live',
      dependencies: ['setup'],
      use: {
        storageState: 'e2e/live/.auth/state.json',
      },
    },
  ],
});
