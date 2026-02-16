#!/bin/bash
# ci-benchmark-filter.sh — Filter benchmark output for benchstat consumption
# Strips zerolog JSON, debug messages, and non-benchmark lines
#
# Input:  stdin (raw go test -bench output)
# Output: stdout (benchstat-compatible lines only)
# Exit:   Always 0 (filter must not fail the pipeline)

grep -E '^(goos:|goarch:|pkg:|cpu:|Benchmark|ok[[:space:]]|PASS)' || true
