# Screenshot and Visual Example Checklist

**Purpose**: Requirements and guidelines for capturing screenshots and creating visual examples for FinFocus documentation.

**Usage**: Follow this checklist when creating visual examples to ensure consistency, quality, and accessibility.

---

## Quick Reference

| Requirement | Specification |
|-------------|---------------|
| **Minimum Resolution** | 1600x900 pixels (for clarity on retina displays) |
| **Format** | PNG (static screenshots), GIF (animations <5MB) |
| **Color Depth** | 24-bit RGB or 8-bit indexed (after compression) |
| **File Size** | PNG <500KB, GIF <1MB (optimize before commit) |
| **Naming Convention** | `feature-component-purpose.{png,gif}` (kebab-case) |
| **Alt Text** | Required, <125 characters, descriptive (not "screenshot of") |
| **Context** | Show command prompt + output (full context) |

---

## Screenshot Capture Requirements

### 1. Terminal Configuration

**Before capturing screenshots:**

- [ ] Use clean terminal session (no extra output or errors)
- [ ] Set terminal size to 100+ columns x 30+ rows (wider is better)
- [ ] Use readable font size (14pt minimum for 1920x1080 resolution)
- [ ] Clear terminal history (`clear` command) before capturing
- [ ] Ensure no personal information (API keys, paths, usernames) visible

**Recommended Terminal Emulators:**

| Platform | Terminal | Notes |
|----------|----------|-------|
| macOS | iTerm2 | Best color rendering, customizable |
| Linux | GNOME Terminal | Good default colors |
| Windows | Windows Terminal | Supports Unicode box drawing |

**Terminal Settings:**

- **Font**: Monospace font with good Unicode support (Fira Code, JetBrains Mono, Cascadia Code)
- **Color Scheme**: Use terminal defaults (light or dark) - capture both if demonstrating adaptive colors
- **Scrollback**: Disable for clean screenshots (or scroll to top of output)

---

### 2. Resolution and Dimensions

**Minimum Resolution**: 1600x900 pixels

**Rationale**: Ensures text remains readable on retina displays (rendered at 800x450 CSS pixels).

**Capture Guidelines**:

- [ ] Set terminal to full screen or maximize window
- [ ] Capture at 2x resolution for retina displays (3200x1800 actual, displayed as 1600x900)
- [ ] Crop to relevant content (remove excessive whitespace)
- [ ] Maintain aspect ratio (16:9 or 4:3 typical)

**Recommended Dimensions by Content Type:**

| Content Type | Ideal Dimensions | Notes |
|--------------|------------------|-------|
| Command + Output | 1600x900 | Full terminal window |
| Table Display | 1400x800 | Crop to table + context |
| Dialog/Menu | 800x600 | Crop to relevant UI element |
| Error Message | 1200x400 | Crop to command + error |

---

### 3. Image Format Guidelines

#### PNG for Static Screenshots

**When to use PNG:**

- Single frame screenshots
- Text-heavy content (terminal output)
- High-contrast images (code, diagrams)

**PNG Settings:**

- [ ] 24-bit RGB color (before compression)
- [ ] 8-bit indexed color (after compression with pngquant)
- [ ] No transparency (use solid background)
- [ ] No interlacing (not needed for modern browsers)

**Compression:**

```bash
# Compress PNG with pngquant (lossy, good quality)
pngquant --quality=80-95 --strip input.png -o output.png

# Or use optipng (lossless, slower)
optipng -o7 input.png
```

**Target file size**: <500KB for full-screen screenshots

---

#### GIF for Animations

**When to use GIF:**

- Interactive keyboard navigation demos
- Loading spinner/progress indicator animations
- Multi-step command sequences

**GIF Settings:**

- [ ] 256 colors (8-bit indexed)
- [ ] Frame rate: 10-15 fps (smooth but small file size)
- [ ] Duration: 3-10 seconds (loop 2-3 times, then stop)
- [ ] Optimize with gifsicle or similar tool

**Creating GIFs from terminal recordings:**

