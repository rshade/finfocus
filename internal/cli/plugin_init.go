package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/spf13/cobra"
)

// PluginInitOptions contains configuration options for plugin initialization.
type PluginInitOptions struct {
	Name           string
	Author         string
	Providers      []string
	OutputDir      string
	Force          bool
	RecordFixtures bool
	FixtureVersion string
	Offline        bool
	Strict         bool
}

const (
	pluginDirPerm  os.FileMode = 0o750
	pluginFilePerm os.FileMode = 0o640
)

const pluginReadmeTemplate = `# {{NAME}}

FinFocus plugin for {{NAME}} cost calculation.

## Overview

This plugin provides cost calculation capabilities for {{PROVIDERS}} resources in FinFocus. It implements both projected cost estimation and actual cost retrieval functionality.

**Supported Providers:** {{PROVIDERS}}

## Installation

### From Source

1. Clone the repository:
   {{BASH_BLOCK_START}}
   git clone <repository-url>
   cd {{NAME}}
   {{CODE_BLOCK_END}}

2. Build the plugin:
   {{BASH_BLOCK_START}}
   make build
   {{CODE_BLOCK_END}}

3. Install to local plugin registry:
   {{BASH_BLOCK_START}}
   make install
   {{CODE_BLOCK_END}}

### Configuration

The plugin may require cloud provider credentials to function properly. See the configuration section for details.

## Usage

Once installed, the plugin will be automatically discovered by FinFocus:

{{BASH_BLOCK_START}}
# List installed plugins
finfocus plugin list

# Validate plugin installation
finfocus plugin validate

# Calculate projected costs
finfocus cost projected --pulumi-json plan.json

# Get actual costs
finfocus cost actual --pulumi-json plan.json --from 2025-01-01
{{CODE_BLOCK_END}}

## Development

### Prerequisites

- Go 1.21+
- FinFocus Core development environment
- Cloud provider credentials (for actual cost retrieval)

### Building

{{BASH_BLOCK_START}}
# Build the plugin
make build

# Run tests
make test

# Run linters
make lint
{{CODE_BLOCK_END}}

### Project Structure

- {{INLINE_CMD_PLUGIN}}: Plugin entry point
- {{INLINE_INTERNAL_PRICING}}: Pricing logic and calculators
- {{INLINE_INTERNAL_CLIENT}}: Cloud provider client implementation
- {{INLINE_EXAMPLES}}: Example usage
- {{INLINE_BIN}}: Compiled binaries

### Implementing Pricing Logic

Edit {{INLINE_CALCULATOR_FILE}} to implement your pricing logic:

{{GO_BLOCK_START}}
func (c *Calculator) GetProjectedCost(ctx context.Context, req *pbc.GetProjectedCostRequest) (*pbc.GetProjectedCostResponse, error) {
    // 1. Check if resource is supported
    if !c.Matcher().Supports(req.Resource) {
        return nil, pluginsdk.NotSupportedError(req.Resource)
    }

    // 2. Extract resource properties
    resourceType := req.Resource.ResourceType
    properties := req.Resource.Tags

    // 3. Calculate pricing based on resource type and properties
    unitPrice := c.calculateResourceCost(resourceType, properties)

    // 4. Return response
    return c.Calculator().CreateProjectedCostResponse("USD", unitPrice, "description"), nil
}
{{CODE_BLOCK_END}}

#### Actual Cost Retrieval

Edit {{INLINE_CLIENT_FILE}} to implement cloud provider API integration:

{{GO_BLOCK_START}}
func (c *Client) GetResourceCost(ctx context.Context, resourceID string, startTime, endTime int64) (float64, error) {
    // 1. Call cloud provider billing API
    // 2. Parse response and calculate total cost
    // 3. Return cost value
    return totalCost, nil
}
{{CODE_BLOCK_END}}

### Testing

The project includes testing utilities from the FinFocus SDK:

{{GO_BLOCK_START}}
func TestPluginName(t *testing.T) {
    plugin := pricing.NewCalculator()
    testPlugin := pluginsdk.NewTestPlugin(t, plugin)
    testPlugin.TestName({{QUOTED_NAME}})
}
{{CODE_BLOCK_END}}

### Adding Pricing Data

1. Update pricing data structures in {{INLINE_DATA_FILE}}
2. Implement pricing lookups in {{INLINE_CALCULATOR_FILE}}
3. Add test cases for new resource types

### Configuration

The plugin supports the following configuration options:

- Environment variables for cloud provider credentials
- Pricing data files for offline pricing calculations
- Regional pricing variations

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run {{INLINE_MAKE_LINT_TEST}}
6. Submit a pull request

## License

[Add your license information here]

## Support

[Add support contact information here]
`

