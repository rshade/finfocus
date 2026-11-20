---
title: "Custom plugin registry web API"
description: "Operations available from the Plugins page."
layout: "docs"
---

| Request | Shared method |
| --- | --- |
| GET /api/plugins/registries | `registry.CustomRegistryService.List` |
| POST /api/plugins/registries | `registry.CustomRegistryService.Add` |
| DELETE /api/plugin-registries/{name} | `registry.CustomRegistryService.Remove` |
| GET /api/plugins/catalog | `registry.CustomRegistryService.Catalog` |
| Plugin plan | `registry.CustomRegistryService.Resolve` |
| Plugin install and update | `registry.CustomRegistryService.Install` |

Registry names restrict catalog and install selection. Mutation endpoints
use the dashboard's existing authentication and origin checks. An install
confirmation carries the source, selected release, and repository. The first
install also carries the typed source name.
