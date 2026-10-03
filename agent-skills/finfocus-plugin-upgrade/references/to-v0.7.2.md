# Upgrade to finfocus-spec v0.7.2: additive release, manifests and provider

v0.7.2 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for most plugins.

## Manual: plugins that ship a registry manifest

The SDK now writes manifests that the schema and the validator both accept,
and `pluginsdk` rejects a manifest in which two keys name the same field. If
validation reports a duplicate for one field, keep one key and remove the
other.

## Manual: plugins that list IaC package names as providers

The spec now defines `ResourceDescriptor.provider` as the cloud that bills the
resource (`aws`, `azure`, `gcp`, `kubernetes`), not the Pulumi package or
Terraform provider. finfocus core sends the cloud and keeps the package in
`resource_type` (`aws-native:ec2:Instance` has provider `aws`).

A plugin that lists a package name such as `azure-native` in
`GetPluginInfo.providers` only so that routing matches can list the cloud
instead:

```go
// Before
Providers: []string{"azure", "azure-native"},

// After
Providers: []string{"azure"},
```

The old list keeps working, because core normalizes both sides when it routes.
A plugin that lists a package name now also receives resources of the same
cloud that use another package; answer `Supports` with false for the ones it
cannot price. A plugin that needs the package reads it from `resource_type`.

## Optional

Nothing in this release needs opting in.
