#!/bin/bash
# Validate the public llms.txt content contract.
#
# Single source of truth invoked by both `make docs-validate` and the
# docs-validate workflow, so local and CI enforce identical rules.
#
# Enforced: file exists, update timestamp present, all required sections
# present, and every link is a deployed URL rather than a repository path.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LLMS_FILE="${1:-$PROJECT_ROOT/docs/public/llms.txt}"

failures=0

fail() {
    echo "❌ $1"
    failures=$((failures + 1))
}

if [ ! -f "$LLMS_FILE" ]; then
    echo "❌ $LLMS_FILE not found"
    exit 1
fi

if grep -q "Last Updated:" "$LLMS_FILE"; then
    echo "✓ llms.txt has update timestamp"
else
    fail "llms.txt missing update timestamp"
fi

required_sections=(
    "Quick Navigation"
    "Complete Documentation Map"
    "Key Concepts"
)

for section in "${required_sections[@]}"; do
    if grep -q "$section" "$LLMS_FILE"; then
        echo "✓ Found section: $section"
    else
        fail "Missing section: $section"
    fi
done

# llms.txt is served over HTTP, so repository-relative paths never resolve
# for the clients that consume it.
if stale=$(grep -nE '\(docs/|: docs/|\.md\)' "$LLMS_FILE"); then
    fail "llms.txt contains repository-relative paths; regenerate with scripts/update-llms-txt.sh"
    printf '%s\n' "$stale" | head -10
else
    echo "✓ All llms.txt links are deployed URLs"
fi

if [ "$failures" -gt 0 ]; then
    echo "❌ llms.txt validation failed with $failures problem(s)"
    exit 1
fi

echo "✓ llms.txt validation passed"
