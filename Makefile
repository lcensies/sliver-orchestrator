#
# Makefile for Sliver
#

GO ?= go
ARTIFACT_SUFFIX ?= 
ENV =
TAGS ?= -tags go_sqlite
CGO_ENABLED = 0

ifneq (,$(findstring cgo_sqlite,$(TAGS)))
	CGO_ENABLED = 1
endif

#
# Prerequisites 
#
# https://stackoverflow.com/questions/5618615/check-if-a-program-exists-from-a-makefile
EXECUTABLES = uname sed git date cut $(GO)
K := $(foreach exec,$(EXECUTABLES),\
        $(if $(shell which $(exec)),some string,$(error "No $(exec) in PATH")))

#
# Build Information
#
GO_MAJOR_VERSION = $(shell $(GO) version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f1)
GO_MINOR_VERSION = $(shell $(GO) version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f2)
MIN_SUPPORTED_GO_MAJOR_VERSION = 1
MIN_SUPPORTED_GO_MINOR_VERSION = 25
GO_VERSION_VALIDATION_ERR_MSG = Golang version is not supported, please update to at least $(MIN_SUPPORTED_GO_MAJOR_VERSION).$(MIN_SUPPORTED_GO_MINOR_VERSION)

SLIVER_PUBLIC_KEY ?= RWTZPg959v3b7tLG7VzKHRB1/QT+d3c71Uzetfa44qAoX5rH7mGoQTTR
ARMORY_PUBLIC_KEY ?= RWSBpxpRWDrD7Fe+VvRE3c2VEDC2NK80rlNCj+BX0gz44Xw07r6KQD9L
ARMORY_REPO_URL ?= https://api.github.com/repos/sliverarmory/armory/releases
CLIENT_ASSETS_PKG = github.com/bishopfox/sliver/client/assets
SLIVER_UPDATE_PKG = github.com/bishopfox/sliver/client/command/update

LDFLAGS = -ldflags "-s -w \
	-X $(SLIVER_UPDATE_PKG).SliverPublicKey=$(SLIVER_PUBLIC_KEY) \
	-X $(CLIENT_ASSETS_PKG).DefaultArmoryPublicKey=$(ARMORY_PUBLIC_KEY) \
	-X $(CLIENT_ASSETS_PKG).DefaultArmoryRepoURL=$(ARMORY_REPO_URL)"

# Debug builds shouldn't be stripped (-s -w flags)
LDFLAGS_DEBUG = -ldflags "-X $(CLIENT_ASSETS_PKG).DefaultArmoryPublicKey=$(ARMORY_PUBLIC_KEY) \
	-X $(CLIENT_ASSETS_PKG).DefaultArmoryRepoURL=$(ARMORY_REPO_URL)"

SED_INPLACE := sed -i
STATIC_TARGET := linux

UNAME_S := $(shell uname -s)
UNAME_P := $(shell uname -p)

# Programs required for generating protobuf/grpc files
PB_COMPILERS = protoc protoc-gen-go protoc-gen-go-grpc
ifeq ($(MAKECMDGOALS), pb)
	K := $(foreach exec,$(PB_COMPILERS),\
			$(if $(shell which $(exec)),some string,$(error "Missing protobuf util $(exec) in PATH")))
endif

# *** Darwin ***
ifeq ($(UNAME_S),Darwin)
	SED_INPLACE := sed -i ''
	STATIC_TARGET := macos
endif

# If no target is specified, determine GOARCH
ifeq ($(UNAME_P),arm)
	ifeq ($(MAKECMDGOALS), )
		ifeq ($(origin GOARCH), undefined)
			ENV += GOARCH=arm64
		endif
	endif
endif

ifeq ($(MAKECMDGOALS), linux)
	# Redefine LDFLAGS to add the static part
	LDFLAGS = -ldflags "-s -w \
		-extldflags '-static' \
		-X $(CLIENT_ASSETS_PKG).DefaultArmoryPublicKey=$(ARMORY_PUBLIC_KEY) \
		-X $(CLIENT_ASSETS_PKG).DefaultArmoryRepoURL=$(ARMORY_REPO_URL)"
endif

