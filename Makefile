BINARY=finfocus
VERSION?=$(shell git describe --tags --match 'v[0-9]*' --always --dirty)
COMMIT=$(shell git rev-parse HEAD)
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

GOLANGCI_LINT?=$(HOME)/go/bin/golangci-lint
# Read from mise.toml, the single source of tool versions, so Renovate bumps
# to mise.toml cannot leave this check expecting an older release.
GOLANGCI_LINT_VERSION?=$(shell sed -n 's/^golangci-lint = "\(.*\)"/\1/p' mise.toml)
MARKDOWNLINT?=markdownlint
MARKDOWNLINT_CLI2?=markdownlint-cli2
MARKDOWNLINT_FILES?=AGENTS.md
ACTIONLINT?=$(HOME)/go/bin/actionlint

LDFLAGS=-ldflags "-X 'github.com/rshade/finfocus/pkg/version.version=$(VERSION)' \
                  -X 'github.com/rshade/finfocus/pkg/version.gitCommit=$(COMMIT)' \
                  -X 'github.com/rshade/finfocus/pkg/version.buildDate=$(BUILD_DATE)'"

.PHONY: all
all: build build-plugin

.PHONY: build
build:
	@echo "Building $(BINARY)..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/finfocus

.PHONY: build-recorder
build-recorder:
	@echo "Building recorder plugin..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/finfocus-plugin-recorder ./plugins/recorder/cmd

.PHONY: build-plugin
build-plugin:
	@echo "Building Pulumi tool plugin..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/pulumi-tool-finfocus ./cmd/finfocus

RECORDER_VERSION=0.1.0
RECORDER_INSTALL_DIR=$(HOME)/.finfocus/plugins/recorder/$(RECORDER_VERSION)

.PHONY: install-recorder
install-recorder: build-recorder
	@echo "Installing recorder plugin to $(RECORDER_INSTALL_DIR)..."
	@mkdir -p $(RECORDER_INSTALL_DIR)
	cp bin/finfocus-plugin-recorder $(RECORDER_INSTALL_DIR)/
	cp plugins/recorder/plugin.manifest.json $(RECORDER_INSTALL_DIR)/
	chmod 644 $(RECORDER_INSTALL_DIR)/plugin.manifest.json
	@echo "Recorder plugin installed successfully."
	@echo "Verify with: finfocus plugin list"

KUBERNETES_PLUGIN_DIR=plugins/kubernetes
KUBERNETES_VERSION=$(shell jq -r '."plugins/kubernetes" // "0.0.0"' .release-please-manifest.json)
KUBERNETES_INSTALL_DIR=$(HOME)/.finfocus/plugins/kubernetes/v$(KUBERNETES_VERSION)

.PHONY: build-kubernetes
build-kubernetes:
	@mkdir -p bin
	go -C $(KUBERNETES_PLUGIN_DIR) build -ldflags "-X main.version=v$(KUBERNETES_VERSION)" \
		-o $(CURDIR)/bin/finfocus-plugin-kubernetes ./cmd

.PHONY: install-kubernetes
install-kubernetes: build-kubernetes
	@command -v jq >/dev/null 2>&1 || { \
		echo "install-kubernetes requires jq to stamp the installed plugin manifest version; install jq and retry" >&2; \
		exit 1; \
	}
	@if [ -z "$(KUBERNETES_VERSION)" ]; then \
		echo "install-kubernetes: KUBERNETES_VERSION resolved empty (check jq and .release-please-manifest.json)" >&2; \
		exit 1; \
	fi
	@mkdir -p $(KUBERNETES_INSTALL_DIR)
	cp bin/finfocus-plugin-kubernetes $(KUBERNETES_INSTALL_DIR)/
	jq --arg v "v$(KUBERNETES_VERSION)" '.version = $$v' \
		$(KUBERNETES_PLUGIN_DIR)/plugin.manifest.json > $(KUBERNETES_INSTALL_DIR)/plugin.manifest.json
	chmod 644 $(KUBERNETES_INSTALL_DIR)/plugin.manifest.json
	@echo "Verify with: finfocus plugin list"

