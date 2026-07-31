# Makefile — Sliver Scenario Orchestrator
#
# Docker-first. The whole E2E range comes up with one command:
#
#   make up          # C2 (Sliver + scenario-server) + web frontend
#   make up-victim   # + one Linux victim container
#   make lab         # extended multi-victim lab (adds vulnweb initial-access target)
#   make down        # stop everything, remove volumes
#   make logs        # follow C2 logs
#
# Standalone Go builds (for local dev without Docker) are under "Standalone builds".
# Run `make help` for the full target list.

GO      ?= go
COMPOSE ?= docker compose

# Root compose = full E2E stack (C2 + frontend; victim behind the `victim` profile).
# lab/docker-compose.yml = extended lab (2nd victim + vulnweb initial-access target).
LAB_COMPOSE ?= lab/docker-compose.yml

# `make run` params: chain file (required), API url + session id (optional).
CHAIN   ?=
API     ?= http://127.0.0.1:18080/api/v1
SESSION ?=

# Sliver version the C2 image installs — also what `sync-proto` pins vendored
# protobufs to, so the client protos match the running sliver-server.
SLIVER_VERSION ?= $(shell grep -oP 'SLIVER_VERSION=\K[^ ]+' Dockerfile | head -1)
PROTO_BASE_URL  = https://raw.githubusercontent.com/BishopFox/sliver/$(SLIVER_VERSION)/protobuf
PROTO_VENDOR    = vendor/github.com/bishopfox/sliver/protobuf
PROTO_TMP       = /tmp/sliver-proto-$(SLIVER_VERSION)
PROTO_FILES = \
	commonpb/common.proto \
	sliverpb/sliver.proto \
	clientpb/client.proto \
	rpcpb/services.proto

.DEFAULT_GOAL := help

## help: list available targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | \
		awk -F': ' '{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ── Docker (primary workflow) ─────────────────────────────────────────────────

## up: build + start the E2E stack (C2 + frontend) in the background
.PHONY: up
up:
	$(COMPOSE) up --build -d

## up-web: `up` plus the vulnerable web target (web initial-access demo)
.PHONY: up-web
up-web:
	$(COMPOSE) --profile web up --build -d

## up-victim: `up` plus a self-beaconing Linux victim container
.PHONY: up-victim
up-victim:
	$(COMPOSE) --profile victim up --build -d

## lab: start the extended multi-victim lab (adds vulnweb initial-access target)
.PHONY: lab
lab:
	$(COMPOSE) -f $(LAB_COMPOSE) up --build -d

## run: load + execute a chain (CHAIN=<chain.yaml> [API=<url>] [SESSION=<id>])
.PHONY: run
run:
	@test -n "$(CHAIN)" || { echo "usage: make run CHAIN=examples/initial-access-web.yaml [API=<url>] [SESSION=<id>]"; exit 1; }
	./examples/run.sh "$(CHAIN)" "$(API)" "$(SESSION)"

## logs: follow C2 container logs
.PHONY: logs
logs:
	$(COMPOSE) logs -f c2

## ps: show stack status
.PHONY: ps
ps:
	$(COMPOSE) ps

## build: build all images without starting them
.PHONY: build
build:
	$(COMPOSE) build

## down: stop the stack and remove volumes (root + lab compose)
.PHONY: down
down:
	-$(COMPOSE) --profile web --profile victim down -v
	-$(COMPOSE) -f $(LAB_COMPOSE) down -v

# ── Standalone builds (local dev, no Docker) ──────────────────────────────────

## scenario-server: build ./scenario-server standalone (needs CGO + libsqlite3-dev)
.PHONY: scenario-server scenario
scenario-server scenario:
	CGO_ENABLED=1 $(GO) build -mod=vendor -trimpath -tags go_sqlite -ldflags "-s -w" -o scenario-server ./cmd/server

## frontend-build: build only the frontend image
.PHONY: frontend-build
frontend-build:
	$(COMPOSE) build frontend

