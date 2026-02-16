#!/bin/bash
#
# compare-benchmarks.sh - Compare benchmark results against baseline
#
# Usage:
#   ./scripts/compare-benchmarks.sh [options]
#
# Options:
#   --reset    Force regeneration of baseline benchmarks
#   --help     Show this help message
#
# Description:
#   This script runs the benchmark suite and compares results against a baseline.
#   If no baseline exists, one is created. Use benchstat for detailed statistical
#   comparison, or falls back to diff if benchstat is not installed.
#
# Environment:
#   GO111MODULE=on is assumed for go test invocation.
#
# Output files:
#   test/benchmarks/baseline.txt - Baseline benchmark results
#   test/benchmarks/current.txt  - Current benchmark results
#
# Exit codes:
#   0 - Success
#   1 - Benchmark execution failed
#

set -e

BASELINE_FILE="test/benchmarks/baseline.txt"
CURRENT_FILE="test/benchmarks/current.txt"

# Parse command line arguments
if [[ "$1" == "--help" ]]; then
    head -n 25 "$0" | tail -n +2 | sed 's/^# //' | sed 's/^#//'
    exit 0
fi

if [[ "$1" == "--reset" ]]; then
    rm -f "$BASELINE_FILE"
fi

if [ ! -f "$BASELINE_FILE" ]; then
    echo "No baseline found. Creating..."
    if ! go test -bench=. -benchmem ./test/benchmarks/... > "$BASELINE_FILE" 2>&1; then
        echo "Error: Failed to generate baseline benchmarks"
        rm -f "$BASELINE_FILE"
        exit 1
    fi
    echo "Baseline created."
    exit 0
fi

echo "Running benchmarks..."
if ! go test -bench=. -benchmem ./test/benchmarks/... > "$CURRENT_FILE" 2>&1; then
    echo "Error: Failed to run current benchmarks"
    exit 1
fi

echo "Comparing results..."
# Simple comparison using benchstat if available, or diff
if command -v benchstat >/dev/null; then
    benchstat "$BASELINE_FILE" "$CURRENT_FILE"
else
    echo "benchstat not found. Showing raw diff:"
    diff "$BASELINE_FILE" "$CURRENT_FILE" || true
fi
