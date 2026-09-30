# Changelog

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
