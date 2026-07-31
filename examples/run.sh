#!/usr/bin/env bash
# examples/run.sh — Load and execute a chain YAML against the lab.
#
# Works for both session-based chains (needs a live Sliver session) and
# session-less chains (e.g. initial_access, which obtain their own session).
#
# Usage (from repo root):
#   ./examples/run.sh <chain.yaml> [api_url] [session_id]
#
# Examples:
#   ./examples/run.sh examples/t1082-basic-discovery.yaml            # auto-pick a session
#   ./examples/run.sh examples/initial-access-web.yaml               # session-less (chain breaches a target)
#   ./examples/run.sh examples/linux-full-chain.yaml http://172.20.0.10:8080/api/v1
#   ./examples/run.sh examples/win-discovery.yaml "" <session_id>    # force a specific session
#
# Prerequisites: curl, jq
set -euo pipefail

CHAIN_FILE="${1:-}"
API="${2:-}"; API="${API:-http://127.0.0.1:18080/api/v1}"
SESSION_ARG="${3:-}"

if [ -z "${CHAIN_FILE}" ]; then
  echo "Usage: $0 <chain.yaml> [api_url]"
  exit 1
fi

if ! command -v jq &>/dev/null; then
  echo "Error: jq is required (apt install jq)"
  exit 1
fi

# ── Health check ──────────────────────────────────────────────────────────────
echo "[run] Checking scenario API at ${API}..."
if ! curl -sf "${API}/health" > /dev/null; then
  echo "[run] ERROR: scenario API is not reachable at ${API}"
  echo "[run] Start the stack first:  make up   (or make up-web / make up-victim)"
  exit 1
fi

# ── Session selection ───────────────────────────────────────────────────────────
# Explicit session id ($3) wins; otherwise auto-pick the first live session.
# A missing session is NOT fatal — initial-access chains obtain their own session,
# so we fall back to a session-less execution and let the chain create one.
SESSION="${SESSION_ARG}"
if [ -z "${SESSION}" ]; then
  echo "[run] Fetching sessions..."
  SESSION=$(curl -sf "${API}/sessions" | jq -r '.[0].id // empty')
fi

if [ -n "${SESSION}" ]; then
  SESSION_NAME=$(curl -sf "${API}/sessions" | jq -r --arg id "${SESSION}" '.[] | select(.id==$id) | .name' | head -1)
  echo "[run] Using session: ${SESSION} (${SESSION_NAME:-unknown})"
  DRY_BODY="{\"session_id\":\"${SESSION}\",\"dry_run\":true}"
  EXEC_BODY="{\"session_id\":\"${SESSION}\"}"
else
  echo "[run] No active session — running session-less (chain must obtain its own, e.g. initial_access)."
  DRY_BODY="{\"dry_run\":true}"
  EXEC_BODY="{}"
fi

# ── Upload chain ───────────────────────────────────────────────────────────────
echo "[run] Loading chain from ${CHAIN_FILE}..."
CHAIN_RESP=$(curl -sf -X POST "${API}/chains" \
  -H 'Content-Type: application/yaml' \
  --data-binary "@${CHAIN_FILE}")

CHAIN_ID=$(echo "${CHAIN_RESP}" | jq -r '.id')
CHAIN_NAME=$(echo "${CHAIN_RESP}" | jq -r '.name')
echo "[run] Chain registered: ${CHAIN_NAME} (${CHAIN_ID})"

# ── Dry-run validation ─────────────────────────────────────────────────────────
echo "[run] Dry-run (DAG validation)..."
DRY=$(curl -sf -X POST "${API}/chains/${CHAIN_ID}/execute" \
  -H 'Content-Type: application/json' \
  -d "${DRY_BODY}")
echo "[run] Resolved step order: $(echo "${DRY}" | jq -r '.order | join(" → ")')"

# ── Execute ────────────────────────────────────────────────────────────────────
echo "[run] Executing chain..."
EXEC_RESP=$(curl -sf -X POST "${API}/chains/${CHAIN_ID}/execute" \
  -H 'Content-Type: application/json' \
  -d "${EXEC_BODY}")
EXEC_ID=$(echo "${EXEC_RESP}" | jq -r '.execution_id')
echo "[run] Execution started: ${EXEC_ID}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Streaming results for execution ${EXEC_ID}"
echo "  (Ctrl-C to detach — execution continues in background)"
echo "  To replay later: curl -N ${API}/executions/${EXEC_ID}/stream"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Stream SSE events and pretty-print them
curl -sN "${API}/executions/${EXEC_ID}/stream" | while IFS= read -r line; do
  if [[ "${line}" == event:* ]]; then
    EVENT="${line#event: }"
  elif [[ "${line}" == data:* ]]; then
    DATA="${line#data: }"
    case "${EVENT:-}" in
      step_start)
        STEP=$(echo "${DATA}" | jq -r '.step_id')
        echo "  ▶ ${STEP}"
        ;;
      step_done)
        STEP=$(echo "${DATA}" | jq -r '.step_id')
        STDOUT=$(echo "${DATA}" | jq -r '.stdout // ""')
        DUR=$(echo "${DATA}" | jq -r '.duration_ms // 0')
        echo "  ✔ ${STEP} (${DUR}ms)"
        if [ -n "${STDOUT}" ]; then
          echo "${STDOUT}" | sed 's/^/    │ /'
        fi
        ;;
      step_failed)
        STEP=$(echo "${DATA}" | jq -r '.step_id')
        ERR=$(echo "${DATA}" | jq -r '.error // .stderr // ""')
        echo "  ✘ ${STEP} FAILED: ${ERR}"
        ;;
      step_skipped)
        STEP=$(echo "${DATA}" | jq -r '.step_id')
        MSG=$(echo "${DATA}" | jq -r '.message // ""')
        echo "  ⊘ ${STEP} skipped: ${MSG}"
        ;;
      chain_done)
        echo ""
        echo "  ✔ Chain completed successfully"
        ;;
      chain_failed)
        echo ""
        echo "  ✘ Chain failed: $(echo "${DATA}" | jq -r '.message // ""')"
        ;;
      done)
        break
        ;;
    esac
    EVENT=""
  fi
done

echo ""
echo "[run] Full results: curl -s ${API}/executions/${EXEC_ID} | jq ."
