# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

## [9.4.0](https://github.com/FitGlue/server/compare/v9.3.1...v9.4.0) (2026-01-26)


### Features

* allow parkrun-fetcher more time to get results, add debugging to ai-banner, add skip reasons to strava-uploader ([354c80b](https://github.com/FitGlue/server/commit/354c80b1387429ab1deab9ab878db9fd056309b0))


### Bug Fixes

* generate activity_id for pending inputs correctly ([5515f18](https://github.com/FitGlue/server/commit/5515f184f94e4f2800502a7442a5327c0d307e9d))

### [9.3.1](https://github.com/FitGlue/server/compare/v9.3.0...v9.3.1) (2026-01-26)


### Bug Fixes

* parkrun matching failure and showcase not handling updates correctly ([8071c8f](https://github.com/FitGlue/server/commit/8071c8fce1a5e5a98b05529968e01b0188a952e6))

## [9.3.0](https://github.com/FitGlue/server/compare/v9.2.0...v9.3.0) (2026-01-26)


### Features

* fix strava uploader erroring on duplicate upload, fix parkrun not attempting enrichresume or initial fetching of results ([d28a393](https://github.com/FitGlue/server/commit/d28a393370e0db8ac22ae3195bcc1e5f49aca94b))


### Bug Fixes

* add PARKRUN_FETCHER_URL to enricher func ([dd52f46](https://github.com/FitGlue/server/commit/dd52f46bdd5170f67fbc7440e41e846ce74346e6))

## [9.2.0](https://github.com/FitGlue/server/compare/v9.1.0...v9.2.0) (2026-01-26)


### Features

* new parkrun-fetcher for playwright ([12b3b87](https://github.com/FitGlue/server/commit/12b3b87ee33106ac8e526ed6bfa4bbdd1783e199))


### Bug Fixes

* actually pass pipeline_id from/to firestore ([94b02ca](https://github.com/FitGlue/server/commit/94b02ca75dd42af33b21c065c0f0ae89b3645ff8))
* attempt manual deployment of parkrun-fetcher ([f284dd5](https://github.com/FitGlue/server/commit/f284dd53362e2f0153fdef02050fb3c5d69ab0d1))
* auth for internal call from parkrun results source to parkrun fetcher ([b39bbe7](https://github.com/FitGlue/server/commit/b39bbe7dcd0b82bf9ddbacedf2c9c4c80c4bc957))
* CICD prepare for parkrun-fetcher ([b68c527](https://github.com/FitGlue/server/commit/b68c527e93c15f3f7323d66a18134908297635af))
* deletion_protection false ([da29178](https://github.com/FitGlue/server/commit/da291780e51ba3427b536648faf8f05df109cde6))
* enricher only processes one pipeline now ([c99fc6c](https://github.com/FitGlue/server/commit/c99fc6c68507ca08db07e9dbe45e95530cc7e846))
* hevy update pathway now functional, parkrun results now generates unique pipeline execution ID ([c8a39d7](https://github.com/FitGlue/server/commit/c8a39d7b0f2cf05e4052a7db00248a0eda388b4f))
* more attempting to call the fetcher successfully ([ebbe765](https://github.com/FitGlue/server/commit/ebbe765d87050b56d394f059b90d801dda63e170))
* orchestrator to get all pipelines when in resume mode ([0bfe1a0](https://github.com/FitGlue/server/commit/0bfe1a0afca5bf783c1960f729717d1ce1d425b2))
* parkrun provider to correctly set pipeline_id in resume payload ([befaa7a](https://github.com/FitGlue/server/commit/befaa7a2ee5259cb4e3fd0398c693424b2ba9336))
* terraform build failure ([680730c](https://github.com/FitGlue/server/commit/680730cc5e0dc4b81bfa21aa6231116219b88fcc))
* typescript converter update [skip ci] ([09a7d9d](https://github.com/FitGlue/server/commit/09a7d9de49c6f45d8e6bce0f0fff30aaf0e438c5))

## [9.1.0](https://github.com/FitGlue/server/compare/v9.0.0...v9.1.0) (2026-01-25)


### Features

* **core:** implement pipeline fan-out, section-based descriptions, and robust loop prevention ([a0bce15](https://github.com/FitGlue/server/commit/a0bce159eb9bfc8242e4e8f20f639d5b3ff4ed2c))


### Bug Fixes

* parkrun results to use correct topic ([e65cccc](https://github.com/FitGlue/server/commit/e65cccc3c1bffa783ddcab67a25668910aaed0c4))

## [9.0.0](https://github.com/FitGlue/server/compare/v8.0.0...v9.0.0) (2026-01-25)


### ⚠ BREAKING CHANGES

* lots of changes to function params to account for passing loggers around

### Features

* add extensive debug logging to Golang funcs ([d9fdb83](https://github.com/FitGlue/server/commit/d9fdb83d97d613ffbb42c9704d32c15f0934bb19))

## [8.0.0](https://github.com/FitGlue/server/compare/v7.1.0...v8.0.0) (2026-01-24)


### ⚠ BREAKING CHANGES

* remove Mock Publish capability. It's fucked us.

### Features

* remove Mock Publish capability. It's fucked us. ([30dff86](https://github.com/FitGlue/server/commit/30dff86a107eeca735dd5ed458809db678c2c555))


### Bug Fixes

* add new pending input fields to firestore golang converter ([806ae5b](https://github.com/FitGlue/server/commit/806ae5b5d246b19edd1f8066f3ea394207aa1370))
* allow 202 http response for parkrun ([859fbac](https://github.com/FitGlue/server/commit/859fbac0ba1ba80c53a80bc718e1ff43d8b7d9aa))
* filtering of pending inputs now sends down auto_populated: true as expected ([c900ac6](https://github.com/FitGlue/server/commit/c900ac619fb8c39ae3d0542a6fbbf44f81a1a8e3))
* make converter handle original payload in JSON format not ProtobufJSON format ([7577bfd](https://github.com/FitGlue/server/commit/7577bfd4c20f0fb08156d9f8d205caff830aa922))
* parkrun html request to use browser-like user-agent header ([fb975e0](https://github.com/FitGlue/server/commit/fb975e009062d501d0a4bd8a3db04c245ef19b3b))
* **parkrun:** publish to right topic ([7d349ad](https://github.com/FitGlue/server/commit/7d349ad756526bbcd37d4ca2cc151c243cbf43c6))

## [7.1.0](https://github.com/FitGlue/server/compare/v7.0.0...v7.1.0) (2026-01-24)


### Features

* **parkrun:** add placeholder description, rich results with PB tracking, and tests ([806b076](https://github.com/FitGlue/server/commit/806b076274bf549e05349ddf2b6c19b3679c9399))

## [7.0.0](https://github.com/FitGlue/server/compare/v6.1.0...v7.0.0) (2026-01-24)


### ⚠ BREAKING CHANGES

* Adds new ActivitySource enum values (INTERVALS, TRAININGPEAKS, GOOGLESHEETS) which may require Protobuf regeneration in downstream consumers.

- Implement isBounceback() with retry logic to handle webhook race conditions
- Add source-level loop prevention check in webhook-processor before deduplication
- Filter pending inputs by auto-deadline to allow automated resolution first
- Fix Hevy external URL template (workouts -> workout)
- Exempt destination-only sources from handler coverage linting

### Features

* add bounceback detection with exponential backoff and support for destination-only sources ([625bed1](https://github.com/FitGlue/server/commit/625bed1e2620e8ef634d5a39db72749a229c5d01))


### Bug Fixes

* add index to firestore for parkrun results etc ([0cb8fec](https://github.com/FitGlue/server/commit/0cb8fec02eb43d709eff07508968b3e74995344c))

## [6.1.0](https://github.com/FitGlue/server/compare/v6.0.0...v6.1.0) (2026-01-24)


### Features

* Implement muscle heatmap image enricher provider using SVG body diagrams to visualize muscle activation. ([f5e8456](https://github.com/FitGlue/server/commit/f5e845675d7bc5d9e5768671490d98dab4e47b0d))
* many things I'm tired ([225471f](https://github.com/FitGlue/server/commit/225471fcfa975367c967af217362045cc18b5a7c))


### Bug Fixes

* ACTUALLY UPLOAD CORRECT GO ZIPS OH MY GOD ([549a58c](https://github.com/FitGlue/server/commit/549a58ce0da8efd77e564ee76c74f5416a00c19d))
* **build:** limit build concurrency to 4 jobs to prevent CI OOM ([9f23ce9](https://github.com/FitGlue/server/commit/9f23ce9db9f1f21b8b1dec49f5f9a35bfed3733f))
* **build:** limit build/test concurrency to 4 jobs to prevent CI OOM ([c491e8e](https://github.com/FitGlue/server/commit/c491e8eed175a11b925c61fae7e01b347c7b9fa6))
* fix pipeline imports ([e087c62](https://github.com/FitGlue/server/commit/e087c620d203a802b3ed7592f130e741125a8e95))
* include subdirs in go function zip builds ([29cea1b](https://github.com/FitGlue/server/commit/29cea1b06637f63ff4d2fc240ff4ab88564d4530))

## [6.0.0](https://github.com/FitGlue/server/compare/v5.0.1...v6.0.0) (2026-01-23)


### ⚠ BREAKING CHANGES

* moves enricher providers to the enricher function, so non-enricher functions aren't needlessly redployed on amending enricher providers

### Features

* move enricher providers to enricher function ([2d116c7](https://github.com/FitGlue/server/commit/2d116c74f55b2049d9e4354c02e056b07cbae7d0))
* use cloud cdn for assets bucket exposure plus SSL cert ([15964bf](https://github.com/FitGlue/server/commit/15964bf1bf344d9643dcd0120931a6a8dee4bb15))


### Bug Fixes

* enricher ordering bug ([1ca1a4c](https://github.com/FitGlue/server/commit/1ca1a4cc0e75d5a5a7c799e8432f4e14b42c0c05))
* enricher providers only return their description additions, not whole description plus addition ([012418b](https://github.com/FitGlue/server/commit/012418bfe2b9da1ae2358bf7fb1f19eb78135e74))
* failing go tests ([16cf090](https://github.com/FitGlue/server/commit/16cf090ea02d38361f2099bc50c60090abe088ad))
* make hyde park virtual route much nicer ([1838427](https://github.com/FitGlue/server/commit/183842792572886f1852a503b2842dc34365d301))
* some enrichers still returning total description ([d0b4304](https://github.com/FitGlue/server/commit/d0b43049f3ff94dd59fac077c8d0b01b2568069f))

### [5.0.1](https://github.com/FitGlue/server/compare/v5.0.0...v5.0.1) (2026-01-23)


### Bug Fixes

* bugs since refactoring ([a49f005](https://github.com/FitGlue/server/commit/a49f0059a33781efe513eb933b153f4291e4070c))
* failing showcase-handler test ([b432692](https://github.com/FitGlue/server/commit/b43269289310fa3af0fb11520e24b94d251b1a66))
* gofmt ([5b26656](https://github.com/FitGlue/server/commit/5b26656240246becccc3bd73173bab3e8ee3688d))
* image generation enricher failures ([f557697](https://github.com/FitGlue/server/commit/f557697e42532adbfc805146cbbe78bd006d96e1))
* prevent endless redeploys, fix tool build ([3b11517](https://github.com/FitGlue/server/commit/3b11517b8c72b37a587aaf87c47b3f697e893816))
* showcase assets bucket config ([43b8893](https://github.com/FitGlue/server/commit/43b88933d8d94e291411ce24e34818e1b1df8cb2))

## [5.0.0](https://github.com/FitGlue/server/compare/v4.0.1...v5.0.0) (2026-01-23)


### ⚠ BREAKING CHANGES

* **shared:** SafeHandler signature changed from (req, res, ctx) to (req, ctx).
Handlers must now return a value or a FrameworkResponse instance instead of
directly manipulating the Express 'res' object. Direct usage of 'res.send()'
or 'res.status()' in handlers is now deprecated and discouraged.
* **shared:** Standardized secret management. The direct 'GetSecret'
capability has been removed from the shared library and Go implementations.
Secrets are now injected via environment variables or accessed through
the SecretsHelper which uses SecretManagerServiceClient.
Changes include:
- Refactored 'createCloudFunction' to handle both HTTP and CloudEvent triggers.
- Introduced 'FrameworkResponse' for declarative control over response codes and headers.
- Integrated Sentry error capture directly into the framework lifecycle.
- Updated all existing handlers (admin, activities, showcase, etc.) to the new signature.
- Implemented 'Zero-Debt Convergence' standard (0 Errors, 0 Warnings) across TypeScript.
- Added Sentry environment variable injection in Terraform.
- Updated Plugin Registry with High-Fidelity Icon support (Rule G16).

### Features

* error handling, build and lint fixes ([93f0386](https://github.com/FitGlue/server/commit/93f0386010876308b0b1c31923f7f419f1bcb41d))
* sentry integration and safe handling of errors across TS ([da337e9](https://github.com/FitGlue/server/commit/da337e933112a312df94448cf6de93597ef6fbe2))


### Bug Fixes

* circleci and linter ([a4a87a7](https://github.com/FitGlue/server/commit/a4a87a704b32270e9e2934e5cf4f40aefd1d7509))
* pipelines in legacy format breaking converters ([85bd2bc](https://github.com/FitGlue/server/commit/85bd2bc51dfab8b0acfc2fabf93268616f9ef886))
* upload sourcemaps fix ([256349f](https://github.com/FitGlue/server/commit/256349f17701d09b5d46aca59dfd9a616621a3d4))


* **shared:** unify handler signatures and standardize secret management ([d6bc891](https://github.com/FitGlue/server/commit/d6bc8910e844c7a997b2a75e54bb17e5c9a4fea2))

### [4.0.1](https://github.com/FitGlue/server/compare/v4.0.0...v4.0.1) (2026-01-22)


### Bug Fixes

* sentry setup and some bug fixing ([aac480f](https://github.com/FitGlue/server/commit/aac480f56325c5a3fdcadfd7639d9820095303d4))
* sentry setup and some bug fixing ([0ad9c76](https://github.com/FitGlue/server/commit/0ad9c7690632576a242e875a0c1daea2a32c5fd0))

## [4.0.0](https://github.com/FitGlue/server/compare/v3.0.0...v4.0.0) (2026-01-22)


### ⚠ BREAKING CHANGES

* **server:** Protobuf enum updates for EnricherProviderType and DestinationType require re-generation of clients and database migrations for existing records.

### Features

* add pipeline toggling and sentry integration ([8e0f470](https://github.com/FitGlue/server/commit/8e0f4700fba9db08ab98c6d42853e1ccde198365))
* Implement Oura integration, temporarily disable various plugins, and add new deployment and secret management scripts. ([e19593c](https://github.com/FitGlue/server/commit/e19593c6db1e79186aca3d37935f62fcf323720b))
* **server:** major integration expansion and rich asset overhaul ([0e16eba](https://github.com/FitGlue/server/commit/0e16ebabad81c4d54529d3a59bf1921ad7435018))


### Bug Fixes

* change assets bucket name to use project_id prefix ([88b9e7a](https://github.com/FitGlue/server/commit/88b9e7afa7ceabd1df59d2754ffc1f604d332953))
* define variable for sentrY_dsn ([b8890c0](https://github.com/FitGlue/server/commit/b8890c02134c298447cc439aa1f8b88c5695b196))

## [3.0.0](https://github.com/FitGlue/server/compare/v2.1.0...v3.0.0) (2026-01-21)


### ⚠ BREAKING CHANGES

* Updated Database interface to include Personal Records methods and modified Protobuf definitions for integrations.

- Added new Activity Enrichers:
  - Personal Records (Cardio/Strength tracking)
  - Training Load (TRIMP calculation)
  - Spotify Tracks integration
  - Weather (Open-Meteo)
  - Elevation Summary
  - Location Naming (Reverse Geocoding)
- Implemented new Integrations:
  - Spotify (OAuth and Auth monitoring)
  - TrainingPeaks (Uploader and OAuth)
- Updated core infrastructure:
  - Extended Database interface with Firestore persistence for PRs
  - Modified Protobuf schemas for User and Events
  - Configured Terraform for new Cloud Functions and secrets
- Improved shared TypeScript utilities and registry

### Features

* add sorting to plugins and stop secrets not being defined from failing terraform ([5aaec96](https://github.com/FitGlue/server/commit/5aaec961a900654d14a0011df73a7aad54e79e2c))
* comprehensive 2026 feature expansion and core architecture updates ([6d318e3](https://github.com/FitGlue/server/commit/6d318e308c05b723fabf844078f341c12fadadea))

## [2.1.0](https://github.com/FitGlue/server/compare/v2.0.0...v2.1.0) (2026-01-21)


### Features

* Introduce new pace, cadence, power, and speed summary enrichers, refine AI companion prompt, and update user tier naming. ([8f1e325](https://github.com/FitGlue/server/commit/8f1e325fde2c16e66acbe90ee21102cea8f32f33))

## [2.0.0](https://github.com/FitGlue/server/compare/v1.9.1...v2.0.0) (2026-01-21)


### ⚠ BREAKING CHANGES

* strava source and changes to user mappings

### Features

* strava source and changes to user mappings ([f0d2b3c](https://github.com/FitGlue/server/commit/f0d2b3ce0d1067389b89678c9fc20c2b1128565f))


### Bug Fixes

* register-strava-webhook script works ([30f6cea](https://github.com/FitGlue/server/commit/30f6ceaacc18a12fffb661077a13c9633753f52c))

### [1.9.1](https://github.com/FitGlue/server/compare/v1.9.0...v1.9.1) (2026-01-21)


### Bug Fixes

* parkrun import and integrations endpoint ([940c6cb](https://github.com/FitGlue/server/commit/940c6cb8b22fccf15fb5ea1bd811287cbdafef51))

## [1.9.0](https://github.com/FitGlue/server/compare/v1.8.0...v1.9.0) (2026-01-21)


### Features

* Add AI description and heart rate summary enrichers, and refactor Fitbit HR provider to support force/skip logic. ([5085f6d](https://github.com/FitGlue/server/commit/5085f6d8aaf9be9b7a42d77643f06c54997732c6))
* improvements to enricher registration and enum usage ([f9340a2](https://github.com/FitGlue/server/commit/f9340a2c08e0aec7e5ebff30f5d506551bad2d74))
* Introduce comprehensive user tier management fields and support 'athlete' tier as 'pro' in effective tier calculations, updating Firestore converters and admin handler. ([2d2fd3f](https://github.com/FitGlue/server/commit/2d2fd3fdd42519fcc659145442bdd1974e0cc49e))
* Introduce separate `cleanTitle` and `cleanDescription` functions with distinct truncation logic and add corresponding tests. ([6ae561f](https://github.com/FitGlue/server/commit/6ae561f6e6000070eda4f9730a7b72fd945bdc53))
* Wrap full-pipeline repost messages in a CloudEvent using an updated `createCloudEvent` function that accepts a custom type. ([b8ab5d3](https://github.com/FitGlue/server/commit/b8ab5d302baebb0c31a4ea6b13a60a3fde7ae821))


### Bug Fixes

* add registry manifest to showcase response ([f59594f](https://github.com/FitGlue/server/commit/f59594f8ec0214b55068fba58ee5aafb0f7090c6))
* added firestore admin iam ([25d8506](https://github.com/FitGlue/server/commit/25d85068bf61d450f51409e726926e8f50b9ebab))
* Standardize activity, user, and pipeline execution ID fields to snake_case in repost events to prevent Go duplicate field errors. ([e7ebdb7](https://github.com/FitGlue/server/commit/e7ebdb7549458ab9882ade254149a1bf346f96cd))
* tf failures ([41b95f2](https://github.com/FitGlue/server/commit/41b95f2b2efc4c65b619662d143b6458049a36b4))

## [1.8.0](https://github.com/FitGlue/server/compare/v1.7.0...v1.8.0) (2026-01-20)


### Features

* Introduce activity counters, optimize execution fetching with projection queries, and add external URL templates for plugins. ([d10ab6b](https://github.com/FitGlue/server/commit/d10ab6b87f67f0651b264748798f8cad160df3e2))


### Bug Fixes

* emoji linter, remove unneeded firestore indexes ([91150b2](https://github.com/FitGlue/server/commit/91150b26113c77529c4fba7081401f69033b0785))

## [1.7.0](https://github.com/FitGlue/server/compare/v1.6.0...v1.7.0) (2026-01-20)


### Features

* Add execution logging controls including service-specific disabling, output truncation, and a CLI command to clean logs by service. ([4ceea0b](https://github.com/FitGlue/server/commit/4ceea0bcfa942df3c3bd4659d5414b5d2e1dcb9e))
* add new admin API handler for user management and platform statistics. ([2a0c047](https://github.com/FitGlue/server/commit/2a0c047bb50472f8bd43e4951544ca137c3b567a))
* admin capability updates ([185a89b](https://github.com/FitGlue/server/commit/185a89ba8f2c1da916cd6b47679b4a364d24a17d))
* Enhance CloudEvent publisher with extensions, add PENDING_STRAVA_PROCESSING status, and refactor repost-handler to publish to a central router topic. ([a17b69b](https://github.com/FitGlue/server/commit/a17b69b097e04a23cc6c703cc5a017fbcbf02fa2))
* Implement activity repost logic for Go uploaders and standardize TypeScript Cloud Function build entry points. ([8562e0d](https://github.com/FitGlue/server/commit/8562e0dc40fcd51ece6384b6b8ff237bebbe2430))
* Implement email prefix fallback for showcase owner display names and disable execution logging for several handlers. ([98d8978](https://github.com/FitGlue/server/commit/98d897864dcf3d4b7402b94f06be7e7445ac2ce3))
* Implement per-handler TypeScript Cloud Function deployments by adding a new build script and updating the Makefile and Terraform configurations to use individual function ZIPs. ([d16cd3f](https://github.com/FitGlue/server/commit/d16cd3f2fa65586eae435c4481bea61e4e8e6742))
* Implement standardized HTTP error logging with response body capture for Go and TypeScript HTTP clients. ([126dcf1](https://github.com/FitGlue/server/commit/126dcf12475bd5125d2eec86659e2ceb0d023424))


### Bug Fixes

* make activities-handler return ([429861b](https://github.com/FitGlue/server/commit/429861b8148a838c8cb95c76258727c953e3bf3a))
* repost-handler cloud event format publish ([0fb4a9f](https://github.com/FitGlue/server/commit/0fb4a9f98591c18a2e8de26309fff74599a9bd79))
* repost-handler not parsing previous events correctly ([7d01a48](https://github.com/FitGlue/server/commit/7d01a483c42543e7eecc2b7e5e11694f4cf327d7))

## [1.6.0](https://github.com/FitGlue/server/compare/v1.5.0...v1.6.0) (2026-01-19)


### Features

* Add comprehensive linting checks for environment variable access, protobuf freshness, enum definitions, formatter coverage, and handler configurations, alongside new enum formatter generation. ([569bb3d](https://github.com/FitGlue/server/commit/569bb3d382861ce2847fc1fe49e0364f98d06705))
* Introduce unit tests for mock and integration handlers, configure Jest, and refine linting rules with error configurations. ([bae9af8](https://github.com/FitGlue/server/commit/bae9af8df1ca2b9a79fe7fd8f5bb6a56908d15ce))


### Bug Fixes

* linting ([9045ad9](https://github.com/FitGlue/server/commit/9045ad9c56e6de30a6859a856e088b51fd39a04f))

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
