import { test, expect } from '@playwright/test';

test('atomics page shows T1082', async ({ page }) => {
  await page.goto('/atomics');
  await expect(page.getByText('T1082').first()).toBeVisible({ timeout: 15_000 });
});

test('atomics page shows technique name', async ({ page }) => {
  await page.goto('/atomics');
  await expect(page.getByText(/system information discovery/i).first()).toBeVisible({ timeout: 15_000 });
});
