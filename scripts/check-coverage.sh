#!/usr/bin/env bash

# Coverage threshold validation script
# Usage: check-coverage.sh <coverage-file> <threshold>
#
# Validates that overall test coverage meets minimum threshold
# Returns 0 if coverage >= threshold, 1 otherwise

set -eo pipefail

COVERAGE_FILE="${1:-coverage.out}"
THRESHOLD="${2:-61}"

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

# Calculate coverage using go tool cover
COVERAGE_OUTPUT=$(go tool cover -func="$COVERAGE_FILE" 2>&1)

if [ $? -ne 0 ]; then
    echo "Error: Failed to process coverage file" >&2
    echo "$COVERAGE_OUTPUT" >&2
    exit 1
fi

# Extract total coverage percentage
TOTAL_COVERAGE=$(echo "$COVERAGE_OUTPUT" | grep 'total:' | awk '{print $NF}' | sed 's/%//')

if [ -z "$TOTAL_COVERAGE" ]; then
    echo "Error: Could not extract coverage percentage from coverage file" >&2
    exit 1
fi

# Convert to integer for comparison (bash doesn't do floating point)
TOTAL_COVERAGE_INT=$(echo "$TOTAL_COVERAGE" | awk '{print int($1)}')

echo "==============================================="
echo "Coverage Validation"
echo "==============================================="
echo "Coverage file: $COVERAGE_FILE"
echo "Coverage: ${TOTAL_COVERAGE}%"
echo "Threshold: ${THRESHOLD}%"
echo "==============================================="

# Compare coverage against threshold
if [ "$TOTAL_COVERAGE_INT" -ge "$THRESHOLD" ]; then
    echo "✓ PASS: Coverage ($TOTAL_COVERAGE%) meets minimum threshold ($THRESHOLD%)"
    exit 0
else
    echo "✗ FAIL: Coverage ($TOTAL_COVERAGE%) below minimum threshold ($THRESHOLD%)"
    echo ""
    echo "To improve coverage:"
    echo "  1. Add more unit tests for uncovered code paths"
    echo "  2. Review test/COVERAGE.md for detailed coverage analysis"
    echo "  3. Focus on critical path packages (CLI, engine, pluginhost)"
    exit 1
fi
