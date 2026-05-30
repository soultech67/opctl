VERSION ?= 0.0.0
SELF_UPDATE_REPO ?= soultech67/opctl
GITHUB_AUTH_TEST_OP_REF ?= github.com/soultech67/test-suite-auth\#1.0.0

# Run the CLI e2e (docker-in-docker) suite only in CI by default; it's
# unreliable locally (nested dind inside opctl's macOS WireGuard network).
# CI providers set CI=true. Override with `make test RUN_CLI_E2E=true` to force.
RUN_CLI_E2E ?= $(if $(filter true,$(CI)),true,false)

# Host detection used by `install`. Override on the command line if you need
# to install a non-host binary, e.g. `make install GOOS=linux GOARCH=arm64`.
GOOS     ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
RAW_ARCH := $(shell uname -m)
GOARCH   ?= $(if $(filter x86_64,$(RAW_ARCH)),amd64,$(if $(filter aarch64 arm64,$(RAW_ARCH)),arm64,$(RAW_ARCH)))

SRC_BIN := ./cli/opctl-$(GOOS)-$(GOARCH)

.DEFAULT_GOAL := help

.PHONY: build bld install uninstall reset-backup docker-logs docker-daemon-logs up doctor docker-restart test clean release help

build: ## Cross-compile the CLI for all platforms via `opctl run compile`; warns if opctl-managed containers leak.
	@before=$$(docker ps -a --filter label=opctl.managed=true --filter status=created -q 2>/dev/null | wc -l | tr -d ' '); \
	 opctl run -a version=$(VERSION) -a selfUpdateRepo=$(SELF_UPDATE_REPO) compile; \
	 rc=$$?; \
	 after=$$(docker ps -a --filter label=opctl.managed=true --filter status=created -q 2>/dev/null | wc -l | tr -d ' '); \
	 if [ "$$after" -gt "$$before" ]; then \
	   delta=$$((after - before)); \
	   printf '\n\033[33mWarning:\033[0m build leaked %s opctl-managed container(s); %s now in Created state. Run `make clean` to remove them.\n' "$$delta" "$$after" >&2; \
	 elif [ "$$after" -gt 0 ]; then \
	   printf '\nNote: %s opctl-managed container(s) in Created state (unchanged this build). Run `make clean` to remove.\n' "$$after" >&2; \
	 fi; \
	 exit $$rc

bld: build ## Alias for `build`.

install: build ## Build (passing VERSION), stop the running node (keeping its data dir), back up the existing opctl (once), then install ./cli/opctl-$(GOOS)-$(GOARCH).
	@GOOS="$(GOOS)" GOARCH="$(GOARCH)" SRC_BIN="$(SRC_BIN)" PREFIX="$(PREFIX)" ./make.sh install

uninstall: ## Delete the running node and restore the highest-version opctl-* backup over the current binary.
	@./make.sh uninstall

reset-backup: ## Remove opctl-* backups in the install prefix; next `make install` will create a fresh one. FORCE=1 skips the prompt.
	@FORCE="$(FORCE)" ./make.sh reset-backup

docker-logs: ## Stream filtered Docker VM init.log + docker events for opctl-managed containers to ./docker-logs/. Ctrl+C to stop. (macOS Docker Desktop)
	@OPCTL_DOCKER_LOG_DIR="$(OPCTL_DOCKER_LOG_DIR)" ./make.sh docker-logs

docker-daemon-logs: ## Signal dockerd to dump every goroutine's stack. Run this WHILE a hang is in progress. Output: ./docker-logs/dockerd-goroutines-*.log. (macOS Docker Desktop)
	@OPCTL_DOCKER_LOG_DIR="$(OPCTL_DOCKER_LOG_DIR)" ./make.sh docker-daemon-logs

up: ## Run opctl daemon in foreground with OPCTL_DEBUG_DOCKER=1 (kills any background daemon first). Pair with `make docker-logs` / `make docker-daemon-logs` to capture full visibility while reproducing a hang.
	@OPCTL_DEBUG_DOCKER="$(OPCTL_DEBUG_DOCKER)" OPCTL_DOCKER_TIMEOUT_MULTIPLIER="$(OPCTL_DOCKER_TIMEOUT_MULTIPLIER)" ./make.sh up

