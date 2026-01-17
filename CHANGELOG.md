# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

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
