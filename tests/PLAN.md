# sliver-orchestrator — Test Plan

## What already exists

| Location | What it tests |
|---|---|
| `atomic/library_test.go` | ART YAML loading, PathToAtomicsFolder substitution, Resolve() |
| `chain/condition_test.go` | Sigma-style condition parsing and evaluation |
| `flexible-platform/…/executionSlice.test.ts` | Redux slice reducer |
| `flexible-platform/…/dagUtils.test.ts` | DAG utilities |
| `flexible-platform/…/yamlUtils.test.ts` | YAML serialisation |
| `flexible-platform/…/formatUtils.test.ts` | Display formatting |

## What's missing

- Store layer (SQLite CRUD)
- Chain resolver (cycle detection, topological order, `any`/`all` groups)
- Chain executor (step scheduling, `on_fail` policies, variable capture, conditions)
- HTTP handlers (full request/response cycle without a Sliver dependency)
- SSE streaming endpoint
- CORS preflight
- Python E2E against a real running server
- Playwright UI tests against `flexible-platform` + real backend

---

## Test layers

```
Layer 4 — UI E2E (Playwright)        flexible-platform + real API server
Layer 3 — API E2E (pytest + httpx)   real server, temp SQLite
Layer 2 — Go integration             httptest.Server, real store, mock Sliver RPC
Layer 1 — Go unit                    pure logic, no I/O
```

---

## Layer 1 — Go unit tests

Standard `go test`, no extra deps. Each test opens a real SQLite DB in `t.TempDir()`.

### `store/store_test.go`

| Test | What it checks |
|---|---|
| `TestChainCRUD` | Create → Get → List → Update → Delete round-trip |
| `TestChainListOrdering` | Newest chain first |
| `TestGetChainNotFound` | Error on unknown ID |
| `TestExecutionCRUD` | Create → Get → UpdateStatus → ListByChainID |
| `TestExecutionListAll` | Empty chain_id filter returns everything |
| `TestStepLogUpsert` | Second write to same step_id updates, not inserts |
| `TestStepLogOrdering` | GetStepLogs returns created_at ASC |
| `TestCountStepLogs` | Count after a given ID |

### `chain/resolver_test.go`

| Test | What it checks |
|---|---|
| `TestResolveLinearChain` | A→B→C returns topological order |
| `TestResolveDiamond` | A→B, A→C, B→D, C→D — all 4 resolved |
| `TestResolveParallelRoots` | Multiple roots all included |
| `TestResolveCycleDetected` | Mutual dependency → error |
| `TestResolveDuplicateID` | Duplicate step ID → error |
| `TestResolveUnknownDep` | Reference to nonexistent step → error |
| `TestResolveEmptyID` | Empty step ID → error |
| `TestReadyStepsAnyGroup` | `{any:[b,c]}` — ready when first of b/c settles |
| `TestReadyStepsAnyGroupHopeless` | All members failed/skipped → gate opens |
| `TestReadyStepsAllGroup` | `{all:[b,c]}` — both must settle |

### `chain/executor_test.go`

Uses `fakeStepExecutor` (in-process mock) and temp SQLite.

| Test | What it checks |
|---|---|
| `TestExecutorLinearHappyPath` | 3 sequential steps succeed, events emitted |
| `TestExecutorParallelSteps` | Steps without shared deps run concurrently |
| `TestExecutorOutputVar` | stdout → `output_var` → substituted in next step |
| `TestExecutorOutputExtract` | Regex group extracted → named var forwarded |
| `TestExecutorOnFailAbort` | First failure with `abort` stops everything |
| `TestExecutorOnFailContinue` | `continue` — siblings run, chain still fails |
| `TestExecutorOnFailContinueNoErr` | `continue_no_err` — failure silent, chain succeeds |
| `TestExecutorSkipDependents` | `skip_dependents` — downstream steps skipped |
| `TestExecutorConditionSkip` | False condition → skip, not fail |
| `TestExecutorConditionMissingVar` | Var not in scope → skip |
| `TestExecutorContextCancel` | `ctx.Cancel()` mid-run → ctx.Err() returned cleanly |
| `TestExecutorTimeout` | Per-step deadline causes step failure |
| `TestExecutorEventsChannel` | All EventType values in correct order |
| `TestExecutorAtomicResolution` | Mock AtomicResolver substitutes technique |
| `TestExecutorStepLogPersisted` | step_logs rows match emitted events |

---

## Layer 2 — Go integration tests

**Location:** `tests/api/` — package `api_test`.

`httptest.Server` + real temp SQLite + `stubRPC` implementing the full `rpcpb.SliverRPCClient` interface. No real Sliver C2 needed; stub returns empty protos for `GetSessions`, `ImplantBuilds`, etc.

### Health / CORS

| Test | Expected |
|---|---|
| `TestHealthOK` | 200, `{"status":"ok"}` |
| `TestCORSPreflight` | OPTIONS → 204 + CORS headers |
| `TestCORSResponseHeaders` | CORS headers on every response |

### Atomics

| Test | Expected |
|---|---|
| `TestListAtomicsEmpty` | 200, `[]` |
| `TestListAtomicsWithData` | Loaded techniques sorted by ID |
| `TestListAtomicsFilterTactic` | `?tactic=execution` filters correctly |
| `TestListAtomicsFilterPlatform` | `?platform=linux` filters correctly |
| `TestGetAtomicFound` | 200, full technique JSON |
| `TestGetAtomicNotFound` | 404 |

### Chains CRUD