```bash
# Record terminal with asciinema
asciinema rec recording.cast

# Convert to GIF with agg (better than terminalizer)
agg --fps 15 --speed 1.5 recording.cast output.gif

# Optimize GIF size
gifsicle -O3 --colors 256 --lossy=80 output.gif -o optimized.gif
```

**Target file size**: <1MB for 5-second animation

---

### 4. File Naming Convention

**Pattern**: `feature-component-purpose.{png,gif}`

**Rules**:

- Use kebab-case (lowercase, hyphens)
- Start with feature name (budget, recommendations, tui)
- Include component (table, dialog, spinner, error)
- End with purpose (light, dark, high-contrast, usage)

**Examples**:

| Scenario | Filename |
|----------|----------|
| Budget display (TTY mode) | `budget-tty-mode.png` |
| Budget display (plain mode) | `budget-plain-mode.png` |
| Recommendations table | `recommendations-table.png` |
| Recommendation detail view | `recommendations-detail-view.png` |
| Loading spinner animation | `loading-spinner.gif` |
| Error message | `error-message-invalid-json.png` |
| High contrast mode | `tui-high-contrast-mode.png` |
| Light terminal theme | `tui-adaptive-colors-light.png` |
| Dark terminal theme | `tui-adaptive-colors-dark.png` |

---

### 5. Context and Composition

**Include Full Context:**

- [ ] Show command prompt with command being run
- [ ] Include working directory in prompt (if relevant)
- [ ] Capture full output (scroll if necessary, or show representative section)
- [ ] Show cursor position (if demonstrating interactive mode)

**Good Context Example:**

```text
$ finfocus cost recommendations --pulumi-json plan.json
┌──────────────────────────────────────────────────────────────────┐
│ RESOURCE              │ RECOMMENDATION  │ SAVINGS   │ PRIORITY   │
├──────────────────────────────────────────────────────────────────┤
│ aws:ec2:Instance      │ Rightsize       │ $127.50   │ High       │
│ aws:rds:Instance      │ Terminate       │ $450.00   │ Medium     │
└──────────────────────────────────────────────────────────────────┘
Press ↑/↓ to navigate, Enter for details, q to quit
```

**Bad Context Example** (missing command):

```text
│ RESOURCE              │ RECOMMENDATION  │ SAVINGS   │ PRIORITY   │
├──────────────────────────────────────────────────────────────────┤
│ aws:ec2:Instance      │ Rightsize       │ $127.50   │ High       │
```

---

### 6. Annotation Requirements

**When to Add Annotations:**

- Highlighting specific UI elements (boxes, arrows)
- Labeling keyboard shortcuts (text overlays)
- Indicating click/interaction areas (circles)
- Calling attention to important output (underlines)

**Annotation Tools:**

| Platform | Tool | Notes |
|----------|------|-------|
| macOS | Skitch, Annotate | Built-in annotation tools |
| Linux | Flameshot, GIMP | Screenshot + annotation |
| Windows | Greenshot, Snip & Sketch | Built-in annotation |

**Annotation Guidelines:**

