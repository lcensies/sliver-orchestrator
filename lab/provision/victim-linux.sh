#!/usr/bin/env bash
# Provision a Linux victim VM:
#   1. Wait for the C2 server to come up
#   2. Install the OS-level dependencies the box needs to be a valid target:
#        - curl/ca-certificates: so an injected exploit payload can fetch and
#          run the Sliver implant staged by the scenario API
#        - python3: runtime for the deliberately-vulnerable vulnweb app
#          (deployed separately by setup.sh's linux_pivot services step)
#        - nmap/arp-scan: used by later post-exploitation recon/lateral-movement
#          chain steps that run once a session exists
#
# IMPORTANT: this script deliberately does NOT fetch or install a Sliver
# implant. There is no pre-baked foothold on this host. The only way to obtain
# a session on linux_pivot is to actually breach it — see the vulnweb service
# (setup.sh) and examples/full-attack-chain-v2.yaml's "initial_access" step,
# which exploits vulnweb's command-injection endpoint to stage the beacon.
set -euo pipefail

C2_HOST="${C2_HOST:-192.168.56.10}"
SCENARIO_API="http://${C2_HOST}:8080/api/v1"

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y --no-install-recommends curl ca-certificates nmap arp-scan python3

# ── Wait for scenario API ────────────────────────────────────────────────────
# Not required for a foothold anymore (none is auto-installed here), but a
# useful readiness signal / sanity check that the C2 is reachable before the
# vulnweb service (and later the initial_access exploit) needs it.
echo "[provision] Waiting for scenario API at ${SCENARIO_API}..."
for i in $(seq 1 60); do
  if curl -sf "${SCENARIO_API}/health" >/dev/null 2>&1; then
    echo "[provision] Scenario API is up"
    break
  fi
  if [ "${i}" -eq 60 ]; then
    echo "[provision] WARNING: Scenario API not reachable after 120s."
  fi
  sleep 2
done

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  Linux victim provisioned (no implant — clean target)"
echo "  Hostname: $(hostname)"
echo "  IP:       $(hostname -I | awk '{print $1}')"
echo "  C2:       ${C2_HOST}:31337"
echo "  Next:     run setup.sh (deploys the vulnweb + honeypot services),"
echo "            then breach it via the chain's initial_access step."
echo "═══════════════════════════════════════════════════════"