.PHONY: test-kubernetes
test-kubernetes:
	go -C $(KUBERNETES_PLUGIN_DIR) test -race ./...

.PHONY: lint-kubernetes
lint-kubernetes:
	cd $(KUBERNETES_PLUGIN_DIR) && $(GOLANGCI_LINT) run --allow-parallel-runners ./...

JEV_PLUGIN_DIR=plugins/jev
JEV_VERSION=$(shell jq -r '."plugins/jev" // "0.0.0"' .release-please-manifest.json)
JEV_INSTALL_DIR=$(HOME)/.finfocus/plugins/jev/v$(JEV_VERSION)

.PHONY: build-jev
build-jev:
	@mkdir -p bin
	go -C $(JEV_PLUGIN_DIR) build -ldflags "-X main.version=v$(JEV_VERSION)" \
		-o $(CURDIR)/bin/finfocus-plugin-jev ./cmd

.PHONY: install-jev
install-jev: build-jev
	@command -v jq >/dev/null 2>&1 || { \
		echo "install-jev requires jq to stamp the installed plugin manifest version; install jq and retry" >&2; \
		exit 1; \
	}
	@if [ -z "$(JEV_VERSION)" ]; then \
		echo "install-jev: JEV_VERSION resolved empty (check jq and .release-please-manifest.json)" >&2; \
		exit 1; \
	fi
	@mkdir -p $(JEV_INSTALL_DIR)
	cp bin/finfocus-plugin-jev $(JEV_INSTALL_DIR)/
	jq --arg v "v$(JEV_VERSION)" '.version = $$v' \
		$(JEV_PLUGIN_DIR)/plugin.manifest.json > $(JEV_INSTALL_DIR)/plugin.manifest.json
	chmod 644 $(JEV_INSTALL_DIR)/plugin.manifest.json
	@echo "Set TYPESAFE_API_KEY to enable scoring; see plugins/jev/README.md for what is sent to TypeSafe AI."
	@echo "Verify with: finfocus plugin list"

.PHONY: test-jev
test-jev:
	go -C $(JEV_PLUGIN_DIR) test -race ./...

.PHONY: lint-jev
lint-jev:
	cd $(JEV_PLUGIN_DIR) && $(GOLANGCI_LINT) run --allow-parallel-runners ./...

PROMETHEUS_PLUGIN_DIR=plugins/prometheus
PROMETHEUS_VERSION=$(shell jq -r '."plugins/prometheus" // "0.0.0"' .release-please-manifest.json)
PROMETHEUS_INSTALL_DIR=$(HOME)/.finfocus/plugins/prometheus/v$(PROMETHEUS_VERSION)

.PHONY: build-prometheus
build-prometheus:
	@mkdir -p bin
	go -C $(PROMETHEUS_PLUGIN_DIR) build -ldflags "-X main.version=v$(PROMETHEUS_VERSION)" \
		-o $(CURDIR)/bin/finfocus-plugin-prometheus ./cmd

.PHONY: install-prometheus
install-prometheus: build-prometheus
	@command -v jq >/dev/null 2>&1 || { \
		echo "install-prometheus requires jq to stamp the installed plugin manifest version; install jq and retry" >&2; \
		exit 1; \
	}
	@if [ -z "$(PROMETHEUS_VERSION)" ]; then \
		echo "install-prometheus: PROMETHEUS_VERSION resolved empty (check jq and .release-please-manifest.json)" >&2; \
		exit 1; \
	fi
	@mkdir -p $(PROMETHEUS_INSTALL_DIR)
	cp bin/finfocus-plugin-prometheus $(PROMETHEUS_INSTALL_DIR)/
	jq --arg v "v$(PROMETHEUS_VERSION)" '.version = $$v' \
		$(PROMETHEUS_PLUGIN_DIR)/plugin.manifest.json > $(PROMETHEUS_INSTALL_DIR)/plugin.manifest.json
	chmod 644 $(PROMETHEUS_INSTALL_DIR)/plugin.manifest.json
	@echo "Verify with: finfocus plugin list"

