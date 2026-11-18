#!/bin/bash
# Check that docs/.vale.ini lowers every synced ai-tells rule to suggestion.
#
# The ai-tells package ships each rule at error level, and Vale has no setting
# that lowers a whole style. Each rule is listed in docs/.vale.ini instead, so
# a package bump that adds a rule would otherwise fail docs pull requests.
# Run after `vale sync`; invoked by `make docs-vale` and the docs-validate
# workflow.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG="$PROJECT_ROOT/docs/.vale.ini"
STYLE_DIR="$PROJECT_ROOT/docs/.vale/styles/ai-tells"

if [ ! -d "$STYLE_DIR" ]; then
    echo "❌ $STYLE_DIR not found; run vale sync first"
    exit 1
fi

failures=0
rules=0
for rule_file in "$STYLE_DIR"/*.yml; do
    rule="$(basename "$rule_file" .yml)"
    rules=$((rules + 1))
    if ! grep -qx "ai-tells\.$rule = suggestion" "$CONFIG"; then
        echo "❌ ai-tells.$rule is not set to suggestion in docs/.vale.ini"
        failures=$((failures + 1))
    fi
done

while IFS= read -r rule; do
    if [ ! -f "$STYLE_DIR/$rule.yml" ]; then
        echo "❌ ai-tells.$rule is in docs/.vale.ini but not in the synced package"
        failures=$((failures + 1))
    fi
done < <(sed -n 's/^ai-tells\.\([A-Za-z0-9]*\) = .*/\1/p' "$CONFIG")

if [ "$failures" -gt 0 ]; then
    exit 1
fi
echo "✓ All $rules ai-tells rules are suggestions"
