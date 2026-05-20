# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: scenarios.spec.ts >> delete chain via ui removes it from list
- Location: tests/scenarios.spec.ts:18:5

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.click: Test timeout of 30000ms exceeded.
Call log:
  - waiting for getByRole('button', { name: 'Delete' }).last()
    - locator resolved to <button type="button" aria-label="Delete" data-variant="subtle" class="mantine-focus-auto mantine-active m_8d3f4000 mantine-ActionIcon-root m_87cf2631 mantine-UnstyledButton-root">…</button>
  - attempting click action
    2 × waiting for element to be visible, enabled and stable
      - element is visible, enabled and stable
      - scrolling into view if needed
      - done scrolling
      - <div data-fixed="true" class="mantine-Modal-overlay m_9814e45f mantine-Overlay-root"></div> from <div data-portal="true">…</div> subtree intercepts pointer events
    - retrying click action
    - waiting 20ms
    2 × waiting for element to be visible, enabled and stable
      - element is visible, enabled and stable
      - scrolling into view if needed
      - done scrolling
      - <div data-fixed="true" class="mantine-Modal-overlay m_9814e45f mantine-Overlay-root"></div> from <div data-portal="true">…</div> subtree intercepts pointer events
    - retrying click action
      - waiting 100ms
    59 × waiting for element to be visible, enabled and stable
       - element is visible, enabled and stable
       - scrolling into view if needed
       - done scrolling
       - <div data-fixed="true" class="mantine-Modal-overlay m_9814e45f mantine-Overlay-root"></div> from <div data-portal="true">…</div> subtree intercepts pointer events
     - retrying click action
       - waiting 500ms

