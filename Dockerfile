# Dockerfile.c2 — Sliver C2 server + scenario-server (fully self-contained)
#
# Multi-stage build — no host pre-requisites beyond Docker itself:
#   docker-compose up --build -d   ← only command needed
#
# ┌──────────────────────────────────────────────────────┐
# │  Stage 1 (builder)  — compile scenario-server        │
# │    golang:1.25-bookworm + gcc + libsqlite3-dev        │
# │    go build -mod=vendor -tags go_sqlite               │
# └──────────────────────────────────────────────────────┘
# ┌──────────────────────────────────────────────────────┐
# │  Stage 2 (runtime)  — lean Debian image              │
# │    sliver-server binary (released, from GitHub)       │
# │    scenario-server binary (copied from stage 1)       │
# │    atomics + scenario/examples mounted at runtime     │
# └──────────────────────────────────────────────────────┘
#
# Runtime entrypoint (c2-docker-entrypoint.sh):
#   1. Use atomics mounted at /opt/atomics.
#   2. Generate Sliver operator config once (persisted in volume).
#   3. Start sliver-server daemon and wait for gRPC port 31337.
#   4. Start scenario-server (foreground, PID 1).

# ── Stage 1: build scenario-server ───────────────────────────────────────────
FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
      gcc libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy only what the server packages need.
# The vendor/ tree carries every external dependency, so no network access
# is required during the build (reproducible + air-gap friendly).
COPY go.mod go.sum ./
COPY vendor/    vendor/
COPY api/       api/
COPY atomic/    atomic/
COPY chain/     chain/
COPY cmd/       cmd/
COPY config/    config/
COPY sliver/    sliver/
COPY store/     store/

RUN CGO_ENABLED=1 go build \
      -mod=vendor \
      -trimpath \
      -tags go_sqlite \
      -ldflags "-s -w" \
      -o /scenario-server \
      ./cmd/server

# ── Stage 2: runtime image ────────────────────────────────────────────────────
FROM debian:bookworm-slim

ARG SLIVER_VERSION=v1.7.3

RUN apt-get update && apt-get install -y --no-install-recommends \
      curl ca-certificates sqlite3 libsqlite3-0 netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

# Install sliver-server release binary
RUN ARCH=$(dpkg --print-architecture) \
    && case "$ARCH" in \
         amd64) SUFFIX="linux-amd64" ;; \
         arm64) SUFFIX="linux-arm64" ;; \
         *) echo "Unsupported arch: $ARCH" && exit 1 ;; \
       esac \
    && curl -fsSL \
       "https://github.com/BishopFox/sliver/releases/download/${SLIVER_VERSION}/sliver-server_${SUFFIX}" \
       -o /usr/local/bin/sliver-server \
    && chmod +x /usr/local/bin/sliver-server

# Pre-extract embedded Go toolchain + build assets (large, ~500 MB, but
# cached as a Docker layer — runs once per SLIVER_VERSION bump).
RUN sliver-server unpack --force

# Copy the scenario-server binary built in stage 1
COPY --from=builder /scenario-server /usr/local/bin/scenario-server
RUN chmod +x /usr/local/bin/scenario-server

# Atomics and scenario/examples are mounted at runtime (see docker-compose volumes).
RUN mkdir -p /etc/sliver /var/lib/scenario /opt/atomics

COPY lab/provision/c2-docker-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 31337 80 8080

ENTRYPOINT ["/entrypoint.sh"]