.PHONY: test-prometheus
test-prometheus:
	go -C $(PROMETHEUS_PLUGIN_DIR) test -race ./...

.PHONY: lint-prometheus
lint-prometheus:
	cd $(PROMETHEUS_PLUGIN_DIR) && $(GOLANGCI_LINT) run --allow-parallel-runners ./...

.PHONY: check-plugin-boundaries
check-plugin-boundaries:
	@for dir in $(KUBERNETES_PLUGIN_DIR) $(JEV_PLUGIN_DIR) $(PROMETHEUS_PLUGIN_DIR); do \
		if go -C $$dir list -deps ./... | grep -E '^github.com/rshade/finfocus/(internal|pkg)(/|$$)'; then \
			echo "$$dir must not import finfocus core packages" >&2; exit 1; fi; \
	done
	@echo "plugin boundaries OK"

.PHONY: build-all
build-all: build build-recorder build-plugin build-kubernetes build-jev build-prometheus

# Default test target - runs unit tests only (fast, for CI and local dev)
# Unit tests are colocated with source; see test/README.md for details
.PHONY: test
test: test-unit test-frontend test-kubernetes test-jev test-prometheus

# Browser renderer regression tests use the Node version pinned in mise.toml.
.PHONY: test-frontend
test-frontend:
	node --experimental-vm-modules --disable-warning=ExperimentalWarning --test internal/webui/frontendtest/*.test.mjs

# Browser acceptance is credential-free and requires the real binary and Chromium.
# Resolve the installer in the nested module, keeping its driver version matched.
PLAYWRIGHT_INSTALL_ARGS?=chromium
.PHONY: test-e2e-web
test-e2e-web: build
	go -C test/e2e run github.com/mxschmitt/playwright-go/cmd/playwright install $(PLAYWRIGHT_INSTALL_ARGS)
	FINFOCUS_BINARY=$(CURDIR)/bin/finfocus FINFOCUS_WEB_E2E_REQUIRED=1 go -C test/e2e test -tags e2e_web -run '^TestWebUI' -v -count=1 -timeout 10m ./...

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	go test -v ./internal/... ./pkg/...

.PHONY: test-race
test-race:
	@echo "Running unit tests with race detector..."
	go test -v -race ./internal/... ./pkg/...

# Integration tests - slower, requires more setup
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	go test -v -timeout 10m ./test/integration/...

.PHONY: test-integration-plugin
test-integration-plugin:
	go test -v ./test/integration/plugin/...

# E2E tests - requires AWS credentials and real infrastructure
.PHONY: test-e2e
test-e2e:
	@echo "Running E2E tests..."
	./test/e2e/run-e2e-tests.sh $(TEST_ARGS)

# Kind-based cost cluster E2E - requires Docker, kind, and kubectl; no cloud credentials
# Pinned so a just-published release whose assets are still uploading cannot break the run.
# renovate: datasource=github-releases depName=rshade/finfocus-plugin-aws-public
E2E_AWS_PUBLIC_VERSION?=v0.2.1

.PHONY: test-e2e-kind
test-e2e-kind: build install-kubernetes build-prometheus
	./test/e2e/kind/setup.sh
	./bin/finfocus plugin install aws-public@$(E2E_AWS_PUBLIC_VERSION) --metadata region=us-east-1 --force
	cd test/e2e && FINFOCUS_BINARY=$(CURDIR)/bin/finfocus go test -tags e2e_kind -run 'TestCostCluster_Kind($$|_)' -v -timeout 10m ./...
	kubectl --context "kind-$${KIND_CLUSTER:-finfocus-e2e}" apply -f test/e2e/kind/prometheus.yaml
	kubectl --context "kind-$${KIND_CLUSTER:-finfocus-e2e}" -n monitoring rollout status deployment/prometheus --timeout=180s
	cd test/e2e && FINFOCUS_BINARY=$(CURDIR)/bin/finfocus go test -tags e2e_kind -run '^TestCostCluster_KindHistorical$$' -v -timeout 15m ./...

# Regenerate the real Terraform state goldens (requires docker + mise; no cloud access)
.PHONY: gen-terraform-goldens
gen-terraform-goldens:
	./scripts/gen-terraform-goldens.sh

# Run all tests (unit + integration, excludes E2E which requires special setup)
.PHONY: test-all
test-all:
	@echo "Running all tests (unit + integration)..."
	go test -v -timeout 15m ./internal/... ./pkg/... ./test/integration/...

.PHONY: lint
lint: lint-kubernetes lint-jev lint-prometheus check-plugin-boundaries
	@echo "Running golangci-lint (expected version $(GOLANGCI_LINT_VERSION))..."
	@$(GOLANGCI_LINT) --version | grep -q "$(GOLANGCI_LINT_VERSION)" || \
		(echo "golangci-lint $(GOLANGCI_LINT_VERSION) required. Install with"; \
		echo "  curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $$HOME/go/bin v$(GOLANGCI_LINT_VERSION)"; exit 1)
	$(GOLANGCI_LINT) run --allow-parallel-runners
	@echo "Running markdownlint..."
	@command -v $(MARKDOWNLINT) >/dev/null 2>&1 || \
		(echo "markdownlint CLI not found. Install with"; \
		echo "  npm install -g markdownlint-cli@0.45.0"; exit 1)
	$(MARKDOWNLINT) $(MARKDOWNLINT_FILES)
	@$(MAKE) lint-actions

.PHONY: lint-actions
lint-actions:
	@echo "Running actionlint..."
	@command -v $(ACTIONLINT) >/dev/null 2>&1 || \
		(echo "actionlint not found. Install with"; \
		echo "  go install github.com/rhysd/actionlint/cmd/actionlint@latest"; exit 1)
	find .github/workflows -name '*.yml' -not -name '*.lock.yml' -print0 | xargs -0 $(ACTIONLINT)

.PHONY: validate
validate:
	@echo "Running validation..."
	@echo "Checking go modules..."
	go mod tidy -diff
	@echo "Running go vet..."
	go vet ./...
	@echo "Validation complete."

.PHONY: tools
tools: ## Install the toolchain pinned in mise.toml
	mise install

.PHONY: ensure
ensure: ensure-golangci-lint ensure-markdownlint ensure-markdownlint-cli2 ensure-actionlint
	@echo "All dev tools are ready."

.PHONY: ensure-golangci-lint
ensure-golangci-lint:
	@echo "==> golangci-lint $(GOLANGCI_LINT_VERSION)"
	@$(GOLANGCI_LINT) --version 2>/dev/null | grep -q "$(GOLANGCI_LINT_VERSION)" || \
		(echo "    Installing golangci-lint v$(GOLANGCI_LINT_VERSION)..." && \
		curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $$HOME/go/bin v$(GOLANGCI_LINT_VERSION))
	@echo "    OK"

.PHONY: ensure-markdownlint
ensure-markdownlint:
	@echo "==> markdownlint-cli"
	@command -v $(MARKDOWNLINT) >/dev/null 2>&1 || \
		(echo "    Installing markdownlint-cli@0.45.0..." && \
		npm install -g markdownlint-cli@0.45.0)
	@echo "    OK"

.PHONY: ensure-markdownlint-cli2
ensure-markdownlint-cli2:
	@echo "==> markdownlint-cli2"
	@command -v $(MARKDOWNLINT_CLI2) >/dev/null 2>&1 || \
		(echo "    Installing markdownlint-cli2..." && \
		npm install -g markdownlint-cli2)
	@echo "    OK"

.PHONY: ensure-actionlint
ensure-actionlint:
	@echo "==> actionlint"
	@command -v $(ACTIONLINT) >/dev/null 2>&1 || \
		(echo "    Installing actionlint..." && \
		go install github.com/rhysd/actionlint/cmd/actionlint@latest)
	@echo "    OK"

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf bin/

.PHONY: run
run: build
	@echo "Running $(BINARY)..."
	bin/$(BINARY) --help

.PHONY: dev
dev: build
	@echo "Running development build..."
	bin/$(BINARY)

.PHONY: inspect
inspect: build ## Launch the MCP Inspector for interactive testing
	@echo "Starting MCP Inspector for $(BINARY)..."
	@echo "Open the URL shown below in your browser to interact with the MCP server"
	npx @modelcontextprotocol/inspector $$(realpath bin/$(BINARY)) --mcp

.PHONY: docs-lint
docs-lint:
	@echo "Linting documentation..."
	@command -v markdownlint-cli2 >/dev/null 2>&1 || \
		(echo "markdownlint-cli2 not found. Install with:"; \
		echo "  npm install -g markdownlint-cli2"; exit 1)
	markdownlint-cli2 --config docs/.markdownlint-cli2.jsonc 'docs/**/*.md' '#docs/dist/**' '#docs/.astro/**' '#docs/node_modules/**'
	@echo "Documentation linting complete."