| Test | Expected |
|---|---|
| `TestListChainsEmpty` | 200, `[]` |
| `TestCreateChainJSON` | 201, ID generated |
| `TestCreateChainYAML` | 201 with `Content-Type: application/yaml` body |
| `TestCreateChainWithExplicitID` | Uses provided ID (upsert) |
| `TestCreateChainMissingName` | 400 |
| `TestCreateChainInvalidDAG` | 400 on cycle |
| `TestGetChainFound` | 200 |
| `TestGetChainNotFound` | 404 |
| `TestUpdateChain` | 200, fields updated |
| `TestUpdateChainNotFound` | 404 |
| `TestDeleteChain` | 204, subsequent GET → 404 |

### Execute

| Test | Expected |
|---|---|
| `TestDryRunReturnsOrder` | `dry_run:true`, `order` matches topological sort |
| `TestExecuteMissingSessionID` | 400 |
| `TestExecuteChainNotFound` | 404 |

### Executions

| Test | Expected |
|---|---|
| `TestListExecutionsEmpty` | 200, `[]` |
| `TestListExecutionsFilterByChain` | `?chain_id=X` returns only matching |
| `TestGetExecutionWithSteps` | 200, `{execution:…, steps:[…]}` |
| `TestGetExecutionNotFound` | 404 |

### Cancel

| Test | Expected |
|---|---|
| `TestCancelRunningExecution` | 200, status → `cancelled` |
| `TestCancelAlreadyDone` | 409 |
| `TestCancelNotFound` | 404 |

### SSE stream

| Test | Expected |
|---|---|
| `TestStreamReplaysDoneExecution` | Replays step logs, emits `done`, closes |
| `TestStreamNotFound` | 404 |

### Sessions proxy

| Test | Expected |
|---|---|
| `TestListSessionsProxied` | Delegates to stub RPC, returns mapped JSON |
| `TestListSessionsRPCError` | RPC error → 502 |

---

## Layer 3 — Python E2E (pytest)

**Location:** `tests/e2e/`

```
tests/e2e/
  pyproject.toml      # pytest, httpx, pyyaml, sseclient-py
  conftest.py         # session fixture: start testserver, yield base_url, teardown
  test_health.py
  test_chains.py
  test_atomics.py
  test_executions.py
  test_sse.py
```

`conftest.py` starts `/tmp/testserver` (built from `tests/cmd/testserver/`) on a random port, polls `/api/v1/health` until ready, tears down on session end. Points at `/tmp/test-atomics` for the T1082 fixture.

| File | What it covers |
|---|---|
| `test_health.py` | GET /health → 200; OPTIONS preflight → CORS headers |
| `test_chains.py` | Full CRUD; YAML body; explicit ID upsert; cycle → 400; list ordering |
| `test_atomics.py` | List; tactic filter; get by ID; 404 |
| `test_executions.py` | List, filter by chain_id, get with steps, 404, dry-run |
| `test_sse.py` | Stream on finished execution replays logs + done event; cancel |

---

## Layer 4 — UI E2E (Playwright)

**Location:** `~/repos/flexible-platform/e2e/`

Same testserver binary started via a global setup fixture; pre-seeded SQLite with a few chains and one synthetic "done" execution.

```
flexible-platform/e2e/
  global-setup.ts
  global-teardown.ts
  fixtures.ts
  dashboard.spec.ts
  scenarios.spec.ts
  editor.spec.ts
  executions.spec.ts
```

| File | What it covers |
|---|---|
| `dashboard.spec.ts` | Health card shows "ok"; sessions list; nav links |
| `scenarios.spec.ts` | List shows seeded chains; delete confirm modal; empty state |
| `editor.spec.ts` | Create chain → add step → save → appears in list; YAML import; DAG view; unsaved-changes warning |
| `executions.spec.ts` | List; click row → viewer; step logs visible; done execution statuses; cancel button |

---

## Docker / CI

`tests/docker-compose.test.yml` runs the API server in a container and Playwright in a second container against it.

```yaml
services:
  api:
    build: { context: ../, dockerfile: Dockerfile }
    environment:
      SCENARIO_DB: /data/test.db
      SCENARIO_ATOMICS_DIR: /data/atomics
      ALLOW_ORIGIN: http://localhost:5173
    ports: ["18765:8080"]
    volumes: [testdata:/data]

  playwright:
    image: mcr.microsoft.com/playwright:v1.44.0-jammy
    working_dir: /app
    volumes:
      - ../../flexible-platform:/app
      - testdata:/data
    environment:
      API_BASE_URL: http://api:8080/api/v1
    command: npx playwright test
    depends_on: [api]

volumes:
  testdata:
```

```bash
# dev-vm-3 needs sudo for docker
cd ~/repos/sliver-orchestrator/tests
sudo docker compose -f docker-compose.test.yml up --abort-on-container-exit
```

---

## Running tests

```bash
# Go (dev-vm-3)
cd ~/repos/sliver-orchestrator
CGO_ENABLED=1 /home/ubuntu/go-toolchain/bin/go test ./store/... ./chain/... ./tests/api/... -v

# Python E2E
cd ~/repos/sliver-orchestrator/tests/e2e
uv run pytest -v

# Playwright
cd ~/repos/flexible-platform
npx playwright test

# Full Docker CI
cd ~/repos/sliver-orchestrator/tests
sudo docker compose -f docker-compose.test.yml up --abort-on-container-exit --build
```

---

## Design notes

- **No real Sliver C2 required.** `stubRPC` satisfies all 186 methods of `rpcpb.SliverRPCClient`; the E2E binary starts without a `-config` flag.
- **Real SQLite everywhere.** Using temp-file SQLite in all layers means schema migrations get exercised and mock drift is not a concern.
- **Python E2E is language-independent.** It catches serialisation quirks (PascalCase field names in execution responses, etc.) that Go tests sharing internal types would miss.
- **Playwright talks to the real API** — not a mock — so CORS, SSE, and Redux normalisation all get exercised together.
