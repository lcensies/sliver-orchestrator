# sliver-orchestrator-workspace

Runtime library for the [sliver-orchestrator](https://github.com/lcensies/sliver-orchestrator)
cyber range. This repo is a **git submodule** of the main orchestrator and holds
only *runtime* content — never a copy of the orchestrator's Go application.

```
atomics/                     Atomic Red Team YAML library (fetched, see below)
scenarios/                   Self-contained attack scenarios + their fixtures
  windows-pth/               Windows Pass-the-Hash demo (chain + SAM fixture)
services/                    Deliberately-vulnerable / decoy services for targets
  honeypot/                  Resolver command-injection decoy + fake IP-camera
tools/                       Operator helper scripts (e.g. interactive lab CLI)
```

## Atomics

The Docker lab mounts atomics from `../sliver-orchestrator-workspace/atomics`
(a sibling of the main repo). Populate them from the main repo with:

```bash
./atomic/fetch.sh ../sliver-orchestrator-workspace/atomics
```

Atomics are intentionally **not committed** here (large, upstream-sourced). See the
main repo's README for the full lab-setup flow.
