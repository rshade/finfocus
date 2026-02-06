#!/bin/bash
# E2E Test Runner Script
# This script sets up the environment and runs E2E tests against real AWS infrastructure

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}FinFocus E2E Test Runner${NC}"
echo "=============================="
echo ""

# Check for required tools
echo -e "${YELLOW}Checking prerequisites...${NC}"

# Check for AWS CLI
if ! command -v aws &> /dev/null; then
    echo -e "${RED}ERROR: AWS CLI not found. Please install it first.${NC}"
    exit 1
fi

# Check for Pulumi CLI
PULUMI_PATH="${HOME}/.pulumi/bin"
if [ -f "${PULUMI_PATH}/pulumi" ]; then
    export PATH="${PULUMI_PATH}:${PATH}"
    echo "  Pulumi CLI: Found at ${PULUMI_PATH}"
elif command -v pulumi &> /dev/null; then
    echo "  Pulumi CLI: Found in PATH"
else
    echo -e "${RED}ERROR: Pulumi CLI not found. Install from https://www.pulumi.com/docs/get-started/install/${NC}"
    exit 1
fi

# Check for Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}ERROR: Go not found. Please install Go 1.25.7+${NC}"
    exit 1
fi

# Validate Go version >= 1.25.7
GO_VERSION=$(go version | grep -oP 'go(\d+\.\d+(\.\d+)?)' | sed 's/go//')
REQUIRED_VERSION="1.25.7"

version_ge() {
    printf '%s\n%s\n' "$2" "$1" | sort -V -C
}

if ! version_ge "${GO_VERSION}" "${REQUIRED_VERSION}"; then
    echo -e "${RED}ERROR: Go ${GO_VERSION} is too old. Minimum required: ${REQUIRED_VERSION}${NC}"
    exit 1
fi
echo "  Go version: ${GO_VERSION} (>= ${REQUIRED_VERSION})"

# Check for AWS credentials
echo ""
echo -e "${YELLOW}Checking AWS credentials...${NC}"
if aws sts get-caller-identity &> /dev/null; then
    echo "  AWS credentials: Found and valid"
else
    echo -e "${RED}ERROR: AWS credentials not configured. Run 'aws configure' or set environment variables.${NC}"
    exit 1
fi

# Set required environment variables
export AWS_REGION="${AWS_REGION:-us-east-1}"
export E2E_REGION="${E2E_REGION:-${AWS_REGION}}"
export PULUMI_CONFIG_PASSPHRASE="${PULUMI_CONFIG_PASSPHRASE:-e2e-test-passphrase}"
export E2E_TIMEOUT_MINS="${E2E_TIMEOUT_MINS:-60}"

# Enable debug logging for troubleshooting
export FINFOCUS_LOG_LEVEL="${FINFOCUS_LOG_LEVEL:-debug}"
export FINFOCUS_LOG_FORMAT="${FINFOCUS_LOG_FORMAT:-console}"

echo "  AWS_REGION: ${AWS_REGION}"
echo "  E2E_REGION: ${E2E_REGION}"
echo "  FINFOCUS_LOG_LEVEL: ${FINFOCUS_LOG_LEVEL}"
echo "  FINFOCUS_LOG_FORMAT: ${FINFOCUS_LOG_FORMAT}"

# Build finfocus binary if needed
echo ""
echo -e "${YELLOW}Building finfocus binary...${NC}"
cd "${PROJECT_ROOT}"
make build
export FINFOCUS_BINARY="${PROJECT_ROOT}/bin/finfocus"
echo "  Binary: ${FINFOCUS_BINARY}"

# Run the tests
echo ""
echo -e "${YELLOW}Running E2E tests...${NC}"
echo ""

cd "${SCRIPT_DIR}"

# Parse command line arguments
TEST_FILTER=""
VERBOSE="-v"
SHORT_FLAG=""
DEBUG_MODE=""
INSTALL_PLUGIN=""
CLEANUP_PLUGINS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -run)
            TEST_FILTER="-run $2"
            shift 2
            ;;
        -short)
            SHORT_FLAG="-short"
            shift
            ;;
        -timeout)
            E2E_TIMEOUT_MINS="$2"
            shift 2
            ;;
        -debug)
            DEBUG_MODE="true"
            export FINFOCUS_LOG_LEVEL="trace"
            echo -e "${YELLOW}Debug mode enabled - FINFOCUS_LOG_LEVEL=trace${NC}"
            shift
            ;;
        -install-plugin)
            INSTALL_PLUGIN="$2"
            shift 2
            ;;
        -cleanup-plugins)
            CLEANUP_PLUGINS="true"
            export E2E_CLEANUP_PLUGINS="true"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -run TestName        Run only tests matching TestName"
            echo "  -short               Disable verbose output"
            echo "  -timeout minutes     Set test timeout (default: 60)"
            echo "  -debug               Enable trace-level logging"
            echo "  -install-plugin name Install a plugin before running tests"
            echo "  -cleanup-plugins     Remove plugins after tests (E2E_CLEANUP_PLUGINS=true)"
            echo "  -h, --help           Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Usage: $0 [-run TestName] [-short] [-timeout minutes] [-debug] [-install-plugin name] [-cleanup-plugins]"
            exit 1
            ;;
    esac
done

# Print environment summary
echo ""
echo -e "${YELLOW}Environment Summary:${NC}"
echo "  PROJECT_ROOT: ${PROJECT_ROOT}"
echo "  SCRIPT_DIR: ${SCRIPT_DIR}"
echo "  FINFOCUS_BINARY: ${FINFOCUS_BINARY:-will be set after build}"
echo "  TEST_FILTER: ${TEST_FILTER:-<all tests>}"
echo "  TIMEOUT: ${E2E_TIMEOUT_MINS}m"
if [ -n "${DEBUG_MODE}" ]; then
    echo "  DEBUG_MODE: enabled"
fi
if [ -n "${INSTALL_PLUGIN}" ]; then
    echo "  INSTALL_PLUGIN: ${INSTALL_PLUGIN}"
fi
if [ -n "${CLEANUP_PLUGINS}" ]; then
    echo "  E2E_CLEANUP_PLUGINS: enabled"
fi

# Install plugin if requested
if [ -n "${INSTALL_PLUGIN}" ]; then
    echo ""
    echo -e "${YELLOW}Installing plugin: ${INSTALL_PLUGIN}...${NC}"
    if "${FINFOCUS_BINARY}" plugin install "${INSTALL_PLUGIN}" --force; then
        echo -e "${GREEN}Plugin ${INSTALL_PLUGIN} installed successfully${NC}"
    else
        echo -e "${RED}WARNING: Failed to install plugin ${INSTALL_PLUGIN}${NC}"
        echo "Tests will continue but may fail if they require this plugin"
    fi

    # Show installed plugins
    echo ""
    echo -e "${YELLOW}Installed plugins:${NC}"
    "${FINFOCUS_BINARY}" plugin list || true
fi

# Run the tests
set +e  # Don't exit on test failure
# shellcheck disable=SC2086 # Intentional word splitting for VERBOSE, SHORT_FLAG and TEST_FILTER
go test ${VERBOSE} -tags e2e -timeout "${E2E_TIMEOUT_MINS}m" ${SHORT_FLAG} ${TEST_FILTER} ./...
TEST_EXIT_CODE=$?
set -e

echo ""
if [ ${TEST_EXIT_CODE} -eq 0 ]; then
    echo -e "${GREEN}E2E tests passed!${NC}"
else
    echo -e "${RED}E2E tests failed with exit code ${TEST_EXIT_CODE}${NC}"
fi

exit ${TEST_EXIT_CODE}