#
# Targets
#
.PHONY: default
default: clean validate-go-version
	env -u GOOS -u GOARCH $(MAKE) GOOS= GOARCH= .downloaded_assets
	$(ENV) $(if $(GOOS),GOOS=$(GOOS)) $(if $(GOARCH),GOARCH=$(GOARCH)) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server$(ARTIFACT_SUFFIX) ./server
	$(ENV) $(if $(GOOS),GOOS=$(GOOS)) $(if $(GOARCH),GOARCH=$(GOARCH)) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client$(ARTIFACT_SUFFIX) ./client

# Allows you to build a CGO-free client for any target e.g. `GOOS=windows GOARCH=arm64 make client`
# NOTE: WireGuard is not supported on all platforms, but most 64-bit GOOS/GOARCH combinations should work.
.PHONY: client
client: clean .downloaded_assets validate-go-version
	$(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client ./client

.PHONY: macos-amd64
macos-amd64: clean .downloaded_assets validate-go-version
	GOOS=darwin GOARCH=amd64 $(ENV) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server$(ARTIFACT_SUFFIX) ./server
	GOOS=darwin GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client$(ARTIFACT_SUFFIX) ./client

.PHONY: macos-arm64
macos-arm64: clean .downloaded_assets validate-go-version
	GOOS=darwin GOARCH=arm64 $(ENV) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server$(ARTIFACT_SUFFIX) ./server
	GOOS=darwin GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client$(ARTIFACT_SUFFIX) ./client

.PHONY: linux-amd64
linux-amd64: clean .downloaded_assets validate-go-version
	GOOS=linux GOARCH=amd64 $(ENV) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server$(ARTIFACT_SUFFIX) ./server
	GOOS=linux GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client$(ARTIFACT_SUFFIX) ./client

.PHONY: linux-arm64
linux-arm64: clean .downloaded_assets validate-go-version
	GOOS=linux GOARCH=arm64 $(ENV) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server$(ARTIFACT_SUFFIX) ./server
	GOOS=linux GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client$(ARTIFACT_SUFFIX) ./client

.PHONY: windows-amd64
windows-amd64: clean .downloaded_assets validate-go-version
	GOOS=windows GOARCH=amd64 $(ENV) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server$(ARTIFACT_SUFFIX).exe ./server
	GOOS=windows GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client$(ARTIFACT_SUFFIX).exe ./client

.PHONY: clients
clients: clean .downloaded_assets validate-go-version
	GOOS=darwin GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_macos-amd64$(ARTIFACT_SUFFIX) ./client
	GOOS=darwin GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_macos-arm64$(ARTIFACT_SUFFIX) ./client
	GOOS=linux GOARCH=386 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_linux-386$(ARTIFACT_SUFFIX) ./client
	GOOS=linux GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_linux-amd64$(ARTIFACT_SUFFIX) ./client
	GOOS=linux GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_linux-arm64$(ARTIFACT_SUFFIX) ./client
	GOOS=windows GOARCH=386 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_windows-386$(ARTIFACT_SUFFIX).exe ./client
	GOOS=windows GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_windows-amd64$(ARTIFACT_SUFFIX).exe ./client
	GOOS=windows GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_windows-arm64$(ARTIFACT_SUFFIX).exe ./client
	GOOS=freebsd GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_freebsd-amd64$(ARTIFACT_SUFFIX) ./client
	GOOS=freebsd GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),client $(LDFLAGS) -o sliver-client_freebsd-arm64$(ARTIFACT_SUFFIX) ./client

