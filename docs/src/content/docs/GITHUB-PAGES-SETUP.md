---
title: GitHub Pages Setup Guide
---
This document explains how to set up GitHub Pages for FinFocus documentation.

## Overview

FinFocus documentation is deployed to GitHub Pages from the `docs/` directory using Jekyll.
This guide shows how to configure the repository for automatic deployment.

## Prerequisites

- GitHub repository admin access
- Main branch with `docs/` directory
- `.github/workflows/docs-build-deploy.yml` workflow file (included)

## Setup Steps

### 1. Enable GitHub Pages

Go to your repository settings:

1. Navigate to **Settings** → **Pages**
2. Under "Build and deployment":
   - **Source**: Select "GitHub Actions"
   - This enables GitHub Actions to deploy to Pages

### 2. Verify Workflow Permissions

Ensure the workflow has permission to deploy:

1. Go to **Settings** → **Actions** → **General**
2. Under "Workflow permissions":
   - Select "Read and write permissions"
   - Enable "Allow GitHub Actions to create and approve pull requests"
3. Click **Save**

### 3. Configure Custom Domain (Optional)

If using a custom domain:

1. Create a `docs/CNAME` file with your domain:

   ```text
   docs.finfocus.com
   ```

2. Configure domain in **Settings** → **Pages**:
   - Enter your custom domain
   - GitHub will add DNS records instructions

3. Update DNS records at your domain provider

### 4. Verify Deployment

After push to main:

1. Go to **Actions** tab
2. Look for "Build & Deploy Documentation" workflow
3. Verify workflow completes successfully
4. Visit GitHub Pages URL:
   - Default: `https://rshade.github.io/finfocus/`
   - Custom domain: `https://docs.finfocus.com/`

---

## Troubleshooting

### Workflow Fails to Deploy

**Check workflow permissions:**

```bash
gh api repos/rshade/finfocus/actions/permissions
```

### Build Fails

Check the workflow logs:

1. Go to **Actions** tab
2. Click on failed workflow
3. Expand "Build Documentation" job
4. Look for error messages

Common issues:

- Missing Ruby dependencies: Run `bundle install`
- Jekyll build errors: Check `_config.yml` syntax
- Link errors: Run `make docs-validate` locally

### Pages Not Updating

1. Verify workflow is enabled (check `.github/workflows/docs-build-deploy.yml` exists)
2. Check that changes are pushed to `main` branch
3. Verify `docs/` directory changes triggered the workflow
4. Check workflow logs for failures

---

## Manual Deployment (if needed)

If automatic deployment fails, you can deploy manually:

```bash
# Build locally
cd docs
bundle install
bundle exec jekyll build

# Upload to GitHub Pages (requires authentication)
# Note: This is rarely needed - use workflows instead
```

---

## Maintenance

### Monitor Deployments

View all deployments:

```bash
gh deployment list --repo rshade/finfocus
```

### View Site Analytics

1. Go to **Settings** → **Pages**
2. View deployment history and status

### Update Workflow

If you modify the workflow:

```bash
# Validate workflow syntax
gh workflow validate .github/workflows/docs-build-deploy.yml

# Push changes
git add .github/workflows/docs-build-deploy.yml
git commit -m "docs: Update GitHub Pages workflow"
git push origin main
```

---

## Configuration Files

### Key Files

- **docs/\_config.yml** - Jekyll configuration
- **docs/Gemfile** - Ruby dependencies
- **docs/.markdownlintrc.json** - Markdown linting
- **.github/workflows/docs-build-deploy.yml** - Deployment workflow

### Modify Configuration

To change Jekyll configuration:

1. Edit `docs/_config.yml`
2. Test locally: `make docs-serve`
3. Push to main (triggers rebuild)

---

## Reference

- [GitHub Pages Documentation](https://docs.github.com/en/pages)
- [Jekyll Documentation](https://jekyllrb.com/docs/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)

---

**Last Updated:** 2025-10-29
