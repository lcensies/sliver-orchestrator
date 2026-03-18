#!/usr/bin/env bash
# Docker entrypoint for the C2 container.
#
# Sequence:
#   1. Unpack Sliver assets into the volume on first start.
#   2. Generate operator configs (scenario + builder) once.
#   3. Start sliver-server daemon and wait until its gRPC port is open.
#   4. Start sliver-server builder (required in v1.5+ for implant compilation).
#   5. Start scenario-server (foreground, PID 1 via exec).
set -euo pipefail

OPERATOR_CFG="/etc/sliver/scenario-operator.cfg"
BUILDER_CFG="/etc/sliver/builder-operator.cfg"
C2_HOST="${C2_HOST:-172.20.0.10}"
ATOMICS_DIR="/opt/atomics"

# ── 1. Unpack assets into volume if not already present ──────────────────────
# sliver-server unpack writes Go toolchain into /root/.sliver.
# The sliver-data volume overlays /root/.sliver at runtime, so we re-run
# unpack on first start to populate the toolchain into the volume.
if [ ! -f "/root/.sliver/go/bin/go" ]; then
  echo "[c2] Unpacking Sliver assets into volume (first-run)..."
  sliver-server unpack --force
  echo "[c2] Unpack complete."
fi

# ── 2. Operator configs (once) ───────────────────────────────────────────────
if [ ! -f "${OPERATOR_CFG}" ] || [ ! -f "${BUILDER_CFG}" ]; then
  echo "[c2] Generating operator configs for ${C2_HOST}..."
  sliver-server daemon &
  SERVER_PID=$!
  for i in $(seq 1 30); do
    if nc -z 127.0.0.1 31337 2>/dev/null; then break; fi
    sleep 2
  done
  sliver-server operator \
    --name scenario \
    --lhost "${C2_HOST}" \
    --permissions all \
    --save "${OPERATOR_CFG}"
  sliver-server operator \
    --name builder \
    --lhost "${C2_HOST}" \
    --permissions all \
    --save "${BUILDER_CFG}"
  kill "${SERVER_PID}" 2>/dev/null || true
  wait "${SERVER_PID}" 2>/dev/null || true
  chmod 600 "${OPERATOR_CFG}" "${BUILDER_CFG}"
  echo "[c2] Operator configs saved."
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

# ── 4. Start builder (required in Sliver v1.5+ to compile implants) ──────────
echo "[c2] Starting sliver builder..."
sliver-server builder --config "${BUILDER_CFG}" &

# ── 5. Start scenario-server ─────────────────────────────────────────────────
echo "[c2] Starting scenario-server on ${SCENARIO_LISTEN:-:8080}..."
exec scenario-server \
  --config "${OPERATOR_CFG}" \
  --atomics "${ATOMICS_DIR}" \
  --db "${SCENARIO_DB_PATH:-/var/lib/scenario/scenario.db}" \
  --listen "${SCENARIO_LISTEN:-:8080}"
