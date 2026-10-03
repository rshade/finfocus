---
title: Resource References
description: How FinFocus resolves Pulumi propertyDependencies and sends them to cost plugins.
---

## Overview

Some cost-bearing resources do not carry their own region or SKU. They point at
another resource in the same stack. Pulumi preview JSON records that pointer on
`propertyDependencies`, and often writes the unknown-value sentinel
`04da6b54-80e4-46f7-96ec-b56ff0331ba9` into the input. Only core sees the whole
plan, so core resolves the pointer before it calls a plugin.

The Pulumi `parent` field is the stack component, not the referenced resource.
Core does not use it for pricing.

## Tag contract

For each input property that `propertyDependencies` maps to exactly one resource
in the plan, and that resource has a region or a SKU, the child request gains
these tags:

| Tag | Value |
| --- | --- |
| `ref.<property>.urn` | URN of the referenced resource |
| `ref.<property>.type` | Pulumi type token of the referenced resource |
| `ref.<property>.region` | Referenced resource's own region, when it has one |
| `ref.<property>.sku` | Referenced resource's own SKU, when it has one |

`<property>` is the input name (`servicePlanId`, `serverId`, `serverFarmId`,
`accountName`, and so on). Core does not decide which property is the pricing
parent. A plugin reads the key it knows.

The tags are absent when the property maps to an empty list, to more than one
URN, to a URN that is not in the plan, or to a resource that has neither a
region nor a SKU. Nothing is guessed. The sentinel is never sent as a tag,
region, or SKU.

## Region and SKU on the child

The child's own `Sku` is never replaced with the referenced resource's SKU. A
web app must not be priced as its App Service plan.

When the child's own region is empty and exactly one referenced resource has a
region, that region is copied onto the request `Region` field only. If several
referenced resources have regions, `Region` stays empty.

A child with no SKU of its own that already carries `ref.*` tags passes lenient
pre-flight validation and is sent to the plugin. The plugin should return an
explicit note and no monthly cost when the cost lives on the referenced
resource. Strict validation still applies when the child has no `ref.*` tags.

Classic Azure plans store size in `skuName`. That value is placed on
`ref.<property>.sku` only. It is not treated as the plan resource's own SKU.

## What does not change

A plan with no `propertyDependencies` produces the same tags, SKU, region, and
cache key as before. When `ref.*` tags are present, the projected cache key
gains a `/refs-<hash>` suffix so a changed reference does not reuse a stale
price.

Terraform state dependencies are not resolved by this path.
