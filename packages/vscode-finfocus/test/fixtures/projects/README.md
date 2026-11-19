# Project fixtures

- `stack-ls-local.json` is the unedited output of `pulumi stack ls --json`
  (Pulumi v3.272.0, file backend, two stacks).
- `stack-ls-cloud.json` is the same command against a Pulumi Cloud project:
  fully qualified names and a `url` field. Written by hand from the documented
  shape, not captured from a live organization.
