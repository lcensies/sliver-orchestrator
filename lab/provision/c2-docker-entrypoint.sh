#!/usr/bin/env bash
# Docker entrypoint for the C2 container.
#
# Sequence:
#   1. Use atomics mounted at /opt/atomics.
#   2. Generate Sliver operator config once (volume-persisted between restarts).
#   3. Start sliver-server daemon and wait until its gRPC port is open.
#   4. Start scenario-server (foreground, PID 1 via exec).
set -euo pipefail

OPERATOR_CFG="/etc/sliver/scenario-operator.cfg"
C2_HOST="${C2_HOST:-172.20.0.10}"
ATOMICS_DIR="/opt/atomics"

# ── 2. Operator config (once) ────────────────────────────────────────────────
if [ ! -f "${OPERATOR_CFG}" ]; then
  echo "[c2] Generating operator config for ${C2_HOST}..."
  # Start server just long enough to produce the config, then stop it.
  sliver-server daemon &
  SERVER_PID=$!
  # Wait for gRPC port 31337 before asking for operator config.
  for i in $(seq 1 30); do
    if nc -z 127.0.0.1 31337 2>/dev/null; then break; fi
    sleep 2
  done
  sliver-server operator \
    --name scenario \
    --lhost "${C2_HOST}" \
    --permissions all \
    --save "${OPERATOR_CFG}"
  kill "${SERVER_PID}" 2>/dev/null || true
  wait "${SERVER_PID}" 2>/dev/null || true
  chmod 600 "${OPERATOR_CFG}"
  echo "[c2] Operator config saved: ${OPERATOR_CFG}"
fi

# ── 3. Start sliver-server daemon ────────────────────────────────────────────
echo "[c2] Starting sliver-server daemon..."
sliver-server daemon &

echo "[c2] Waiting for sliver-server gRPC (port 31337)..."
for i in $(seq 1 60); do
  if nc -z 127.0.0.1 31337 2>/dev/null; then
    echo "[c2] sliver-server ready"
    break
  fi
  sleep 2
done

# ── 4. Start scenario-server ─────────────────────────────────────────────────
echo "[c2] Starting scenario-server on ${SCENARIO_LISTEN:-:8080}..."
exec scenario-server \
  --config "${OPERATOR_CFG}" \
  --atomics "${ATOMICS_DIR}" \
  --db "${SCENARIO_DB_PATH:-/var/lib/scenario/scenario.db}" \
  --listen "${SCENARIO_LISTEN:-:8080}"