.PHONY: docs-sync
docs-sync:
	@echo "Syncing root documentation..."
	@test -f CONTRIBUTING.md || (echo "Error: CONTRIBUTING.md not found"; exit 1)
	@test -f README.md || (echo "Error: README.md not found"; exit 1)
	@mkdir -p docs/src/content/docs/support
	@echo "---" > docs/src/content/docs/support/contributing.md
	@echo "title: Contributing" >> docs/src/content/docs/support/contributing.md
	@echo "description: Development setup, guidelines, and workflow for contributing to FinFocus." >> docs/src/content/docs/support/contributing.md
	@echo "---" >> docs/src/content/docs/support/contributing.md
	@echo "" >> docs/src/content/docs/support/contributing.md
	@cat CONTRIBUTING.md | sed -E \
		-e '/^# /d' \
		-e 's|\]\(docs/src/content/docs/|](../|g' \
		-e 's|\]\(docs/\)|](../../)|g' \
		-e 's|\]\((\.specify/[^)]*)\)|](https://github.com/rshade/finfocus/blob/main/\1)|g' \
		-e 's|\]\(([A-Za-z0-9_][^):]*)\)|](https://github.com/rshade/finfocus/blob/main/\1)|g' \
		>> docs/src/content/docs/support/contributing.md
	@echo "---" > docs/src/content/docs/README.md
	@echo "title: Project README" >> docs/src/content/docs/README.md
	@echo "description: Cloud cost analysis for Pulumi infrastructure with projected costs, budgets, and plugin architecture." >> docs/src/content/docs/README.md
	@echo "---" >> docs/src/content/docs/README.md
	@echo "" >> docs/src/content/docs/README.md
	@echo "<!-- markdownlint-disable MD013 -->" >> docs/src/content/docs/README.md
	@cat README.md | sed -E \
		-e '/^# /d' \
		-e 's|\]\(docs/src/content/docs/|](./|g' \
		-e 's|\]\(docs/\)|](../)|g' \
		-e 's|\]\(CONTRIBUTING\.md\)|](./support/contributing.md)|g' \
		-e 's|\]\(([A-Za-z0-9_][^):]*/)\)|](https://github.com/rshade/finfocus/tree/main/\1)|g' \
		-e 's|\]\(([A-Za-z0-9_][^):]*)\)|](https://github.com/rshade/finfocus/blob/main/\1)|g' \
		>> docs/src/content/docs/README.md
	@echo "Documentation synced."

