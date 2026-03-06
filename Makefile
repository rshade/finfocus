BINARY=finfocus
VERSION?=$(shell git describe --tags --always --dirty)
COMMIT=$(shell git rev-parse HEAD)
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

GOLANGCI_LINT?=$(HOME)/go/bin/golangci-lint
GOLANGCI_LINT_VERSION?=2.9.0
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

.PHONY: build-all
build-all: build build-recorder build-plugin

# Default test target - runs unit tests only (fast, for CI and local dev)
# Unit tests are colocated with source; see test/README.md for details
.PHONY: test
test: test-unit

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

# Run all tests (unit + integration, excludes E2E which requires special setup)
.PHONY: test-all
test-all:
	@echo "Running all tests (unit + integration)..."
	go test -v -timeout 15m ./internal/... ./pkg/... ./test/integration/...

.PHONY: lint
lint:
	@echo "Running golangci-lint (expected version $(GOLANGCI_LINT_VERSION))..."
	@$(GOLANGCI_LINT) --version | grep -q "$(GOLANGCI_LINT_VERSION)" || \
		(echo "golangci-lint $(GOLANGCI_LINT_VERSION) required. Install with"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$HOME/go/bin v$(GOLANGCI_LINT_VERSION)"; exit 1)
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
	$(ACTIONLINT)

.PHONY: validate
validate:
	@echo "Running validation..."
	@echo "Checking go modules..."
	go mod tidy -diff
	@echo "Running go vet..."
	go vet ./...
	@echo "Validation complete."

.PHONY: ensure
ensure: ensure-golangci-lint ensure-markdownlint ensure-markdownlint-cli2 ensure-actionlint
	@echo "All dev tools are ready."

.PHONY: ensure-golangci-lint
ensure-golangci-lint:
	@echo "==> golangci-lint $(GOLANGCI_LINT_VERSION)"
	@$(GOLANGCI_LINT) --version 2>/dev/null | grep -q "$(GOLANGCI_LINT_VERSION)" || \
		(echo "    Installing golangci-lint v$(GOLANGCI_LINT_VERSION)..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$HOME/go/bin v$(GOLANGCI_LINT_VERSION))
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
	npx @modelcontextprotocol/inspector $$(realpath bin/$(BINARY))

.PHONY: docs-lint
docs-lint:
	@echo "Linting documentation..."
	@command -v markdownlint-cli2 >/dev/null 2>&1 || \
		(echo "markdownlint-cli2 not found. Install with:"; \
		echo "  npm install -g markdownlint-cli2"; exit 1)
	markdownlint-cli2 --config docs/.markdownlint-cli2.jsonc 'docs/**/*.md' --ignore 'docs/_site/**'
	@echo "Documentation linting complete."

.PHONY: docs-sync
docs-sync:
	@echo "Syncing root documentation..."
	@test -f CONTRIBUTING.md || (echo "Error: CONTRIBUTING.md not found"; exit 1)
	@test -f ROADMAP.md || (echo "Error: ROADMAP.md not found"; exit 1)
	@echo "---" > docs/support/contributing.md
	@echo "title: Contributing" >> docs/support/contributing.md
	@echo "layout: default" >> docs/support/contributing.md
	@echo "---" >> docs/support/contributing.md
	@echo "" >> docs/support/contributing.md
	@cat CONTRIBUTING.md | sed 's|docs/|../|g' >> docs/support/contributing.md
	@echo "---" > docs/architecture/roadmap.md
	@echo "title: Roadmap" >> docs/architecture/roadmap.md
	@echo "layout: default" >> docs/architecture/roadmap.md
	@echo "---" >> docs/architecture/roadmap.md
	@echo "" >> docs/architecture/roadmap.md
	@cat ROADMAP.md >> docs/architecture/roadmap.md
	@echo "Documentation synced."

.PHONY: docs-serve
docs-serve: docs-sync
	@echo "Serving documentation locally at http://localhost:4000/finfocus"
	@cd docs && bundle install > /dev/null 2>&1
	@cd docs && bundle exec jekyll serve --host 0.0.0.0

.PHONY: docs-build
docs-build: docs-sync
	@echo "Building documentation site..."
	@cd docs && bundle install > /dev/null 2>&1
	@cd docs && bundle exec jekyll build
	@echo "Documentation built to docs/_site/"

.PHONY: docs-validate
docs-validate: docs-lint
	@echo "Validating documentation structure..."
	@test -f docs/README.md || (echo "Missing: docs/README.md"; exit 1)
	@test -f docs/plan.md || (echo "Missing: docs/plan.md"; exit 1)
	@test -f docs/llms.txt || (echo "Missing: docs/llms.txt"; exit 1)
	@test -f docs/_config.yml || (echo "Missing: docs/_config.yml"; exit 1)
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
	@echo "  build-all        - Build binary and all plugins"
	@echo "  test             - Run unit tests (fast, default)"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-race        - Run unit tests with race detector"
	@echo "  test-integration - Run integration tests (slower)"
	@echo "  test-integration-plugin - Run plugin integration tests"
	@echo "  test-e2e         - Run E2E tests (requires AWS credentials)"
	@echo "  test-all         - Run all tests except E2E"
	@echo "  lint             - Run Go + Markdown linters"
	@echo "  lint-actions     - Run actionlint on GitHub workflows"
	@echo "  validate         - Run validation (go mod tidy, go vet)"
	@echo "  ensure           - Install all required dev tools"
	@echo "  clean            - Clean build artifacts"
	@echo "  run              - Build and run with --help"
	@echo "  dev              - Build and run"
	@echo "  inspect          - Launch MCP Inspector for interactive testing"
	@echo ""
	@echo "Documentation targets:"
	@echo "  docs-lint        - Lint documentation markdown"
	@echo "  docs-sync        - Sync root docs (CONTRIBUTING.md, ROADMAP.md) to docs site"
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
