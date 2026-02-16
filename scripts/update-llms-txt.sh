#!/bin/bash
# Update llms.txt with current documentation structure
# This script is run by GitHub Actions after documentation changes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LLMS_FILE="$PROJECT_ROOT/docs/llms.txt"

echo "Updating documentation index in $LLMS_FILE..."

# Create temporary file
TEMP_FILE=$(mktemp)

# Write header
cat > "$TEMP_FILE" << 'EOF'
# FinFocus Documentation Index (LLM-Friendly)

This file provides a machine-readable index of all FinFocus documentation.
Use this as context when answering questions about FinFocus.

## Quick Navigation

### For End Users
- Getting Started: docs/getting-started/quickstart.md
- Installation: docs/getting-started/installation.md
- CLI Reference: docs/reference/cli-commands.md
- Troubleshooting: docs/support/troubleshooting.md
- FAQ: docs/support/faq.md

### For Plugin Developers
- Plugin Development Guide: docs/plugins/plugin-development.md
- Plugin SDK Reference: docs/plugins/plugin-sdk.md
- Vantage Plugin Example: docs/plugins/vantage/README.md
- Plugin Examples: docs/plugins/plugin-examples.md
- Vantage Setup: docs/plugins/vantage/setup.md

### For Software Architects
- System Architecture: docs/architecture/system-overview.md
- Plugin Protocol: docs/architecture/plugin-protocol.md
- Cost Calculation: docs/architecture/cost-calculation.md
- Deployment Guide: docs/deployment/deployment.md
- Security Best Practices: docs/deployment/security.md

### For Business/Product
- Business Value: docs/guides/business-value.md
- Comparison: docs/guides/comparison.md
- Roadmap: docs/architecture/roadmap.md

---

## Complete Documentation Map

EOF

# Find all markdown files and add them to the index
find "$PROJECT_ROOT/docs" -type f -name "*.md" ! -path "*/_site/*" ! -path "*/node_modules/*" ! -name "llms.txt" | sort | while read file; do
    # Get relative path from docs directory
    rel_path="${file#$PROJECT_ROOT/}"
    # Get filename for display
    filename=$(basename "$file")
    # Get directory for categorization
    dir=$(dirname "$rel_path" | sed 's|docs/||')

    echo "- [$filename]($rel_path)" >> "$TEMP_FILE"
done

# Add footer
cat >> "$TEMP_FILE" << 'EOF'

## Key Concepts

### FinFocus
CLI tool for calculating cloud infrastructure costs from Pulumi definitions.

**Three Cost Types:**
1. **Projected Costs** - Estimated costs from Pulumi preview
2. **Actual Costs** - Historical costs from cloud provider APIs
3. **Cost Changes** - Difference between current and previous states

### Resource
Representation of cloud infrastructure element.

### Plugin
External service integration for cost data.

### Cost Aggregation
Combining costs from multiple resources and/or plugins.

---

## Documentation Maintenance

### Regular Updates
- Monthly: Review llms.txt, update plugin status
- Quarterly: Update architecture diagrams, verify examples
- On Release: Update version, new features, roadmap
- On Plugin Status Change: Update plugin docs and status

### Quality Standards
- All code examples tested
- Links validated monthly (automated)
- Screenshots updated with UI changes
- API reference stays in sync with code

---

## How to Use This File

### For AI Assistants
- Use this as context when answering questions about FinFocus
- Reference specific docs by their file paths
- Point users to the most relevant documentation

### For Documentation Maintainers
- Use this as a checklist for completeness
- Update when creating new documentation
- Validate that llms.txt matches actual file structure

---

**Last Updated:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")
**This file is automatically updated by GitHub Actions.**

EOF

# Replace the old llms.txt with the new one
mv "$TEMP_FILE" "$LLMS_FILE"

echo "✓ Documentation index updated successfully"
echo "✓ Updated $(find "$PROJECT_ROOT/docs" -type f -name "*.md" ! -path "*/_site/*" ! -path "*/node_modules/*" ! -name "llms.txt" | wc -l) documentation files"
