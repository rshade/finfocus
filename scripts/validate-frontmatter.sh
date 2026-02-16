#!/bin/bash
# Validate YAML frontmatter in documentation files
# This script ensures all documentation files have proper frontmatter

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DOCS_DIR="$PROJECT_ROOT/docs"

ERRORS=0
WARNINGS=0

echo "Validating documentation frontmatter..."

# Find all markdown files in docs directory
find "$DOCS_DIR" -type f -name "*.md" ! -path "*/_site/*" ! -path "*/node_modules/*" | while read file; do
    filename=$(basename "$file")
    rel_path="${file#$DOCS_DIR/}"

    # Check if file starts with frontmatter (---)
    if head -1 "$file" | grep -q "^---"; then
        # File has frontmatter, check for required fields
        frontmatter=$(sed -n '1,/^---$/p' "$file" | head -n -1)

        # Check for title
        if ! echo "$frontmatter" | grep -q "^title:"; then
            echo "⚠ WARNING: $rel_path - Missing 'title:' field"
            ((WARNINGS++))
        fi

        # Check for layout
        if ! echo "$frontmatter" | grep -q "^layout:"; then
            echo "⚠ WARNING: $rel_path - Missing 'layout:' field"
            ((WARNINGS++))
        fi

        echo "✓ $rel_path - Valid frontmatter"
    else
        # File doesn't have frontmatter
        # This is OK for index files and coming-soon files, but warn
        if [[ "$filename" != "README.md" && "$filename" != "coming-soon.md" && "$filename" != "llms.txt" && "$filename" != "plan.md" ]]; then
            echo "⚠ WARNING: $rel_path - No frontmatter found"
            ((WARNINGS++))
        else
            echo "✓ $rel_path - OK (no frontmatter needed)"
        fi
    fi
done

echo ""
echo "---"
if [ $ERRORS -gt 0 ]; then
    echo "❌ Validation failed: $ERRORS errors"
    exit 1
elif [ $WARNINGS -gt 0 ]; then
    echo "⚠ Validation passed with $WARNINGS warnings"
    echo "Note: Consider adding frontmatter to all documentation files"
    exit 0
else
    echo "✓ All documentation files validated successfully"
    exit 0
fi
