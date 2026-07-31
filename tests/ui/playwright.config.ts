import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    // Opt-in: run against a system-provided browser (e.g. on NixOS where the
    // Playwright-downloaded chromium can't run). Unset → default managed browser.
    launchOptions: process.env.PW_CHROME
      ? { executablePath: process.env.PW_CHROME, args: ['--no-sandbox'] }
      : undefined,
  },
  reporter: [['list'], ['html', { open: 'never' }]],
});