//nolint:lll // Template content requires long lines for readability
const pluginIssuesTemplate = `# Plugin Development Issues

This file contains recommended issues to create for your plugin development.
Use these as templates when creating GitHub issues for your plugin repository.

---

## Issue 1: E2E Testing with Real Cloud APIs

**Title:** Implement E2E tests with real {{PROVIDERS}} billing APIs

**Labels:** testing, e2e, priority:high

**Description:**

Implement end-to-end tests that validate the plugin against real cloud billing APIs.

### Acceptance Criteria

- [ ] E2E test for {{BACKTICK}}GetProjectedCost(){{BACKTICK}} with real {{PROVIDERS}} resources
- [ ] E2E test for {{BACKTICK}}GetActualCost(){{BACKTICK}} calling real billing APIs ({{PROVIDERS}} Cost Explorer/Billing API)
- [ ] Test account setup documented in README
- [ ] CI workflow for periodic E2E runs (weekly/monthly)
- [ ] Cost validation within ±5% of official pricing

### Technical Notes

- Use isolated test accounts with budget alerts
- Store credentials as CI secrets, never in code
- Consider using tagged resources for easy cleanup
- Billing API data may have 24-72 hour delay

### Test Account Requirements

- Dedicated test account with billing access
- Budget alerts set (e.g., $50/month limit)
- IAM role/service principal with minimal permissions
- Resource tagging for cost tracking

---

## Issue 2: Pricing Data Validation

**Title:** Validate pricing accuracy against official {{PROVIDERS}} pricing

**Labels:** testing, accuracy, priority:high

**Description:**

Create a validation suite that compares plugin pricing against official cloud provider pricing pages.

### Acceptance Criteria

- [ ] Automated comparison against official pricing API/pages
- [ ] Coverage for all supported resource types
- [ ] Regional pricing variations tested
- [ ] Currency handling validated
- [ ] Pricing update detection (alert when official prices change)

### Resources

- Official pricing APIs (if available)
- Pricing calculator URLs for manual verification
- Historical pricing data for trend analysis

---

## Issue 3: Regional Coverage

**Title:** Ensure complete regional coverage for {{PROVIDERS}}

**Labels:** feature, coverage, priority:medium

**Description:**

Verify the plugin supports all target regions with accurate pricing.

### Acceptance Criteria

- [ ] List of supported regions documented
- [ ] Pricing data for each region
- [ ] Region-specific resource variations handled
- [ ] Tests for region edge cases (new regions, deprecated regions)

### Regions to Support

Document the regions you plan to support:
- [ ] Primary regions (e.g., us-east-1, westeurope, us-central1)
- [ ] Secondary regions
- [ ] Edge/specialized regions (govcloud, china, etc.)

---

## Issue 4: CI/CD Pipeline Setup

**Title:** Set up CI/CD pipeline for automated testing and releases

**Labels:** infrastructure, ci-cd, priority:high

**Description:**

Configure continuous integration and deployment for the plugin.

### Acceptance Criteria

- [ ] GitHub Actions workflow for PRs (lint, test, build)
- [ ] Conformance tests run on every PR
- [ ] E2E tests run on schedule (not every PR - too slow/expensive)
- [ ] Automated releases on version tags
- [ ] Binary builds for multiple platforms

### Workflow Structure

{{CODE_BLOCK_START}}yaml
# .github/workflows/ci.yml
- Lint (golangci-lint)
- Unit tests with coverage
- Conformance tests (finfocus plugin conformance)
- Build verification

# .github/workflows/e2e.yml (scheduled)
- E2E tests with real cloud resources
- Pricing accuracy validation
- Run weekly or monthly
{{CODE_BLOCK_END}}

---

## Issue 5: Documentation

**Title:** Complete plugin documentation

**Labels:** documentation, priority:medium

**Description:**

Create comprehensive documentation for plugin users and contributors.

### Acceptance Criteria

- [ ] README with installation and usage instructions
- [ ] Supported resources list with pricing details
- [ ] Configuration guide (credentials, regions, options)
- [ ] Troubleshooting guide
- [ ] Contributing guide
- [ ] Changelog maintenance

---

## Issue 6: API Response Caching

**Title:** Implement caching for external API calls

**Labels:** performance, enhancement, priority:low

**Description:**

If your plugin makes external API calls (pricing APIs, billing APIs), implement caching to reduce latency and API costs.

### Acceptance Criteria

- [ ] LRU cache for pricing API responses (if applicable)
- [ ] Configurable TTL (e.g., 1 hour for pricing data)
- [ ] Cache key includes all pricing factors (SKU, region, etc.)
- [ ] Cache bypass option for testing
- [ ] Memory-bounded cache size

### When to Implement

This is only needed if your plugin:
- Calls external pricing APIs (AWS Pricing API, Azure Retail Prices, etc.)
- Has expensive computation per resource type
- Experiences latency issues with large Pulumi previews

**Skip this if:** Your plugin uses embedded pricing data (no external API calls).

### Implementation Notes

{{CODE_BLOCK_START}}go
// Example using hashicorp/golang-lru
import lru "github.com/hashicorp/golang-lru/v2"

type PriceCache struct {
    cache *lru.Cache[string, *PriceResult]
    ttl   time.Duration
}

func (c *PriceCache) Get(key string) (*PriceResult, bool) {
    // Check cache, validate TTL
}
{{CODE_BLOCK_END}}

### Cache Key Strategy

For projected costs, cache key should include:
- Provider (aws, azure, gcp)
- Resource type (ec2:instance, compute:vm)
- SKU/instance type (t3.micro, Standard_B1s)
- Region (us-east-1, westeurope)

**Note:** Actual costs (Cost Explorer data) should NOT be cached - each resource has unique historical costs.

---

## How to Use This File

1. Create a GitHub repository for your plugin
2. Copy each issue section above into a new GitHub issue
3. Customize the details for your specific provider
4. Prioritize and assign issues
5. Delete this file once issues are created (or keep for reference)

### Creating Issues via GitHub CLI

{{CODE_BLOCK_START}}bash
# Example: Create the E2E testing issue
gh issue create \
  --title "Implement E2E tests with real {{PROVIDERS}} billing APIs" \
  --label "testing,e2e,priority:high" \
  --body "$(cat << 'EOF'
[Paste issue description here]
EOF
)"
{{CODE_BLOCK_END}}
`

