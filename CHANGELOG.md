# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

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