.PHONY: docs-serve
docs-serve: docs-sync
	@echo "Serving documentation locally at http://localhost:4321/finfocus/"
	@cd docs && npm ci > /dev/null 2>&1
	@cd docs && npm run dev

.PHONY: docs-build
docs-build: docs-sync
	@echo "Building documentation site..."
	@cd docs && npm ci > /dev/null 2>&1
	@cd docs && npm run build
	@echo "Documentation built to docs/dist/"

.PHONY: docs-validate
docs-validate: docs-sync docs-lint
	@echo "Validating documentation structure..."
	@test -f docs/src/content/docs/README.md || (echo "Missing: docs/src/content/docs/README.md"; exit 1)
	@test -f docs/src/content/docs/plan.md || (echo "Missing: docs/src/content/docs/plan.md"; exit 1)
	@bash scripts/validate-llms-txt.sh
	@test -f docs/astro.config.mjs || (echo "Missing: docs/astro.config.mjs"; exit 1)
	@test -f docs/.markdownlint-cli2.jsonc || (echo "Missing: docs/.markdownlint-cli2.jsonc"; exit 1)
	@echo "All required documentation files present"
	@echo "Documentation validation passed"

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build            - Build the binary"
	@echo "  build-recorder   - Build the recorder plugin"
	@echo "  build-plugin     - Build Pulumi tool plugin (pulumi-tool-finfocus)"
	@echo "  install-recorder - Build and install recorder plugin to ~/.finfocus/plugins/"
	@echo "  build-kubernetes - Build the kubernetes plugin"
	@echo "  install-kubernetes - Build and install kubernetes plugin to ~/.finfocus/plugins/"
	@echo "  build-jev        - Build the jev scorer plugin"
	@echo "  install-jev      - Build and install jev scorer plugin to ~/.finfocus/plugins/"
	@echo "  build-prometheus - Build the prometheus usage plugin"
	@echo "  install-prometheus - Build and install prometheus plugin to ~/.finfocus/plugins/"
	@echo "  build-all        - Build binary and all plugins"
	@echo "  test             - Run unit tests (fast, default)"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-kubernetes  - Run kubernetes plugin module tests"
	@echo "  test-jev         - Run jev plugin module tests"
	@echo "  test-prometheus  - Run prometheus plugin module tests"
	@echo "  test-race        - Run unit tests with race detector"
	@echo "  test-integration - Run integration tests (slower)"
	@echo "  test-integration-plugin - Run plugin integration tests"
	@echo "  test-e2e         - Run E2E tests (requires AWS credentials)"
	@echo "  test-e2e-kind    - Run kind-based cost cluster E2E (requires Docker, kind, kubectl)"
	@echo "  test-e2e-web     - Run real Chromium web acceptance (no cloud credentials)"
	@echo "  test-frontend    - Run web renderer regression tests (Node pinned in mise.toml)"
	@echo "  test-all         - Run all tests except E2E"
	@echo "  gen-terraform-goldens - Regenerate real Terraform state goldens (docker + mise)"
	@echo "  lint             - Run Go + Markdown linters"
	@echo "  lint-kubernetes  - Run golangci-lint on the kubernetes plugin module"
	@echo "  lint-jev         - Run golangci-lint on the jev plugin module"
	@echo "  lint-prometheus  - Run golangci-lint on the prometheus plugin module"
	@echo "  check-plugin-boundaries - Verify plugins/kubernetes, plugins/jev, and plugins/prometheus do not import finfocus core packages"
	@echo "  lint-actions     - Run actionlint on GitHub workflows"
	@echo "  validate         - Run validation (go mod tidy, go vet)"
	@echo "  tools            - Install toolchain pinned in mise.toml (mise install)"
	@echo "  ensure           - Install all required dev tools"
	@echo "  clean            - Clean build artifacts"
	@echo "  run              - Build and run with --help"
	@echo "  dev              - Build and run"
	@echo "  inspect          - Launch MCP Inspector for interactive testing"
	@echo ""
	@echo "Documentation targets:"
	@echo "  docs-lint        - Lint documentation markdown"
	@echo "  docs-sync        - Sync root docs (README.md, CONTRIBUTING.md) to docs site"
	@echo "  docs-build       - Build documentation site"
	@echo "  docs-serve       - Serve documentation locally (http://localhost:4000)"
	@echo "  docs-validate    - Validate documentation structure"
	@echo ""
	@echo "E2E test options (make test-e2e TEST_ARGS='...'):"
	@echo "  -run TestName    - Run specific test"
	@echo "  -short           - Run without verbose output"
	@echo "  -timeout N       - Set timeout to N minutes"
	@echo ""
	@echo "  help             - Show this help message"
