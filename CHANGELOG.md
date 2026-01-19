# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

## [1.5.0](https://github.com/FitGlue/server/compare/v1.4.0...v1.5.0) (2026-01-19)


### Features

* change parkrun location detection logic ([871a6c0](https://github.com/FitGlue/server/commit/871a6c0ce8c81763f37d92fe05ceaf2c8ce41442))
* Introduce Hevy uploader, add a repost handler, and enhance linting for destination topic and uploader consistency. ([3c65b11](https://github.com/FitGlue/server/commit/3c65b114952e09e3481b802f0b08a7bad67f6598))
* Introduce Hevy uploader, add a repost handler, and enhance linting for destination topic and uploader consistency. ([812d4b6](https://github.com/FitGlue/server/commit/812d4b6b8c55b705e93f466cfa4fba32cab81849))
* Introduce owner display name for showcased activities, populate it via Firebase Auth, and add new Parkrun integration fields. ([d919f02](https://github.com/FitGlue/server/commit/d919f02632d5681886b77f6029da8e9037f07efc))


### Bug Fixes

* increase parkrun detection distance ([ef8d299](https://github.com/FitGlue/server/commit/ef8d299674af6d92c33895bc87538f9d1fc66881))
* parkrun locations service not unwrapping JSON correctly ([77ef6c7](https://github.com/FitGlue/server/commit/77ef6c791156f5407113f90fdc1fd53aeeb1ff47))

## [1.4.0](https://github.com/FitGlue/server/compare/v1.3.0...v1.4.0) (2026-01-18)


### Features

* improve Fitbit activity type mapping by fetching detailed activity data, enhance webhook processing with per-activity traceability, and add unit tests for activity type mapping. ([7d4a35e](https://github.com/FitGlue/server/commit/7d4a35e83d8108b135c905204f9f969067614822))
* introduce Logic Gate enricher provider for conditional pipeline halting based on activity rules. ([bd9de86](https://github.com/FitGlue/server/commit/bd9de86212ff6b1f0bdb8bfb870f7911841e49e4))
* Log virtual source executions for each processed activity to enhance tracing visibility. ([6814c86](https://github.com/FitGlue/server/commit/6814c86931ae26163724c63dabb89ef93ca39df4))
* Map "structured workout" to run activity type and add related tests. ([3b75aec](https://github.com/FitGlue/server/commit/3b75aecd049eceb01516a7d2b6c7097254530d4c))
* Replace the TypeScript file upload handler with a new Go-based FIT parser function, updating pipeline configuration and protobuf definitions. ([b752fe6](https://github.com/FitGlue/server/commit/b752fe6ce7985cfc11504760fcac4c8192d11f8c))

## [1.3.0](https://github.com/FitGlue/server/compare/v1.2.1...v1.3.0) (2026-01-18)


### Features

* Add a new showcase handler cloud function to serve public activity data and viewer redirects. ([970f350](https://github.com/FitGlue/server/commit/970f350ddffbde2eea604094855de019db49e515))
* Add ShowcaseStore for typed access to showcased activities and integrate it into the showcase handler. ([1ef538d](https://github.com/FitGlue/server/commit/1ef538d5ab330f794ac830e9a693adbe1b28847c))
* Implement a new file upload handler service for direct FIT file uploads. ([a1f84e1](https://github.com/FitGlue/server/commit/a1f84e141ee86a7f03dd4069fb37969d4d6f99a4))


### Bug Fixes

* failing tests ([bfea8d5](https://github.com/FitGlue/server/commit/bfea8d51c9866521bdff57bb3a3f87229a07e7ee))

### [1.2.1](https://github.com/FitGlue/server/compare/v1.2.0...v1.2.1) (2026-01-18)

## [1.2.0](https://github.com/FitGlue/server/compare/v1.1.1...v1.2.0) (2026-01-18)


### Features

* Enhance `mapDestinations` to accept numeric and more flexible string inputs for destinations. ([a8fb836](https://github.com/FitGlue/server/commit/a8fb836a8553ba6063db5006e548ae48e102a0ff))
* introduce `PUBLIC_ID` integration authentication type and refactor configuration handler for generic auth support. ([ee25784](https://github.com/FitGlue/server/commit/ee257843ce5eedcd973af964347fca6955e45197))

### [1.1.1](https://github.com/FitGlue/server/compare/v1.1.0...v1.1.1) (2026-01-17)


### Bug Fixes

* run create-release after deploy-prod ([a6703d1](https://github.com/FitGlue/server/commit/a6703d11a56221fad4720fd07b1c56131505083f))

## [1.1.0](https://github.com/FitGlue/server/compare/v1.0.0...v1.1.0) (2026-01-17)


### Features

* add combined version control between web and server ([ae26975](https://github.com/FitGlue/server/commit/ae26975b2ec7728bef48c64158e516684852faa8))
* expand Fitbit activity type mapping, add sync count increment for billing, and refine orchestrator pipeline handling. ([480b62b](https://github.com/FitGlue/server/commit/480b62bdd7c1dd7e9602ecf37a1ae5155e88f40b))
* introduce `showcase-uploader` function and `ShowcasedActivity` data model to enable public activity sharing. ([0a70922](https://github.com/FitGlue/server/commit/0a70922d2b88daee0a7d88f9ce8b639aa4d0eaf3))


### Bug Fixes

* allow hevy api key setup via UI ([50fc54b](https://github.com/FitGlue/server/commit/50fc54bb60d55a83d7a94622de3d91af4c9277e4))
* versioning bumping ([e9112e0](https://github.com/FitGlue/server/commit/e9112e0b89ed98dad4d56eb307c5304f2c13c960))

## 1.0.0 (2026-01-17)

This is the first proper release of FitGlue Server, consolidating all development work since project inception.

### ⚠ BREAKING CHANGES

* **auth:** implement centralized AuthorizationService and refactor handlers
* Initial setup with protobuf-based architecture

### Features

* Add Parkrun results source and destination framework, refactor Parkrun enricher and plugin system ([33243ec](https://github.com/fitglue/server/commit/33243ec))
* **auth:** implement centralized AuthorizationService and refactor handlers ([bb14ee1](https://github.com/fitglue/server/commit/bb14ee1))
* Enable fetching pipeline execution details for activities ([19f14a1](https://github.com/fitglue/server/commit/19f14a1))
* Add mobile health integrations (Apple Health, Health Connect) and billing logic ([fff7354](https://github.com/fitglue/server/commit/fff7354))
* Add transformations and use cases fields to PluginManifest proto ([6279bf9](https://github.com/fitglue/server/commit/6279bf9))
* **plugins:** add marketing metadata to plugin and integration manifests ([5097f88](https://github.com/fitglue/server/commit/5097f88))
* Add example and use case details to Volume Analytics and Muscle Heatmap enrichers ([573b7db](https://github.com/fitglue/server/commit/573b7db))
* Implement profile handler and user management APIs
* Add Strava, Fitbit, and Hevy integration handlers
* Implement orchestrator and enricher pipeline processing
* Add Firebase authentication and user profile management
* **ci:** configure OIDC authentication for GCP deployments ([7064672](https://github.com/fitglue/server/commit/7064672))
* protobuf based shared types implemented ([57083bb](https://github.com/fitglue/server/commit/57083bb))
* secrets management implemented properly ([5d0a618](https://github.com/fitglue/server/commit/5d0a618))
* one-command install and local running capability ([dadec62](https://github.com/fitglue/server/commit/dadec62))
* Initial setup with Terraform, Cloud Functions, and multi-environment support ([e48db6f](https://github.com/fitglue/server/commit/e48db6f))

### Bug Fixes

* incorrect cron defs ([e013a34](https://github.com/fitglue/server/commit/e013a34))
* add new go function to build function zips python script ([2db62de](https://github.com/fitglue/server/commit/2db62de))
* billing-handler and allowUnauthenticated calls to functions using auth strategies ([5d71918](https://github.com/fitglue/server/commit/5d71918))
* add mobile-sync-handler terraform ([8d0a499](https://github.com/fitglue/server/commit/8d0a499))
* **ci:** various CI/CD fixes for OIDC authentication and cache persistence
* protobuf generation and usage fixed across all functions ([fc74c84](https://github.com/fitglue/server/commit/fc74c84))
* all version and lint issues fixed ([1a79cf5](https://github.com/fitglue/server/commit/1a79cf5))
