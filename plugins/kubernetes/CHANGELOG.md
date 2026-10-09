# Changelog

## [0.1.7](https://github.com/rshade/finfocus/compare/kubernetes-v0.1.6...kubernetes-v0.1.7) (2026-10-09)


### Bug Fixes

* **deps:** update golang.org/x/net to v0.60.0 and CI Go to 1.27.2 ([#1754](https://github.com/rshade/finfocus/issues/1754)) ([4383b26](https://github.com/rshade/finfocus/commit/4383b26266c0b3f3f532be9ca8f8577abd0ec0bb))

## [0.1.6](https://github.com/rshade/finfocus/compare/kubernetes-v0.1.5...kubernetes-v0.1.6) (2026-10-06)


### Features

* **cluster:** price historical windows from a Prometheus usage source ([#1717](https://github.com/rshade/finfocus/issues/1717)) ([d474f27](https://github.com/rshade/finfocus/commit/d474f27082a1345444943de078e9282f88bb5458))

## [0.1.5](https://github.com/rshade/finfocus/compare/kubernetes-v0.1.4...kubernetes-v0.1.5) (2026-10-05)


### Bug Fixes

* **deps:** update module github.com/rshade/finfocus-spec to v0.7.5 ([#1698](https://github.com/rshade/finfocus/issues/1698)) ([2223683](https://github.com/rshade/finfocus/commit/22236838eba02cc86a4736c099e0f75422d08e7a))
* **kubernetes:** decline reason names priced workloads ([#1692](https://github.com/rshade/finfocus/issues/1692)) ([58c07e3](https://github.com/rshade/finfocus/commit/58c07e3848fb55e0f29a236ecbb97a85963b1478))

## [0.1.4](https://github.com/rshade/finfocus/compare/kubernetes-v0.1.3...kubernetes-v0.1.4) (2026-10-04)


### Features

* **engine:** send the resource descriptor on actual cost requests ([#1680](https://github.com/rshade/finfocus/issues/1680)) ([13330a7](https://github.com/rshade/finfocus/commit/13330a736feeece6f2d27d839ccd2baf5e87d493))
* **kubernetes:** price workloads declared in a Pulumi plan ([3bb6c11](https://github.com/rshade/finfocus/commit/3bb6c116b19fa64c95a6281f0b1118d26f5bffbd)), closes [#1525](https://github.com/rshade/finfocus/issues/1525)


### Bug Fixes

* **deps:** update go dependencies ([#1678](https://github.com/rshade/finfocus/issues/1678)) ([02a2525](https://github.com/rshade/finfocus/commit/02a2525732f4b1ffc429325857673804d36c5f7a))

## [0.1.3](https://github.com/rshade/finfocus/compare/kubernetes-v0.1.2...kubernetes-v0.1.3) (2026-10-03)


### Bug Fixes

* **kubernetes:** keep same-named nodes separate across clusters ([e580864](https://github.com/rshade/finfocus/commit/e580864b22566d3105f8365a2b0e8bec5c193432)), closes [#1588](https://github.com/rshade/finfocus/issues/1588)

## [0.1.2](https://github.com/rshade/finfocus/compare/kubernetes-v0.1.1...kubernetes-v0.1.2) (2026-10-03)


### Features

* **cli:** implement finfocus cost cluster (SP3) ([#1594](https://github.com/rshade/finfocus/issues/1594)) ([203333d](https://github.com/rshade/finfocus/commit/203333dcae034d8fd43f3e46a5a246410c778838))
* **kubernetes:** link workloads to Pulumi URNs ([#1606](https://github.com/rshade/finfocus/issues/1606)) ([0ecdecf](https://github.com/rshade/finfocus/commit/0ecdecfd517a95f5aabd8e611ea738d7b70f622e)), closes [#1527](https://github.com/rshade/finfocus/issues/1527)
* **kubernetes:** price EKS Fargate pods ([#1605](https://github.com/rshade/finfocus/issues/1605)) ([6a46b29](https://github.com/rshade/finfocus/commit/6a46b296cd5195e99dd41820bbb800414b07dccb)), closes [#1532](https://github.com/rshade/finfocus/issues/1532)
* **kubernetes:** share idle and system workload cost ([#1604](https://github.com/rshade/finfocus/issues/1604)) ([9bbe55b](https://github.com/rshade/finfocus/commit/9bbe55b2d8e1ce356b05aeaba2791f201b7ac59a)), closes [#1533](https://github.com/rshade/finfocus/issues/1533)
* **skills:** add finfocus-budget skill ([#1607](https://github.com/rshade/finfocus/issues/1607)) ([f5a125e](https://github.com/rshade/finfocus/commit/f5a125ed9ff7555733451770a62d2a8b44c0649c)), closes [#914](https://github.com/rshade/finfocus/issues/914)


### Bug Fixes

* **plugins/kubernetes:** honor GetStats metrics filter and warn on unknown names ([#1582](https://github.com/rshade/finfocus/issues/1582)) ([6947a43](https://github.com/rshade/finfocus/commit/6947a433278a9af109084354f262ae431af95c34)), closes [#1575](https://github.com/rshade/finfocus/issues/1575)
* **plugins/kubernetes:** key allocation workloads by cluster to stop cross-cluster merges ([#1581](https://github.com/rshade/finfocus/issues/1581)) ([5301404](https://github.com/rshade/finfocus/commit/5301404ce5deac425381c18b2a6f532423175484)), closes [#1576](https://github.com/rshade/finfocus/issues/1576)

## [0.1.1](https://github.com/rshade/finfocus/compare/kubernetes-v0.1.0...kubernetes-v0.1.1) (2026-09-30)


### Features

* **plugins/jev:** add Jev-backed RecommendationScorerService plugin ([#1573](https://github.com/rshade/finfocus/issues/1573)) ([97822ef](https://github.com/rshade/finfocus/commit/97822ef1a6390bc5054affb721355f310dd102ed)), closes [#1570](https://github.com/rshade/finfocus/issues/1570)
* **recommendations:** retain full records, fix cache key, add optional scoring ([#1572](https://github.com/rshade/finfocus/issues/1572)) ([a0c036c](https://github.com/rshade/finfocus/commit/a0c036c86f8a51c5ad54e1037de9d2843cd514e7)), closes [#1569](https://github.com/rshade/finfocus/issues/1569)


### Bug Fixes

* **kubernetes:** account for pod-level spec.resources requests in EffectiveRequests ([8078a22](https://github.com/rshade/finfocus/commit/8078a224370fba94eac408a02e3c09390e89465a)), closes [#1518](https://github.com/rshade/finfocus/issues/1518)
* **kubernetes:** detect EKS control planes in the China partition ([e604a7f](https://github.com/rshade/finfocus/commit/e604a7f34cbca8e37c162639dd4908e712ad15d3)), closes [#1520](https://github.com/rshade/finfocus/issues/1520)
* **kubernetes:** emit valid Pulumi resource type tokens for GCP and Azure nodes ([#1519](https://github.com/rshade/finfocus/issues/1519)) ([b5cd21e](https://github.com/rshade/finfocus/commit/b5cd21ebf9e6187f45da01a04e58fc1937f650f6))
* **kubernetes:** map BadRequest/Invalid and in-flight context errors to precise gRPC codes ([#1517](https://github.com/rshade/finfocus/issues/1517)) ([f508fa1](https://github.com/rshade/finfocus/commit/f508fa1959b114855ab38800c64bc28b6df2132a))

## 0.1.0 (2026-09-28)


### Features

* **kubernetes:** add allocation policy and request-based allocator ([7abaea5](https://github.com/rshade/finfocus/commit/7abaea5e978796b07d73bdc47f72516be713cdb2))
* **kubernetes:** collect run-rate usage and priceable nodes from the API ([d4d0259](https://github.com/rshade/finfocus/commit/d4d0259a35847263995694a2bf590d200f978bcc))
* **kubernetes:** serve GetStats and Allocate with explicit capabilities ([5e72082](https://github.com/rshade/finfocus/commit/5e7208290202dc188858148159f8ed9655d4a22f))


### Bug Fixes

* **kubernetes:** validate label selector keys and values ([71f53a9](https://github.com/rshade/finfocus/commit/71f53a9a2c8b28f07f3bfc4b43837f9799715906)), closes [#1516](https://github.com/rshade/finfocus/issues/1516)
