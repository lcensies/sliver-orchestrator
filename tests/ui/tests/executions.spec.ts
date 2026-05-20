import { test, expect } from '@playwright/test';

test('executions page loads', async ({ page }) => {
  await page.goto('/executions');
  await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
});

test('execution seeded via api appears in executions list', async ({ page }) => {
  const chainResp = await page.request.post('/api/v1/chains', {
    data: {
      name: 'pw-exec-test',
      steps: [{
        id: 's1',
        technique_id: 'T1082',
        name: 'Discovery',
        args: {},
        depends_on: [],
        on_fail: 'abort',
      }],
    },
  });
  expect(chainResp.status()).toBe(201);
  const chain = await chainResp.json();
  const chainId = chain.id || chain.ID;

  await page.request.post(`/api/v1/chains/${chainId}/execute`, {
    headers: { 'X-Session-ID': 'pw-session' },
  });

  await page.goto('/executions');
  await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10_000 });
});
