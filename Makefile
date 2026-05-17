VERSION ?= 0.0.0
SELF_UPDATE_REPO ?= soultech67/opctl

# Host detection used by `install`. Override on the command line if you need
# to install a non-host binary, e.g. `make install GOOS=linux GOARCH=arm64`.
GOOS     ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
RAW_ARCH := $(shell uname -m)
GOARCH   ?= $(if $(filter x86_64,$(RAW_ARCH)),amd64,$(if $(filter aarch64 arm64,$(RAW_ARCH)),arm64,$(RAW_ARCH)))

PREFIX  ?= $(HOME)/bin
DEST    := $(PREFIX)/opctl
SRC_BIN := ./cli/opctl-$(GOOS)-$(GOARCH)

.DEFAULT_GOAL := help

.PHONY: build bld install help

build: ## Cross-compile the CLI for all platforms via `opctl run compile`.
	opctl run -a version=$(VERSION) -a selfUpdateRepo=$(SELF_UPDATE_REPO) compile

bld: build ## Alias for `build`.

install: ## Delete the running node and copy ./cli/opctl-$(GOOS)-$(GOARCH) to $(DEST).
	@if [ ! -f $(SRC_BIN) ]; then \
	  echo "error: $(SRC_BIN) not found — run 'make build' first" >&2; \
	  exit 1; \
	fi
	@if command -v opctl >/dev/null 2>&1; then \
	  echo "running 'sudo opctl node delete' (requires root)..."; \
	  sudo opctl node delete; \
	else \
	  echo "opctl not on PATH; skipping 'node delete'"; \
	fi
	@mkdir -p $(PREFIX)
	cp $(SRC_BIN) $(DEST)
	@chmod +x $(DEST)
	@echo "installed $(DEST) (from $(GOOS)/$(GOARCH) build)"

help: ## Show this help.
	@awk 'BEGIN { FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m  [VAR=value ...]\n\nTargets:\n" } \
	      /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@printf "\nVariables (override on the command line):\n"
	@printf "  %-18s %s\n" "VERSION"          "Semver baked into the binary (default: $(VERSION))"
	@printf "  %-18s %s\n" "SELF_UPDATE_REPO" "owner/repo used by self-update (default: $(SELF_UPDATE_REPO))"
	@printf "  %-18s %s\n" "GOOS"             "Target OS for install (default: $(GOOS))"
	@printf "  %-18s %s\n" "GOARCH"           "Target arch for install (default: $(GOARCH))"
	@printf "  %-18s %s\n" "PREFIX"           "Install dir (default: $(PREFIX))"
