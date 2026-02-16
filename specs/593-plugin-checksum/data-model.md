# Data Model: Plugin Checksum Verification

**Feature**: 593-plugin-checksum
**Date**: 2026-02-15

## Entities

### ChecksumEntry

Represents a single line parsed from a checksums file.

| Field | Type | Description |
| ----- | ---- | ----------- |
| Hash | string | 64-character lowercase hexadecimal SHA256 hash |
| Filename | string | Asset filename (no path, just the name) |

**Validation rules**:

- Hash must be exactly 64 hex characters (`[0-9a-f]{64}`)
- Filename must not be empty
- Filename must not contain path separators

**Source**: Parsed from `checksums.txt` release asset.

### InstallOptions (modified)

Existing struct with new field.

| Field | Type | Description | New |
| ----- | ---- | ----------- | --- |
| Force | bool | Reinstall even if version exists | No |
| NoSave | bool | Don't add to config file | No |
| PluginDir | string | Custom plugin directory | No |
| FallbackToLatest | bool | Auto-fallback on missing assets | No |
| NoFallback | bool | Disable fallback behavior | No |
| Metadata | map[string]string | User-supplied metadata | No |
| SkipChecksum | bool | Bypass checksum verification | **Yes** |

### UpdateOptions (modified)

Existing struct with new field.

| Field | Type | Description | New |
| ----- | ---- | ----------- | --- |
| DryRun | bool | Show what would be updated | No |
| Version | string | Target version | No |
| PluginDir | string | Custom plugin directory | No |
| SkipChecksum | bool | Bypass checksum verification | **Yes** |

## State Transitions

Checksum verification is stateless. No persistent state is created or modified by this
feature. The verification result is transient and affects only the current install/update
operation's success or failure.

### Verification Flow States

```text
START
  │
  ├─ SkipChecksum=true ──→ SKIPPED (install continues)
  │
  ├─ checksums.txt not in release ──→ WARN_MISSING (install continues)
  │
  ├─ checksums.txt download fails ──→ WARN_NETWORK (install continues)
  │
  ├─ checksums.txt parse fails ──→ WARN_MALFORMED (install continues)
  │
  ├─ asset not listed in checksums ──→ WARN_UNLISTED (install continues)
  │
  ├─ hash matches ──→ VERIFIED (install continues)
  │
  └─ hash mismatch ──→ FAILED (install aborted, temp file cleaned up)
```

## Relationships

```text
GitHubRelease 1──* ReleaseAsset (existing)
    │
    └── One asset MAY be "checksums.txt" (found by name search)

checksums.txt 1──* ChecksumEntry (parsed from file content)
    │
    └── One entry MAY match the downloaded asset name

InstallOptions ──→ installRelease() ──→ verifyChecksum()
```