// runRecordingWorkflow resolves fixtures, downloads required plan and state fixtures,
// executes a recording workflow for each provider in opts, validates the recordings,
// and copies recorded requests into the project's testdata/recorded_requests directory.
//
// The ctx controls cancellation for network and IO operations. The cmd is used for
// user-facing progress output. The logger records structured diagnostics. opts
// provides initialization flags and the list of providers to record. projectDir
// is the root directory of the generated plugin project where recordings are stored.
//
// On success, recorded request files are written under projectDir/testdata/recorded_requests.
// The function returns an error if fixture resolution or download fails, if the recording
// workflow or validation fails for any provider, or if copying recorded requests into
// the project directory fails. Temporary downloaded fixtures are removed when opts.Offline
// is false.
func runRecordingWorkflow(
	ctx context.Context,
	cmd *cobra.Command,
	logger zerolog.Logger,
	opts *PluginInitOptions,
	projectDir string,
) error {
	resolver := NewFixtureResolver(
		logger,
		opts.Offline,
		opts.FixtureVersion,
		"test/fixtures",
	)

	// Resolve and download state fixture once (provider-agnostic)
	stateSource, stateErr := resolver.ResolveStateFixture(ctx)
	if stateErr != nil {
		return fmt.Errorf("resolving state fixture: %w", stateErr)
	}

	cmd.Printf("  Resolved state fixture: %s\n", stateSource.Origin)

	statePath, stateDownloadErr := resolver.DownloadFixture(ctx, stateSource)
	if stateDownloadErr != nil {
		return fmt.Errorf("downloading state fixture: %w", stateDownloadErr)
	}
	// Only remove downloaded files, not local fixtures
	if !opts.Offline {
		defer os.Remove(statePath)
	}

	recordingWorkflow := NewRecorderWorkflow(
		logger,
		projectDir,
	)

	if prepareErr := recordingWorkflow.PrepareOutputDirectory(); prepareErr != nil {
		return fmt.Errorf("preparing recording directory: %w", prepareErr)
	}

	totalRecorded := 0

	// Iterate over all providers
	for _, provider := range opts.Providers {
		cmd.Printf("  Processing provider: %s\n", provider)

		planSource, planErr := resolver.ResolvePlanFixture(ctx, provider)
		if planErr != nil {
			logger.Warn().
				Str("provider", provider).
				Err(planErr).
				Msg("failed to resolve plan fixture for provider")
			return fmt.Errorf("resolving plan fixture for %s: %w", provider, planErr)
		}

		cmd.Printf("    Resolved plan fixture: %s\n", planSource.Origin)

		planPath, downloadErr := resolver.DownloadFixture(ctx, planSource)
		if downloadErr != nil {
			logger.Warn().
				Str("provider", provider).
				Err(downloadErr).
				Msg("failed to download plan fixture")
			return fmt.Errorf("downloading plan fixture for %s: %w", provider, downloadErr)
		}
		// Only remove downloaded files, not local fixtures
		if !opts.Offline {
			defer os.Remove(planPath)
		}

		cmd.Printf("    Recording requests for %s...\n", provider)

		if recordErr := recordingWorkflow.RunWithRecorder(ctx, planPath, statePath, true); recordErr != nil {
			logger.Warn().
				Str("provider", provider).
				Err(recordErr).
				Msg("recording workflow failed for provider")
			return fmt.Errorf("running recording workflow for %s: %w", provider, recordErr)
		}

		if validateErr := recordingWorkflow.ValidateRecordings(); validateErr != nil {
			logger.Warn().
				Str("provider", provider).
				Err(validateErr).
				Msg("validation failed for provider recordings")
			return fmt.Errorf("validating recordings for %s: %w", provider, validateErr)
		}

		session := recordingWorkflow.Session()
		totalRecorded += len(session.Recorded)
		cmd.Printf("    ✅ Recorded %d requests for %s\n", len(session.Recorded), provider)
	}

	if copyErr := recordingWorkflow.CopyRecordedRequestsToTestdata(projectDir); copyErr != nil {
		return fmt.Errorf("copying recorded requests: %w", copyErr)
	}

	cmd.Printf("  ✅ Total recorded %d requests to testdata/recorded_requests/\n", totalRecorded)
	return nil
}

