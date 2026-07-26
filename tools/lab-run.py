#!/usr/bin/env python3
"""
lab-run.py — Interactive CLI for Sliver C2 Orchestration Cyber Range
Usage: ./lab-run.py
"""
import sys, json, time, requests, sseclient

API = "http://192.168.56.5:8080/api/v1"

# ── Colors ───────────────────────────────────────────────────────────────────
R='\033[0;31m'; G='\033[0;32m'; Y='\033[1;33m'; B='\033[0;34m'; C='\033[0;36m'; W='\033[1;37m'; NC='\033[0m'; BOLD='\033[1m'

def ok(s):   print(f"  {G}✓{NC} {s}")
def err(s):  print(f"  {R}✗{NC} {s}")
def info(s): print(f"  {C}→{NC} {s}")
def warn(s): print(f"  {Y}!{NC} {s}")

def banner():
    print(f"""
{B}╔══════════════════════════════════════════════════════════╗
║     Sliver C2 Orchestration — Interactive Lab Runner    ║
╚══════════════════════════════════════════════════════════╝{NC}
""")

def get_chains():
    try:
        r = requests.get(f"{API}/chains", timeout=5)
        return r.json()
    except Exception as e:
        print(f"{R}[!] Cannot reach API: {e}{NC}")
        sys.exit(1)

def get_sessions():
    try:
        r = requests.get(f"{API}/sessions", timeout=10)
        return r.json()
    except:
        return []

def pick(items, label_fn, title):
    print(f"\n{BOLD}{title}{NC}")
    print("─" * 50)
    for i, item in enumerate(items, 1):
        print(f"  {Y}{i}{NC}. {label_fn(item)}")
    print()
    while True:
        try:
            choice = input(f"Select [1-{len(items)}]: ").strip()
            idx = int(choice) - 1
            if 0 <= idx < len(items):
                return items[idx]
            print(f"  {R}Invalid — enter 1 to {len(items)}{NC}")
        except (ValueError, KeyboardInterrupt):
            print(f"\n{Y}Cancelled.{NC}")
            sys.exit(0)

def execute_chain(chain_id, session_id):
    r = requests.post(
        f"{API}/chains/{chain_id}/execute",
        json={"session_id": session_id},
        timeout=10
    )
    data = r.json()
    return data.get("execution_id") or data.get("id") or data.get("ID")

def stream_execution(exec_id):
    url = f"{API}/executions/{exec_id}/stream"
    try:
        r = requests.get(url, stream=True, timeout=300)
        client = sseclient.SSEClient(r)
        step_num = 0
        for event in client.events():
            if not event.data:
                continue
            try:
                d = json.loads(event.data)
            except:
                continue

            if event.event == "step_start":
                step_num += 1
                print(f"\n  {B}── {d.get('step_id','')} ──{NC}")

            elif event.event == "step_done":
                sid = d.get("step_id","")
                ms  = d.get("duration_ms", 0)
                out = (d.get("stdout","") or "").strip()[:80]
                ok(f"{BOLD}{sid}{NC} ({ms}ms)" + (f"\n     {C}{out}{NC}" if out else ""))

            elif event.event == "step_failed":
                sid = d.get("step_id","")
                ms  = d.get("duration_ms", 0)
                e   = (d.get("error","") or d.get("stderr","") or "")[:80]
                err(f"{BOLD}{sid}{NC} ({ms}ms)" + (f"\n     {R}{e}{NC}" if e else ""))

            elif event.event == "step_skipped":
                sid = d.get("step_id","")
                warn(f"{sid} — skipped")

            elif event.event == "chain_done":
                msg = d.get("message","")
                print(f"\n{G}{'═'*50}")
                print(f"  ✓ Execution complete{NC}" + (f" — {msg}" if msg else ""))
                print(f"{G}{'═'*50}{NC}\n")
                break

            elif event.event == "chain_failed":
                msg = d.get("message","")
                print(f"\n{R}{'═'*50}")
                print(f"  ✗ Execution failed — {msg}")
                print(f"{'═'*50}{NC}\n")
                break

    except KeyboardInterrupt:
        print(f"\n{Y}[!] Interrupted.{NC}")
    except Exception as e:
        print(f"\n{R}[!] Stream error: {e}{NC}")
        # Fallback — poll:
        print(f"  Polling for result...")
        time.sleep(5)
        r2 = requests.get(f"{API}/executions/{exec_id}", timeout=10)
        data = r2.json()
        status = data.get("execution",{}).get("Status","unknown")
        steps  = data.get("steps",[])
        done   = sum(1 for s in steps if s.get("Status")=="done")
        failed = sum(1 for s in steps if s.get("Status")=="failed")
        print(f"  Status: {status} | done:{done} failed:{failed}")

def main():
    banner()

    # Health check:
    try:
        requests.get(f"{API}/health", timeout=3)
    except:
        print(f"{R}[!] Backend unreachable at {API}{NC}")
        print(f"    Run: vagrant up c2")
        sys.exit(1)

    # Get chains:
    chains = get_chains()
    if not chains:
        print(f"{R}[!] No chains found{NC}")
        sys.exit(1)

    # Pick chain:
    chain = pick(
        chains,
        lambda c: f"{W}{c.get('name','?')}{NC}  {C}({len(c.get('steps',[]))} steps){NC}",
        "Available Scenarios"
    )

    # Get sessions:
    print(f"\n{C}[→] Loading live sessions...{NC}")
    sessions = get_sessions()
    if not sessions:
        print(f"{R}[!] No active sessions — start VMs first{NC}")
        sys.exit(1)

    # Pick session:
    session = pick(
        sessions,
        lambda s: f"{W}{s.get('name','?')}{NC}  {s.get('os','')}  {C}{s.get('hostname','')}{NC}  {s.get('username','')}  pid:{s.get('pid','')}",
        "Available Sessions"
    )

    chain_id   = chain.get("id") or chain.get("ID")
    session_id = session.get("id") or session.get("ID")
    chain_name = chain.get("name","?")

    print(f"\n{G}[✓] Executing:{NC} {BOLD}{chain_name}{NC}")
    print(f"    Session: {session.get('hostname','')} ({session.get('os','')})")
    print(f"{'─'*50}")

    exec_id = execute_chain(chain_id, session_id)
    if not exec_id:
        print(f"{R}[!] Failed to start execution{NC}")
        sys.exit(1)

    info(f"Execution ID: {exec_id}")
    stream_execution(exec_id)

if __name__ == "__main__":
    main()