.PHONY: servers
servers: clean .downloaded_assets validate-go-version
	GOOS=windows GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server_windows-amd64$(ARTIFACT_SUFFIX).exe ./server
	GOOS=windows GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server_windows-arm64$(ARTIFACT_SUFFIX).exe ./server
	GOOS=linux GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server_linux-amd64$(ARTIFACT_SUFFIX) ./server
	GOOS=linux GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server_linux-arm64$(ARTIFACT_SUFFIX) ./server
	GOOS=darwin GOARCH=arm64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server_darwin-arm64$(ARTIFACT_SUFFIX) ./server
	GOOS=darwin GOARCH=amd64 $(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath $(TAGS),server $(LDFLAGS) -o sliver-server_darwin-amd64$(ARTIFACT_SUFFIX) ./server

# ── Proto sync ────────────────────────────────────────────────────────────────
# Fetch proto files from the BishopFox/sliver GitHub repo at the same version
# as the server binary in Dockerfile, then regenerate the pb.go files in vendor.
#
# Usage:
#   make sync-proto
#   make sync-proto SLIVER_VERSION=v1.7.3   # override version
#
# Requires: protoc, and Go (protoc-gen-go / protoc-gen-go-grpc auto-installed)

SLIVER_VERSION ?= $(shell grep -oP 'SLIVER_VERSION=\K[^ ]+' Dockerfile | head -1)
PROTO_BASE_URL  = https://raw.githubusercontent.com/BishopFox/sliver/$(SLIVER_VERSION)/protobuf
PROTO_VENDOR    = vendor/github.com/bishopfox/sliver/protobuf
PROTO_TMP       = /tmp/sliver-proto-$(SLIVER_VERSION)

PROTO_FILES = \
	commonpb/common.proto \
	sliverpb/sliver.proto \
	clientpb/client.proto \
	rpcpb/services.proto

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
	rm -rf $(PROTO_TMP); \
	echo "==> Done. Vendor protos updated for $(SLIVER_VERSION)"

# Build the scenario orchestrator (requires CGO for SQLite)
.PHONY: scenario
scenario:
	CGO_ENABLED=1 $(GO) build -mod=vendor -trimpath -tags go_sqlite $(LDFLAGS) -o scenario-server ./scenario/cmd/server

# Build the scenario runner (client for scenario API; no CGO)
.PHONY: scenario-runner
scenario-runner:
	cd cmd/scenario-runner && $(GO) build -mod=vendor -trimpath -o ../../scenario-runner .

.PHONY: pb
pb:
	protoc -I protobuf/ protobuf/commonpb/common.proto --go_out=paths=source_relative:protobuf/
	protoc -I protobuf/ protobuf/sliverpb/sliver.proto --go_out=paths=source_relative:protobuf/
	protoc -I protobuf/ protobuf/clientpb/client.proto --go_out=paths=source_relative:protobuf/
	protoc -I protobuf/ protobuf/dnspb/dns.proto --go_out=paths=source_relative:protobuf/
	protoc -I protobuf/ protobuf/rpcpb/services.proto --go_out=paths=source_relative:protobuf/ --go-grpc_out=protobuf/ --go-grpc_opt=paths=source_relative 

.PHONY: debug
debug: clean
	$(ENV) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=vendor $(TAGS),server $(LDFLAGS_DEBUG) -o sliver-server$(ARTIFACT_SUFFIX) ./server
	$(ENV) CGO_ENABLED=0 $(GO) build -mod=vendor $(TAGS),client $(LDFLAGS_DEBUG) -o sliver-client$(ARTIFACT_SUFFIX) ./client

validate-go-version:
	@if [ $(GO_MAJOR_VERSION) -gt $(MIN_SUPPORTED_GO_MAJOR_VERSION) ]; then \
		exit 0 ;\
	elif [ $(GO_MAJOR_VERSION) -lt $(MIN_SUPPORTED_GO_MAJOR_VERSION) ]; then \
		echo '$(GO_VERSION_VALIDATION_ERR_MSG)';\
		exit 1; \
	elif [ $(GO_MINOR_VERSION) -lt $(MIN_SUPPORTED_GO_MINOR_VERSION) ] ; then \
		echo '$(GO_VERSION_VALIDATION_ERR_MSG)';\
		exit 1; \
	fi

.PHONY: clean-all
clean-all: clean
	rm -rf ./server/assets/fs/darwin/amd64
	rm -rf ./server/assets/fs/darwin/arm64
	rm -rf ./server/assets/fs/windows/amd64
	rm -rf ./server/assets/fs/linux/amd64
	rm -f ./server/assets/fs/*.zip
	rm -f ./.downloaded_assets

.PHONY: clean
clean:
	rm -f sliver-client sliver-client_* sliver-server sliver-server_* sliver-*.exe scenario-server scenario-runner

.downloaded_assets:
	$(ENV) $(GO) run -mod=vendor ./util/cmd/assets
	touch ./.downloaded_assets
