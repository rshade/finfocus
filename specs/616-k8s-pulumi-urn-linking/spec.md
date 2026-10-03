# Feature Specification: Link allocated workloads to Pulumi URNs

**Feature Branch**: `616-k8s-pulumi-urn-linking`
**Created**: 2026-10-03
**Status**: Implemented (issue #1527)
**Input**: GitHub issue #1527.

## Overview

`cost cluster` groups by Kubernetes identity. A Pulumi URN is attached only
when the user sets annotation `finfocus.dev/pulumi-urn` on the pod.

The Pulumi Kubernetes provider does not write that URN. Its own annotations
are `app.kubernetes.io/managed-by` (omitted under server-side apply) and
`pulumi.com/autonamed`. Neither is a stack URN. Name matching is not used:
the provider appends a random suffix unless `metadata.name` is pinned.

The value is an annotation, not a label. Kubernetes label values are at most
63 characters and cannot contain `:`. A URN is
`urn:pulumi:<stack>::<project>::<type>::<name>` and may include a parent
type before `$`.

### Non-goals

- Inferring ownership from object names or from provider annotations.
- Joining live pods to a loaded Pulumi plan or state file.
- Cross-cluster attribution.

## Contract

The collector copies a non-empty annotation onto subject key
`label.finfocus.dev/pulumi-urn`. The `label.` prefix is required:
`ValidateStatsResponse` rejects any other new subject key. An empty
annotation sets nothing. The allocator keeps the usage subject, so the
allocation row carries the same key.

`--group-by pulumi-stack` aggregates workload rows by `<stack>/<project>`.
A missing annotation, or a value that is not
`urn:pulumi:<stack>::<project>::<type>::<name>`, uses `<none>`. The row is
still counted. `__idle__` and `__cluster__` stay their own groups.

JSON and NDJSON encode `pulumi_urns` on each group that has at least one
such annotation. The list is sorted and unique. A malformed value is listed
there and still groups under `<none>`.
