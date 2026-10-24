# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [v1.1.0] - 2026-10-24

### Added

* Report Service/Endpoints instance snapshots to ai-gateway-api through the InnerAPI `PUT /inner-api/v1/k8s_pools/{name}/instances`. Add the `InnerApiClient` (full-replacement idempotent upsert, explicit `[]` for zero instances, 404-idempotent delete), k8s pool API types, and `EnsureK8sPool` / `DeleteK8sPoolByList` in the ALB provider.
* Add k8s pool naming in the form `k8s_<namespace>_<service>_<port>[_<cluster>]`, with a 64-character `prefix + 8-character hash` folding scheme for long names and validation against the `k8s_pools` name rules.
* Add a cross-component integration harness (`test/integration/harness`) that drives the real `ServiceReconciler` against a real ai-gateway-api process with a fake Kubernetes client, plus mock-based integration tests for filter, service discovery and pool discovery, with their run and clean scripts.
* Add user documentation for the InnerAPI `k8s_pools` channel, the `ai-gateway-api-name` label and flag, service discovery design, API definitions and integration test guides.

### Changed

* **Breaking** Replace the OpenAPI `product_pool` write channel with the InnerAPI `k8s_pools` channel: the controller is now the only writer of k8s pool instances, and ai-gateway-api fans the snapshot out to provider mirrors and cluster-derived pools. Existing product pools are not cleaned by the new channel and must be migrated out of band.
* **Breaking** Select Services by the `ai-gateway-api-name` label (matching `--ai-gateway-api-name`, `*` matches any) instead of `bfe-product`; the controller now fails fast at startup when `--ai-gateway-api-name` is empty.
* Track pool results in the `k8s.bfenetworks.com/k8spool-result` annotation (replacing `productpool-result`), delete stale pools on reconcile, and keep pools whose deletion failed for the next reconcile.
* Update deployment manifests, examples, README/README-CN and the usage guide for the new label, flags and InnerAPI channel.

### Removed

* **Breaking** Remove the InferencePool/Pod discovery chain and the `product_pool` client.
* **Breaking** Remove the `--enable-rs-pool`, `--enable-inference-pool` and `--bfe-product-name` flags, their options, and the `inferencepool` examples.

### Fixed

* Report an explicit empty instance snapshot when a Service has no Ready endpoints, so drained pools are cleared instead of retaining stale instances.
* Delete pools that are no longer reported (for example after a port rename or removal), and retry failed deletions on the next reconcile.
* Treat HTTP 404 on k8s pool deletion as success so cleanup is idempotent.
* Deduplicate instances by `addr:port` and report only Ready endpoints, matching the `k8s_pools` instance uniqueness rules.

## [v1.1.0-preview.1] - 2026-10-20

### Added

* Service discovery now writes instance snapshots through the ai-gateway-api InnerAPI `PUT /inner-api/v1/k8s_pools/{name}/instances`; add the `InnerApiClient` (idempotent upsert, `[]` for zero instances, 404-idempotent delete), k8s pool API types, and `EnsureK8sPool` / `DeleteK8sPoolByList` in the ALB provider.
* Add a cross-component integration harness (`test/integration/harness`) that drives the real `ServiceReconciler` against a real ai-gateway-api process with a fake Kubernetes client, plus mock-based integration tests for filter, service discovery and pool discovery, with their run and clean scripts.

### Changed

* **Breaking** Replace the OpenAPI `product_pool` write channel with the InnerAPI `k8s_pools` channel: the controller is now the only writer of k8s pool instances, and ai-gateway-api fans the snapshot out to provider mirrors and cluster-derived pools. Existing product pools are not cleaned by the new channel and must be migrated out of band.
* **Breaking** Remove the InferencePool/Pod discovery chain and the `product_pool` client; remove the `--enable-rs-pool`, `--enable-inference-pool` and `--bfe-product-name` flags and the `inferencepool` examples; add the required `--ai-gateway-api-name` flag.
* Track pool results in the `k8s.bfenetworks.com/k8spool-result` annotation (replacing `productpool-result`), delete stale pools on reconcile, and keep pools whose deletion failed for the next reconcile.
* Add k8s pool naming with a 64-character `prefix + 8-character hash` folding scheme.

## [v1.0.0] - 2026-05-06

### Added

* Add initial implementation of ai-gateway-controller