// renderIssues renders the issues.md template by substituting placeholder tokens.
// The providers slice is joined with ", " and placed into the `{{PROVIDERS}}` token; additional tokens for backticks and code block delimiters are also replaced.
// It returns the rendered markdown content as a string.
func renderIssues(providers []string) string {
	replacements := map[string]string{
		"{{PROVIDERS}}":        strings.Join(providers, ", "),
		"{{BACKTICK}}":         "`",
		"{{CODE_BLOCK_START}}": "```",
		"{{CODE_BLOCK_END}}":   "```",
	}
	content := pluginIssuesTemplate
	for token, value := range replacements {
		content = strings.ReplaceAll(content, token, value)
	}
	return content
}

func renderReadme(name string, providers []string) string {
	quotedName := fmt.Sprintf("%q", name)
	replacements := map[string]string{
		"{{NAME}}":                    name,
		"{{PROVIDERS}}":               strings.Join(providers, ", "),
		"{{QUOTED_NAME}}":             quotedName,
		"{{BASH_BLOCK_START}}":        "```bash",
		"{{GO_BLOCK_START}}":          "```go",
		"{{CODE_BLOCK_END}}":          "```",
		"{{INLINE_CMD_PLUGIN}}":       "`cmd/plugin`",
		"{{INLINE_INTERNAL_PRICING}}": "`internal/pricing`",
		"{{INLINE_INTERNAL_CLIENT}}":  "`internal/client`",
		"{{INLINE_EXAMPLES}}":         "`examples`",
		"{{INLINE_BIN}}":              "`bin`",
		"{{INLINE_CALCULATOR_FILE}}":  "`internal/pricing/calculator.go`",
		"{{INLINE_CLIENT_FILE}}":      "`internal/client/client.go`",
		"{{INLINE_DATA_FILE}}":        "`internal/pricing/data.go`",
		"{{INLINE_MAKE_LINT_TEST}}":   "`make lint test`",
	}
	content := pluginReadmeTemplate
	for token, value := range replacements {
		content = strings.ReplaceAll(content, token, value)
	}
	return content
}

// NewPluginInitCmd builds the Cobra command that scaffolds a plugin project and writes
// NewPluginInitCmd creates and returns a Cobra command for initializing a plugin development project.
// The command exposes "init <plugin-name>" and scaffolds a new plugin project including module setup,
// manifest, boilerplate source, Makefile, README, and example tests. It registers flags for author,
// providers, output directory, force overwrite, and fixture recording options (`--record-fixtures`,
// `--fixture-version`, `--offline`, `--strict`) and marks the author and providers flags as required.
func NewPluginInitCmd() *cobra.Command {
	var opts PluginInitOptions

	cmd := &cobra.Command{
		Use:   "init <plugin-name>",
		Short: "Initialize a new plugin development project",
		Long: `Initialize a new plugin development project with boilerplate code and manifest.

This command creates a new directory structure for plugin development including:
- Go module initialization
- Plugin manifest (manifest.yaml)
- Boilerplate main.go and plugin implementation
- Makefile with build scripts
- README.md with development instructions
- Example tests`,
		Args: cobra.ExactArgs(1),
		Example: `  # Initialize an AWS plugin
  finfocus plugin init aws-plugin --author "Your Name" --providers aws

  # Initialize a multi-provider plugin
  finfocus plugin init cloud-plugin --author "Your Name" --providers aws,azure,gcp

  # Initialize in a specific directory
  finfocus plugin init my-plugin --author "Your Name" --providers aws --output-dir /path/to/plugins`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return RunPluginInit(cmd, &opts)
		},
	}

	cmd.Flags().StringVar(&opts.Author, "author", "", "Plugin author name (required)")
	cmd.Flags().
		StringSliceVar(&opts.Providers, "providers", []string{}, "Supported cloud providers (e.g., aws,azure,gcp)")
	cmd.Flags().StringVar(&opts.OutputDir, "output-dir", ".", "Output directory for plugin project")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Overwrite existing files if directory exists")
	cmd.Flags().BoolVar(&opts.RecordFixtures, "record-fixtures", false,
		"Generate recorded request fixtures during initialization")
	cmd.Flags().StringVar(&opts.FixtureVersion, "fixture-version", "latest",
		"Version of fixtures to fetch (tag or 'latest')")
	cmd.Flags().BoolVar(&opts.Offline, "offline", false,
		"Use local fixtures without network access")
	cmd.Flags().BoolVar(&opts.Strict, "strict", false,
		"Fail initialization if recording workflow encounters errors")

	_ = cmd.MarkFlagRequired("author")
	_ = cmd.MarkFlagRequired("providers")

	return cmd
}