```

# Page snapshot

```yaml
- generic [ref=e1]:
  - generic [ref=e3]:
    - banner [ref=e4]:
      - generic [ref=e5]:
        - button [ref=e6] [cursor=pointer]
        - generic [ref=e8]:
          - paragraph [ref=e9]: Attack Scenario Platform
          - generic [ref=e10]:
            - generic [ref=e12]: Mentor
            - button "RU" [ref=e13] [cursor=pointer]:
              - generic [ref=e14]:
                - img [ref=e16]
                - generic [ref=e21]: RU
    - navigation [ref=e22]:
      - generic [ref=e23]:
        - generic [ref=e24] [cursor=pointer]:
          - img [ref=e26]
          - generic [ref=e30]: Dashboard
        - generic [ref=e31] [cursor=pointer]:
          - img [ref=e33]
          - generic [ref=e36]: Scenarios
        - generic [ref=e37] [cursor=pointer]:
          - img [ref=e39]
          - generic [ref=e41]: Atomics
        - generic [ref=e42] [cursor=pointer]:
          - img [ref=e44]
          - generic [ref=e46]: Sessions
        - generic [ref=e47] [cursor=pointer]:
          - img [ref=e49]
          - generic [ref=e51]: Executions
    - main [ref=e52]:
      - generic [ref=e53]:
        - generic [ref=e54]:
          - heading "Scenarios" [level=2] [ref=e55]
          - generic [ref=e56]:
            - button "Import YAML" [ref=e57] [cursor=pointer]:
              - generic [ref=e58]:
                - img [ref=e60]
                - generic [ref=e63]: Import YAML
            - button "New Scenario" [ref=e64] [cursor=pointer]:
              - generic [ref=e65]:
                - img [ref=e67]
                - generic [ref=e68]: New Scenario
        - generic [ref=e70]:
          - img [ref=e72]
          - textbox "Search..." [ref=e75]
        - table "Scenarios list" [ref=e80]:
          - rowgroup [ref=e81]:
            - row "Name Description Tags MITRE Tactics Actions" [ref=e82]:
              - columnheader "Name" [ref=e83] [cursor=pointer]
              - columnheader "Description" [ref=e84] [cursor=pointer]
              - columnheader "Tags" [ref=e85]
              - columnheader "MITRE Tactics" [ref=e86]
              - columnheader "Actions" [ref=e87]
          - rowgroup [ref=e88]:
            - row "pw-delete-1779291042402 Edit Execute Clone Delete" [ref=e89]:
              - cell "pw-delete-1779291042402" [ref=e90]:
                - strong [ref=e91]: pw-delete-1779291042402
              - cell [ref=e92]
              - cell [ref=e93]
              - cell [ref=e94]
              - cell "Edit Execute Clone Delete" [ref=e95]:
                - generic [ref=e96]:
                  - button "Edit" [ref=e97] [cursor=pointer]:
                    - img [ref=e99]
                  - button "Execute" [ref=e102] [cursor=pointer]:
                    - img [ref=e104]
                  - button "Clone" [ref=e106] [cursor=pointer]:
                    - img [ref=e108]
                  - button "Delete" [ref=e111] [cursor=pointer]:
                    - img [ref=e113]
            - row "pw-list-test Edit Execute Clone Delete" [ref=e116]:
              - cell "pw-list-test" [ref=e117]:
                - strong [ref=e118]: pw-list-test
              - cell [ref=e119]
              - cell [ref=e120]
              - cell [ref=e121]
              - cell "Edit Execute Clone Delete" [ref=e122]:
                - generic [ref=e123]:
                  - button "Edit" [ref=e124] [cursor=pointer]:
                    - img [ref=e126]
                  - button "Execute" [ref=e129] [cursor=pointer]:
                    - img [ref=e131]
                  - button "Clone" [ref=e133] [cursor=pointer]:
                    - img [ref=e135]
                  - button "Delete" [ref=e138] [cursor=pointer]:
                    - img [ref=e140]
            - row "pw-exec-test Edit Execute Clone Delete" [ref=e143]:
              - cell "pw-exec-test" [ref=e144]:
                - strong [ref=e145]: pw-exec-test
              - cell [ref=e146]
              - cell [ref=e147]
              - cell [ref=e148]
              - cell "Edit Execute Clone Delete" [ref=e149]:
                - generic [ref=e150]:
                  - button "Edit" [ref=e151] [cursor=pointer]:
                    - img [ref=e153]
                  - button "Execute" [ref=e156] [cursor=pointer]:
                    - img [ref=e158]
                  - button "Clone" [ref=e160] [cursor=pointer]:
                    - img [ref=e162]
                  - button "Delete" [ref=e165] [cursor=pointer]:
                    - img [ref=e167]
            - row "pw-list-test Edit Execute Clone Delete" [ref=e170]:
              - cell "pw-list-test" [ref=e171]:
                - strong [ref=e172]: pw-list-test
              - cell [ref=e173]
              - cell [ref=e174]
              - cell [ref=e175]
              - cell "Edit Execute Clone Delete" [ref=e176]:
                - generic [ref=e177]:
                  - button "Edit" [ref=e178] [cursor=pointer]:
                    - img [ref=e180]
                  - button "Execute" [ref=e183] [cursor=pointer]:
                    - img [ref=e185]
                  - button "Clone" [ref=e187] [cursor=pointer]:
                    - img [ref=e189]
                  - button "Delete" [ref=e192] [cursor=pointer]:
                    - img [ref=e194]
            - row "pw-exec-test Edit Execute Clone Delete" [ref=e197]:
              - cell "pw-exec-test" [ref=e198]:
                - strong [ref=e199]: pw-exec-test
              - cell [ref=e200]
              - cell [ref=e201]
              - cell [ref=e202]
              - cell "Edit Execute Clone Delete" [ref=e203]:
                - generic [ref=e204]:
                  - button "Edit" [ref=e205] [cursor=pointer]:
                    - img [ref=e207]
                  - button "Execute" [ref=e210] [cursor=pointer]:
                    - img [ref=e212]
                  - button "Clone" [ref=e214] [cursor=pointer]:
                    - img [ref=e216]
                  - button "Delete" [ref=e219] [cursor=pointer]:
                    - img [ref=e221]
            - row "pw-delete-test Edit Execute Clone Delete" [ref=e224]:
              - cell "pw-delete-test" [ref=e225]:
                - strong [ref=e226]: pw-delete-test
              - cell [ref=e227]
              - cell [ref=e228]
              - cell [ref=e229]
              - cell "Edit Execute Clone Delete" [ref=e230]:
                - generic [ref=e231]:
                  - button "Edit" [ref=e232] [cursor=pointer]:
                    - img [ref=e234]
                  - button "Execute" [ref=e237] [cursor=pointer]:
                    - img [ref=e239]
                  - button "Clone" [ref=e241] [cursor=pointer]:
                    - img [ref=e243]
                  - button "Delete" [ref=e246] [cursor=pointer]:
                    - img [ref=e248]
            - row "pw-delete-test Edit Execute Clone Delete" [ref=e251]:
              - cell "pw-delete-test" [ref=e252]:
                - strong [ref=e253]: pw-delete-test
              - cell [ref=e254]
              - cell [ref=e255]
              - cell [ref=e256]
              - cell "Edit Execute Clone Delete" [ref=e257]:
                - generic [ref=e258]:
                  - button "Edit" [ref=e259] [cursor=pointer]:
                    - img [ref=e261]
                  - button "Execute" [ref=e264] [cursor=pointer]:
                    - img [ref=e266]
                  - button "Clone" [ref=e268] [cursor=pointer]:
                    - img [ref=e270]
                  - button "Delete" [ref=e273] [cursor=pointer]:
                    - img [ref=e275]
            - row "pw-list-test Edit Execute Clone Delete" [ref=e278]:
              - cell "pw-list-test" [ref=e279]:
                - strong [ref=e280]: pw-list-test
              - cell [ref=e281]
              - cell [ref=e282]
              - cell [ref=e283]
              - cell "Edit Execute Clone Delete" [ref=e284]:
                - generic [ref=e285]:
                  - button "Edit" [ref=e286] [cursor=pointer]:
                    - img [ref=e288]
                  - button "Execute" [ref=e291] [cursor=pointer]:
                    - img [ref=e293]
                  - button "Clone" [ref=e295] [cursor=pointer]:
                    - img [ref=e297]
                  - button "Delete" [ref=e300] [cursor=pointer]:
                    - img [ref=e302]
            - row "pw-exec-test Edit Execute Clone Delete" [ref=e305]:
              - cell "pw-exec-test" [ref=e306]:
                - strong [ref=e307]: pw-exec-test
              - cell [ref=e308]
              - cell [ref=e309]
              - cell [ref=e310]
              - cell "Edit Execute Clone Delete" [ref=e311]:
                - generic [ref=e312]:
                  - button "Edit" [ref=e313] [cursor=pointer]:
                    - img [ref=e315]
                  - button "Execute" [ref=e318] [cursor=pointer]:
                    - img [ref=e320]
                  - button "Clone" [ref=e322] [cursor=pointer]:
                    - img [ref=e324]
                  - button "Delete" [ref=e327] [cursor=pointer]:
                    - img [ref=e329]
  - dialog "Delete" [ref=e333]:
    - banner [ref=e334]:
      - heading "Delete" [level=2] [ref=e335]
      - button [active] [ref=e336] [cursor=pointer]:
        - img [ref=e337]
    - generic [ref=e339]:
      - paragraph [ref=e341]: Are you sure you want to delete this item?
      - generic [ref=e342]:
        - button "Cancel" [ref=e343] [cursor=pointer]:
          - generic [ref=e345]: Cancel
        - button "Delete" [ref=e346] [cursor=pointer]:
          - generic [ref=e348]: Delete
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test';
  2  | 
  3  | test('scenarios page loads', async ({ page }) => {
  4  |   await page.goto('/scenarios');
  5  |   await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  6  | });
  7  | 
  8  | test('chain created via api appears in scenarios list', async ({ page }) => {
  9  |   const resp = await page.request.post('/api/v1/chains', {
  10 |     data: { name: 'pw-list-test', steps: [] },
  11 |   });
  12 |   expect(resp.status()).toBe(201);
  13 | 
  14 |   await page.goto('/scenarios');
  15 |   await expect(page.getByText('pw-list-test').first()).toBeVisible({ timeout: 10_000 });
  16 | });
  17 | 
  18 | test('delete chain via ui removes it from list', async ({ page }) => {
  19 |   const name = `pw-delete-${Date.now()}`;
  20 |   const resp = await page.request.post('/api/v1/chains', {
  21 |     data: { name, steps: [] },
  22 |   });
  23 |   expect(resp.status()).toBe(201);
  24 | 
  25 |   await page.goto('/scenarios');
  26 |   await expect(page.getByText(name).first()).toBeVisible({ timeout: 10_000 });
  27 | 
  28 |   const row = page.locator('tr', { hasText: name });
  29 |   await row.getByRole('button', { name: 'Delete' }).click({ force: true });
> 30 |   await page.getByRole('button', { name: 'Delete' }).last().click();
     |                                                             ^ Error: locator.click: Test timeout of 30000ms exceeded.
  31 | 
  32 |   // Assert the table row is gone (toast may still show the name briefly)
  33 |   await expect(page.locator('tr', { hasText: name })).toHaveCount(0, { timeout: 10_000 });
  34 | });
  35 | 
```