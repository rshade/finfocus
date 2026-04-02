---
title: Developer Setup Guide
---
This guide helps developers set up their local environment to work on FinFocus documentation.

## Prerequisites

- Git
- Ruby 3.2+ (for Jekyll)
- Node.js 18+ (for npm tools)
- Make

## Quick Setup

### 1. Clone Repository

```bash
git clone https://github.com/rshade/finfocus
cd finfocus
```

### 2. Install Documentation Tools

#### Ruby Dependencies (for Jekyll)

```bash
cd docs
bundle install
cd ..
```

#### Node Dependencies (for npm tools)

```bash
npm install
```

### 3. Verify Setup

```bash
# Check Ruby/Jekyll
ruby --version
bundle --version
cd docs && bundle exec jekyll --version && cd ..

# Check Node/npm
node --version
npm --version

# Test build
make docs-build
make docs-serve
```

## Working with Documentation

### Local Preview

Serve documentation locally on http://localhost:4000/finfocus/:

```bash
make docs-serve
```

Or directly with Jekyll:

```bash
cd docs
bundle exec jekyll serve --host 0.0.0.0
cd ..
```

### Lint Documentation

Check markdown formatting and links:

```bash
make docs-lint
```

Or use npm:

```bash
npm run docs:lint
```

### Format Documentation

Auto-format all markdown files:

```bash
npm run docs:format
```

### Validate Structure

Ensure documentation is complete:

```bash
make docs-validate
```

## Development Workflow

### 1. Create Feature Branch

```bash
git checkout -b docs/my-feature
```

### 2. Make Changes

```bash
# Edit documentation files
nano docs/guides/my-guide.md

# Preview locally
make docs-serve

# Lint changes
make docs-lint
```

### 3. Test Build

```bash
# Build static site
make docs-build

# Run validation
make docs-validate
```

### 4. Commit Changes

```bash
git add docs/
git commit -m "docs: Add my new guide"
```

### 5. Push and Create PR

```bash
git push origin docs/my-feature

# Create PR on GitHub (via web interface)
```

## Common Tasks

### Add a New Guide

1. Create file in appropriate directory:

   ```bash
   touch docs/guides/my-guide.md
   ```

2. Add frontmatter:

   ```yaml
   ---
   layout: default
   title: My Guide Title
   description: Brief description for search
   ---
   ```

3. Write content using [Google style guide](https://developers.google.com/style)

4. Test locally:

   ```bash
   make docs-serve
   ```

5. Lint and validate:

   ```bash
   make docs-lint
   make docs-validate
   ```

### Add Documentation to New Directory

1. Create directory:

   ```bash
   mkdir -p docs/my-section/
   ```

2. Create README.md:

   ```bash
   cat > docs/my-section/README.md << 'EOF'
   # My Section

   Overview of this section.

   ---

   **Status:** 🔴 Not Started
   EOF
   ```

3. Update `docs/plan.md` to reference new section

4. Update `docs/llms.txt`:

   ```bash
   ./scripts/update-llms-txt.sh
   ```

### Fix Linting Issues

Fix formatting automatically:

```bash
npm run docs:format
```

Or manually:

```bash
# Check what prettier wants to fix
npm run docs:check-format

# Fix all issues
npm run docs:format
```

## File Structure

```text
docs/
├── README.md                    # Documentation home
├── _config.yml                 # Jekyll configuration
├── _includes/                  # Reusable components
├── _layouts/                   # Page layouts
├── guides/                      # Audience guides
├── getting-started/            # Quick start guides
├── architecture/               # Architecture docs
├── plugins/                    # Plugin docs
├── reference/                  # API reference
├── deployment/                 # Operations
└── support/                    # Help & community

scripts/
├── update-llms-txt.sh          # Update documentation index
└── validate-frontmatter.sh     # Validate YAML frontmatter
```

## Troubleshooting

### Bundle Install Fails

**Issue:** Ruby version mismatch or gem conflicts

**Solution:**

```bash
# Update gems
bundle update

# Or reinstall
rm Gemfile.lock
bundle install
```

### Jekyll Serve Not Working

**Issue:** Port already in use or Jekyll won't start

**Solution:**

```bash
# Kill existing process
pkill -f jekyll

# Try different port
cd docs && bundle exec jekyll serve --port 5000 --host 0.0.0.0
```

### npm Install Fails

**Issue:** Node version too old or permission issues

**Solution:**

```bash
# Update Node
nvm install 18
nvm use 18

# Clear npm cache
npm cache clean --force
npm install
```

### Linting Errors

**Issue:** Markdownlint or prettier finding errors

**Solution:**

```bash
# Show what needs fixing
npm run docs:check-format

# Fix automatically
npm run docs:format

# Or fix manually based on linter output
```

## Documentation Guidelines

### Style

- Follow [Google Developer Style Guide](https://developers.google.com/style)
- Use clear, concise language
- Use active voice
- Provide examples for complex topics

### Formatting

- Use proper markdown headings (# not bold)
- Code blocks with language: ` ```bash `
- Links relative to docs directory
- Line length: 120 characters (soft limit)

### Testing

- Test all code examples
- Verify all links work
- Preview on http://localhost:4000/finfocus/

### Frontmatter

All content pages should have:

```yaml
---
layout: default
title: Page Title
description: Short description for search results
---
```

## Useful Commands

```bash
# Documentation
make docs-lint              # Lint docs
make docs-build             # Build static site
make docs-serve             # Serve locally
make docs-validate          # Validate structure

# NPM tasks
npm run docs:lint           # Lint with markdownlint
npm run docs:format         # Format with prettier
npm run docs:check-format   # Check formatting
npm run lint                # Run all linting

# Git
git status                  # Check changes
git diff                    # View changes
git add docs/               # Stage docs
git commit -m "msg"         # Commit
```

## Getting Help

- **Markdown issues**: Check [CommonMark spec](https://spec.commonmark.org/)
- **Jekyll issues**: See [Jekyll docs](https://jekyllrb.com/docs/)
- **Google Style**: Check [Google Developer Style Guide](https://developers.google.com/style)
- **Questions**: Open [GitHub Discussion](https://github.com/rshade/finfocus/discussions)

---

**Last Updated:** 2025-10-29