doctor: ## Read-only health check for Docker Desktop, gRPC-FUSE wedge symptoms, and orphan containers. Run BEFORE debugging a hang to spot the obvious issues. (macOS Docker Desktop)
	@./make.sh doctor

docker-restart: ## Nuclear recovery: kill opctl daemon, quit + relaunch Docker Desktop, wait for daemon. Use when `make doctor` shows docker info unresponsive. (macOS Docker Desktop)
	@./make.sh docker-restart

clean: ## Remove cross-compiled CLI binaries and orphaned opctl-managed containers.
	@removed=0; \
	 for bin in cli/opctl-darwin-amd64 cli/opctl-darwin-arm64 cli/opctl-linux-amd64 cli/opctl-linux-arm64; do \
	   if [ -f $$bin ]; then rm -f $$bin && removed=$$((removed + 1)); fi; \
	 done; \
	 echo "removed $$removed cross-compiled CLI binary file(s)"
	@orphans=$$(docker ps -a --filter label=opctl.managed=true --filter status=created -q 2>/dev/null); \
	 if [ -n "$$orphans" ]; then \
	   count=$$(echo "$$orphans" | wc -l | tr -d ' '); \
	   echo "removing $$count orphaned opctl-managed container(s)..."; \
	   docker rm $$orphans >/dev/null; \
	 else \
	   echo "no orphaned opctl-managed containers"; \
	 fi

test: ## Run the test suite. CLI e2e (dind) runs only when RUN_CLI_E2E=true (default: true in CI, false locally); that path mints a PAT via `astro auth github`.
	@if [ "$(RUN_CLI_E2E)" = "true" ]; then \
	   command -v astro >/dev/null || { echo "error: 'astro' not on PATH (needed for CLI e2e)" >&2; exit 1; }; \
	   token=$$(astro auth github); \
	 else \
	   echo "skipping CLI e2e (RUN_CLI_E2E=$(RUN_CLI_E2E)); set RUN_CLI_E2E=true to include it"; \
	   token=""; \
	 fi; \
	 opctl run -a githubAccessToken="$$token" -a githubAuthTestOpRef="$(GITHUB_AUTH_TEST_OP_REF)" -a dockerSocket=/var/run/docker.sock -a runCliE2e=$(RUN_CLI_E2E) test

release: ## Run the release op via `opctl run release` (PAT from astro, user from active gh login / soultech67).
	@command -v astro >/dev/null || { echo "error: 'astro' not on PATH" >&2; exit 1; }
	@USER=$$(gh api user --jq .login 2>/dev/null || echo soultech67); \
	 TOKEN=$$(astro auth github); \
	 echo "release as user=$$USER"; \
	 opctl run -a github="{\"username\":\"$$USER\",\"accessToken\":\"$$TOKEN\"}" release

help: ## Show this help.
	@awk 'BEGIN { FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m  [VAR=value ...]\n\nTargets:\n" } \
	      /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@printf "\nVariables (override on the command line):\n"
	@printf "  %-18s %s\n" "VERSION"          "Semver baked into the binary (default: $(VERSION))"
	@printf "  %-18s %s\n" "SELF_UPDATE_REPO" "owner/repo used by self-update and update hints (default: $(SELF_UPDATE_REPO))"
	@printf "  %-18s %s\n" "GITHUB_AUTH_TEST_OP_REF" "private auth test op ref (default: $(GITHUB_AUTH_TEST_OP_REF))"
	@printf "  %-18s %s\n" "GOOS"             "Target OS for install (default: $(GOOS))"
	@printf "  %-18s %s\n" "GOARCH"           "Target arch for install (default: $(GOARCH))"
	@printf "  %-18s %s\n" "PREFIX"           "Install dir override (default: existing opctl, then ~/bin or ~/.local/bin)"
	@printf "\nThe 'test' and 'release' targets mint a short-lived GitHub PAT via 'astro auth github';\n"
	@printf "'release' also picks the username from 'gh api user', falling back to 'soultech67'.\n"