// RunPluginInit validates the provided PluginInitOptions, creates the target project directory (honoring the force flag),
// generates the boilerplate project files, and prints progress and next-step instructions to the command output.
// RunPluginInit initializes a new plugin project on disk and scaffolds all required files.
//
// RunPluginInit validates the plugin name and provider list, creates the project
// directory, generates project files, and optionally runs a fixture recording
// workflow to populate testdata. It prints progress and next-step instructions to
// the provided Cobra command's output.
//
// cmd is the Cobra command used for printing progress and for deriving context.
// opts provides initialization options such as Name, Author, Providers, OutputDir,
// Force, and flags controlling fixture recording.
//
// It returns an error if validation fails (invalid name or no providers), if the
// project directory cannot be created, if file generation fails, or if the optional
// fixture recording workflow fails when the Strict option is set.
func RunPluginInit(cmd *cobra.Command, opts *PluginInitOptions) error {
	// Validate plugin name
	if !IsValidPluginName(opts.Name) {
		return fmt.Errorf(
			"invalid plugin name: %s (must contain only lowercase letters, numbers, and hyphens)",
			opts.Name,
		)
	}

	// Validate providers
	if len(opts.Providers) == 0 {
		return errors.New("at least one provider must be specified")
	}

	// Create project directory
	projectDir := filepath.Join(opts.OutputDir, opts.Name)
	if err := createProjectDirectory(projectDir, opts.Force); err != nil {
		return fmt.Errorf("creating project directory: %w", err)
	}

	cmd.Printf("Initializing plugin project: %s\n", opts.Name)
	cmd.Printf("Project directory: %s\n", projectDir)
	cmd.Printf("Author: %s\n", opts.Author)
	cmd.Printf("Supported providers: %s\n", strings.Join(opts.Providers, ", "))

	// Generate project files
	generator := &projectGenerator{
		name:       opts.Name,
		author:     opts.Author,
		providers:  opts.Providers,
		projectDir: projectDir,
		cmd:        cmd,
	}

	if err := generator.generateAll(); err != nil {
		return fmt.Errorf("generating project files: %w", err)
	}

	// Run fixture recording workflow if enabled
	if opts.RecordFixtures {
		cmd.Printf("\n📦 Setting up recorded fixtures...\n")

		loggerPtr := logging.FromContext(cmd.Context())
		if loggerPtr == nil {
			// Fallback to a visible console logger if context doesn't have one
			consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
			logger := zerolog.New(consoleWriter).With().Timestamp().Logger()
			logger.Warn().Msg("context logger not available, using fallback console logger")
			loggerPtr = &logger
		}

		if recordErr := runRecordingWorkflow(cmd.Context(), cmd, *loggerPtr, opts, projectDir); recordErr != nil {
			if opts.Strict {
				return fmt.Errorf("recording workflow failed: %w", recordErr)
			}
			cmd.Printf("⚠️  Recording workflow failed: %v\n", recordErr)
			cmd.Printf("   Plugin initialized but without recorded fixtures.\n")
		}
	}

	cmd.Printf("\n✅ Plugin project initialized successfully!\n\n")
	cmd.Printf("Next steps:\n")
	cmd.Printf("1. cd %s\n", projectDir)
	cmd.Printf("2. go mod tidy\n")
	cmd.Printf("3. make build\n")
	cmd.Printf("4. Edit internal/pricing/calculator.go to implement your pricing logic\n")
	cmd.Printf("5. Edit internal/client/client.go to implement your cloud provider client\n\n")

	if opts.RecordFixtures {
		testdataDir := filepath.Join(projectDir, "testdata", "recorded_requests")
		if _, err := os.Stat(testdataDir); err == nil {
			cmd.Printf("Recorded fixtures available at: %s\n", testdataDir)
		}
	}

	cmd.Printf("For more information, see the README.md file in your project.\n")

	return nil
}

type projectGenerator struct {
	name       string
	author     string
	providers  []string
	projectDir string
	cmd        *cobra.Command
}

