#!/usr/bin/env bash
# setup.sh — One-command setup for the Sliver C2 Orchestration Cyber Range Lab
#
# Usage:
#   git clone <repo> sliver-orchestrator
#   cd sliver-orchestrator
#   chmod +x lab/vm-setup/setup.sh && ./lab/vm-setup/setup.sh
#
# What this does:
#   1. Checks dependencies (vagrant, virtualbox, go, node, npm)
#   2. Builds the scenario-server binary
#   3. Boots VMs in order (c2 → linux_pivot → win_target)
#   4. Deploys scenario-server to c2 VM
#   5. Imports Sliver operator config to Kali
#   6. Sets up linux_pivot services (honeypot, vulnweb initial-access target, svc-server)
#   7. Sets up Kali auto-boot service (vagrant-lab.service)
#   8. Fetches atomic techniques
#   9. Starts frontend dev server
#
set -euo pipefail

# Resolve the repo root (this script lives in lab/vm-setup/, two levels down).
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
C2_IP="192.168.56.5"
C2_PORT="8080"
FRONTEND_DIR="${REPO_DIR}/flexible-platform"

# ── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
ok() { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err() {
  echo -e "${RED}[✗]${NC} $*"
  exit 1
}
info() { echo -e "    $*"; }

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     Sliver C2 Orchestration Cyber Range — Setup Script      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

cd "${REPO_DIR}"

# ── Step 1: Check dependencies ────────────────────────────────────────────────
echo "── Step 1: Checking dependencies ──────────────────────────────"
for cmd in vagrant vboxmanage go node npm curl jq; do
  if command -v "$cmd" &>/dev/null; then
    ok "$cmd found ($(command -v $cmd))"
  else
    err "$cmd not found — install it first"
  fi
done
echo ""

# ── Step 2: Build scenario-server ────────────────────────────────────────────
echo "── Step 2: Building scenario-server ───────────────────────────"
if make scenario; then
  ok "scenario-server built successfully"
else
  err "make scenario failed"
fi
echo ""

# ── Step 3: Boot VMs ─────────────────────────────────────────────────────────
echo "── Step 3: Booting VMs ─────────────────────────────────────────"
info "Starting c2 VM..."
vagrant up c2
ok "c2 VM started"

info "Waiting 30s for services to initialize..."
sleep 30

# Wait for backend health
for i in $(seq 1 12); do
  if curl -sf "http://${C2_IP}:${C2_PORT}/api/v1/health" &>/dev/null; then
    ok "Backend API healthy (http://${C2_IP}:${C2_PORT})"
    break
  fi
  info "Waiting for backend... ($i/12)"
  sleep 10
done

info "Starting linux_pivot VM..."
vagrant up linux_pivot
ok "linux_pivot VM started"

info "Starting win_target VM..."
vagrant up win_target
ok "win_target VM started"
echo ""

# ── Step 4: Deploy scenario-server to c2 ─────────────────────────────────────
echo "── Step 4: Deploying scenario-server to c2 ────────────────────"
vagrant ssh c2 -c "sudo systemctl stop scenario-server" 2>/dev/null || true
vagrant upload scenario-server /tmp/scenario-server c2
vagrant ssh c2 -c "sudo cp /tmp/scenario-server /usr/local/bin/scenario-server && \
  sudo chmod +x /usr/local/bin/scenario-server && \
  sudo systemctl start scenario-server && \
  sleep 3 && sudo systemctl is-active scenario-server"
ok "scenario-server deployed and running"
echo ""

# ── Step 5: Import Sliver operator config ────────────────────────────────────
echo "── Step 5: Importing Sliver operator config ────────────────────"
vagrant ssh c2 -- -q "sudo cat /etc/sliver/scenario-operator.cfg" 2>/dev/null >/tmp/op.cfg
if [ -s /tmp/op.cfg ]; then
  sliver-client import /tmp/op.cfg 2>/dev/null && ok "Sliver operator config imported" || warn "Config import failed (may already exist)"
else
  warn "Could not fetch operator config — import manually: sliver-client import /tmp/op.cfg"
fi
echo ""

# ── Step 6: Setup linux_pivot services ───────────────────────────────────────
echo "── Step 6: Setting up linux_pivot services ─────────────────────"

# Copy honeypot.py if not already there
if [ -f "${REPO_DIR}/honeypot.py" ]; then
  vagrant upload "${REPO_DIR}/honeypot.py" /tmp/honeypot.py linux_pivot
  ok "honeypot.py uploaded"
fi

# Copy the deliberately-vulnerable vulnweb app — this IS the real initial-access
# target (command injection in GET /ping?host=). The honeypot above is cosmetic
# decoration; vulnweb is what examples/full-attack-chain-v2.yaml's initial_access
# step actually exploits to obtain the first Linux session.
if [ -f "${REPO_DIR}/lab/provision/vulnweb/server.py" ]; then
  vagrant upload "${REPO_DIR}/lab/provision/vulnweb/server.py" /tmp/vulnweb.py linux_pivot
  ok "vulnweb server.py uploaded"
fi

vagrant ssh linux_pivot <<'SSHEOF'
set -e

# ── Honeypot service ──────────────────────────────────────────────────────────
sudo tee /etc/systemd/system/honeypot.service > /dev/null << 'EOF'
[Unit]
Description=Fake IP Camera Honeypot
After=network.target
[Service]
Type=simple
ExecStart=/usr/bin/python3 /tmp/honeypot.py 0.0.0.0 8080
Restart=always
RestartSec=3
[Install]
WantedBy=multi-user.target
EOF

# ── vulnweb service (real initial-access target — command injection) ─────────
sudo tee /etc/systemd/system/vulnweb.service > /dev/null << 'EOF'
[Unit]
Description=Vulnerable Web App (initial-access target, CWE-78 command injection)
After=network.target
[Service]
Type=simple
Environment=PORT=9090
ExecStart=/usr/bin/python3 /tmp/vulnweb.py
Restart=always
RestartSec=3
[Install]
WantedBy=multi-user.target
EOF

# ── svc-server (Windows implant HTTP server) ──────────────────────────────────
sudo tee /etc/systemd/system/svc-server.service > /dev/null << 'EOF'
[Unit]
Description=Implant HTTP Server
After=network.target
[Service]
Type=simple
ExecStartPre=/bin/bash -c 'for i in $(seq 1 20); do curl -sf --max-time 30 -o /tmp/svc.exe "http://192.168.56.5:8080/api/v1/implant/windows?c2=192.168.56.5" && echo "svc.exe fetched" && break; echo "retry $i..."; sleep 30; done || true'
ExecStart=/usr/bin/python3 -m http.server 8000 --bind 172.16.1.10
WorkingDirectory=/tmp
Restart=always
RestartSec=30
TimeoutStartSec=600
[Install]
WantedBy=multi-user.target
EOF

# ── UFW firewall rules ────────────────────────────────────────────────────────
sudo ufw allow 8080/tcp 2>/dev/null || true
sudo ufw allow 9090/tcp 2>/dev/null || true

# ── Enable + start all services ───────────────────────────────────────────────
# NOTE: no implant-watchdog here anymore — there is no pre-installed
# sliver-implant systemd unit to babysit. The first Linux session now comes
# exclusively from exploiting vulnweb via the chain's initial_access step.
sudo systemctl daemon-reload
sudo systemctl enable honeypot vulnweb svc-server 2>/dev/null || true
sudo systemctl restart honeypot vulnweb svc-server 2>/dev/null || true

echo "Services status:"
sudo systemctl is-active honeypot vulnweb svc-server 2>/dev/null || true
SSHEOF
ok "linux_pivot services configured (vulnweb :9090 is the initial-access target)"
echo ""

# ── Step 7: Kali auto-boot service ───────────────────────────────────────────
echo "── Step 7: Setting up Kali auto-boot service ───────────────────"
VAGRANT_PATH="$(which vagrant)"
sudo tee /etc/systemd/system/vagrant-lab.service >/dev/null <<EOF
[Unit]
Description=Sliver Lab VMs
After=network.target graphical.target
Wants=network.target

[Service]
Type=oneshot
RemainAfterExit=yes
User=${USER}
WorkingDirectory=${REPO_DIR}
ExecStart=/bin/bash -c 'sleep 10 && ${VAGRANT_PATH} up c2 && sleep 30 && ${VAGRANT_PATH} up linux_pivot && sleep 10 && ${VAGRANT_PATH} up win_target'
TimeoutStartSec=600

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable vagrant-lab
ok "vagrant-lab.service enabled (auto-boot on Kali restart)"
echo ""

# ── Step 8: Fetch atomic techniques ──────────────────────────────────────────
echo "── Step 8: Fetching atomic techniques ─────────────────────────"
if [ -f "${REPO_DIR}/atomic/fetch.sh" ]; then
  # Copy atomics to c2 VM:
  vagrant ssh c2 -c "sudo mkdir -p /opt/atomics && sudo cp -rn /sliver-repo/atomic/T* /opt/atomics/ 2>/dev/null || true && ls /opt/atomics | wc -l"
  ok "Atomics synced to c2 VM"
else
  warn "atomic/fetch.sh not found — run: ./atomic/fetch.sh --clean"
fi
echo ""

# ── Step 9: Frontend setup ───────────────────────────────────────────────────
echo "── Step 9: Setting up frontend ─────────────────────────────────"
if [ -d "${FRONTEND_DIR}" ]; then
  cd "${FRONTEND_DIR}"
  if [ ! -d node_modules ]; then
    npm install --silent && ok "npm packages installed"
  else
    ok "npm packages already installed"
  fi
  cd "${REPO_DIR}"
else
  warn "flexible-platform not found — clone it: git clone <frontend-repo> flexible-platform"
fi
echo ""

# ── Step 10: Check for sessions ───────────────────────────────────────────────
# No session appears automatically anymore: linux_pivot ships clean (no
# pre-installed implant) and win_target never had one (isolated by design).
# The first session now comes exclusively from breaching vulnweb via the
# initial_access step in examples/full-attack-chain-v2.yaml.
echo "── Step 10: Checking for sessions ──────────────────────────────"
SESSIONS=$(curl -sf "http://${C2_IP}:${C2_PORT}/api/v1/sessions" 2>/dev/null | jq -r '.[] | "\(.os) \(.hostname) pid:\(.pid)"' 2>/dev/null || echo "")
if [ -n "$SESSIONS" ]; then
  ok "Active sessions:"
  echo "$SESSIONS" | while read -r s; do info "$s"; done
else
  info "No sessions yet (expected — nothing is pre-installed)."
  info "Run the initial-access chain to breach linux_pivot's vulnweb service:"
  info "  ./examples/run.sh examples/full-attack-chain-v2.yaml"
fi
echo ""

# ── Done ─────────────────────────────────────────────────────────────────────
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    Setup Complete!                          ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  Backend API:   http://${C2_IP}:${C2_PORT}/api/v1/health"
echo "  Frontend UI:   cd flexible-platform && npm run dev"
echo "                 → http://localhost:5173"
echo "  Sliver CLI:    sliver-client"
echo ""
echo "  Quick commands:"
echo "    vagrant status              # VM states"
echo "    win-off                     # Force-off win_target"
echo "    sliver-client               # Connect to C2"
echo ""
echo "  Run full attack chain:"
echo "    LIN=\$(curl -s http://${C2_IP}:${C2_PORT}/api/v1/sessions | jq -r '.[] | select(.os==\"linux\") | .id' | tail -1)"
echo "    curl -s -X POST http://${C2_IP}:${C2_PORT}/api/v1/chains/cf1efcaf-62e3-4223-afcc-eecf13efddc1/execute \\"
echo "      -H 'Content-Type: application/json' -d \"{\\\"session_id\\\":\\\"\$LIN\\\"}\""
echo ""
