# Upgrade to finfocus-spec v0.7.8: optional action hint on parsed resources

v0.7.8 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for most plugins. The one
item below applies only to plugins that implement `CostParseService`.

## What changed

- `ParsedResource` gains `ParseAction action = 4`, an optional hint for
  plan-style inputs that report a change per resource.
- The new `ParseAction` enum has `PARSE_ACTION_UNSPECIFIED` (0, the default),
  `CREATE`, `UPDATE`, `DELETE`, `REPLACE`, and `NO_CHANGE`.

An absent field decodes as `UNSPECIFIED`, so existing consumers are unaffected.
The hint carries no pricing meaning: how a host prices a `DELETE`, `REPLACE`, or
`NO_CHANGE` resource is the host's concern.

## Cost-parse plugins: set only defined actions

Parsers of declared templates, such as CloudFormation templates, Helm, and
Kustomize, leave `action` unset. A plugin that parses a plan-like artifact, such
as a change set, can report it.

Shared response validation rejects a `ParsedResource` whose `action` is not one
of the six defined values. `UNSPECIFIED` is valid. Run your conformance tests
after setting it: the `descriptors_valid` check applies the same rule.

Plugins that do not parse inputs need no change.
