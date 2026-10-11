# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.6](https://github.com/rshade/finfocus/compare/v0.4.5...v0.4.6) (2026-10-09)


### Added

* **web:** add finfocus --web localhost browser dashboard ([#1736](https://github.com/rshade/finfocus/issues/1736)) ([03628d0](https://github.com/rshade/finfocus/commit/03628d02d0aa918a6d475d42a1d767e4d047eee0))


### Fixed

* **deps:** expand Renovate security update coverage ([#1747](https://github.com/rshade/finfocus/issues/1747)) ([24236a5](https://github.com/rshade/finfocus/commit/24236a53c01c11ba35ab6759859266cad9ece938))
* **deps:** update dompurify to v3.4.16 in docs ([#1760](https://github.com/rshade/finfocus/issues/1760)) ([a8e58e9](https://github.com/rshade/finfocus/commit/a8e58e9f6052d05fc424d145d5b8ce9429bba1f3))
* **deps:** update go dependencies ([#1746](https://github.com/rshade/finfocus/issues/1746)) ([071d0e6](https://github.com/rshade/finfocus/commit/071d0e640f456a971fc2308b5eb1cbf31290015a))
* **deps:** update golang.org/x/net to v0.60.0 and CI Go to 1.27.2 ([#1754](https://github.com/rshade/finfocus/issues/1754)) ([4383b26](https://github.com/rshade/finfocus/commit/4383b26266c0b3f3f532be9ca8f8577abd0ec0bb))
* **webui:** address [#1736](https://github.com/rshade/finfocus/issues/1736) review follow-ups ([#1755](https://github.com/rshade/finfocus/issues/1755)) ([760d808](https://github.com/rshade/finfocus/commit/760d808676530d487a5b124a9044959c22e264ca))


### Documentation

* **github:** correct stale facts in the Copilot code-review skill ([#1729](https://github.com/rshade/finfocus/issues/1729)) ([f5ca6d6](https://github.com/rshade/finfocus/commit/f5ca6d67d5e6ca4ad2ecc2d69d50852abbb681d9))
* **roadmap:** sync with GitHub issues ([#1759](https://github.com/rshade/finfocus/issues/1759)) ([ca0953e](https://github.com/rshade/finfocus/commit/ca0953ef2ea2bd8f25e3ad6cceaaf0925f205f03)), closes [#1736](https://github.com/rshade/finfocus/issues/1736)
* **web:** document the finfocus --web dashboard ([#1756](https://github.com/rshade/finfocus/issues/1756)) ([eb83650](https://github.com/rshade/finfocus/commit/eb8365037ea6ebbae068d9825aaefbc4e8cab996)), closes [#1736](https://github.com/rshade/finfocus/issues/1736)

## [0.4.5](https://github.com/rshade/finfocus/compare/v0.4.4...v0.4.5) (2026-10-06)


### Added

* **cli:** add cost forecast from plugin growth ([#1715](https://github.com/rshade/finfocus/issues/1715)) ([0622f7f](https://github.com/rshade/finfocus/commit/0622f7f9bfb1e5f256564da0512d59f335add132)), closes [#364](https://github.com/rshade/finfocus/issues/364)
* **cluster:** price historical windows from a Prometheus usage source ([#1717](https://github.com/rshade/finfocus/issues/1717)) ([d474f27](https://github.com/rshade/finfocus/commit/d474f27082a1345444943de078e9282f88bb5458))
* **registry:** add flexera plugin ([#1725](https://github.com/rshade/finfocus/issues/1725)) ([c5606a0](https://github.com/rshade/finfocus/commit/c5606a0a0d120cd78b586a2f1e1aa8ea1388b5b9))
* **registry:** add opencost plugin ([#1720](https://github.com/rshade/finfocus/issues/1720)) ([acc0dbf](https://github.com/rshade/finfocus/commit/acc0dbfd00f3c8247120b9710a079a74082fe74f))
* **registry:** add vantage plugin ([#1724](https://github.com/rshade/finfocus/issues/1724)) ([6c92036](https://github.com/rshade/finfocus/commit/6c920362fb9102e83a9442ac1012efdb804aa52b))


### Performance

* **cli:** run stack export and preview concurrently in overview ([#1723](https://github.com/rshade/finfocus/issues/1723)) ([aa97c53](https://github.com/rshade/finfocus/commit/aa97c534d08f52b89be21f63d34b0aaa33d715d1)), closes [#691](https://github.com/rshade/finfocus/issues/691)
* **registry:** open plugins concurrently in Registry.Open ([#1713](https://github.com/rshade/finfocus/issues/1713)) ([bc27ad5](https://github.com/rshade/finfocus/commit/bc27ad50f266b2c419c719cc30c73e50b6af5b6b))


### Documentation

* **claude:** trim CLAUDE.md to commands and gotchas ([#1722](https://github.com/rshade/finfocus/issues/1722)) ([a8bcab4](https://github.com/rshade/finfocus/commit/a8bcab43f37c97e573bdcbab7eeaa34604773e28))
* **plugins:** fold kubecost into opencost and drop the planned page ([#1721](https://github.com/rshade/finfocus/issues/1721)) ([ed19769](https://github.com/rshade/finfocus/commit/ed19769c36b8d08639b88692c10646dfbdedf3c7))

## [0.4.4](https://github.com/rshade/finfocus/compare/v0.4.3...v0.4.4) (2026-10-05)


### Added

* **budget:** send Slack and webhook notifications for budget alerts ([#1708](https://github.com/rshade/finfocus/issues/1708)) ([6824279](https://github.com/rshade/finfocus/commit/68242796c57149d14199dbb26eed8d19b0181bd9))
* **overview:** expand Kubernetes cluster resources into workload rows ([#1705](https://github.com/rshade/finfocus/issues/1705)) ([08236bc](https://github.com/rshade/finfocus/commit/08236bccb41d025723586baa266c2c10b611e365))


### Fixed

* **engine:** report plugin's error instead of 'no cost data available' ([#1701](https://github.com/rshade/finfocus/issues/1701)) ([6619430](https://github.com/rshade/finfocus/commit/66194306bfb9b673cfce69e4cee20382be8afdf1))


### Changed

* **overview:** compute once, render many ([#853](https://github.com/rshade/finfocus/issues/853)) ([#1700](https://github.com/rshade/finfocus/issues/1700)) ([260b45c](https://github.com/rshade/finfocus/commit/260b45ce6515c28c034782c7fb2947eb5ba35fc6))


### Documentation

* **github:** stop pinning versions in Copilot instructions ([#1710](https://github.com/rshade/finfocus/issues/1710)) ([91a226d](https://github.com/rshade/finfocus/commit/91a226d74a931eaa30230ef4252811749c27234b))
* link finfocus-action and its live demo ([#1712](https://github.com/rshade/finfocus/issues/1712)) ([54b046d](https://github.com/rshade/finfocus/commit/54b046d594bf161ee29e682654d3cf2049db5ace))

## [0.4.3](https://github.com/rshade/finfocus/compare/v0.4.2...v0.4.3) (2026-10-05)


### Added

* **cli:** pass include_dismissed on cost recommendations ([890ee39](https://github.com/rshade/finfocus/commit/890ee394161dd0473d7f7516aeb44bbdf1ac0489)), closes [#545](https://github.com/rshade/finfocus/issues/545)
* **registry:** add azure-public plugin ([#1697](https://github.com/rshade/finfocus/issues/1697)) ([61d477c](https://github.com/rshade/finfocus/commit/61d477c21dc6d5f9c0bc4173c57a692712d83c47)), closes [#1685](https://github.com/rshade/finfocus/issues/1685)


### Fixed

* **cli:** plugin inspect resolves versions via registry ([#1694](https://github.com/rshade/finfocus/issues/1694)) ([810ceb7](https://github.com/rshade/finfocus/commit/810ceb7701fdb26a6bad9a3e899195d50409bf2e))
* **cli:** plugin validate prints doubled v in version ([#1695](https://github.com/rshade/finfocus/issues/1695)) ([8cb46b9](https://github.com/rshade/finfocus/commit/8cb46b94ddb8ebfb3e7d89ef456b938d5d3aedb3))
* **engine:** skip scorer-only plugins on cost requests ([b2ec655](https://github.com/rshade/finfocus/commit/b2ec65555fc0cfd9154bb44ba58aa6ebd6b8f93d)), closes [#1687](https://github.com/rshade/finfocus/issues/1687)
* **kubernetes:** decline reason names priced workloads ([#1692](https://github.com/rshade/finfocus/issues/1692)) ([58c07e3](https://github.com/rshade/finfocus/commit/58c07e3848fb55e0f29a236ecbb97a85963b1478))
* **pluginupgrade:** add v0.7.5 hop and guide; golines-format cluster test ([#1699](https://github.com/rshade/finfocus/issues/1699)) ([3b70279](https://github.com/rshade/finfocus/commit/3b70279a8bad451b48b2a0a10430bb7169456547))
* **proto:** collapse nested sku objects in actual-cost tag enrichment ([#1693](https://github.com/rshade/finfocus/issues/1693)) ([a9b0770](https://github.com/rshade/finfocus/commit/a9b07707f6d09b94ef301e6bb132fff26ad7d4a6))


### Documentation

* add multi-plugin walkthrough and fix broken internal links ([d2b651c](https://github.com/rshade/finfocus/commit/d2b651c03c775ea7b4e722d8b8b6760f75c8c949))
* point README directory links at tree/main and clarify EKS control-plane row ([8ce2814](https://github.com/rshade/finfocus/commit/8ce281422bfb7fc773184f382b1990cf59bd66a5))
* stop committing generated root-doc copies and link the roadmap page to ROADMAP.md ([054e857](https://github.com/rshade/finfocus/commit/054e857e7087c77dcb49e14db52e40aa22aa9c03))

## [0.4.2](https://github.com/rshade/finfocus/compare/v0.4.1...v0.4.2) (2026-10-04)


### Added

* **engine:** send redacted resource attributes to plugins ([128cb75](https://github.com/rshade/finfocus/commit/128cb759f2641b6d7354af3776ae0f810aac19f6)), closes [#1525](https://github.com/rshade/finfocus/issues/1525)
* **engine:** send the resource descriptor on actual cost requests ([#1680](https://github.com/rshade/finfocus/issues/1680)) ([13330a7](https://github.com/rshade/finfocus/commit/13330a736feeece6f2d27d839ccd2baf5e87d493))
* **kubernetes:** price workloads declared in a Pulumi plan ([3bb6c11](https://github.com/rshade/finfocus/commit/3bb6c116b19fa64c95a6281f0b1118d26f5bffbd)), closes [#1525](https://github.com/rshade/finfocus/issues/1525)


### Fixed

* **deps:** update go dependencies ([#1678](https://github.com/rshade/finfocus/issues/1678)) ([02a2525](https://github.com/rshade/finfocus/commit/02a2525732f4b1ffc429325857673804d36c5f7a))
* **engine:** keep credentials and Pulumi secrets out of plugin requests ([a884ec0](https://github.com/rshade/finfocus/commit/a884ec0b5a5811be5d286690bc36bf6b859bf7eb)), closes [#1525](https://github.com/rshade/finfocus/issues/1525)
* **engine:** stop logging omitted attribute values ([52f611b](https://github.com/rshade/finfocus/commit/52f611bd9ed09595c0512cf536d384088531a877)), closes [#1525](https://github.com/rshade/finfocus/issues/1525)
* **registry:** handle releases without assets and honor FINFOCUS_HOME ([156c59e](https://github.com/rshade/finfocus/commit/156c59e26e27d52d39c8b490544a76bd5cd0a52a))
* **release:** keep plugin releases from becoming Latest ([b9fdc2e](https://github.com/rshade/finfocus/commit/b9fdc2e69cf9e73d06a34c5ee158ef1ab954a1ca))

## [0.4.1](https://github.com/rshade/finfocus/compare/v0.4.0...v0.4.1) (2026-10-03)


### Added

* **cli:** add accessibility flags for plain and high-contrast output ([#1627](https://github.com/rshade/finfocus/issues/1627)) ([706c5f3](https://github.com/rshade/finfocus/commit/706c5f37f093c45d84a4fb4c650a12d88282ecbd))
* **cli:** add cost history collect, view, and list ([6b6bafc](https://github.com/rshade/finfocus/commit/6b6bafcf674d78514b49dc0f4481f7faefc53c10)), closes [#549](https://github.com/rshade/finfocus/issues/549)
* **cli:** add plugin upgrade command and agent skill ([#1611](https://github.com/rshade/finfocus/issues/1611)) ([6cdc78d](https://github.com/rshade/finfocus/commit/6cdc78d583e41fc08fb6512bd9839f20fd46c3f0))
* **cli:** add show-breakdown and show-confidence cost flags ([c07648b](https://github.com/rshade/finfocus/commit/c07648bec80e4f2e9bccec8d1c208f565078efdd)), closes [#685](https://github.com/rshade/finfocus/issues/685)
* **cli:** diff cost history snapshots by resource ([3e73990](https://github.com/rshade/finfocus/commit/3e7399036d4734ab7a48eb9315f7b4b73edb0220)), closes [#554](https://github.com/rshade/finfocus/issues/554)
* **cli:** export cost history and show table sparklines ([4bcb8bd](https://github.com/rshade/finfocus/commit/4bcb8bdc356f4a56492575811464e756354d3973)), closes [#551](https://github.com/rshade/finfocus/issues/551)
* **cli:** prune old cost history snapshots ([#1646](https://github.com/rshade/finfocus/issues/1646)) ([f6586e9](https://github.com/rshade/finfocus/commit/f6586e9e34944be3bc842264f34b2de48f1fa86e)), closes [#555](https://github.com/rshade/finfocus/issues/555)
* **cli:** show pricing spec details with cost projected --explain ([2683c2d](https://github.com/rshade/finfocus/commit/2683c2de30a671047acd868d66a1b6186253df00)), closes [#636](https://github.com/rshade/finfocus/issues/636)
* **config:** add configuration validation reports with hints ([9697f3f](https://github.com/rshade/finfocus/commit/9697f3f854ead108d6eef9ced0c221b58fb407b6)), closes [#223](https://github.com/rshade/finfocus/issues/223)
* **engine:** add cost diff view to cost projected command ([967f994](https://github.com/rshade/finfocus/commit/967f9941e6b8251e41099a0eb94eff6b9ae3d3de)), closes [#576](https://github.com/rshade/finfocus/issues/576)
* **engine:** add optional in-memory LRU in front of BoltDB cache ([3e67578](https://github.com/rshade/finfocus/commit/3e67578671226f954859fabbd20815bc4c1cf4e8)), closes [#495](https://github.com/rshade/finfocus/issues/495)
* **engine:** add warning column and OverviewWarning type to overview ([734982d](https://github.com/rshade/finfocus/commit/734982d4b5d904b1f0dc9f409a96f775a6501dbd)), closes [#643](https://github.com/rshade/finfocus/issues/643)
* **engine:** emit nested resource inputs as additive dotted tag keys ([8f425fd](https://github.com/rshade/finfocus/commit/8f425fd25678bb5722bfe404dccb8df537442918)), closes [#1608](https://github.com/rshade/finfocus/issues/1608)
* **engine:** price projected cost from plugin GetPricingSpec before YAML ([7be7116](https://github.com/rshade/finfocus/commit/7be711633fec7140b7af980e286737ba91e15f80)), closes [#638](https://github.com/rshade/finfocus/issues/638)
* **engine:** send referenced resource region and SKU as ref tags ([#1615](https://github.com/rshade/finfocus/issues/1615)) ([c32ed2d](https://github.com/rshade/finfocus/commit/c32ed2d991df478967a38d327d4d61b9569b8d8f)), closes [#1610](https://github.com/rshade/finfocus/issues/1610)
* **plugin:** install agent skills from plugin init and plugin upgrade ([811835d](https://github.com/rshade/finfocus/commit/811835d6893733125fcd10a0f451171ed7e640b6))
* **plugin:** scaffold new plugins on core's finfocus-spec version ([2ec0196](https://github.com/rshade/finfocus/commit/2ec0196dd1dff5c579d0bdd9c1e15c49cd073bb6)), closes [#248](https://github.com/rshade/finfocus/issues/248)
* **skills:** add finfocus-diagnose skill ([#1612](https://github.com/rshade/finfocus/issues/1612)) ([d1f9294](https://github.com/rshade/finfocus/commit/d1f929423c62c523950b42f9139639eca2839732))
* **tui:** show GetPricingSpec discovery in cost estimate ([d20257f](https://github.com/rshade/finfocus/commit/d20257fb5a6d3c00549cc8836076f15151a70504)), closes [#637](https://github.com/rshade/finfocus/issues/637)


### Fixed

* **cli:** accept csv and ndjson cost history export ([85a9b80](https://github.com/rshade/finfocus/commit/85a9b807b5e47828d2585457413081cca9a06cb5)), closes [#551](https://github.com/rshade/finfocus/issues/551)
* **cli:** apply accessibility flags consistently ([0b9c333](https://github.com/rshade/finfocus/commit/0b9c333d3e7e4b1592c042de7f74509d4697f5de)), closes [#1635](https://github.com/rshade/finfocus/issues/1635)
* **engine:** align pricing-spec and Supports with the pricing descriptor ([ad8c5ee](https://github.com/rshade/finfocus/commit/ad8c5ee25b16340b8e909758ba3fda17517149b8)), closes [#1639](https://github.com/rshade/finfocus/issues/1639)
* **engine:** bound pricing-spec calls and key them apart in the cache ([#1651](https://github.com/rshade/finfocus/issues/1651)) ([accdf36](https://github.com/rshade/finfocus/commit/accdf36942b5e29d881ff7898456f60f9ca8ebd9)), closes [#1632](https://github.com/rshade/finfocus/issues/1632)
* **engine:** collapse replacement steps into one cost diff entry ([#1644](https://github.com/rshade/finfocus/issues/1644)) ([e73834c](https://github.com/rshade/finfocus/commit/e73834c569e3bc693f5af5bf3568cd31ca65d9e8)), closes [#1630](https://github.com/rshade/finfocus/issues/1630)
* **engine:** keep per_day on the 30-day billing month ([c7bdf24](https://github.com/rshade/finfocus/commit/c7bdf24887538526fece2048668c93d4a3debcf6))
* **engine:** price cost diff before and after from the same basis ([#1642](https://github.com/rshade/finfocus/issues/1642)) ([f5af066](https://github.com/rshade/finfocus/commit/f5af066d6e8198c3b5d1d7a525540962c2c5d649)), closes [#1629](https://github.com/rshade/finfocus/issues/1629)
* **engine:** price tiered specs as graduated tiers ([1d0e018](https://github.com/rshade/finfocus/commit/1d0e018808264aeef7cf7bb32a9f5357d0529639))
* **engine:** re-key cached projected costs and drop internal rows from the diff ([e9fac54](https://github.com/rshade/finfocus/commit/e9fac5460f53906a09012c4e44667ab562bad466)), closes [#1631](https://github.com/rshade/finfocus/issues/1631)
* follow FORCE_COLOR and Pulumi paging conventions from primary sources ([7b2f109](https://github.com/rshade/finfocus/commit/7b2f1099d2caf238ab0cd530682e220e88c0150c))
* **history:** handle mixed-currency snapshots in cost history ([#1643](https://github.com/rshade/finfocus/issues/1643)) ([86588aa](https://github.com/rshade/finfocus/commit/86588aa84dd796ccdf9353a88cf8381a4841cdda)), closes [#556](https://github.com/rshade/finfocus/issues/556)
* **history:** keep cost history per project and stack ([797912b](https://github.com/rshade/finfocus/commit/797912b88c5e8c045ae17976628613cfe5e0ee63)), closes [#1634](https://github.com/rshade/finfocus/issues/1634)
* **history:** keep paging when a full page repeats a version ([ba3bbae](https://github.com/rshade/finfocus/commit/ba3bbae549e125a8661c65d665ad348dff6376a1))
* **history:** read the full stack history and harden list and view ([af7df94](https://github.com/rshade/finfocus/commit/af7df94941d2eae3de98fe9206af9db59045fa02)), closes [#1637](https://github.com/rshade/finfocus/issues/1637)
* **history:** skip Pulumi provider resources when collecting cost history ([87eb430](https://github.com/rshade/finfocus/commit/87eb4301edef79b628a0bae189b1b9a717ca8335)), closes [#1633](https://github.com/rshade/finfocus/issues/1633)
* **kubernetes:** keep same-named nodes separate across clusters ([e580864](https://github.com/rshade/finfocus/commit/e580864b22566d3105f8365a2b0e8bec5c193432)), closes [#1588](https://github.com/rshade/finfocus/issues/1588)
* **overview:** mark plugin cost failures with the error warning ([dfae16e](https://github.com/rshade/finfocus/commit/dfae16e3ee67a1883940badab991bfc3288f4ffb)), closes [#1636](https://github.com/rshade/finfocus/issues/1636)
* **plugin:** correct the plugin init manifest, spec version, and pricing example ([eae59d0](https://github.com/rshade/finfocus/commit/eae59d03d71bc2c908eceda5b03f00090909d047))
* **plugin:** keep skill install writes inside the plugin directory ([39bde0e](https://github.com/rshade/finfocus/commit/39bde0e2c5ff7007b54623da618b944d537fad5b))
* **plugin:** run the skills CLI outside the plugin directory ([fb20b59](https://github.com/rshade/finfocus/commit/fb20b59dfa8db715ed63a93cd85b42f2b6d48a8d))
* **router:** normalize Pulumi package prefixes to the cloud provider ([b64b28a](https://github.com/rshade/finfocus/commit/b64b28a687d5c70fba8f846b00f65f7716241033)), closes [#1645](https://github.com/rshade/finfocus/issues/1645)


### Changed

* **cli:** thread budget flag overrides without mutating config ([0b005e2](https://github.com/rshade/finfocus/commit/0b005e26e24a8f234c643b52ccf408c47e3b9e8b)), closes [#808](https://github.com/rshade/finfocus/issues/808)


### Documentation

* add CI recipes for cost history collection ([9a14ffe](https://github.com/rshade/finfocus/commit/9a14ffed01716150b6aacfff5b25ef3505201948)), closes [#553](https://github.com/rshade/finfocus/issues/553)
* correct overview docs, note the cost projected output change, fix budget skill ([13629dc](https://github.com/rshade/finfocus/commit/13629dc01a9b578e1209ac403e62e2e5495acf53)), closes [#1638](https://github.com/rshade/finfocus/issues/1638)
* **overview:** document flags, columns, keys, and sample output ([3124e40](https://github.com/rshade/finfocus/commit/3124e407a0843f2c0fb63f3fae7aa311f368ba1c)), closes [#646](https://github.com/rshade/finfocus/issues/646)

## [Unreleased]

### Added

* **cli:** `cost projected --explain` shows plugin `GetPricingSpec` billing
  mode, unit, rate, source, assumptions, and tiers beside each resource.
  The calculated cost is unchanged. JSON and NDJSON include `pricing_spec`
  only when the flag is set. A plugin that does not implement the RPC still
  prints the cost (#636)
* **cli:** `cost history export` writes JSON, CSV, or NDJSON for one stack.
  `--from`, `--to`, and `--provider` filter the series. `cost projected` and
  `cost actual` show a Unicode block sparkline in a Trend column when that
  stack has cost history (#551)
* **docs:** CI recipes record a cost snapshot after deploy with
  `cost history collect --versions 1`. The flag still defaults to `0`,
  which stores every successful checkpoint that is not already saved (#553)
* **cli:** `cost history diff` compares two stored snapshots by resource
  URN. `--from` and `--to` accept a version (`v35`) or a date. `--to`
  defaults to the latest snapshot. `--threshold` hides small monthly
  changes, and `--output json` prints the delta (#554)
* **cli:** `cost history prune` drops snapshots with `--keep` and
  `--older-than`. `--dry-run` prints the plan. The command asks before
  deleting unless `--force` or `--yes`, then compacts the database.
  `cost.history.retention.auto_prune` applies `max_snapshots` and
  `max_age_days` after `collect` (#555)
* **cli:** `cost history view` warns when a range mixes currencies and charts
  the currency with the most snapshots. `--currency` keeps one currency.
  `--strict` fails instead of dropping the others (#556)
* **cli:** `config validate` reports syntax, budget rules, and unknown fields
  with line numbers, hints, and a close-name suggestion. `--file` selects a
  document and `--output json` prints the report. Cost commands stop in
  pre-run when a present config file is invalid. Flat `cost.budgets.amount`
  is warned and not applied; use `cost.budgets.global.amount`. Period stays
  monthly, thresholds stay 0–1000, and amount 0 still disables a scope (#223)
* **cli:** `--no-color`, `--plain`, `--color`, and `--high-contrast` on
  `cost projected`, `cost actual`, `cost recommendations`, and `overview`.
  `FINFOCUS_PLAIN`, `NO_COLOR`, `FORCE_COLOR`, and `FINFOCUS_HIGH_CONTRAST`
  fill flags that were not set. `--plain` and `--no-color` select plain text
  and win over the color flags. `--force-color` remains an alias of `--color`.
  High contrast uses ANSI 46, 226, 196, and 231 on the budget box. Plain
  budget status labels are `[OK]` and `[WARNING]` (#224)
* **engine:** optional in-memory LRU in front of the BoltDB cost cache. Off
  unless `cost.cache.lru_enabled` or `FINFOCUS_CACHE_LRU_ENABLED` is set.
  `lru_max_items` and `FINFOCUS_CACHE_LRU_MAX_ITEMS` default to 256. Writes
  go to disk first, and disk stays the source of truth (#495)
* **cli:** `cost history collect` stores projected costs for each successful
  Pulumi checkpoint, `view` charts them offline, and `list` shows the
  databases (#549)
* **engine:** `cost projected` prices the plan as a diff. Creates and resources
  with no operation show a `$0` before, deletes show a `$0` after, and
  unchanged resources are priced once. `summary.totalMonthly` stays the after
  total. Table output is the diff, and JSON adds `finfocus.diff` plus
  per-resource `operation`, `beforeMonthly`, and `deltaMonthly` (#576)
* **tui:** `cost estimate --interactive` shows billing modes, pricing tiers,
  assumptions, and usage hints from plugin `GetPricingSpec`. A missing spec
  leaves the estimate editable, and the lookup is cached per resource type
  for the session (#637)
* **engine:** price a projected resource from plugin `GetPricingSpec` before
  local YAML when `cost projected --pricing-spec-fallback` or
  `cost.pricing_spec_fallback` is set. The fallback stays off unless one of
  those is set (#638)
* **overview:** add a Warn column to the plain table and the TUI, and a
  `warnings` array on JSON and NDJSON rows. Derived values are `drift`,
  `error`, and `new`. The TUI shows `name+N` when the list does not fit (#643)
* **engine:** pass referenced resource region and SKU to plugins as `ref.*` tags
  resolved from Pulumi `propertyDependencies` (#1610)
* **engine:** emit nested resource inputs as dotted tag keys and include that
  map in the projected-cost cache key (#1608)
* **cli:** add `--show-breakdown` component sub-rows to `cost projected` and
  `cost actual` tables, and `--show-confidence` as an actual-cost table column.
  JSON and NDJSON ignore both flags (#685)

### Fixed

* **kubernetes:** allocate each node by its cluster and its name, so
  same-named nodes in different clusters keep their own cost. The priceable
  id stays the Kubernetes node name. A cluster tag on the priced node selects
  that cluster's usage (#1588)
* **cli:** `cost history export --format csv` and `--format ndjson` run through
  `ax.Execute`. Those values are export documents, so the command keeps them
  before the agent-mode check, which still accepts only `json` and `human` (#551)

### Documentation

* **overview:** document every overview flag, the status-aware Delta column,
  TUI keys including `p`, and a plain-text sample (#646)

## [0.4.0](https://github.com/rshade/finfocus/compare/v0.3.9...v0.4.0) (2026-10-03)


### ⚠ BREAKING CHANGES

* **cli:** input and validation errors now exit 2 with error_code validation_error instead of exit 1 with internal_error.

### Added

* **cli:** add overview --force-color and --no-color ([#1602](https://github.com/rshade/finfocus/issues/1602)) ([210da00](https://github.com/rshade/finfocus/commit/210da0051472d7a3df7049bebd9a4dd58a380ba8)), closes [#641](https://github.com/rshade/finfocus/issues/641)
* **cli:** confirm overview pricing before enrichment ([#1601](https://github.com/rshade/finfocus/issues/1601)) ([3059e3e](https://github.com/rshade/finfocus/commit/3059e3e91a1bacfcf00150e6908293c00d25ec4e)), closes [#642](https://github.com/rshade/finfocus/issues/642)
* **cli:** generate golangci-lint config ([#1603](https://github.com/rshade/finfocus/issues/1603)) ([083fa8f](https://github.com/rshade/finfocus/commit/083fa8f3687aea058c09dd49543ce266ad29b6a0)), closes [#493](https://github.com/rshade/finfocus/issues/493)
* **cli:** implement finfocus cost cluster (SP3) ([#1594](https://github.com/rshade/finfocus/issues/1594)) ([203333d](https://github.com/rshade/finfocus/commit/203333dcae034d8fd43f3e46a5a246410c778838))
* **engine:** surface plugin Supports() decline reasons in placeholder results ([#1579](https://github.com/rshade/finfocus/issues/1579)) ([fd7b257](https://github.com/rshade/finfocus/commit/fd7b25715a6645686eb78af16913834ca23b9918)), closes [#1515](https://github.com/rshade/finfocus/issues/1515)
* **kubernetes:** link workloads to Pulumi URNs ([#1606](https://github.com/rshade/finfocus/issues/1606)) ([0ecdecf](https://github.com/rshade/finfocus/commit/0ecdecfd517a95f5aabd8e611ea738d7b70f622e)), closes [#1527](https://github.com/rshade/finfocus/issues/1527)
* **kubernetes:** price EKS Fargate pods ([#1605](https://github.com/rshade/finfocus/issues/1605)) ([6a46b29](https://github.com/rshade/finfocus/commit/6a46b296cd5195e99dd41820bbb800414b07dccb)), closes [#1532](https://github.com/rshade/finfocus/issues/1532)
* **kubernetes:** share idle and system workload cost ([#1604](https://github.com/rshade/finfocus/issues/1604)) ([9bbe55b](https://github.com/rshade/finfocus/commit/9bbe55b2d8e1ce356b05aeaba2791f201b7ac59a)), closes [#1533](https://github.com/rshade/finfocus/issues/1533)
* **registry:** add jev plugin registry entry ([#1585](https://github.com/rshade/finfocus/issues/1585)) ([b0afa14](https://github.com/rshade/finfocus/commit/b0afa14713fd8a5fa55dda1ea261e47fda2792b0)), closes [#1577](https://github.com/rshade/finfocus/issues/1577)
* **registry:** add kubernetes plugin entry and real Installer.Update test ([#1578](https://github.com/rshade/finfocus/issues/1578)) ([37267b8](https://github.com/rshade/finfocus/commit/37267b8418b53fb8bed2da7e2e9203aa1ef7eadc)), closes [#1534](https://github.com/rshade/finfocus/issues/1534)
* **skills:** add finfocus-budget skill ([#1607](https://github.com/rshade/finfocus/issues/1607)) ([f5a125e](https://github.com/rshade/finfocus/commit/f5a125ed9ff7555733451770a62d2a8b44c0649c)), closes [#914](https://github.com/rshade/finfocus/issues/914)


### Fixed

* **cli:** surface terraform-state gaps and use validation exit codes ([#1596](https://github.com/rshade/finfocus/issues/1596)) ([b862265](https://github.com/rshade/finfocus/commit/b862265a4d878faea43a8dd94f76470190649a1b)), closes [#1506](https://github.com/rshade/finfocus/issues/1506)
* **engine:** truncate decline reasons on UTF-8 boundaries ([#1599](https://github.com/rshade/finfocus/issues/1599)) ([18085f8](https://github.com/rshade/finfocus/commit/18085f847be170d5985e511f77a0f129c7ef3c82)), closes [#1589](https://github.com/rshade/finfocus/issues/1589)
* **plugins/jev:** stop sending recommendation ids and correct Jev docs ([#1593](https://github.com/rshade/finfocus/issues/1593)) ([4cf2f27](https://github.com/rshade/finfocus/commit/4cf2f270576d9a3c4db5a8f700475dc61d8361f2))
* **scoring:** carry action_detail and reasons into scorer requests ([#1583](https://github.com/rshade/finfocus/issues/1583)) ([d1fd730](https://github.com/rshade/finfocus/commit/d1fd730eba198309a17e11725c3fd1d2838204f9)), closes [#1574](https://github.com/rshade/finfocus/issues/1574)


### Changed

* apply intrange and modernize rules to non-test code ([#1591](https://github.com/rshade/finfocus/issues/1591)) ([32d20f5](https://github.com/rshade/finfocus/commit/32d20f533a534bb4da770d9c9538cfd1bcee4766)), closes [#1208](https://github.com/rshade/finfocus/issues/1208)


### Documentation

* **mcp:** remove dead link to the retired finfocus-mcp repo ([#1595](https://github.com/rshade/finfocus/issues/1595)) ([a569c25](https://github.com/rshade/finfocus/commit/a569c257352019a7d9dfb05a7718fca1794d6f42))
* **plugins:** add Jev scorer walkthrough with real output ([#1600](https://github.com/rshade/finfocus/issues/1600)) ([379d2f5](https://github.com/rshade/finfocus/commit/379d2f573bc8c8833849820c01767d1c01cf4475))
* **specs:** convert Kubernetes cost-allocation superpowers docs to Spec Kit ([#1580](https://github.com/rshade/finfocus/issues/1580)) ([a228c7f](https://github.com/rshade/finfocus/commit/a228c7f1f1e88e6dbaa8cc494e74f56118e24ad7)), closes [#1523](https://github.com/rshade/finfocus/issues/1523)

## [0.3.9](https://github.com/rshade/finfocus/compare/v0.3.8...v0.3.9) (2026-09-30)


### Added

* **plugins/jev:** add Jev-backed RecommendationScorerService plugin ([#1573](https://github.com/rshade/finfocus/issues/1573)) ([97822ef](https://github.com/rshade/finfocus/commit/97822ef1a6390bc5054affb721355f310dd102ed)), closes [#1570](https://github.com/rshade/finfocus/issues/1570)
* **recommendations:** retain full records, fix cache key, add optional scoring ([#1572](https://github.com/rshade/finfocus/issues/1572)) ([a0c036c](https://github.com/rshade/finfocus/commit/a0c036c86f8a51c5ad54e1037de9d2843cd514e7)), closes [#1569](https://github.com/rshade/finfocus/issues/1569)


### Fixed

* **ci:** run the plugin asset script through bash ([9fe84e6](https://github.com/rshade/finfocus/commit/9fe84e699e54b5784833d76964ef240c01fa8de2))
* **deps:** update go dependencies ([#1553](https://github.com/rshade/finfocus/issues/1553)) ([a6c9458](https://github.com/rshade/finfocus/commit/a6c945836fb2ddec8b911562132fbea71ce9fa6b))
* **lint:** resolve govet shadow and testifylint float-compare findings ([#1571](https://github.com/rshade/finfocus/issues/1571)) ([769abff](https://github.com/rshade/finfocus/commit/769abff1cc090d1df8fd7188a0443fb6dcfc69e4))


### Changed

* normalize domain constants flagged by goconst ([#1204](https://github.com/rshade/finfocus/issues/1204)) ([#1559](https://github.com/rshade/finfocus/issues/1559)) ([c697792](https://github.com/rshade/finfocus/commit/c697792d8f89558a86882235de50ad1e091ab374))
* **test:** migrate mock plugin gRPC helpers off deprecated DialContext ([#1213](https://github.com/rshade/finfocus/issues/1213)) ([#1561](https://github.com/rshade/finfocus/issues/1561)) ([02d478b](https://github.com/rshade/finfocus/commit/02d478b5ad1756a34fb8e867119f77b176a20327))
* **test:** split high-complexity tests flagged by gocognit ([#1199](https://github.com/rshade/finfocus/issues/1199)) ([#1568](https://github.com/rshade/finfocus/issues/1568)) ([61de48e](https://github.com/rshade/finfocus/commit/61de48eaaf99064bd95415b6da6245098033f03a))


### Documentation

* add stdlib doc links for godoclint ([#1211](https://github.com/rshade/finfocus/issues/1211)) ([0e7fdc3](https://github.com/rshade/finfocus/commit/0e7fdc347d0d37fb222d29ab34c328e5552f6359))
* committing roadmap 0928 ([80e8a91](https://github.com/rshade/finfocus/commit/80e8a91936bae865a64e5bda236d776ec79cdc9d))

## [0.3.8](https://github.com/rshade/finfocus/compare/v0.3.7...v0.3.8) (2026-09-28)


### Added

* **registry:** install and update monorepo plugins by tag prefix ([c588699](https://github.com/rshade/finfocus/commit/c588699c2724ec4926694f870ffcb3da3a513157))


### Fixed

* **engine:** send provider, region, and SKU in plugin Supports checks ([952c01f](https://github.com/rshade/finfocus/commit/952c01f22f8e7062b2b9a0288a6a0d867902b62f))
* **release:** bootstrap the kubernetes component's release history ([4c75d49](https://github.com/rshade/finfocus/commit/4c75d4904bd284220070a54330221ea89aca3c4e))


### Documentation

* **plans:** add Kubernetes cost allocation design, plans, and issue drafts ([162268d](https://github.com/rshade/finfocus/commit/162268d14554a73d624d7405347b9d9b9b861619))
* **roadmap:** add Kubernetes cost allocation follow-ups ([e1497cb](https://github.com/rshade/finfocus/commit/e1497cbcd357be4d189d7f9609b84f4d7b99982c))

## [0.3.7](https://github.com/rshade/finfocus/compare/v0.3.6...v0.3.7) (2026-09-23)


### Added

* add --terraform-state flag to cost actual ([eed9bb5](https://github.com/rshade/finfocus/commit/eed9bb57d02930ba9a793c4e060a63c5152b826e))
* add --terraform-state flag to cost projected ([832765a](https://github.com/rshade/finfocus/commit/832765a2afc103cbdc2aabceaffd1899731e41ae))
* add resolve_types cache bucket for type resolution results ([dfe4271](https://github.com/rshade/finfocus/commit/dfe4271a291352327bad19c4ea59be09b7a453f0))
* add terraform resource mapper ([ded0444](https://github.com/rshade/finfocus/commit/ded0444eb3d3bb4eeebae6c265db455e9682b22d))
* add terraform state ingestion (--terraform-state) ([a4ea717](https://github.com/rshade/finfocus/commit/a4ea717c15d5034ccf6eac78c8e63d7dedad9f80))
* add terraform state v4 parser ([e96dec4](https://github.com/rshade/finfocus/commit/e96dec4dd482da934fa70e3ecc5cd7920a4862c0))
* **cli:** add generation-control flags to plugin init ([f91684c](https://github.com/rshade/finfocus/commit/f91684ce9beec07d9a3e7d05824e99ebed781653)), closes [#461](https://github.com/rshade/finfocus/issues/461)
* **cli:** add GetPluginInfo and Supports to plugin init calculator template ([c7e3ff9](https://github.com/rshade/finfocus/commit/c7e3ff9903b3aecef5a9375490871b5fe88c3d63)), closes [#458](https://github.com/rshade/finfocus/issues/458)
* **cli:** add health endpoint to generated plugin main.go ([e7522ec](https://github.com/rshade/finfocus/commit/e7522ecd2410911b683c1d2dd6581f56aa159ad8)), closes [#459](https://github.com/rshade/finfocus/issues/459)
* **cli:** enhance generated plugin Makefile targets ([3ffb85d](https://github.com/rshade/finfocus/commit/3ffb85dfaa9d17fc81e67993e37a305a8a1eff95)), closes [#460](https://github.com/rshade/finfocus/issues/460)
* **cli:** generate Docker support files in plugin init ([#1476](https://github.com/rshade/finfocus/issues/1476)) ([25e04ff](https://github.com/rshade/finfocus/commit/25e04ff0f19eee0699614a5a571869a2a8f3d094)), closes [#456](https://github.com/rshade/finfocus/issues/456)
* **cli:** generate docs templates in plugin init ([f0cade2](https://github.com/rshade/finfocus/commit/f0cade20eea3fd1328302243f9c603a8ad3e8b28)), closes [#457](https://github.com/rshade/finfocus/issues/457)
* **cli:** generate standardized GitHub workflows in plugin init ([07dbcfa](https://github.com/rshade/finfocus/commit/07dbcfa261a5ac43d62eebc9c8066d7645f44319)), closes [#462](https://github.com/rshade/finfocus/issues/462)
* **cli:** plugin init generation flags, health endpoint, docs, workflows ([698b669](https://github.com/rshade/finfocus/commit/698b669496b9375a11abcb727890f116927d326b))
* **engine:** cap per-chunk batch timeout by parent deadline ([65c0317](https://github.com/rshade/finfocus/commit/65c0317460d294ed88028b56e9267b1398613acc)), closes [#979](https://github.com/rshade/finfocus/issues/979)
* expose ResolveResourceTypes on the internal cost source client ([6da7f60](https://github.com/rshade/finfocus/commit/6da7f60357697240b3ea58e1a47e315b6634ea40))
* extract provider from terraform-style type strings ([80cfa3c](https://github.com/rshade/finfocus/commit/80cfa3ce11880d1a8848356dcbdc0eb1d8cf59a6))
* resolve terraform resource types via plugins with cache and fallback ([4eef66d](https://github.com/rshade/finfocus/commit/4eef66dde557e33aac2b276409d83d8c118e04a6))


### Fixed

* add ResolveResourceTypes to CostSourceClient test mocks ([1b835e8](https://github.com/rshade/finfocus/commit/1b835e880217de720696a49fb4f6211c4d4cfc87))
* apply terraform type resolution after filtering in cost actual ([e0479ce](https://github.com/rshade/finfocus/commit/e0479cef50de4495479998d0bb05ac05bef7cae8))
* **cli,pulumi:** cli-history tags group ([#956](https://github.com/rshade/finfocus/issues/956)-[#961](https://github.com/rshade/finfocus/issues/961)) ([19c871e](https://github.com/rshade/finfocus/commit/19c871ec084c8cfb78f038c680699b88237f8d7e))
* **cli:** address CodeRabbit review findings and unblock red CI on main ([#1469](https://github.com/rshade/finfocus/issues/1469)) ([d1b67c9](https://github.com/rshade/finfocus/commit/d1b67c94c52d329da888432eacfea4e6224aa0a4))
* clone properties map when applying terraform property mappings ([8de98a9](https://github.com/rshade/finfocus/commit/8de98a932facdf6371f480dabc4040e6b29ca143))
* **deps:** pin google.golang.org/grpc to v1.83.2 for GO-2026-6443 ([1140a03](https://github.com/rshade/finfocus/commit/1140a0311a28a8bacf7fa8de1cf82b065fe6b330)), closes [#1506](https://github.com/rshade/finfocus/issues/1506)
* **deps:** update dependency @astrojs/starlight to ^0.42.0 ([#1496](https://github.com/rshade/finfocus/issues/1496)) ([190c5b1](https://github.com/rshade/finfocus/commit/190c5b132f06b0a8564332b75b46c45a0724418d))
* **deps:** update dependency mermaid to v12 ([#1504](https://github.com/rshade/finfocus/issues/1504)) ([97f2c02](https://github.com/rshade/finfocus/commit/97f2c02ac6751e1da736cc21cfb6198c6b9f35dc))
* **deps:** update dependency sharp to ^0.35.0 ([#1497](https://github.com/rshade/finfocus/issues/1497)) ([3c8a3f1](https://github.com/rshade/finfocus/commit/3c8a3f11ace0398a7acb89ea084953837e366d3e))
* **deps:** update go dependencies ([#1498](https://github.com/rshade/finfocus/issues/1498)) ([558ea22](https://github.com/rshade/finfocus/commit/558ea22e02520576c9c45d89deeeaa2470c42218))
* **engine:** batch cost mapper fields, re-chunked tail, per-chunk timeout ([dc2468e](https://github.com/rshade/finfocus/commit/dc2468e7358fb8afa028e8a47f23512bc4b9e0e8))
* **engine:** group terraform resource types by provider prefix ([90fe595](https://github.com/rshade/finfocus/commit/90fe5952043a1e3a9f2eb5ec0cbbaefff09b18ca)), closes [#1506](https://github.com/rshade/finfocus/issues/1506)
* **engine:** populate rate fields in batch actual cost mapper ([321f343](https://github.com/rshade/finfocus/commit/321f343e424e958d76c111e062b85634b42576ec)), closes [#974](https://github.com/rshade/finfocus/issues/974)
* **engine:** process re-chunked tail in executeBatchForPlugin ([b262b37](https://github.com/rshade/finfocus/commit/b262b37b2e8de1b9d72d9fab012ba9d11d112ff4)), closes [#978](https://github.com/rshade/finfocus/issues/978)
* **history:** harden stack detection, retention config, and batch timeouts ([#1474](https://github.com/rshade/finfocus/issues/1474)) ([e5fe5c4](https://github.com/rshade/finfocus/commit/e5fe5c46c1248902c30b113b2caa1d2b9d118c40))
* **history:** history store fixes — tag timestamp merge, retention regression test ([e068159](https://github.com/rshade/finfocus/commit/e06815958a187c92b7b296786f3fdfb0941e0b93))
* **history:** merge tag timestamps instead of overwriting in upsertTags ([d2d4588](https://github.com/rshade/finfocus/commit/d2d4588821ce445988cd7fb2498c983c0eb64a6f)), closes [#964](https://github.com/rshade/finfocus/issues/964)
* honor property_mappings for camelCased terraform properties ([3237d8d](https://github.com/rshade/finfocus/commit/3237d8d449435808dc4aba48946c58683fa50c3d))
* **pluginhost:** stop plugins outliving Core and holding inherited pipes ([#1231](https://github.com/rshade/finfocus/issues/1231)) ([#1478](https://github.com/rshade/finfocus/issues/1478)) ([e28b6d0](https://github.com/rshade/finfocus/commit/e28b6d0c40c9cea4bcc6033cc4b00669ba16f774))
* **pulumi:** only ignore missing-file errors in GetProjectName ([1c0c54f](https://github.com/rshade/finfocus/commit/1c0c54f09ee90863b25b56d146412ba05ee64984)), closes [#961](https://github.com/rshade/finfocus/issues/961)
* reimplement plugin installer lock for Windows reliability ([487bec4](https://github.com/rshade/finfocus/commit/487bec4fbe0e121a9e9a0fcf9929f8043e138037))
* reimplement plugin installer lock for Windows reliability ([91f4b64](https://github.com/rshade/finfocus/commit/91f4b6482b587b7aec6fae19e8edfe00db19e398)), closes [#573](https://github.com/rshade/finfocus/issues/573)
* resolve golangci-lint findings in terraform state ingestion code ([d6f66e0](https://github.com/rshade/finfocus/commit/d6f66e007a844a38a3256342364822afb1372c9f))
* resolve nightly test failures from 2026-06-08 run ([fcf7b27](https://github.com/rshade/finfocus/commit/fcf7b278b362aaa3a10ef6a9b3efc50386816302))
* resolve nightly test failures from 2026-06-08 run ([58926be](https://github.com/rshade/finfocus/commit/58926be7682de186501292816fc28ab74fa07d68)), closes [#1247](https://github.com/rshade/finfocus/issues/1247)


### Changed

* clean up terraform state loading and type resolution ([4c23da3](https://github.com/rshade/finfocus/commit/4c23da3f1bb6ee782b74f327a5a0eeb3e32c9fd3))


### Documentation

* document --terraform-state flag and retire superseded tf design docs ([7ffa281](https://github.com/rshade/finfocus/commit/7ffa281e1a4eceb00345afca07384971823fefc1))
* document ActualCostData and ResourceError protobuf messages ([e2287f3](https://github.com/rshade/finfocus/commit/e2287f361801bbaee3bb69022c8e38942a06e09d))
* document ActualCostData and ResourceError protobuf messages ([6ce7434](https://github.com/rshade/finfocus/commit/6ce7434749e6fc8e4d8fba1f1d69b5f4bcd98438)), closes [#977](https://github.com/rshade/finfocus/issues/977)
* fix markdownlint MD032 in finfocus-spec-changes.md ([18883a7](https://github.com/rshade/finfocus/commit/18883a7414350d1035f6bd5258fb5ef1e017d5d5))

## [0.3.6](https://github.com/rshade/finfocus/compare/v0.3.5...v0.3.6) (2026-07-23)


### Added

* **engine:** add per-resource validation to batch cost requests ([#983](https://github.com/rshade/finfocus/issues/983)) ([ee2d1b2](https://github.com/rshade/finfocus/commit/ee2d1b2dadfa734e16e3556008a04304dacafcdf))
* **engine:** add resource history store with BoltDB persistence ([#942](https://github.com/rshade/finfocus/issues/942)) ([82b1c75](https://github.com/rshade/finfocus/commit/82b1c75db3156e537f732efe8cdcbc7450843206))
* **engine:** implement BatchCost RPC consumer for multi-resource que… ([#969](https://github.com/rshade/finfocus/issues/969)) ([176b470](https://github.com/rshade/finfocus/commit/176b4707caf850bb0842eb9557f5723ef7192a96))
* **engine:** implement EstimateCost RPC consumer ([#941](https://github.com/rshade/finfocus/issues/941)) ([e5ce12f](https://github.com/rshade/finfocus/commit/e5ce12fd47e987ddb312000c3705659ea6f0dd63)), closes [#847](https://github.com/rshade/finfocus/issues/847)


### Fixed

* adding logo-readme.png ([#945](https://github.com/rshade/finfocus/issues/945)) ([c24123e](https://github.com/rshade/finfocus/commit/c24123e9ba94aa65fa47d973e43a530c55fa2ad7))
* **ci:** stabilize chronically failing nightly test suite ([#1234](https://github.com/rshade/finfocus/issues/1234)) ([c135893](https://github.com/rshade/finfocus/commit/c1358937d564e73f9ffefaa4a20b947e453e3681)), closes [#1227](https://github.com/rshade/finfocus/issues/1227)
* **deps:** update go dependencies ([#1034](https://github.com/rshade/finfocus/issues/1034)) ([fb78b4c](https://github.com/rshade/finfocus/commit/fb78b4c3227e825930d75b2d239c6d8c4eeea594))
* **deps:** update go dependencies ([#1324](https://github.com/rshade/finfocus/issues/1324)) ([f2ee805](https://github.com/rshade/finfocus/commit/f2ee8050717dcf86c9bf6705d34ad5a09b301de6))
* **deps:** update go dependencies ([#1343](https://github.com/rshade/finfocus/issues/1343)) ([05fb450](https://github.com/rshade/finfocus/commit/05fb45047fbda3c5802b4249fb07e48763585a3e))
* **deps:** update go dependencies ([#946](https://github.com/rshade/finfocus/issues/946)) ([b70ee13](https://github.com/rshade/finfocus/commit/b70ee133332096cdbd527d895572043a9650b050))
* **deps:** update module golang.org/x/term to v0.42.0 ([#1006](https://github.com/rshade/finfocus/issues/1006)) ([90df589](https://github.com/rshade/finfocus/commit/90df589144e70985518de0774838995fae63f77c))
* **docs:** render homepage cards as Starlight components ([#952](https://github.com/rshade/finfocus/issues/952)) ([235c45d](https://github.com/rshade/finfocus/commit/235c45d4f951e1f8310edc7be87be75ed04a3467))
* **engine:** preserve original property types in mergePropertiesWithOverrides ([#989](https://github.com/rshade/finfocus/issues/989)) ([e3383e6](https://github.com/rshade/finfocus/commit/e3383e6c6be608682b402766c5c7ec8e2cd36867)), closes [#971](https://github.com/rshade/finfocus/issues/971)
* **history:** capture analyzer resource properties and extract tags i… ([#981](https://github.com/rshade/finfocus/issues/981)) ([fb48776](https://github.com/rshade/finfocus/commit/fb48776dde4462c4f1d7132f3d351613b79a7c07))
* **history:** fix double-prefixing in StackContext.Hash ([#988](https://github.com/rshade/finfocus/issues/988)) ([022c8b8](https://github.com/rshade/finfocus/commit/022c8b890d3c7e75dc073b6f751ad1bcb353c298)), closes [#966](https://github.com/rshade/finfocus/issues/966)


### Documentation

* amend constitution to v1.7.0 (optional persistence model) ([#936](https://github.com/rshade/finfocus/issues/936)) ([764cf4d](https://github.com/rshade/finfocus/commit/764cf4da48bf1a36f26f74811bf1e1fbc164e09d))
* **contributing:** document 11 agentic workflows added in [#1037](https://github.com/rshade/finfocus/issues/1037) ([#1043](https://github.com/rshade/finfocus/issues/1043)) ([de6a18d](https://github.com/rshade/finfocus/commit/de6a18dbdcf60f7cd9a2287e9a885ae4626b65d4))
* **site:** migrate documentation from jekyll to astro starlight ([#944](https://github.com/rshade/finfocus/issues/944)) ([64926ab](https://github.com/rshade/finfocus/commit/64926ab2282051502bf554044f491765abd3ee0f))
* sync roadmap and update E2E guide for 2026-06-08 ([#1253](https://github.com/rshade/finfocus/issues/1253)) ([b04a9e2](https://github.com/rshade/finfocus/commit/b04a9e2312226726735c51c418b112f2dd83916a))
* update Docker Go build image version to 1.26.3 ([#1117](https://github.com/rshade/finfocus/issues/1117)) ([afec367](https://github.com/rshade/finfocus/commit/afec367746f5fc260c086e43f104642699cf623e))
* update Go toolchain version to 1.26.3 in examples ([#1114](https://github.com/rshade/finfocus/issues/1114)) ([79b6b43](https://github.com/rshade/finfocus/commit/79b6b43aef57f97b150199dd03da0ae4ba59b823))
* update sitemap ([#1295](https://github.com/rshade/finfocus/issues/1295)) ([3cc9421](https://github.com/rshade/finfocus/commit/3cc9421e3b746543140d990592ffdcbb7472152d))

## [0.3.5](https://github.com/rshade/finfocus/compare/v0.3.4...v0.3.5) (2026-03-31)


### Added

* **skills:** add agent skills for install, analyzer-setup, and routing ([#915](https://github.com/rshade/finfocus/issues/915)) ([7ac01d4](https://github.com/rshade/finfocus/commit/7ac01d48a83b329f7d1e21a51581ecdc3aa5b198)), closes [#909](https://github.com/rshade/finfocus/issues/909) [#910](https://github.com/rshade/finfocus/issues/910) [#912](https://github.com/rshade/finfocus/issues/912)


### Fixed

* **deps:** update go dependencies ([#927](https://github.com/rshade/finfocus/issues/927)) ([0ffa540](https://github.com/rshade/finfocus/commit/0ffa540219fcb9c68f693d1da9194cc2cc5078f2))


### Performance

* **cli:** add --state-only flag to skip pulumi preview ([#933](https://github.com/rshade/finfocus/issues/933)) ([2c27c39](https://github.com/rshade/finfocus/commit/2c27c39db2b52d3e73d3d8e4b8f5c2ec72ba9053)), closes [#690](https://github.com/rshade/finfocus/issues/690)


### Documentation

* clarify budget status visibility in overview output modes ([#932](https://github.com/rshade/finfocus/issues/932)) ([6fc93b3](https://github.com/rshade/finfocus/commit/6fc93b3953cccaffdb1efc7bb310f0a70ab1b53b)), closes [#855](https://github.com/rshade/finfocus/issues/855)

## [0.3.4](https://github.com/rshade/finfocus/compare/v0.3.3...v0.3.4) (2026-03-26)


### Added

* **cache:** consume expires_at caching hints from plugin cost responses ([#893](https://github.com/rshade/finfocus/issues/893)) ([b3979b8](https://github.com/rshade/finfocus/commit/b3979b8edec2a289476e6c3ead475f58b0514a7b))
* **router:** add BatchCost feature to capability routing ([#859](https://github.com/rshade/finfocus/issues/859)) ([648a11c](https://github.com/rshade/finfocus/commit/648a11c5844520feb8a0e41451c1982dd6aced24)), closes [#848](https://github.com/rshade/finfocus/issues/848)


### Fixed

* **cli:** preserve file sink on --debug and support qualified stack names ([#857](https://github.com/rshade/finfocus/issues/857)) ([c664f40](https://github.com/rshade/finfocus/commit/c664f404330abeff03634d5e15980b892e5d67a5))
* **deps:** update go dependencies ([#871](https://github.com/rshade/finfocus/issues/871)) ([07dd143](https://github.com/rshade/finfocus/commit/07dd1436956e80accc47f7812d5dfd21495bd458))
* **deps:** update go dependencies ([#884](https://github.com/rshade/finfocus/issues/884)) ([ce794c4](https://github.com/rshade/finfocus/commit/ce794c4baf6dfbc73e0b588cf6d08d9fccd98c13))


### Documentation

* update README badges, SEO metadata, and version references to v0.3.3 ([#907](https://github.com/rshade/finfocus/issues/907)) ([e9bc3c0](https://github.com/rshade/finfocus/commit/e9bc3c011fe49a895bbf1fa5bccf989f6112f30e))

## [0.3.3](https://github.com/rshade/finfocus/compare/v0.3.2...v0.3.3) (2026-03-03)


### Added

* **cli:** add config routes list and config routes test commands ([#840](https://github.com/rshade/finfocus/issues/840)) ([fa07e73](https://github.com/rshade/finfocus/commit/fa07e732cdefb27179b213b10a4f518d64f35198))
* **deps:** upgrade charmbracelet dependencies to v2 ([#843](https://github.com/rshade/finfocus/issues/843)) ([a1b8a97](https://github.com/rshade/finfocus/commit/a1b8a9758653dcd7dbab6dd1f56dc04d69e893af)), closes [#827](https://github.com/rshade/finfocus/issues/827)
* **tui:** show property changes in overview detail view ([#852](https://github.com/rshade/finfocus/issues/852)) ([ce24612](https://github.com/rshade/finfocus/commit/ce246122f810fbc910d814c51dcbd50ea8a57b6d))


### Fixed

* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.224.0 ([#838](https://github.com/rshade/finfocus/issues/838)) ([d60afce](https://github.com/rshade/finfocus/commit/d60afce5d45c19079a37a0788b5451ffd9a2d11b))
* **deps:** update module github.com/rshade/finfocus-spec to v0.5.7 ([#849](https://github.com/rshade/finfocus/issues/849)) ([e0713cd](https://github.com/rshade/finfocus/commit/e0713cdd00a2f1e859076e094c75a38d4b0f3407))

## [0.3.2](https://github.com/rshade/finfocus/compare/v0.3.1...v0.3.2) (2026-02-28)


### Added

* **analyzer:** add check command, setup and stack summary ([#819](https://github.com/rshade/finfocus/issues/819)) ([353d2fa](https://github.com/rshade/finfocus/commit/353d2fa40ea0e75704dacfa5ab988dd50bc5fa86))
* **cli:** add short flags (-s, -f, -a) to overview command ([#836](https://github.com/rshade/finfocus/issues/836)) ([486125b](https://github.com/rshade/finfocus/commit/486125b127aea628c24dc9628ac937a1abb45a96)), closes [#644](https://github.com/rshade/finfocus/issues/644)
* **cli:** wire BoltDB cache into overview command ([#798](https://github.com/rshade/finfocus/issues/798)) ([4e75cd3](https://github.com/rshade/finfocus/commit/4e75cd3d961410cd74c2c0ab48f46204aea91cc5)), closes [#745](https://github.com/rshade/finfocus/issues/745)
* **overview:** display budget status and health in overview command ([#806](https://github.com/rshade/finfocus/issues/806)) ([a2565f7](https://github.com/rshade/finfocus/commit/a2565f7e82d5f5539ac5bcb6a8a6508b8f00f684))
* **overview:** state-first loading, splash screen, context-aware root ([410df5c](https://github.com/rshade/finfocus/commit/410df5ca9865234fe372b6451e839389719d923a)), closes [#728](https://github.com/rshade/finfocus/issues/728)
* **overview:** state-first loading, splash screen, context-aware root ([#730](https://github.com/rshade/finfocus/issues/730)) ([a63ef5e](https://github.com/rshade/finfocus/commit/a63ef5e6fa3a775293bde1fb3c0c76e99ebabc74))
* **pulumi:** recognize .tsx, .jsx, and go.work as Pulumi source files ([#799](https://github.com/rshade/finfocus/issues/799)) ([4917712](https://github.com/rshade/finfocus/commit/49177128bb6c8e143c56afd140c14f04980c355a)), closes [#787](https://github.com/rshade/finfocus/issues/787)
* **tui:** make table separator line extend to terminal width ([#828](https://github.com/rshade/finfocus/issues/828)) ([cd1fa7c](https://github.com/rshade/finfocus/commit/cd1fa7c88afd81a03bca4ae81a78ad47d002ede7)), closes [#718](https://github.com/rshade/finfocus/issues/718)
* **tui:** use lipgloss styles in renderInitializingView ([#729](https://github.com/rshade/finfocus/issues/729)) ([930e53f](https://github.com/rshade/finfocus/commit/930e53f8d3844e50593758b7d4691faea434f3ce)), closes [#719](https://github.com/rshade/finfocus/issues/719)


### Fixed

* **analyzer:** eliminate duplicate ResolvePolicyPackDir call in RunChecks ([#823](https://github.com/rshade/finfocus/issues/823)) ([a12cb8a](https://github.com/rshade/finfocus/commit/a12cb8accbf72ab25a3e39d6feea0c13c2d6354d)), closes [#822](https://github.com/rshade/finfocus/issues/822)
* batch bug fixes for [#723](https://github.com/rshade/finfocus/issues/723), [#747](https://github.com/rshade/finfocus/issues/747), [#748](https://github.com/rshade/finfocus/issues/748), [#749](https://github.com/rshade/finfocus/issues/749), [#750](https://github.com/rshade/finfocus/issues/750), [#751](https://github.com/rshade/finfocus/issues/751), [#752](https://github.com/rshade/finfocus/issues/752), [#753](https://github.com/rshade/finfocus/issues/753) ([#792](https://github.com/rshade/finfocus/issues/792)) ([e62b00f](https://github.com/rshade/finfocus/commit/e62b00fd4f60409df173750911f731158b48bfdb))
* **cli:** isolate CLI tests from real ~/.finfocus config and plugins ([#816](https://github.com/rshade/finfocus/issues/816)) ([d04da71](https://github.com/rshade/finfocus/commit/d04da71a56317e917c493656e82712395ebdf3c0)), closes [#809](https://github.com/rshade/finfocus/issues/809)
* **cli:** thread passphrase via subprocess env, not process-wide os.Setenv ([#770](https://github.com/rshade/finfocus/issues/770)) ([fb30a33](https://github.com/rshade/finfocus/commit/fb30a33d6d10c58c9ee3a9204d321f5e4810c5c0)), closes [#761](https://github.com/rshade/finfocus/issues/761) [#763](https://github.com/rshade/finfocus/issues/763) [#764](https://github.com/rshade/finfocus/issues/764)
* **cli:** track enriched row count incrementally for accurate audit ([#830](https://github.com/rshade/finfocus/issues/830)) ([d33018b](https://github.com/rshade/finfocus/commit/d33018bcad398696b145b028fbbbe03c2bffdea6)), closes [#720](https://github.com/rshade/finfocus/issues/720)
* **cli:** wait for enrichment goroutine before plugin cleanup in overview ([#813](https://github.com/rshade/finfocus/issues/813)) ([178d5e2](https://github.com/rshade/finfocus/commit/178d5e217fd56b06e289cb1cb96eceec82019c15)), closes [#716](https://github.com/rshade/finfocus/issues/716)
* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.223.0 ([#779](https://github.com/rshade/finfocus/issues/779)) ([316a2f9](https://github.com/rshade/finfocus/commit/316a2f9faaf9a531699a35aa45a16c25d089738d))
* disable store on compact() reopen failure to prevent panic ([#817](https://github.com/rshade/finfocus/issues/817)) ([ef86d8e](https://github.com/rshade/finfocus/commit/ef86d8e3c74e30b57796fe356da787bb1dd3c5a5))
* **engine:** handle context.Canceled and context.DeadlineExceeded in classifyError ([#826](https://github.com/rshade/finfocus/issues/826)) ([05b30de](https://github.com/rshade/finfocus/commit/05b30dee08164cb471ce93bc65f3b22b35ee33e2)), closes [#726](https://github.com/rshade/finfocus/issues/726)
* fixing review issues ([11bcf4f](https://github.com/rshade/finfocus/commit/11bcf4f4dcee0d400dbed290ce42c36b98883e71))
* **logging:** add .Ctx(ctx) to log calls in changedetect.go ([#769](https://github.com/rshade/finfocus/issues/769)) ([ed39ee8](https://github.com/rshade/finfocus/commit/ed39ee87c6d9dc337c5ce1a62412893b8ad83023)), closes [#765](https://github.com/rshade/finfocus/issues/765)
* **overview:** correct drift extrapolation for mid-month resources ([#832](https://github.com/rshade/finfocus/issues/832)) ([fcd5530](https://github.com/rshade/finfocus/commit/fcd55307ac66451dd55948b217e8b1937186e5e7)), closes [#760](https://github.com/rshade/finfocus/issues/760)
* **recorder:** implement GetPricingSpec and EstimateCost methods ([#834](https://github.com/rshade/finfocus/issues/834)) ([1d0b889](https://github.com/rshade/finfocus/commit/1d0b889ac9559cf3848b342a741d1d31e6a90cde)), closes [#734](https://github.com/rshade/finfocus/issues/734)
* respect --yes flag when change detection fails in overview TUI ([#803](https://github.com/rshade/finfocus/issues/803)) ([9cc333d](https://github.com/rshade/finfocus/commit/9cc333dbaef5a2a96cc253b7331493133c4198d2)), closes [#762](https://github.com/rshade/finfocus/issues/762)
* review issues ([#767](https://github.com/rshade/finfocus/issues/767)) ([4763af2](https://github.com/rshade/finfocus/commit/4763af2d948d6a5bb86333b4c5d819b83832ab07))
* **test:** remove duplicate nil-map test and grant Claude push access ([#796](https://github.com/rshade/finfocus/issues/796)) ([0adeb1c](https://github.com/rshade/finfocus/commit/0adeb1c345d40ebb44bd6f2711d0987798acc53e)), closes [#789](https://github.com/rshade/finfocus/issues/789)
* **tui:** add state guards for init-only messages in overview model ([#829](https://github.com/rshade/finfocus/issues/829)) ([77105ac](https://github.com/rshade/finfocus/commit/77105ac026b9cfe17dab4c9a6eeb54496ab5d6b9))
* **tui:** widen Recs column to show N(-M) dismissed format ([6262b55](https://github.com/rshade/finfocus/commit/6262b555aff81ff7f33a3725910cc6d3aa1145c5))
* **tui:** widen Recs column to show N(-M) dismissed format ([887995b](https://github.com/rshade/finfocus/commit/887995b85824401a9c448a92a650544695f4f2a9)), closes [#766](https://github.com/rshade/finfocus/issues/766)
* use presence-based check for FINFOCUS_HIDE_ALIAS_HINT ([#802](https://github.com/rshade/finfocus/issues/802)) ([2e45dd2](https://github.com/rshade/finfocus/commit/2e45dd2b6aab94df4554d8c22aeaf57382d88d65)), closes [#783](https://github.com/rshade/finfocus/issues/783)


### Performance

* **engine:** parallelize per-row enrichment sub-calls ([#727](https://github.com/rshade/finfocus/issues/727)) ([2acbefb](https://github.com/rshade/finfocus/commit/2acbefb55eebdf1a0e03f1bc440628022c6f73c1))


### Changed

* **cli:** extract progress constant and add goroutine comment in overview ([#831](https://github.com/rshade/finfocus/issues/831)) ([276b550](https://github.com/rshade/finfocus/commit/276b550e235b0bdd36765aa80dd20061e10725f0)), closes [#721](https://github.com/rshade/finfocus/issues/721)


### Documentation

* fix incorrect analyzers: in Pulumi.yaml claim in architecture doc ([#804](https://github.com/rshade/finfocus/issues/804)) ([7c75efc](https://github.com/rshade/finfocus/commit/7c75efcfd6b660cd41943e1535b3006f4ef04207)), closes [#758](https://github.com/rshade/finfocus/issues/758)
* **routing:** document routing limits in analyzer/policy-pack mode ([#771](https://github.com/rshade/finfocus/issues/771)) ([2cc58ce](https://github.com/rshade/finfocus/commit/2cc58ce59e8a3170c9fb1ad381bb2348c684fcd6)), closes [#759](https://github.com/rshade/finfocus/issues/759)
* v0.3.0 documentation audit fixes ([#711](https://github.com/rshade/finfocus/issues/711)) ([d4a71dc](https://github.com/rshade/finfocus/commit/d4a71dc5cfcd1041e7db1f9a842cb3e56f28f57d))

## [0.3.1](https://github.com/rshade/finfocus/compare/v0.3.0...v0.3.1) (2026-02-18)


### Added

* **logging:** add PhaseTimer and instrument overview command ([#712](https://github.com/rshade/finfocus/issues/712)) ([f721cb2](https://github.com/rshade/finfocus/commit/f721cb268f4488422b2fd87c84fc7f3c3014ed13))


### Fixed

* **cache:** append cache subdirectory to global fallback path ([#700](https://github.com/rshade/finfocus/issues/700)) ([6c3006f](https://github.com/rshade/finfocus/commit/6c3006fea9fdd50c4ae4db51e2b2364b7a45ae41)), closes [#680](https://github.com/rshade/finfocus/issues/680)


### Performance

* **tui:** launch TUI immediately with phase progress feedback ([#713](https://github.com/rshade/finfocus/issues/713)) ([2344de6](https://github.com/rshade/finfocus/commit/2344de6de32b6e7216e8bf918dffbed35d10a745))

## [0.3.0](https://github.com/rshade/finfocus/compare/v0.2.6...v0.3.0) (2026-02-17)


### Added

* **analyzer:** add cost threshold enforcement and structured cost ([#676](https://github.com/rshade/finfocus/issues/676)) ([474f525](https://github.com/rshade/finfocus/commit/474f525d9aa5e6dfc9bcb4578d4eac01abe5fe85))
* **cache:** replace JSON file cache with BoltDB backend ([#677](https://github.com/rshade/finfocus/issues/677)) ([618b718](https://github.com/rshade/finfocus/commit/618b7187b1de0cb5b237954b1401229f9ba479b2))
* **ci:** add benchmark PR reporting with benchstat comparison ([#675](https://github.com/rshade/finfocus/issues/675)) ([4615cba](https://github.com/rshade/finfocus/commit/4615cbaf1390e2ae889a7bb0701a785cbb8015e1))
* **cli:** add analyzer install and uninstall commands ([#633](https://github.com/rshade/finfocus/issues/633)) ([63d7e23](https://github.com/rshade/finfocus/commit/63d7e23fa332da950f98ae70b1b5e922ca156f6a))
* **cli:** add cost estimate command for what-if analysis ([#538](https://github.com/rshade/finfocus/issues/538)) ([bce24df](https://github.com/rshade/finfocus/commit/bce24df43166fd0cfb0aba671a0693db366b5d7b)), closes [#463](https://github.com/rshade/finfocus/issues/463)
* **cli:** add install script for one-command binary installation ([#668](https://github.com/rshade/finfocus/issues/668)) ([66c739a](https://github.com/rshade/finfocus/commit/66c739ac79c877eb5ef3d4c5d6ea7c9bb9fbf05e))
* **cli:** add recommendation dismissal and lifecycle management ([#557](https://github.com/rshade/finfocus/issues/557)) ([04e4f1a](https://github.com/rshade/finfocus/commit/04e4f1aa0981fcd6188e14309cd72c2a45a1d61c)), closes [#464](https://github.com/rshade/finfocus/issues/464)
* **cli:** add setup command for one-command bootstrap ([#650](https://github.com/rshade/finfocus/issues/650)) ([0b0e8e8](https://github.com/rshade/finfocus/commit/0b0e8e837727a301c949bbea2d47875717ead2a5))
* **cli:** add structured errors, semantic exit codes, and plugin lis… ([#647](https://github.com/rshade/finfocus/issues/647)) ([5c94e50](https://github.com/rshade/finfocus/commit/5c94e50492baece7aa91b04fa586c8465c391b8f))
* **cli:** add unified cost overview dashboard ([#509](https://github.com/rshade/finfocus/issues/509)) ([#584](https://github.com/rshade/finfocus/issues/584)) ([bccbc9d](https://github.com/rshade/finfocus/commit/bccbc9da8b5ecaa2c14b456e7b9c268b42386438))
* **cli:** automatic Pulumi project detection for cost commands ([#586](https://github.com/rshade/finfocus/issues/586)) ([2a6db87](https://github.com/rshade/finfocus/commit/2a6db873a1b8f58518a0b52bff12eb82030214f7))
* **cli:** wire router into commands for region-aware plugin selection ([#632](https://github.com/rshade/finfocus/issues/632)) ([e696591](https://github.com/rshade/finfocus/commit/e6965913a2605078ec68c9de6c97d8034166f5c2))
* **config:** split project-local and user-global .finfocus directories ([#651](https://github.com/rshade/finfocus/issues/651)) ([d29f7f9](https://github.com/rshade/finfocus/commit/d29f7f9002e591ec5408672262e5d4f824fa6f5a))
* **engine:** add tag-based filtering to BudgetFilterOptions ([#535](https://github.com/rshade/finfocus/issues/535)) ([085b689](https://github.com/rshade/finfocus/commit/085b689c7d95d602899f16d4cecc1211cf2a13f8)), closes [#532](https://github.com/rshade/finfocus/issues/532)
* **engine:** reliability and quality fixes  ([#661](https://github.com/rshade/finfocus/issues/661)) ([c3abedf](https://github.com/rshade/finfocus/commit/c3abedffdab676df4b5cec28a1a1d1f2c9d8b84e))
* **engine:** unified caching ([#660](https://github.com/rshade/finfocus/issues/660)) ([5ee1299](https://github.com/rshade/finfocus/commit/5ee12992de44d821339feee9625dae1990b16b6a))
* **registry:** add SHA256 checksum verification for plugin install ([#673](https://github.com/rshade/finfocus/issues/673)) ([b094a80](https://github.com/rshade/finfocus/commit/b094a802eb4da5a063f7d91ad77a5b2874078249))
* **router:** filter internal Pulumi resources from cost plugin routing ([#648](https://github.com/rshade/finfocus/issues/648)) ([879e8cb](https://github.com/rshade/finfocus/commit/879e8cbecffa93797e69d6a7584a06fde31f7805))
* **router:** support GCP zone normalization in normalizeToRegion ([#631](https://github.com/rshade/finfocus/issues/631)) ([3c5f69a](https://github.com/rshade/finfocus/commit/3c5f69a701a09428f97235623a902fb816a17433)), closes [#615](https://github.com/rshade/finfocus/issues/615)
* **tui:** display recommendations in resource detail view ([#585](https://github.com/rshade/finfocus/issues/585)) ([a57fcd9](https://github.com/rshade/finfocus/commit/a57fcd9eaa829fee2b66313c6502f90ac346ebe5))


### Fixed

* **ci:** grant write permissions to Claude workflow tokens ([#571](https://github.com/rshade/finfocus/issues/571)) ([427d1e4](https://github.com/rshade/finfocus/commit/427d1e4998cc5b3dc0856e942819f99466a4aba7))
* **deps:** update go dependencies ([#566](https://github.com/rshade/finfocus/issues/566)) ([e783168](https://github.com/rshade/finfocus/commit/e783168adad9e28da408d14fc456452d4b14835f))
* **deps:** update go dependencies ([#626](https://github.com/rshade/finfocus/issues/626)) ([500ced2](https://github.com/rshade/finfocus/commit/500ced2f34e079906f485ba8024bf01dac5c4b24))
* **deps:** update module github.com/charmbracelet/bubbles to v1 ([#627](https://github.com/rshade/finfocus/issues/627)) ([fc17976](https://github.com/rshade/finfocus/commit/fc179765791867ec997c3a3b2c76147c05a6ada2))
* **ingest:** pass cloud resource IDs and ARNs to plugins for actual cost lookup ([#574](https://github.com/rshade/finfocus/issues/574)) ([3bdc6ff](https://github.com/rshade/finfocus/commit/3bdc6ff3a24b0db0fb65142e6558c760df95230a)), closes [#380](https://github.com/rshade/finfocus/issues/380)
* **logging:** auto-create log directory before opening log file ([#618](https://github.com/rshade/finfocus/issues/618)) ([8b8717e](https://github.com/rshade/finfocus/commit/8b8717ea39968acd232fe04ecfc0fe24ece5d2ff)), closes [#591](https://github.com/rshade/finfocus/issues/591)
* **proto:** deep copy CostBreakdown to prevent source mutation ([#622](https://github.com/rshade/finfocus/issues/622)) ([ce45c21](https://github.com/rshade/finfocus/commit/ce45c2192dab54e94ca7c3ba245d702f7e6e7712)), closes [#614](https://github.com/rshade/finfocus/issues/614)
* **proto:** skip phantom $0 results from empty plugin responses ([#623](https://github.com/rshade/finfocus/issues/623)) ([862ead5](https://github.com/rshade/finfocus/commit/862ead53f40eb312319f47023a5ec87815594753)), closes [#593](https://github.com/rshade/finfocus/issues/593) [#595](https://github.com/rshade/finfocus/issues/595)
* **recorder:** remove ACTUAL_COSTS capability and add Supports() override ([#628](https://github.com/rshade/finfocus/issues/628)) ([d2a8b81](https://github.com/rshade/finfocus/commit/d2a8b818be2275bc4f4f956a0dd6ef1574733908)), closes [#594](https://github.com/rshade/finfocus/issues/594) [#596](https://github.com/rshade/finfocus/issues/596)
* **registry:** fall back to filesystem discovery for plugin removal ([#621](https://github.com/rshade/finfocus/issues/621)) ([156bbde](https://github.com/rshade/finfocus/commit/156bbdef7da1877e3b427369ff76fa3c2dd60f1b)), closes [#592](https://github.com/rshade/finfocus/issues/592)


### Changed

* **cli:** wrap bare error returns with descriptive context ([#634](https://github.com/rshade/finfocus/issues/634)) ([ec1c6a7](https://github.com/rshade/finfocus/commit/ec1c6a7e4023a7127a71be47df7de6efc15bcf32)), closes [#609](https://github.com/rshade/finfocus/issues/609)
* **core:** coderabbit follow-up cleanup from pulumi auto-detect PR ([#619](https://github.com/rshade/finfocus/issues/619)) ([ce1ec73](https://github.com/rshade/finfocus/commit/ce1ec73ddcfc4aaa2c829a2512e36b5bb176cc18))


### Chores

* release 0.3.0 ([#697](https://github.com/rshade/finfocus/issues/697)) ([360b061](https://github.com/rshade/finfocus/commit/360b06170fb44cc4383faf0888a65cef8e6ee41a))

## [0.2.6](https://github.com/rshade/finfocus/compare/v0.2.5...v0.2.6) (2026-02-02)


### Added

* **cli:** add flexible budget scoping (per-provider, per-type, per-tag) ([#509](https://github.com/rshade/finfocus/issues/509)) ([54b6680](https://github.com/rshade/finfocus/commit/54b6680506e087a3cd4809bd17be16e612ef7d94))
* **greenops:** add carbon emission equivalency calculations ([#515](https://github.com/rshade/finfocus/issues/515)) ([0b70143](https://github.com/rshade/finfocus/commit/0b70143e7e20b7f19a041bc09f671dcbc552f777))
* **router:** add intelligent multi-plugin routing for cost calculations ([#507](https://github.com/rshade/finfocus/issues/507)) ([3510f92](https://github.com/rshade/finfocus/commit/3510f92c10a5a27b6b0aa5e8ddb3b64fa587331c))


### Fixed

* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.218.0 ([#530](https://github.com/rshade/finfocus/issues/530)) ([dd653f8](https://github.com/rshade/finfocus/commit/dd653f8d4b436ae1b5b2c41007ece13e1e557547))


### Documentation

* updating readme and relevant documentation for new functions ([#524](https://github.com/rshade/finfocus/issues/524)) ([bda0f35](https://github.com/rshade/finfocus/commit/bda0f35a5d16b762658ba2ee777d5dfc064e0aa1))

## [0.2.5](https://github.com/rshade/finfocus/compare/v0.2.4...v0.2.5) (2026-01-30)


### Added

* **cli:** add budget threshold exit codes for CI/CD integration ([#496](https://github.com/rshade/finfocus/issues/496)) ([a5883ea](https://github.com/rshade/finfocus/commit/a5883ea6bf65673606e09aa045f6f06794fefdf1)), closes [#219](https://github.com/rshade/finfocus/issues/219)
* **cli:** add pagination and NDJSON streaming for CI/CD integration ([#488](https://github.com/rshade/finfocus/issues/488)) ([7026346](https://github.com/rshade/finfocus/commit/7026346cab6db708817b1450593113c9c9ebac8c)), closes [#122](https://github.com/rshade/finfocus/issues/122)
* **engine:** add budget health suite with status tracking, forecasting, and thresholds ([#494](https://github.com/rshade/finfocus/issues/494)) ([6c09cc4](https://github.com/rshade/finfocus/commit/6c09cc44ee2bfc5bb54f80e565f2b62da689f12a)), closes [#263](https://github.com/rshade/finfocus/issues/263) [#267](https://github.com/rshade/finfocus/issues/267)


### Fixed

* **deps:** update module github.com/pulumi/pulumi/sdk/v3 to v3.217.0 ([#500](https://github.com/rshade/finfocus/issues/500)) ([ee3bfca](https://github.com/rshade/finfocus/commit/ee3bfcaec88d28d9acce44a2e1c26ea9a0aab3e0))
* **deps:** update module github.com/rshade/finfocus-spec to v0.5.4 ([#477](https://github.com/rshade/finfocus/issues/477)) ([4b2424c](https://github.com/rshade/finfocus/commit/4b2424c02666c48e33105d3019fcbb115108d238))


### Changed

* add ConvertToProto and ConvertValueToString helpers for gRPC plugin communication ([#520](https://github.com/rshade/finfocus/issues/520)) ([5aaefc4](https://github.com/rshade/finfocus/commit/5aaefc42202846544b413a1fab6d62e8c16a7cd9))

## [0.2.4](https://github.com/rshade/finfocus/compare/v0.2.3...v0.2.4) (2026-01-21)


### Added

* **cli:** add budget status display with threshold alerts ([#466](https://github.com/rshade/finfocus/issues/466)) ([c7fee8b](https://github.com/rshade/finfocus/commit/c7fee8bd9951856e2d2ecd26b4d3cd1d9062a966))
* **cli:** complete plugin init with recorded fixtures ([#470](https://github.com/rshade/finfocus/issues/470)) ([dfa62fb](https://github.com/rshade/finfocus/commit/dfa62fb53acacfa15ee5c1defae076286f648a0e))


### Documentation

* **tui:** add budget, recommendations, and accessibility guides ([#472](https://github.com/rshade/finfocus/issues/472)) ([7d34d80](https://github.com/rshade/finfocus/commit/7d34d805e4d9f9c1b56c49d2313e1f823f6f3e27)), closes [#226](https://github.com/rshade/finfocus/issues/226) [#468](https://github.com/rshade/finfocus/issues/468) [#469](https://github.com/rshade/finfocus/issues/469)

## [0.2.3](https://github.com/rshade/finfocus/compare/v0.2.2...v0.2.3) (2026-01-19)


### Added

* **cli:** add version fallback for plugin install command ([#439](https://github.com/rshade/finfocus/issues/439)) ([29ae341](https://github.com/rshade/finfocus/commit/29ae341acfbe146117fa43644a403e6bd98eafaa)), closes [#430](https://github.com/rshade/finfocus/issues/430)
* **engine:** implement budget filtering and summary aggregation logic ([#446](https://github.com/rshade/finfocus/issues/446)) ([39ea80c](https://github.com/rshade/finfocus/commit/39ea80c5dee176986e97dee558c1a4e87fde9108))


### Fixed

* **registry:** make GitHub API tests platform-agnostic ([#453](https://github.com/rshade/finfocus/issues/453)) ([d8eac33](https://github.com/rshade/finfocus/commit/d8eac33ba963b002f72923dc9b31574d27eaf723)), closes [#452](https://github.com/rshade/finfocus/issues/452)


### Documentation

* **cli:** document --estimate-confidence flag for cost actual command ([a2684ae](https://github.com/rshade/finfocus/commit/a2684ae6fe931e273e9cbb8041349ef3b280bd14)), closes [#333](https://github.com/rshade/finfocus/issues/333)
* **core:** update documentation for E2E testing and plugin ecosystem ([#454](https://github.com/rshade/finfocus/issues/454)) ([ee8d893](https://github.com/rshade/finfocus/commit/ee8d89328a5c169a6305f1e7afe6eeca49ac2b13))
* **deployment:** expand deployment, security, config, troubleshooting, and support guides ([#441](https://github.com/rshade/finfocus/issues/441)) ([6edb8ef](https://github.com/rshade/finfocus/commit/6edb8efc4e6cb73dfe67ec6332231af8286ff1fe)), closes [#349](https://github.com/rshade/finfocus/issues/349) [#350](https://github.com/rshade/finfocus/issues/350) [#351](https://github.com/rshade/finfocus/issues/351) [#352](https://github.com/rshade/finfocus/issues/352) [#353](https://github.com/rshade/finfocus/issues/353)

## [0.2.2](https://github.com/rshade/finfocus/compare/v0.2.1...v0.2.2) (2026-01-18)


### Added

* **cli:** implement v0.2.1 developer experience improvements ([#426](https://github.com/rshade/finfocus/issues/426)) ([6de19ee](https://github.com/rshade/finfocus/commit/6de19ee1b938300c56eb58a5d7826ac3d970f13a)), closes [#115](https://github.com/rshade/finfocus/issues/115)


### Fixed

* **registry:** resolve Windows test failures and add plugin robustness improvements ([#436](https://github.com/rshade/finfocus/issues/436)) ([3338686](https://github.com/rshade/finfocus/commit/3338686c43ed469d273f7a1e1dc95478385b68b2))

## [0.2.1](https://github.com/rshade/finfocus/compare/v0.2.0...v0.2.1) (2026-01-17)


### Fixed

* **cli:** resolve plugin mode detection and date validation issues ([#418](https://github.com/rshade/finfocus/issues/418)) ([f3da648](https://github.com/rshade/finfocus/commit/f3da64825ae4dddc881ab2fba817f35da8716e46)), closes [#114](https://github.com/rshade/finfocus/issues/114)
* **test:** align JSON output tests with finfocus wrapper pattern ([#425](https://github.com/rshade/finfocus/issues/425)) ([9ac9dc2](https://github.com/rshade/finfocus/commit/9ac9dc2b03625e349ffd0405b93c1115530ff870)), closes [#424](https://github.com/rshade/finfocus/issues/424) [#417](https://github.com/rshade/finfocus/issues/417) [#414](https://github.com/rshade/finfocus/issues/414)

## [0.2.0](https://github.com/rshade/finfocus/compare/v0.1.4...v0.2.0) (2026-01-15)


### Added

* **plugin:** implement info and dry-run discovery ([#398](https://github.com/rshade/finfocus/issues/398)) ([a768d4a](https://github.com/rshade/finfocus/commit/a768d4aa0ac26aa4b10918aedfe2670cd29f1afc)), closes [#401](https://github.com/rshade/finfocus/issues/401)


### Chores

* release 0.2.0 ([#416](https://github.com/rshade/finfocus/issues/416)) ([d151885](https://github.com/rshade/finfocus/commit/d1518857008257c1f32af6766ba467896f1ddaa2))

## [0.1.4](https://github.com/rshade/finfocus/compare/v0.1.3...v0.1.4) (2026-01-10)


### Added

* **cli:** add cost recommendations command with action type filtering ([#375](https://github.com/rshade/finfocus/issues/375)) ([1d32dca](https://github.com/rshade/finfocus/commit/1d32dca6b19b5191a341d740093e26520f36328a)), closes [#298](https://github.com/rshade/finfocus/issues/298)
* **cli:** add Pulumi tool plugin mode support ([#379](https://github.com/rshade/finfocus/issues/379)) ([62bf5c7](https://github.com/rshade/finfocus/commit/62bf5c7b5ec02f4bbd2d0c4bbec97af56655e26e)), closes [#246](https://github.com/rshade/finfocus/issues/246)
* **cli:** add state-based actual cost estimation with confidence levels ([#382](https://github.com/rshade/finfocus/issues/382)) ([80f8c28](https://github.com/rshade/finfocus/commit/80f8c28164da9671cb62cf7b1efb6c2e96626211)), closes [#380](https://github.com/rshade/finfocus/issues/380)
* **cli:** enhance cost recommendations with TUI and summary mode ([#377](https://github.com/rshade/finfocus/issues/377)) ([4c900cb](https://github.com/rshade/finfocus/commit/4c900cb1e1835ad89bd25e34c404fd7bfbe61dc8)), closes [#216](https://github.com/rshade/finfocus/issues/216)
* **proto:** add pre-flight request validation using pluginsdk ([#372](https://github.com/rshade/finfocus/issues/372)) ([e53f2d6](https://github.com/rshade/finfocus/commit/e53f2d6a09496603ae2f5bac9d623c1537419083)), closes [#233](https://github.com/rshade/finfocus/issues/233)
* **registry:** auto-select latest plugin version ([#391](https://github.com/rshade/finfocus/issues/391)) ([48c4fa3](https://github.com/rshade/finfocus/commit/48c4fa36722eaaf16750ecc3c08c364fce199390))
* **tui:** add interactive cost display with Bubble Tea ([#345](https://github.com/rshade/finfocus/issues/345)) ([de8645c](https://github.com/rshade/finfocus/commit/de8645c543dc354a881f8df3b52a6ae14198cf33)), closes [#106](https://github.com/rshade/finfocus/issues/106)


### Fixed

* **deps:** update go dependencies ([#355](https://github.com/rshade/finfocus/issues/355)) ([f2694d8](https://github.com/rshade/finfocus/commit/f2694d8eef7d4f4bce5db0bc6360c7ae0d0739c8))
* **deps:** update go dependencies ([#388](https://github.com/rshade/finfocus/issues/388)) ([d893f98](https://github.com/rshade/finfocus/commit/d893f98075f88e918bcabb56c85fc9cfd74c513f))


### Documentation

* fixing markdownlint issues ([#381](https://github.com/rshade/finfocus/issues/381)) ([11e21bc](https://github.com/rshade/finfocus/commit/11e21bcb8de8062cd6bf1de08f178fbbe030d717))
* update roadmap and README for completed milestones ([#373](https://github.com/rshade/finfocus/issues/373)) ([2c8f16b](https://github.com/rshade/finfocus/commit/2c8f16b9ff48e81b776040966adb1087bc7592dc)), closes [#320](https://github.com/rshade/finfocus/issues/320)
* updating roadmap and fixing links ([#363](https://github.com/rshade/finfocus/issues/363)) ([98da1c2](https://github.com/rshade/finfocus/commit/98da1c2a3675e89e58ecbc6c27b5ca441288c908))
* updating roadmap and fixing links ([#363](https://github.com/rshade/finfocus/issues/363)) ([8e5395b](https://github.com/rshade/finfocus/commit/8e5395b75033a7c3518f577b995fb77fd57373e4))

## [0.1.3](https://github.com/rshade/finfocus/compare/v0.1.2...v0.1.3) (2025-12-27)


### Added

* add integration tests for --filter flag across cost commands ([#300](https://github.com/rshade/finfocus/issues/300)) ([efcebf6](https://github.com/rshade/finfocus/commit/efcebf60efb48f1f57704a24b738478fa8393518)), closes [#249](https://github.com/rshade/finfocus/issues/249)
* **analyzer:** add ResourceID passthrough for recommendation correlation ([#347](https://github.com/rshade/finfocus/issues/347)) ([680b80a](https://github.com/rshade/finfocus/commit/680b80af73acc657dac79d6bf012a7bf0b3af35b)), closes [#106](https://github.com/rshade/finfocus/issues/106)
* **analyzer:** implement Pulumi Analyzer plugin for zero-click cost estimation ([#229](https://github.com/rshade/finfocus/issues/229)) ([2070b05](https://github.com/rshade/finfocus/commit/2070b05513f6e9ae2580930c02abed8fec3fe790))
* **ci:** add automated nightly failure analysis workflow ([#297](https://github.com/rshade/finfocus/issues/297)) ([ab7c516](https://github.com/rshade/finfocus/commit/ab7c516a8b269f578ba309c68d1dd291ef2d00ef)), closes [#271](https://github.com/rshade/finfocus/issues/271)
* **conformance:** add plugin conformance testing framework ([#215](https://github.com/rshade/finfocus/issues/215)) ([c37cc22](https://github.com/rshade/finfocus/commit/c37cc2283919b4ba4ff736f15f42db7c18297fc5)), closes [#201](https://github.com/rshade/finfocus/issues/201)
* **e2e:** implement E2E testing framework with Pulumi Automation API ([#238](https://github.com/rshade/finfocus/issues/238)) ([ee23ff2](https://github.com/rshade/finfocus/commit/ee23ff2b19b348086e83969457c6927a787b96ac)), closes [#177](https://github.com/rshade/finfocus/issues/177)
* implement CLI filter flag with validation and integration tests ([#332](https://github.com/rshade/finfocus/issues/332)) ([b358566](https://github.com/rshade/finfocus/commit/b3585665e7192b74d6bebfaf3fe5be13c8e8d5e6))
* implement sustainability metrics and finalize plugin sdk mapping ([#315](https://github.com/rshade/finfocus/issues/315)) ([f207c53](https://github.com/rshade/finfocus/commit/f207c534fcdd4c64b5498a459529da6a19eec1fa))
* **plugin:** add reference recorder plugin for request capture and mock responses ([#293](https://github.com/rshade/finfocus/issues/293)) ([733c2f9](https://github.com/rshade/finfocus/commit/733c2f969952718ecde99ea9a8b5a64c74b6ac58))
* **tui:** add shared TUI package with Bubble Tea/Lip Gloss components ([#258](https://github.com/rshade/finfocus/issues/258)) ([e049460](https://github.com/rshade/finfocus/commit/e049460e4ccd5545f456ecf9d2051a6f0bac94f9))
* **tui:** add Spinner and Table components from bubbles library ([#341](https://github.com/rshade/finfocus/issues/341)) ([992db5a](https://github.com/rshade/finfocus/commit/992db5ab4ef20cdce6e1f5d6c1def7382ff03628))


### Fixed

* **deps:** update go dependencies ([#281](https://github.com/rshade/finfocus/issues/281)) ([73364d6](https://github.com/rshade/finfocus/commit/73364d66cf1d53512867cf203689998dcc9b3af6))
* **deps:** update go dependencies ([#314](https://github.com/rshade/finfocus/issues/314)) ([c09f298](https://github.com/rshade/finfocus/commit/c09f298281c8b7e18d47fe086dd6fb5d921fd571))
* **deps:** update module github.com/rshade/finfocus-spec to v0.4.3 ([#211](https://github.com/rshade/finfocus/issues/211)) ([4cb56d9](https://github.com/rshade/finfocus/commit/4cb56d928ab0b5887fd2fc56c182383d9eedfffe))
* **deps:** update module github.com/spf13/cobra to v1.10.2 ([#240](https://github.com/rshade/finfocus/issues/240)) ([ad3bfd7](https://github.com/rshade/finfocus/commit/ad3bfd7b92d189a912dbae3ae10bbda2067e6bf2))
* update Go version to 1.25.6 and improve plugin integration tests ([#244](https://github.com/rshade/finfocus/issues/244)) ([4f383df](https://github.com/rshade/finfocus/commit/4f383df0df1e1d4d3d23259adef8eb29d6ea41e9))


### Changed

* **pluginhost:** remove PORT env var, use --port flag only ([#295](https://github.com/rshade/finfocus/issues/295)) ([46bcdf2](https://github.com/rshade/finfocus/commit/46bcdf24b718e6f43f0d8f5cf3092d79ac35f8ec))
* **pluginsdk:** adopt pluginsdk environment variable constants ([#272](https://github.com/rshade/finfocus/issues/272)) ([8c6e616](https://github.com/rshade/finfocus/commit/8c6e616bcc33bcd79a599d9a31b218e4aa67c34c)), closes [#230](https://github.com/rshade/finfocus/issues/230)


### Documentation

* **all:** synchronize documentation with codebase features ([#257](https://github.com/rshade/finfocus/issues/257)) ([5881cdc](https://github.com/rshade/finfocus/commit/5881cdcbbd27705d35de3de285411ebcabe4b602)), closes [#256](https://github.com/rshade/finfocus/issues/256)

## [0.1.2](https://github.com/rshade/finfocus/compare/v0.1.1...v0.1.2) (2025-12-03)


### Added

* **logging:** integrate zerolog logging across all components ([#206](https://github.com/rshade/finfocus/issues/206)) ([c152d05](https://github.com/rshade/finfocus/commit/c152d0537c394ffd4a0f07554ec12116cb5dc4a2))


### Fixed

* comprehensive input validation and error handling improvements ([#196](https://github.com/rshade/finfocus/issues/196)) ([47b0e36](https://github.com/rshade/finfocus/commit/47b0e369db86f6268a5e9d0aba87ae5f77773379))
* **deps:** update module github.com/masterminds/semver/v3 to v3.4.0 ([#199](https://github.com/rshade/finfocus/issues/199)) ([be86a7e](https://github.com/rshade/finfocus/commit/be86a7ef047d938b4a2c87ad7fff8f727be693ee))
* **pluginhost:** prevent race condition in plugin port allocation ([#192](https://github.com/rshade/finfocus/issues/192)) ([42c4a0a](https://github.com/rshade/finfocus/commit/42c4a0a488a0aa3f528579640e49ba77c3198d71))

## [0.1.1](https://github.com/rshade/finfocus/compare/v0.1.0...v0.1.1) (2025-11-29)


### Added

* **pluginsdk:** add UnaryInterceptors support to ServeConfig ([#191](https://github.com/rshade/finfocus/issues/191)) ([e05757a](https://github.com/rshade/finfocus/commit/e05757ad914d0299387cb6a1377ad5d99c843653))


### Changed

* **core:** use pluginsdk from finfocus-spec ([#189](https://github.com/rshade/finfocus/issues/189)) ([23ae52e](https://github.com/rshade/finfocus/commit/23ae52e4669ba900f6e829d45c63dfb3000cdee7))

## [0.1.0](https://github.com/rshade/finfocus/compare/v0.0.1...v0.1.0) (2025-11-26)


### ⚠ BREAKING CHANGES

* remove encryption from config, use environment variables for secrets ([#149](https://github.com/rshade/finfocus/issues/149))

### Added

* adding in testing ([#155](https://github.com/rshade/finfocus/issues/155)) ([4680d9c](https://github.com/rshade/finfocus/commit/4680d9c9aab57cd8df749dd6f1518805533420a6))
* **cli:** implement plugin install/update/remove commands ([#171](https://github.com/rshade/finfocus/issues/171)) ([c93f761](https://github.com/rshade/finfocus/commit/c93f761e5181830f5b58a6790e7241358999b43e))
* complete actual cost pipeline with cross-provider aggregation t… ([#52](https://github.com/rshade/finfocus/issues/52)) ([c0b032f](https://github.com/rshade/finfocus/commit/c0b032f78531a267b4db155c2f38c35f46c4c3b2))
* complete CLI skeleton implementation with missing flags and tests ([#15](https://github.com/rshade/finfocus/issues/15)) ([994a859](https://github.com/rshade/finfocus/commit/994a859283c1736ee204c3cce745f421ef405927)), closes [#3](https://github.com/rshade/finfocus/issues/3)
* complete plugin development SDK and template system ([#54](https://github.com/rshade/finfocus/issues/54)) ([bee3dec](https://github.com/rshade/finfocus/commit/bee3dec866b9b7f37f686cfa2da10e2bbfa2699b))
* **engine,cli:** implement comprehensive error aggregation system ([#174](https://github.com/rshade/finfocus/issues/174)) ([cc31cb5](https://github.com/rshade/finfocus/commit/cc31cb54fd07d71d6df2117114a07bba200ab962))
* **engine:** implement projected cost pipeline with enhanced spec fa… ([#31](https://github.com/rshade/finfocus/issues/31)) ([2408b47](https://github.com/rshade/finfocus/commit/2408b472154b7b9d92ee09dcbe0fe128557da1a9))
* implement comprehensive actual cost pipeline with aggregation and filtering ([#36](https://github.com/rshade/finfocus/issues/36)) ([db18307](https://github.com/rshade/finfocus/commit/db18307c1ed992ee6a09417341b78bfd43b6e333))
* implement comprehensive CI/CD pipeline setup ([#20](https://github.com/rshade/finfocus/issues/20)) ([71d4a70](https://github.com/rshade/finfocus/commit/71d4a70a083a043529f8ee01ace28284e7a48d0b)), closes [#11](https://github.com/rshade/finfocus/issues/11)
* implement comprehensive configuration management system ([#37](https://github.com/rshade/finfocus/issues/37)) ([4a21a0c](https://github.com/rshade/finfocus/commit/4a21a0cf1a9c815768e90eebb831d61107554fa0))
* implement comprehensive configuration management system ([#38](https://github.com/rshade/finfocus/issues/38)) ([a06d03b](https://github.com/rshade/finfocus/commit/a06d03b4ad0f122a9d9e4967e9562add0a59c03f))
* implement comprehensive logging and error handling infrastructure ([#59](https://github.com/rshade/finfocus/issues/59)) ([615daaf](https://github.com/rshade/finfocus/commit/615daaf7bf3f1ec45b7b83603c2a70cc3d7f7ac1)), closes [#10](https://github.com/rshade/finfocus/issues/10)
* implement comprehensive testing framework and strategy ([#58](https://github.com/rshade/finfocus/issues/58)) ([c8451af](https://github.com/rshade/finfocus/commit/c8451af5f8a57b901aa15bf2287d8cf6e695a4f4))
* integrate real proto definitions from finfocus-spec ([247fd5b](https://github.com/rshade/finfocus/commit/247fd5b96e850669e4277519b367048dcb23d3e2))
* **logging:** implement zerolog distributed tracing with debug mode ([#184](https://github.com/rshade/finfocus/issues/184)) ([4be8b26](https://github.com/rshade/finfocus/commit/4be8b26290e2b9eb182082770f78f7db7f31adb9))
* **pluginsdk:** implement Supports() gRPC handler ([#165](https://github.com/rshade/finfocus/issues/165)) ([2034a52](https://github.com/rshade/finfocus/commit/2034a52f6cd8d160bfdfcbe0d94b4a9cca5020ba))


### Fixed

* add index.md for GitHub Pages landing page and fix workflow validation ([#96](https://github.com/rshade/finfocus/issues/96)) ([609e4e2](https://github.com/rshade/finfocus/commit/609e4e2df7c7b51639b21abd2f5f10081658773c))
* add proper CSS styling and layout improvements for GitHub Pages ([#107](https://github.com/rshade/finfocus/issues/107)) ([242b3d0](https://github.com/rshade/finfocus/commit/242b3d06d0138c86a827b2dc8a3edc687b5d72bb))
* add proper CSS styling and layout improvements for GitHub Pages ([#143](https://github.com/rshade/finfocus/issues/143)) ([de35bac](https://github.com/rshade/finfocus/commit/de35bacf1537c5029e8dfd0a18ca2fa6e79a887f))
* **deps:** update github.com/rshade/finfocus-spec digest to 1130a00 ([#39](https://github.com/rshade/finfocus/issues/39)) ([16112bc](https://github.com/rshade/finfocus/commit/16112bca7bb78716bd1ac4da9c323fabf10c9774))
* **deps:** update github.com/rshade/finfocus-spec digest to 241cb09 ([#32](https://github.com/rshade/finfocus/issues/32)) ([39a83d8](https://github.com/rshade/finfocus/commit/39a83d8b877be68e2cccacd51e7cc564a8abe69f))
* **deps:** update github.com/rshade/finfocus-spec digest to 35b5694 ([#79](https://github.com/rshade/finfocus/issues/79)) ([8d03c3e](https://github.com/rshade/finfocus/commit/8d03c3e2b4d7ffe26428ce1ee5012d3e2c508cb9))
* **deps:** update github.com/rshade/finfocus-spec digest to 5825eaa ([#60](https://github.com/rshade/finfocus/issues/60)) ([3bdc514](https://github.com/rshade/finfocus/commit/3bdc5144141bb05430979fd69614bbcde998cde4))
* **deps:** update github.com/rshade/finfocus-spec digest to 79d1a15 ([#53](https://github.com/rshade/finfocus/issues/53)) ([e9f4add](https://github.com/rshade/finfocus/commit/e9f4add667a4ef4ca26abb724fbfb5dc831530bc))
* **deps:** update github.com/rshade/finfocus-spec digest to a085bd2 ([#25](https://github.com/rshade/finfocus/issues/25)) ([bbf4974](https://github.com/rshade/finfocus/commit/bbf4974e6a18dc956c8e8b25a9ed95cc3203bea2))
* **deps:** update github.com/rshade/finfocus-spec digest to d9f31a6 ([#16](https://github.com/rshade/finfocus/issues/16)) ([644ba4e](https://github.com/rshade/finfocus/commit/644ba4ec5dec924a386a0a0e8613335860ed4e80))
* **deps:** update github.com/rshade/finfocus-spec digest to e3ffb28 ([#67](https://github.com/rshade/finfocus/issues/67)) ([0135b43](https://github.com/rshade/finfocus/commit/0135b4395c4e8fa98e2ed69d3c48ecb8080805a6))
* **deps:** update go dependencies ([#159](https://github.com/rshade/finfocus/issues/159)) ([b2ad29f](https://github.com/rshade/finfocus/commit/b2ad29fff1ef33a2428a851b02e043f235ea0dad))
* **deps:** update go dependencies ([#33](https://github.com/rshade/finfocus/issues/33)) ([e54dcb3](https://github.com/rshade/finfocus/commit/e54dcb39d08beeb16cbd484d547abd88037c7443))
* **deps:** update go dependencies ([#40](https://github.com/rshade/finfocus/issues/40)) ([e59e319](https://github.com/rshade/finfocus/commit/e59e319cb6b620daecbd786174b98c5004613dc3))
* **deps:** update go dependencies ([#49](https://github.com/rshade/finfocus/issues/49)) ([8b99267](https://github.com/rshade/finfocus/commit/8b99267eb48d6a6f0cbf79d6d84e82b34b1025ff))
* **deps:** update module github.com/rshade/finfocus-spec to v0.2.0 ([#167](https://github.com/rshade/finfocus/issues/167)) ([b6c9271](https://github.com/rshade/finfocus/commit/b6c92712fc62c90a476e937d4c1dc90882229eaf))
* **deps:** update module github.com/spf13/cobra to v1.9.1 ([#17](https://github.com/rshade/finfocus/issues/17)) ([2e0e8aa](https://github.com/rshade/finfocus/commit/2e0e8aaf7633dfb32e44ab999845bce595be7827))
* **deps:** update module google.golang.org/protobuf to v1.36.10 ([#61](https://github.com/rshade/finfocus/issues/61)) ([5dd8cae](https://github.com/rshade/finfocus/commit/5dd8cae604c72d646afe2adc61d3589b3ace763e))


### Changed

* remove encryption from config, use environment variables for secrets ([#149](https://github.com/rshade/finfocus/issues/149)) ([2e3a07b](https://github.com/rshade/finfocus/commit/2e3a07b6d122ef37e0cff9b9a3d025855b92881b)), closes [#99](https://github.com/rshade/finfocus/issues/99)


### Documentation

* complete Vantage plugin documentation ([#145](https://github.com/rshade/finfocus/issues/145)) ([06e6cd7](https://github.com/rshade/finfocus/commit/06e6cd70a9328bde6d6d736146fe16b088aa1f6d)), closes [#103](https://github.com/rshade/finfocus/issues/103)
* first pass at github pages ([#88](https://github.com/rshade/finfocus/issues/88)) ([ceee2f3](https://github.com/rshade/finfocus/commit/ceee2f3fb632f0d1c8960bb36fce1e111988efd3))
* ratify constitution v1.0.0 (establish governance principles) ([#152](https://github.com/rshade/finfocus/issues/152)) ([d40ac0f](https://github.com/rshade/finfocus/commit/d40ac0fab2707b1acf7a0e2ba0db87e424f4afbe))
* update constitution for docstrings ([#176](https://github.com/rshade/finfocus/issues/176)) ([5053db5](https://github.com/rshade/finfocus/commit/5053db5865b6ecf6e2ec430181a7c9445b47cdab))

## [Unreleased]

### BREAKING CHANGES

- **Removed encryption functionality from config package**: The built-in encryption system using PBKDF2 has been completely removed due to security concerns about weak key derivation. Users should now store sensitive values (API keys, credentials) as environment variables instead of in configuration files. This is the industry-standard approach for CLI tools and follows best practices for secret management.
  - Removed `EncryptValue()` and `DecryptValue()` methods from Config
  - Removed `--encrypt` flag from `finfocus config set` command
  - Removed `--decrypt` flag from `finfocus config get` command
  - Removed all encryption-related infrastructure (deriveKey, master key management)

  **Migration Guide**:
  - Remove any encrypted values from your `~/.finfocus/config.yaml`
  - Store sensitive values as environment variables using the pattern: `FINFOCUS_PLUGIN_<PLUGIN_NAME>_<KEY_NAME>`
  - Example: `export FINFOCUS_PLUGIN_AWS_SECRET_KEY="your-secret"`
  - Environment variables automatically override config file values

### Changed

- Updated CLI command documentation to recommend environment variables for sensitive data
- Updated README with comprehensive configuration and environment variable documentation
- Simplified config package by removing unused encryption dependencies

### Removed

- PBKDF2-based encryption key derivation (security vulnerability)
- AES-256-GCM encryption for configuration values
- Master key file creation and management
- Encryption-related tests and validation

## [0.1.0] - 2025-01-14

### Added

- Initial release of FinFocus Core CLI
- Projected cost calculation from Pulumi plans
- Actual cost queries with time ranges and filtering
- Cross-provider cost aggregation
- Plugin-based architecture for extensibility
- Configuration management system
- Multiple output formats (table, JSON, NDJSON)
- Resource grouping and filtering capabilities
- Comprehensive testing framework