- [ ] Use red for emphasis (RGB: #FF0000 or similar)
- [ ] Use arrow thickness of 3-5px (visible but not overwhelming)
- [ ] Add text labels with white background + black text for readability
- [ ] Keep annotations minimal (only what's necessary)

**Example Annotations:**

- Red box around keyboard shortcut in footer
- Arrow pointing to budget threshold indicator
- Text label: "Press Enter here" with arrow

---

## Accessibility Requirements

### 1. Alt Text Guidelines

**Required for all images**. Alt text should:

- Describe what the image shows (not "screenshot of" or "image of")
- Be concise (<125 characters for optimal screen reader experience)
- Use proper capitalization (sentence case)
- Avoid redundant phrases ("picture of", "graphic showing")

**Alt Text Examples:**

| ❌ Poor Alt Text | ✅ Good Alt Text |
|-----------------|-----------------|
| `screenshot` | `Budget status display with color-coded threshold bars` |
| `TUI output` | `Recommendations table showing 3 high-priority optimizations` |
| `image of terminal` | `Loading spinner with elapsed time indicator` |
| `Budget display in TTY mode` | `Budget at 75% with green indicator and progress bar` |

**Alt Text Formula:**

```text
[Primary element] + [Key visual feature] + [Current state]
```

Examples:

- "Budget status display with color-coded bars at 75% usage"
- "Recommendations table sorted by savings with 5 entries"
- "Error message indicating invalid Pulumi JSON format"

---

### 2. High Contrast Validation

**Test screenshots for accessibility:**

- [ ] Text readable at arm's length (14pt minimum font size)
- [ ] Color contrast ratio ≥4.5:1 (WCAG AA standard)
- [ ] Important information not conveyed by color alone
- [ ] Works in grayscale (test by desaturating)

**Validation Tools:**

- **Color Contrast Analyzer**: <https://www.tpgi.com/color-contrast-checker/>
- **WAVE Browser Extension**: <https://wave.webaim.org/>
- **Desaturate Test**: Convert to grayscale in image editor

---

### 3. Plain Text Alternative

For TUI screenshots showing interactive elements, provide plain text alternative:

**Example:**

```markdown
![Budget status display with color-coded threshold bars](../assets/screenshots/budget-tty-mode.png)

**Figure 1**: Budget display in TTY mode with adaptive colors

**Plain text equivalent:**

\```text
Budget Status: $375.00 / $500.00 (75%)
Alert Threshold: 80% (not exceeded)
Period: Monthly
Currency: USD
Status: UNDER BUDGET
\```
```

---

## Visual Example Scenarios

### Required Screenshots for TUI Documentation

| Screenshot | Filename | Purpose |
|------------|----------|---------|
| Budget TTY mode | `budget-tty-mode.png` | Show full color output with Unicode |
| Budget plain mode | `budget-plain-mode.png` | Show no-color ASCII output |
| Recommendations table | `recommendations-table.png` | Show interactive table with keyboard shortcuts |
| Recommendation detail | `recommendations-detail-view.png` | Show expanded detail view |
| Loading spinner | `loading-spinner.gif` | Animated spinner during calculation |
| Error message | `error-invalid-json.png` | Show helpful error message |
| High contrast mode | `tui-high-contrast-mode.png` | Accessibility feature demonstration |
| Light terminal | `tui-adaptive-colors-light.png` | Color adaptation for light backgrounds |
| Dark terminal | `tui-adaptive-colors-dark.png` | Color adaptation for dark backgrounds |

---

### Screenshot Capture Workflow

1. **Prepare Environment**
   - [ ] Clean terminal session
   - [ ] Correct terminal size (100x30 minimum)
   - [ ] Font size 14pt+ for readability

2. **Run Command**
   - [ ] Execute command showing feature
   - [ ] Wait for output to stabilize
   - [ ] Verify no errors or warnings

3. **Capture Screenshot**
   - [ ] Full window capture (Cmd+Shift+4, Space on macOS)
   - [ ] Or selection capture (Cmd+Shift+4, drag on macOS)
   - [ ] Save to desktop or temp location

4. **Crop and Edit**
   - [ ] Crop to relevant content (remove excess whitespace)
   - [ ] Add annotations (if needed)
   - [ ] Verify text readability at target size

5. **Optimize File Size**
   - [ ] Compress with pngquant or optipng
   - [ ] Verify file size <500KB (PNG) or <1MB (GIF)
   - [ ] Test display quality after compression

6. **Add to Documentation**
   - [ ] Move to `docs/assets/screenshots/` directory
   - [ ] Reference in markdown with alt text
   - [ ] Add figure caption below image
   - [ ] Commit with descriptive message

---

## Markdown Integration

### Image Syntax

```markdown
![Descriptive alt text](../assets/screenshots/filename.png)

**Figure N**: Caption explaining what the image demonstrates.
```

### Example with Context

```markdown
## Budget Display Modes

FinFocus adapts budget displays to your terminal capabilities.

### TTY Mode (Default)

![Budget status display with color-coded threshold bars at 75% usage](../assets/screenshots/budget-tty-mode.png)

**Figure 1**: Budget display in TTY mode with adaptive colors, Unicode box drawing, and emoji indicators.

### Plain Text Mode

For CI/CD environments or accessibility, use `--plain` flag:

\```bash
finfocus cost projected --pulumi-json plan.json --plain
\```

![Budget status in plain text mode without colors or Unicode](../assets/screenshots/budget-plain-mode.png)

**Figure 2**: Budget display in plain text mode using ASCII characters and no color.
```

---

## File Organization

### Directory Structure

```text
docs/assets/screenshots/
├── budget-tty-mode.png
├── budget-plain-mode.png
├── recommendations-table.png
├── recommendations-detail-view.png
├── loading-spinner.gif
├── error-invalid-json.png
├── tui-high-contrast-mode.png
├── tui-adaptive-colors-light.png
└── tui-adaptive-colors-dark.png
```

**No subdirectories**: Keep all screenshots in `docs/assets/screenshots/` for simplicity.

---

## Quality Assurance Checklist

Before committing screenshots:

- [ ] Resolution ≥1600x900 pixels
- [ ] File size: PNG <500KB, GIF <1MB
- [ ] Filename follows kebab-case convention
- [ ] Alt text under 125 characters and descriptive
- [ ] Figure caption explains what's shown
- [ ] Text readable at target display size
- [ ] No personal information visible (API keys, paths)
- [ ] Color contrast meets WCAG AA (≥4.5:1)
- [ ] Compressed with pngquant or optipng
- [ ] Tested in grayscale for accessibility
- [ ] Referenced in markdown documentation
- [ ] Committed with descriptive message

---

## Troubleshooting

### Issue: Screenshot text too small

**Solution**: Increase terminal font size to 16-18pt before capturing. Capture at 2x resolution.

### Issue: Colors look washed out after compression

**Solution**: Use `pngquant --quality=85-95` for better color preservation.

### Issue: GIF file size exceeds 1MB

**Solution**: Reduce frame rate (`--fps 10`), shorten duration (<5s), or use fewer colors (`--colors 128`).

### Issue: Unicode box drawing renders incorrectly

**Solution**: Use terminal with good Unicode support (iTerm2, Windows Terminal). Verify font has box drawing characters.

### Issue: Screenshot shows personal paths or API keys

**Solution**: Use `~` for home directory, mask API keys with `export API_KEY=sk-...`. Re-capture if needed.

---

## Tools Reference

### Screenshot Capture

| Platform | Tool | Command |
|----------|------|---------|
| macOS | Built-in | `Cmd+Shift+4` (selection), `Cmd+Shift+4 Space` (window) |
| Linux | Flameshot | `flameshot gui` (interactive) |
| Windows | Snip & Sketch | `Win+Shift+S` (selection) |

### Terminal Recording

| Tool | Purpose | Command |
|------|---------|---------|
| asciinema | Record terminal sessions | `asciinema rec output.cast` |
| agg | Convert asciinema to GIF | `agg recording.cast output.gif` |
| terminalizer | Record + render GIF | `terminalizer record demo` |

### Image Optimization

| Tool | Purpose | Command |
|------|---------|---------|
| pngquant | Lossy PNG compression | `pngquant --quality=80-95 input.png` |
| optipng | Lossless PNG optimization | `optipng -o7 input.png` |
| gifsicle | GIF optimization | `gifsicle -O3 --lossy=80 input.gif` |

### Validation

| Tool | Purpose | URL |
|------|---------|-----|
| Color Contrast Checker | WCAG contrast validation | <https://www.tpgi.com/color-contrast-checker/> |
| WAVE | Accessibility testing | <https://wave.webaim.org/> |

---

**Checklist Version**: 1.0.0
**Last Updated**: 2026-01-20
**Maintainer**: Documentation Team
