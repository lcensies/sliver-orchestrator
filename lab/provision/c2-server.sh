#!/usr/bin/env bash
# Provision the C2 server VM:
#   1. Install sliver-server (from repo build or GitHub release)
#   2. Generate operator config for the scenario service
#   3. Start sliver-server as a systemd service
#   4. Install and start the scenario-server binary
#   5. Copy atomics library to /opt/atomics
#   6. Copy initial-access exploit scripts to /opt/exploits (used by the
#      "external" initial-access module, e.g. lab/exploits/web_rce.py)
set -euo pipefail

SLIVER_VERSION="${SLIVER_VERSION:-v1.5.42}"
C2_HOST="${C2_HOST:-192.168.56.10}"
SCENARIO_PORT="${SCENARIO_PORT:-8080}"
OPERATOR_NAME="scenario"
OPERATOR_CFG="/etc/sliver/scenario-operator.cfg"
DB_PATH="/var/lib/scenario/scenario.db"
ATOMICS_DIR="/opt/atomics"
ATOMICS_SRC="${SCENARIO_ATOMICS_SRC:-/sliver-repo/atomics}"
EXPLOITS_DIR="/opt/exploits"
EXPLOITS_SRC="${SCENARIO_EXPLOITS_SRC:-/sliver-repo/lab/exploits}"

# ── System dependencies ──────────────────────────────────────────────────────
# python3 is required here too: initial_access "external" module steps (e.g.
# type: initial_access, module: external, config.run: ["python3", "/opt/exploits/web_rce.py"])
# shell out to a local child process ON THIS HOST (the scenario-server host), not
# on the victim — the exploit script is what reaches out to the victim over HTTP.
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y --no-install-recommends curl wget ca-certificates sqlite3 libsqlite3-dev python3

# ── Install sliver-server ────────────────────────────────────────────────────
if [ ! -x /usr/local/bin/sliver-server ]; then
  # Prefer locally-built binary from the synced repo
  if [ -f /sliver-repo/sliver-server ]; then
    echo "[provision] Using locally-built sliver-server"
    cp /sliver-repo/sliver-server /usr/local/bin/sliver-server
    chmod +x /usr/local/bin/sliver-server
  else
    echo "[provision] Downloading sliver-server ${SLIVER_VERSION}"
    ARCH=$(uname -m)
    case "${ARCH}" in
      x86_64) ARCH_SUFFIX="linux" ;;
      aarch64) ARCH_SUFFIX="linux-arm64" ;;
      *)
        echo "Unsupported arch: ${ARCH}"
        exit 1
        ;;
    esac
    curl -fsSL \
      "https://github.com/BishopFox/sliver/releases/download/${SLIVER_VERSION}/sliver-server_${ARCH_SUFFIX}" \
      -o /usr/local/bin/sliver-server
    chmod +x /usr/local/bin/sliver-server
  fi
fi

# Unpack embedded assets (toolchains, templates, etc.)
sliver-server unpack --force

# ── Operator config ──────────────────────────────────────────────────────────
mkdir -p /etc/sliver
if [ ! -f "${OPERATOR_CFG}" ]; then
  echo "[provision] Generating operator config for '${OPERATOR_NAME}'"
  sliver-server operator \
    --name "${OPERATOR_NAME}" \
    --lhost "${C2_HOST}" \
    --save "${OPERATOR_CFG}"
fi
chmod 600 "${OPERATOR_CFG}"

# ── Systemd: sliver-server ───────────────────────────────────────────────────
cat >/etc/systemd/system/sliver-server.service <<'EOF'
[Unit]
Description=Sliver C2 Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/sliver-server daemon
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable sliver-server
systemctl start sliver-server

# Wait for sliver-server to be ready
echo "[provision] Waiting for sliver-server to start..."
for i in $(seq 1 30); do
  if systemctl is-active --quiet sliver-server; then break; fi
  sleep 2
done

# ── Install scenario-server ──────────────────────────────────────────────────
if [ ! -x /usr/local/bin/scenario-server ]; then
  if [ -f /sliver-repo/scenario-server ]; then
    echo "[provision] Installing locally-built scenario-server"
    cp /sliver-repo/scenario-server /usr/local/bin/scenario-server
    chmod +x /usr/local/bin/scenario-server
  else
    echo "[provision] scenario-server binary not found at /sliver-repo/scenario-server"
    echo "[provision] Build it with: make scenario   (in the sliver repo root)"
  fi
fi

# ── Atomics library ──────────────────────────────────────────────────────────
mkdir -p "${ATOMICS_DIR}"
if [ -d "${ATOMICS_SRC}" ]; then
  echo "[provision] Copying atomics library from ${ATOMICS_SRC}"
  cp -r "${ATOMICS_SRC}/." "${ATOMICS_DIR}/"
else
  echo "[provision] Warning: atomics source not found at ${ATOMICS_SRC}"
fi

# ── Initial-access exploit scripts ───────────────────────────────────────────
mkdir -p "${EXPLOITS_DIR}"
if [ -d "${EXPLOITS_SRC}" ]; then
  echo "[provision] Copying initial-access exploits from ${EXPLOITS_SRC}"
  cp -r "${EXPLOITS_SRC}/." "${EXPLOITS_DIR}/"
  chmod +x "${EXPLOITS_DIR}"/*.py "${EXPLOITS_DIR}"/*.sh 2>/dev/null || true
else
  echo "[provision] Warning: exploits source not found at ${EXPLOITS_SRC}"
fi

# ── Scenario database dir ────────────────────────────────────────────────────
mkdir -p "$(dirname "${DB_PATH}")"

# ── Systemd: scenario-server ─────────────────────────────────────────────────
if [ -x /usr/local/bin/scenario-server ]; then
  cat >/etc/systemd/system/scenario-server.service <<EOF
[Unit]
Description=Sliver Scenario Orchestrator
After=sliver-server.service
Requires=sliver-server.service

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/scenario-server \\
  --config ${OPERATOR_CFG} \\
  --atomics ${ATOMICS_DIR} \\
  --db ${DB_PATH} \\
  --listen :${SCENARIO_PORT}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable scenario-server
  systemctl start scenario-server
fi

# Copy operator config to vagrant home for convenience
cp "${OPERATOR_CFG}" /home/vagrant/.sliver-scenario.cfg
chown vagrant:vagrant /home/vagrant/.sliver-scenario.cfg

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  C2 Server provisioned"
echo "  Sliver gRPC:   ${C2_HOST}:31337"
echo "  Scenario API:  http://${C2_HOST}:${SCENARIO_PORT}/api/v1/"
echo "  Operator cfg:  ${OPERATOR_CFG}"
echo "  Exploits dir:  ${EXPLOITS_DIR}"
echo "═══════════════════════════════════════════════════════"
