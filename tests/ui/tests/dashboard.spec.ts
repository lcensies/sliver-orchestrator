import { test, expect } from '@playwright/test';

test('health card shows healthy status', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByText('Healthy').first()).toBeVisible({ timeout: 15_000 });
});

test('dashboard nav links are present', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('a').filter({ hasText: /scenario/i }).first()).toBeVisible({ timeout: 10_000 });
});
