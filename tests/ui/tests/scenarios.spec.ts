import { test, expect } from '@playwright/test';

test('scenarios page loads', async ({ page }) => {
  await page.goto('/scenarios');
  await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
});

test('chain created via api appears in scenarios list', async ({ page }) => {
  const resp = await page.request.post('/api/v1/chains', {
    data: { name: 'pw-list-test', steps: [] },
  });
  expect(resp.status()).toBe(201);

  await page.goto('/scenarios');
  await expect(page.getByText('pw-list-test').first()).toBeVisible({ timeout: 10_000 });
});

test('delete chain via ui removes it from list', async ({ page }) => {
  const name = `pw-delete-${Date.now()}`;
  const resp = await page.request.post('/api/v1/chains', {
    data: { name, steps: [] },
  });
  expect(resp.status()).toBe(201);

  await page.goto('/scenarios');
  await expect(page.getByText(name).first()).toBeVisible({ timeout: 10_000 });

  const row = page.locator('tr', { hasText: name });
  await row.getByRole('button', { name: 'Delete' }).click({ force: true });
  await page.getByRole('button', { name: 'Delete' }).last().click();

  // Assert the table row is gone (toast may still show the name briefly)
  await expect(page.locator('tr', { hasText: name })).toHaveCount(0, { timeout: 10_000 });
});
