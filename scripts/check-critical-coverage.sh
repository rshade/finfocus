#!/usr/bin/env bash

# Critical path coverage validation script
# Usage: check-critical-coverage.sh <coverage-file> <threshold>
#
# Validates that critical packages meet minimum coverage threshold
# Critical packages: internal/cli, internal/engine, internal/pluginhost
# Returns 0 if all critical packages >= threshold, 1 otherwise

set -eo pipefail

COVERAGE_FILE="${1:-coverage.out}"
THRESHOLD="${2:-60}"

# Define critical packages
CRITICAL_PACKAGES=(
    "github.com/rshade/finfocus/internal/cli"
    "github.com/rshade/finfocus/internal/engine"
    "github.com/rshade/finfocus/internal/pluginhost"
)

# Validate inputs
if [ ! -f "$COVERAGE_FILE" ]; then
    echo "Error: Coverage file not found: $COVERAGE_FILE" >&2
    exit 1
fi

if ! [[ "$THRESHOLD" =~ ^[0-9]+$ ]]; then
    echo "Error: Threshold must be a number, got: $THRESHOLD" >&2
    exit 1
fi

if [ "$THRESHOLD" -lt 0 ] || [ "$THRESHOLD" -gt 100 ]; then
    echo "Error: Threshold must be between 0 and 100, got: $THRESHOLD" >&2
    exit 1
fi

# Check if file is empty
if [ ! -s "$COVERAGE_FILE" ]; then
    echo "Error: Coverage file is empty: $COVERAGE_FILE" >&2
    exit 1
fi

# Get coverage for each package using go tool cover
COVERAGE_OUTPUT=$(go tool cover -func="$COVERAGE_FILE" 2>&1)

if [ $? -ne 0 ]; then
    echo "Error: Failed to process coverage file" >&2
    echo "$COVERAGE_OUTPUT" >&2
    exit 1
fi

echo "==============================================="
echo "Critical Path Coverage Validation"
echo "==============================================="
echo "Coverage file: $COVERAGE_FILE"
echo "Threshold: ${THRESHOLD}%"
echo "==============================================="

FAILURES=0
MISSING_PACKAGES=()

# Check coverage for each critical package
for pkg in "${CRITICAL_PACKAGES[@]}"; do
    # Extract coverage for this package by averaging function coverage percentages
    PKG_COVERAGE=$(echo "$COVERAGE_OUTPUT" | grep "^${pkg}/" | awk '{print $NF}' | sed 's/%//' | awk '{
        sum += $1
        count++
    } END {
        if (count > 0) {
            printf "%.1f", sum / count
        } else {
            print "0.0"
        }
    }')

    # Check if package was found in coverage
    if [ -z "$PKG_COVERAGE" ] || [ "$PKG_COVERAGE" == "0.0" ]; then
        # Double-check by looking for any coverage lines for this package
        if ! echo "$COVERAGE_OUTPUT" | grep -q "^${pkg}/"; then
            echo "⚠ WARNING: Package not found in coverage: $(basename $pkg)"
            MISSING_PACKAGES+=("$pkg")
            continue
        fi
    fi

    # Convert to integer for comparison
    PKG_COVERAGE_INT=$(echo "$PKG_COVERAGE" | awk '{print int($1)}')

    PKG_NAME=$(basename "$pkg")

    if [ "$PKG_COVERAGE_INT" -ge "$THRESHOLD" ]; then
        echo "✓ PASS: $PKG_NAME: ${PKG_COVERAGE}%"
    else
        echo "✗ FAIL: $PKG_NAME: ${PKG_COVERAGE}% (below ${THRESHOLD}%)"
        FAILURES=$((FAILURES + 1))
    fi
done

echo "==============================================="

# Report results
if [ ${#MISSING_PACKAGES[@]} -gt 0 ]; then
    echo ""
    echo "WARNING: ${#MISSING_PACKAGES[@]} critical package(s) not found in coverage:"
    for pkg in "${MISSING_PACKAGES[@]}"; do
        echo "  - $(basename $pkg)"
    done
fi

if [ $FAILURES -eq 0 ] && [ ${#MISSING_PACKAGES[@]} -eq 0 ]; then
    echo "✓ All critical paths meet minimum coverage threshold"
    exit 0
else
    echo "✗ ${FAILURES} critical path(s) below threshold"
    echo ""
    echo "To improve critical path coverage:"
    echo "  1. Add more unit tests for CLI, engine, and pluginhost packages"
    echo "  2. Review test/COVERAGE.md for detailed package-level analysis"
    echo "  3. Focus on error handling and edge cases"
    echo "  4. Ensure all exported functions have tests"
    exit 1
fi
