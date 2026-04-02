---
title: Developer Setup Guide
---
This guide helps developers set up their local environment to work on FinFocus documentation.

## Prerequisites

- Git
- Node.js 20+ (for Astro)
- Make

## Quick Setup

### 1. Clone Repository

```bash
git clone https://github.com/rshade/finfocus
cd finfocus
```

### 2. Install Documentation Tools

#### Node Dependencies (for Astro)

```bash
cd docs
npm install
cd ..
```

### 3. Verify Setup

```bash
# Check Node/npm
node --version
npm --version

# Check Astro
cd docs && npx astro --version && cd ..

# Test build
make docs-build
make docs-serve
```

## Working with Documentation

### Local Preview

Serve documentation locally on http://localhost:4321/finfocus/:

```bash
make docs-serve
```

Or directly with npm:

```bash
cd docs
npm run dev
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
├── package.json                 # Node.js dependencies
├── astro.config.mjs            # Astro/Starlight configuration
├── tsconfig.json               # TypeScript configuration
├── public/                     # Static assets (images, etc.)
│   └── screenshots/            # UI screenshots
├── src/
│   └── content/
│       └── docs/               # Documentation content
│           ├── getting-started/ # Quick start guides
│           ├── guides/         # Audience guides
│           ├── architecture/   # Architecture docs
│           ├── plugins/        # Plugin docs
│           ├── reference/      # API reference
│           ├── deployment/     # Operations
│           └── support/        # Help & community
└── .markdownlint-cli2.jsonc    # Markdown linting config
```

## Troubleshooting

### npm Install Fails

**Issue:** Node version too old or dependency conflicts

**Solution:**

```bash
# Update Node
nvm install 20
nvm use 20

# Clear cache and reinstall
cd docs
rm -rf node_modules package-lock.json
npm install
cd ..
```

### Astro Dev Server Not Working

**Issue:** Port already in use or build errors

**Solution:**

```bash
# Try different port
cd docs && npx astro dev --port 5000

# Clear Astro cache
rm -rf docs/.astro docs/dist
cd docs && npm run dev
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
- Preview on http://localhost:4321/finfocus/

### Frontmatter

All content pages should have:

```yaml
---
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
- **Astro issues**: See [Astro docs](https://docs.astro.build/) and [Starlight docs](https://starlight.astro.build/)
- **Google Style**: Check [Google Developer Style Guide](https://developers.google.com/style)
- **Questions**: Open [GitHub Discussion](https://github.com/rshade/finfocus/discussions)

---

**Last Updated:** 2025-10-29