func (g *projectGenerator) generateAll() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Creating directory structure", g.createDirectories},
		{"Generating go.mod", g.generateGoMod},
		{"Generating plugin manifest", g.generateManifest},
		{"Generating main.go", g.generateMainGo},
		{"Generating plugin implementation", g.generatePlugin},
		{"Generating pricing calculator", g.generatePricingCalculator},
		{"Generating cloud client", g.generateCloudClient},
		{"Generating Makefile", g.generateMakefile},
		{"Generating README.md", g.generateReadme},
		{"Generating example tests", g.generateTests},
		{"Generating issues.md", g.generateIssues},
	}

	for _, step := range steps {
		g.cmd.Printf("  %s...\n", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	return nil
}

func (g *projectGenerator) createDirectories() error {
	dirs := []string{
		"cmd/plugin",
		"internal/pricing",
		"internal/client",
		"examples",
		"bin",
	}

	for _, dir := range dirs {
		fullDir := filepath.Join(g.projectDir, dir)
		if err := os.MkdirAll(fullDir, pluginDirPerm); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}

func (g *projectGenerator) generateGoMod() error {
	content := fmt.Sprintf(`module github.com/example/%s

go 1.25.5

require (
	github.com/rshade/finfocus-spec v0.4.1
	google.golang.org/grpc v1.77.0
)
`, g.name)

	return g.writeFile("go.mod", content)
}

func (g *projectGenerator) generateManifest() error {
	// Create manifest using proto structs
	manifest := &pbc.PluginManifest{
		Metadata: &pbc.PluginMetadata{
			Name:        g.name,
			Version:     "0.1.0",
			Description: fmt.Sprintf("FinFocus plugin for %s", g.name),
			Author:      g.author,
			License:     "Apache-2.0",
			Keywords:    append([]string{"finfocus", "cost", "plugin"}, g.providers...),
		},
		Specification: &pbc.PluginSpecification{
			SpecVersion:        "1.0",
			SupportedProviders: g.providers,
			Capabilities:       []string{"projected_cost", "actual_cost"},
		},
	}

	// Save as YAML
	if err := pluginsdk.SaveManifest(filepath.Join(g.projectDir, "manifest.yaml"), manifest); err != nil {
		return err
	}
	// Save as JSON
	return pluginsdk.SaveManifest(filepath.Join(g.projectDir, "manifest.json"), manifest)
}

func (g *projectGenerator) generateMainGo() error {
	content := fmt.Sprintf(`package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/%s/internal/pricing"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

func main() {
	// Create the plugin implementation
	plugin := pricing.NewCalculator()

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Start serving the plugin
	config := pluginsdk.ServeConfig{
		Plugin: plugin,
		Port:   0, // Let the system choose a port
	}

	log.Printf("Starting %%s plugin...", plugin.Name())
	if err := pluginsdk.Serve(ctx, config); err != nil {
		log.Fatalf("Failed to serve plugin: %%v", err)
	}
}
`, g.name)

	return g.writeFile("cmd/plugin/main.go", content)
}

func (g *projectGenerator) generatePlugin() error {
	providersStr := strings.Join(g.providers, `", "`)
	content := fmt.Sprintf(`package pricing

import (
	"context"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// Calculator implements the FinFocus plugin interface for %s.
type Calculator struct {
	*pluginsdk.BasePlugin
}

// NewCalculator creates a new %s cost calculator plugin.
func NewCalculator() *Calculator {
	base := pluginsdk.NewBasePlugin("%s")
	
	// Configure supported providers
	providers := []string{"%s"}
	for _, provider := range providers {
		base.Matcher().AddProvider(provider)
	}

	return &Calculator{
		BasePlugin: base,
	}
}

// GetProjectedCost calculates projected costs for resources.
func (c *Calculator) GetProjectedCost(ctx context.Context, req *pbc.GetProjectedCostRequest) (*pbc.GetProjectedCostResponse, error) {
	// Check if we support this resource
	if !c.Matcher().Supports(req.Resource) {
		return nil, pluginsdk.NotSupportedError(req.Resource)
	}

	// [TEMPLATE] Implementation Required: Pricing Logic
	// Calculate the unit price based on the resource type and tags.
	// The example below demonstrates how to handle AWS EC2 instances.
	// Replace this with your provider's specific logic.
	unitPrice := 0.0
	billingDetail := "Pricing not implemented"

	// Example: Basic EC2 instance pricing
	switch req.Resource.ResourceType {
	case "aws:ec2:Instance":
		unitPrice = c.calculateEC2InstanceCost(req.Resource)
		billingDetail = "EC2 instance hourly cost"
	default:
		return nil, pluginsdk.NotSupportedError(req.Resource)
	}

	return c.Calculator().CreateProjectedCostResponse("USD", unitPrice, billingDetail), nil
}

// GetActualCost retrieves actual historical costs.
func (c *Calculator) GetActualCost(ctx context.Context, req *pbc.GetActualCostRequest) (*pbc.GetActualCostResponse, error) {
	// [TEMPLATE] Implementation Required: Actual Cost Retrieval
	// Connect to your cloud provider's billing API to retrieve historical cost data.
	// If actual cost retrieval is not supported, keep returning NoDataError.
	return nil, pluginsdk.NoDataError(req.ResourceId)
}

// calculateEC2InstanceCost is an example pricing calculation.
func (c *Calculator) calculateEC2InstanceCost(resource *pbc.ResourceDescriptor) float64 {
	// [TEMPLATE] Implementation Required: Pricing Calculation
	// This is a simplified example. A real implementation should:
	// 1. Parse instance type from resource properties
	// 2. Look up pricing from your provider's Pricing API or local pricing data
	// 3. Consider region, operating system, tenancy, etc.
	
	instanceType := resource.Tags["instanceType"]
	if instanceType == "" {
		instanceType = "t3.micro" // default
	}

	// Simplified pricing - replace with real pricing data
	switch instanceType {
	case "t3.micro":
		return 0.0104 // $0.0104/hour
	case "t3.small":
		return 0.0208 // $0.0208/hour
	case "t3.medium":
		return 0.0416 // $0.0416/hour
	default:
		return 0.0104 // fallback
	}
}
`, g.name, g.name, g.name, providersStr)

	return g.writeFile("internal/pricing/calculator.go", content)
}

func (g *projectGenerator) generatePricingCalculator() error {
	content := `package pricing

// This file contains pricing calculation utilities and data structures.
// Implement your cloud provider pricing logic here.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PricingData represents pricing information for resources.
type PricingData struct {
	Provider     string             ` + "`json:\"provider\"`" + `
	Region       string             ` + "`json:\"region\"`" + `
	ResourceType string             ` + "`json:\"resource_type\"`" + `
	Pricing      map[string]float64 ` + "`json:\"pricing\"`" + `
}

// LoadPricingData loads pricing data from a JSON file.
func LoadPricingData(path string) ([]PricingData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pricing data: %w", err)
	}

	var pricing []PricingData
	if err := json.Unmarshal(data, &pricing); err != nil {
		return nil, fmt.Errorf("parsing pricing data: %w", err)
	}

	return pricing, nil
}

// SavePricingData saves pricing data to a JSON file.
func SavePricingData(path string, data []PricingData) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling pricing data: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("writing pricing data: %w", err)
	}

	return nil
}

// Example pricing data structure - customize for your provider
func CreateExamplePricingData() []PricingData {
	return []PricingData{
		{
			Provider:     "aws",
			Region:       "us-east-1",
			ResourceType: "aws:ec2:Instance",
			Pricing: map[string]float64{
				"t3.micro":  0.0104,
				"t3.small":  0.0208,
				"t3.medium": 0.0416,
				"t3.large":  0.0832,
				"t3.xlarge": 0.1664,
			},
		},
	}
}
`

	return g.writeFile("internal/pricing/data.go", content)
}

func (g *projectGenerator) generateCloudClient() error {
	content := `package client

// This file contains the cloud provider client implementation.
// Implement API clients for your specific cloud provider here.

import (
	"context"
	"fmt"
)

// Client represents a client for your cloud provider's APIs.
type Client struct {
	// Add your cloud provider client configuration here
	// Examples:
	// - AWS SDK client
	// - Azure SDK client  
	// - GCP SDK client
	// - Custom API client
}

// Config holds configuration for the cloud provider client.
type Config struct {
	// Add configuration fields specific to your provider
	// Examples:
	// APIKey      string
	// Region      string
	// Credentials *Credentials
}

// NewClient creates a new cloud provider client.
func NewClient(config Config) (*Client, error) {
	// [TEMPLATE] Implementation Required: Client Initialization
	// Initialize your cloud provider's SDK client using the provided config.
	//
	// Example:
	// return &Client{
	//     api: someprovider.NewClient(config.APIKey),
	// }, nil
	
	return &Client{}, nil
}

// GetResourceCost retrieves actual cost data for a specific resource.
func (c *Client) GetResourceCost(ctx context.Context, resourceID string, startTime, endTime int64) (float64, error) {
	// [TEMPLATE] Implementation Required: Actual Cost Retrieval
	// This method should call the appropriate billing/cost management API
	// to get the actual cost for the given resource and time range.
	//
	// Examples:
	// - AWS Cost Explorer API
	// - Azure Cost Management API
	// - GCP Cloud Billing API
	
	return 0.0, fmt.Errorf("not implemented: GetResourceCost for resource %s", resourceID)
}

// ValidateCredentials checks if the client credentials are valid.
func (c *Client) ValidateCredentials(ctx context.Context) error {
	// [TEMPLATE] Implementation Required: Credential Validation
	// Make a lightweight API call to verify that the credentials are valid
	// and have the necessary permissions.
	
	return fmt.Errorf("not implemented: ValidateCredentials")
}

// GetSupportedRegions returns the list of supported regions.
func (c *Client) GetSupportedRegions(ctx context.Context) ([]string, error) {
	// [TEMPLATE] Implementation Required: Region Discovery
	// Return the list of regions supported by your cloud provider.
	// This can be hardcoded or fetched dynamically.
	
	return []string{"us-east-1", "us-west-2"}, nil
}
`

	return g.writeFile("internal/client/client.go", content)
}

func (g *projectGenerator) generateMakefile() error {
	content := fmt.Sprintf(`# Makefile for %s plugin

.PHONY: build test clean lint help install

# Variables
PLUGIN_NAME = %s
BINARY_NAME = finfocus-plugin-$(PLUGIN_NAME)
BUILD_DIR = bin
CMD_DIR = cmd/plugin

# Default target
help:
	@echo "Available commands:"
	@echo "  build     - Build the plugin binary"
	@echo "  test      - Run tests"
	@echo "  clean     - Clean build artifacts"
	@echo "  lint      - Run linters"
	@echo "  install   - Install plugin to local registry"
	@echo "  help      - Show this help"

# Build the plugin binary
build:
	@echo "Building $(PLUGIN_NAME) plugin..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "✅ Plugin built: $(BUILD_DIR)/$(BINARY_NAME)"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

# Run linters
lint:
	@echo "Running linters..."
	@golangci-lint run --allow-parallel-runners
	@echo "✅ Linting complete"

# Install plugin to local registry
install: build
	@echo "Installing plugin to local registry..."
	@mkdir -p ~/.finfocus/plugins/$(PLUGIN_NAME)/1.0.0
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.finfocus/plugins/$(PLUGIN_NAME)/1.0.0/
	@cp manifest.yaml ~/.finfocus/plugins/$(PLUGIN_NAME)/1.0.0/plugin.manifest.json
	@echo "✅ Plugin installed to ~/.finfocus/plugins/$(PLUGIN_NAME)/1.0.0/"

# Development build with debug info
build-debug:
	@echo "Building $(PLUGIN_NAME) plugin with debug info..."
	@mkdir -p $(BUILD_DIR)
	@go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "✅ Debug build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatting complete"

# Update dependencies
deps:
	@echo "Updating dependencies..."
	@go mod tidy
	@go mod download
	@echo "✅ Dependencies updated"

# Check for security vulnerabilities
security:
	@echo "Checking for security vulnerabilities..."
	@govulncheck ./...
	@echo "✅ Security check complete"
`, g.name, g.name)

	return g.writeFile("Makefile", content)
}

func (g *projectGenerator) generateReadme() error {
	content := renderReadme(g.name, g.providers)
	return g.writeFile("README.md", content)
}

func (g *projectGenerator) generateTests() error {
	content := fmt.Sprintf(`package pricing

import (
	"testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

func TestCalculatorName(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)
	testPlugin.TestName("%s")
}

func TestProjectedCostSupported(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test supported resource
	resource := pluginsdk.CreateTestResource("aws", "aws:ec2:Instance", map[string]string{
		"instanceType": "t3.micro",
		"region":       "us-east-1",
	})

	resp := testPlugin.TestProjectedCost(resource, false)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Currency != "USD" {
		t.Errorf("Expected currency USD, got %%s", resp.Currency)
	}

	if resp.UnitPrice <= 0 {
		t.Errorf("Expected positive unit price, got %%f", resp.UnitPrice)
	}
}

func TestProjectedCostUnsupported(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test unsupported resource
	resource := pluginsdk.CreateTestResource("unsupported", "unsupported:resource:Type", nil)
	testPlugin.TestProjectedCost(resource, true) // Expect error
}

func TestActualCost(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test actual cost (should return error since not implemented)
	testPlugin.TestActualCost("resource-id-123", 1640995200, 1641081600, true) // Expect error
}

// Example of more specific test cases
func TestEC2InstancePricing(t *testing.T) {
	calculator := NewCalculator()

	testCases := []struct {
		name         string
		instanceType string
		expectedCost float64
	}{
		{"t3.micro", "t3.micro", 0.0104},
		{"t3.small", "t3.small", 0.0208},
		{"t3.medium", "t3.medium", 0.0416},
		{"unknown", "unknown-type", 0.0104}, // fallback
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &pbc.ResourceDescriptor{
				Provider:     "aws",
				ResourceType: "aws:ec2:Instance",
				Tags: map[string]string{
					"instanceType": tc.instanceType,
				},
			}

			cost := calculator.calculateEC2InstanceCost(resource)
			if cost != tc.expectedCost {
				t.Errorf("Expected cost %%f for %%s, got %%f", tc.expectedCost, tc.instanceType, cost)
			}
		})
	}
}
`, g.name)

	return g.writeFile("internal/pricing/calculator_test.go", content)
}

func (g *projectGenerator) generateIssues() error {
	content := renderIssues(g.providers)
	return g.writeFile("issues.md", content)
}

func (g *projectGenerator) writeFile(relativePath, content string) error {
	fullPath := filepath.Join(g.projectDir, relativePath)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, pluginDirPerm); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), pluginFilePerm); err != nil {
		return fmt.Errorf("writing file %s: %w", relativePath, err)
	}

	return nil
}

// createProjectDirectory ensures a directory exists at the given path, creating it and any
// necessary parent directories with 0o750 permissions.
// If the path already exists and force is false, it returns an error indicating the directory
// already exists. Returns an error on failure to create the directory, or nil on success.
func createProjectDirectory(path string, force bool) error {
	if _, err := os.Stat(path); err == nil {
		if !force {
			return fmt.Errorf("directory already exists: %s (use --force to overwrite)", path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking directory: %w", err)
	}

	return os.MkdirAll(path, pluginDirPerm)
}

// IsValidPluginName reports whether the provided name satisfies the plugin naming rules.
// The name must be between 2 and 50 characters, contain only lowercase letters (`a`–`z`),
// digits (`0`–`9`) or hyphens (`-`), and must not start or end with a hyphen.
// It returns true when all conditions are met, false otherwise.
func IsValidPluginName(name string) bool {
	if len(name) < 2 || len(name) > 50 {
		return false
	}

	for _, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit && r != '-' {
			return false
		}
	}

	// Cannot start or end with hyphen
	return name[0] != '-' && name[len(name)-1] != '-'
}