## sync-proto: re-vendor Sliver protobufs to match SLIVER_VERSION (needs protoc)
.PHONY: sync-proto
sync-proto:
	@echo "==> Syncing Sliver protobuf $(SLIVER_VERSION)"
	@command -v protoc >/dev/null 2>&1 || { echo "ERROR: protoc not found. Install protobuf-compiler."; exit 1; }
	@echo "==> Installing/updating protoc plugins..."
	@GOBIN=$(shell go env GOPATH)/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@GOBIN=$(shell go env GOPATH)/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@export PATH="$(shell go env GOPATH)/bin:$$PATH"; \
	rm -rf $(PROTO_TMP) && mkdir -p $(PROTO_TMP); \
	echo "==> Downloading proto files from $(PROTO_BASE_URL)..."; \
	for f in $(PROTO_FILES); do \
		mkdir -p $(PROTO_TMP)/$$(dirname $$f); \
		curl -fsSL $(PROTO_BASE_URL)/$$f -o $(PROTO_TMP)/$$f || { echo "ERROR: failed to fetch $$f"; exit 1; }; \
		echo "  fetched $$f"; \
	done; \
	echo "==> Regenerating pb.go files..."; \
	mkdir -p $(PROTO_VENDOR)/commonpb $(PROTO_VENDOR)/sliverpb $(PROTO_VENDOR)/clientpb $(PROTO_VENDOR)/rpcpb; \
	protoc -I $(PROTO_TMP) \
		$(PROTO_TMP)/commonpb/common.proto \
		$(PROTO_TMP)/sliverpb/sliver.proto \
		$(PROTO_TMP)/clientpb/client.proto \
		--go_out=$(PROTO_VENDOR)/.. \
		--go_opt=module=github.com/bishopfox/sliver \
		--go_opt=Mcommonpb/common.proto=github.com/bishopfox/sliver/protobuf/commonpb \
		--go_opt=Msliverpb/sliver.proto=github.com/bishopfox/sliver/protobuf/sliverpb \
		--go_opt=Mclientpb/client.proto=github.com/bishopfox/sliver/protobuf/clientpb; \
	protoc -I $(PROTO_TMP) \
		$(PROTO_TMP)/rpcpb/services.proto \
		--go_out=$(PROTO_VENDOR)/.. \
		--go-grpc_out=$(PROTO_VENDOR)/.. \
		--go_opt=module=github.com/bishopfox/sliver \
		--go_opt=Mcommonpb/common.proto=github.com/bishopfox/sliver/protobuf/commonpb \
		--go_opt=Msliverpb/sliver.proto=github.com/bishopfox/sliver/protobuf/sliverpb \
		--go_opt=Mclientpb/client.proto=github.com/bishopfox/sliver/protobuf/clientpb \
		--go_opt=Mrpcpb/services.proto=github.com/bishopfox/sliver/protobuf/rpcpb \
		--go-grpc_opt=module=github.com/bishopfox/sliver \
		--go-grpc_opt=Mcommonpb/common.proto=github.com/bishopfox/sliver/protobuf/commonpb \
		--go-grpc_opt=Msliverpb/sliver.proto=github.com/bishopfox/sliver/protobuf/sliverpb \
		--go-grpc_opt=Mclientpb/client.proto=github.com/bishopfox/sliver/protobuf/clientpb \
		--go-grpc_opt=Mrpcpb/services.proto=github.com/bishopfox/sliver/protobuf/rpcpb; \
	cp -v $(PROTO_TMP)/commonpb/common.proto $(PROTO_VENDOR)/commonpb/; \
	cp -v $(PROTO_TMP)/sliverpb/sliver.proto  $(PROTO_VENDOR)/sliverpb/; \
	cp -v $(PROTO_TMP)/clientpb/client.proto  $(PROTO_VENDOR)/clientpb/; \
	cp -v $(PROTO_TMP)/rpcpb/services.proto   $(PROTO_VENDOR)/rpcpb/; \
	echo "==> Fetching version-specific companion sliverpb/constants.go..."; \
	curl -fsSL $(PROTO_BASE_URL)/sliverpb/constants.go -o $(PROTO_VENDOR)/sliverpb/constants.go || { echo "ERROR: failed to fetch constants.go"; exit 1; }; \
	rm -rf $(PROTO_TMP); \
	echo "==> Done. Vendor protos updated for $(SLIVER_VERSION)."; \
	echo "    NOTE: if the generated code needs a newer Go, bump the sliver module's"; \
	echo "    'go' line in vendor/modules.txt (currently go 1.23)."

## test: run Go unit + integration tests
.PHONY: test
test:
	$(GO) test -mod=vendor -tags go_sqlite ./...

## clean: remove built binaries
.PHONY: clean
clean:
	rm -f scenario-server scenario-runner
