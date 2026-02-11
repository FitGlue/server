/**
 * TypeScript Plugin Registry
 *
 * This module provides a centralized registry for all FitGlue plugins.
 * Enricher manifests are defined here to match the Go implementations.
 */

import {
  PluginManifest,
  PluginRegistryResponse,
  PluginType,
  ConfigFieldType,
  ConfigFieldSchema,
  IntegrationManifest,
  IntegrationAuthType,
} from '../types/pb/plugin';
import { EnricherProviderType } from '../types/pb/user';

/**
 * TypeScript ergonomics for ConfigFieldSchema.
 *
 * In protobuf3, repeated fields default to empty arrays. However, ts-proto generates
 * them as required arrays in TypeScript. This type allows omitting keyOptions and
 * valueOptions when not needed - they'll be normalized to empty arrays.
 *
 * The actual schema is defined in plugin.proto - this is just a TypeScript convenience.
 */
type ConfigFieldSchemaInput = Omit<ConfigFieldSchema, 'keyOptions' | 'valueOptions'> & {
  keyOptions?: ConfigFieldSchema['keyOptions'];
  valueOptions?: ConfigFieldSchema['valueOptions'];
};

/**
 * Normalizes ConfigFieldSchemaInput to full ConfigFieldSchema by adding default empty arrays.
 */
function normalizeConfigField(input: ConfigFieldSchemaInput): ConfigFieldSchema {
  return {
    ...input,
    keyOptions: input.keyOptions ?? [],
    valueOptions: input.valueOptions ?? [],
  };
}

/**
 * TypeScript ergonomics for PluginManifest - allows optional keyOptions/valueOptions in configSchema.
 */
type PluginManifestInput = Omit<PluginManifest, 'configSchema'> & {
  configSchema: ConfigFieldSchemaInput[];
};

/**
 * Normalizes PluginManifestInput to full PluginManifest.
 */
function normalizeManifest(input: PluginManifestInput): PluginManifest {
  return {
    ...input,
    configSchema: input.configSchema.map(normalizeConfigField),
  };
}

// Registry stores
const sources: Map<string, PluginManifest> = new Map();
const enrichers: Map<EnricherProviderType, PluginManifest> = new Map();
const destinations: Map<string, PluginManifest> = new Map();
const integrations: Map<string, IntegrationManifest> = new Map();

/**
 * Context passed to lifecycle hooks during pipeline CRUD operations.
 */
export interface PluginLifecycleContext {
  userId: string;
  pipelineId: string;
  config: Record<string, string>;
  /** Retrieve a valid OAuth token for the given provider */
  getValidToken: (userId: string, provider: string) => Promise<string>;
  /** Structured logger */
  logger: { info: (msg: string, meta?: Record<string, unknown>) => void; warn: (msg: string, meta?: Record<string, unknown>) => void; error: (msg: string, meta?: Record<string, unknown>) => void };
}

/**
 * Lifecycle hooks for plugins. These are called during pipeline CRUD operations.
 * Not serialised — runtime-only callbacks registered alongside plugin manifests.
 */
export interface PluginLifecycleHooks {
  /**
   * Called when a pipeline using this plugin is created.
   * Return a partial config update (e.g. { webhook_id: '...' }) to merge into sourceConfig.
   * Throws on failure → pipeline creation is blocked.
   */
  onPipelineCreate?: (ctx: PluginLifecycleContext) => Promise<Record<string, string> | void>;
  /**
   * Called when a pipeline using this plugin is deleted.
   * Best-effort — failures are logged but don't block deletion.
   */
  onPipelineDelete?: (ctx: PluginLifecycleContext) => Promise<void>;
}

const pluginHooks: Map<string, PluginLifecycleHooks> = new Map();

/**
 * Register lifecycle hooks for a plugin.
 * @param pluginId - The plugin's unique ID (e.g. "github")
 * @param hooks - The lifecycle callbacks
 */
export function registerPluginHooks(pluginId: string, hooks: PluginLifecycleHooks): void {
  pluginHooks.set(pluginId, hooks);
}

/**
 * Get lifecycle hooks for a plugin by ID.
 */
export function getPluginHooks(pluginId: string): PluginLifecycleHooks | undefined {
  return pluginHooks.get(pluginId);
}

/**
 * Clear all plugin hooks (for testing).
 */
export function clearPluginHooks(): void {
  pluginHooks.clear();
}

/**
 * Register a source plugin manifest
 */
export function registerSource(manifest: PluginManifestInput): void {
  sources.set(manifest.id, normalizeManifest(manifest));
}

/**
 * Register an enricher plugin manifest
 */
export function registerEnricher(providerType: EnricherProviderType, input: PluginManifestInput): void {
  const manifest = normalizeManifest(input);
  manifest.enricherProviderType = providerType;
  enrichers.set(providerType, manifest);
}

/**
 * Register a destination plugin manifest
 */
export function registerDestination(manifest: PluginManifestInput): void {
  destinations.set(manifest.id, normalizeManifest(manifest));
}

/**
 * Register an integration manifest
 */
export function registerIntegration(manifest: IntegrationManifest): void {
  integrations.set(manifest.id, manifest);
}

/**
 * Get the full plugin registry
 */
export function getRegistry(): PluginRegistryResponse {
  return {
    sources: Array.from(sources.values()),
    enrichers: Array.from(enrichers.values()),
    destinations: Array.from(destinations.values()),
    integrations: Array.from(integrations.values()).filter(i => i.enabled),
  };
}

/**
 * Get a specific enricher manifest by provider type
 */
export function getEnricherManifest(providerType: EnricherProviderType): PluginManifest | undefined {
  return enrichers.get(providerType);
}

/**
 * Clear registry (for testing)
 */
export function clearRegistry(): void {
  sources.clear();
  enrichers.clear();
  destinations.clear();
  integrations.clear();
  clearPluginHooks();
}

// ============================================================================
// Register all known source manifests
// ============================================================================

registerSource({
  id: 'hevy',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Hevy',
  description: 'Import strength training workouts from Hevy',
  icon: '🏋️',
  enabled: true,
  requiredIntegrations: ['hevy'],
  configSchema: [],
  marketingDescription: `
### Strength Training Source
Import your weight training workouts from Hevy into FitGlue for enhancement and distribution. Every exercise, set, rep, and weight is captured with full fidelity.

### How it works
When you complete a workout in Hevy, FitGlue receives a webhook notification and imports the full workout data. The activity enters your FitGlue pipeline where it can be enriched with AI descriptions, muscle heatmaps, and more.
  `,
  features: [
    '✅ Import strength workouts with full exercise details',
    '✅ Capture sets, reps, weights, and rest periods',
    '✅ Real-time sync via webhooks',
    '✅ Works seamlessly with all FitGlue boosters',
  ],
  transformations: [],
  useCases: [
    'Enhance strength workouts with AI descriptions and muscle heatmaps',
    'Cross-post Hevy workouts to Strava with rich metadata',
    'Generate public Showcase links for your training sessions',
  ],
  // UX Organization
  category: 'apps',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 80,
  iconType: 'svg',
  iconPath: '/images/icons/hevy.svg',
});

registerSource({
  id: 'fitbit',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Fitbit',
  description: 'Import activities from Fitbit',
  icon: '⌚',
  enabled: true,
  requiredIntegrations: ['fitbit'],
  configSchema: [],
  marketingDescription: `
### Wearable Activity Source
Import activities tracked by your Fitbit device into FitGlue. Runs, walks, bike rides, and workouts with heart rate and calorie data are all supported.

### How it works
FitGlue connects to the Fitbit API and receives notifications when you complete activities. The full activity data, including heart rate zones and GPS tracks (if available), is imported into your pipeline.
  `,
  features: [
    '✅ Import all Fitbit-tracked activities',
    '✅ Heart rate data included automatically',
    '✅ GPS tracks for outdoor activities',
    '✅ Automatic sync when activities complete',
  ],
  transformations: [],
  useCases: [
    'Enhance Fitbit activities with AI descriptions and stats',
    'Overlay Fitbit heart rate data onto Hevy strength workouts',
    'Cross-post Fitbit activities to Strava with boosted content',
  ],
  // UX Organization
  category: 'wearables',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 85,
  iconType: 'svg',
  iconPath: '/images/icons/fitbit.svg',
});

registerSource({
  id: 'mock',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Mock',
  description: 'Testing source for development',
  icon: '🧪',
  enabled: false,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: '',
  features: [],
  transformations: [],
  useCases: [],
  // UX Organization (internal - not shown to users)
  category: 'manual',
  sortOrder: 99,
  isPremium: false,
  popularityScore: 0,
});

registerSource({
  id: 'apple_health',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Apple Health',
  description: 'Import workouts and health data from iOS devices',
  icon: '🍎',
  enabled: true,
  requiredIntegrations: ['apple-health'],
  configSchema: [],
  marketingDescription: `
### iOS Health Data Source
Import workouts, heart rate data, and GPS routes directly from Apple Health on your iPhone or Apple Watch. FitGlue's mobile app syncs your health data seamlessly.

### How it works
Install the FitGlue mobile app on your iOS device and grant access to Apple Health. Workouts are automatically synced in the background and flow through your FitGlue pipeline.
  `,
  features: [
    '✅ Import workouts from Apple Watch and iPhone',
    '✅ Heart rate data included automatically',
    '✅ GPS routes for outdoor activities',
    '✅ Background sync when app is installed',
  ],
  transformations: [],
  useCases: [
    'Enhance Apple Watch workouts with AI descriptions and stats',
    'Cross-post iOS workouts to Strava with rich metadata',
    'Import heart rate and GPS data for use with boosters',
  ],
  // UX Organization
  category: 'wearables',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 90,
  iconType: 'png',
  iconPath: '/images/icons/healthkit.png',
});

registerSource({
  id: 'health_connect',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Health Connect',
  description: 'Import workouts and health data from Android devices',
  icon: '🤖',
  enabled: true,
  requiredIntegrations: ['health-connect'],
  configSchema: [],
  marketingDescription: `
### Android Health Data Source
Import workouts, heart rate data, and GPS routes from Android Health Connect. FitGlue's mobile app syncs your health data from any compatible fitness tracker or app.

### How it works
Install the FitGlue mobile app on your Android device and grant access to Health Connect. Workouts are automatically synced in the background and flow through your FitGlue pipeline.
  `,
  features: [
    '✅ Import workouts from any Health Connect-compatible device',
    '✅ Heart rate data included automatically',
    '✅ GPS routes for outdoor activities',
    '✅ Background sync when app is installed',
  ],
  transformations: [],
  useCases: [
    'Enhance Android workouts with AI descriptions and stats',
    'Import data from Garmin, Samsung, and other Health Connect devices',
    'Cross-post Android activities to Strava with boosted content',
  ],
  // UX Organization
  category: 'wearables',
  sortOrder: 4,
  isPremium: false,
  popularityScore: 75,
  iconType: 'svg',
  iconPath: '/images/icons/healthconnect.svg',
});

registerSource({
  id: 'parkrun_results',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Parkrun Results',
  description: 'Create activities from official Parkrun results',
  icon: '🏃',
  enabled: false, // Internal source - not user-configurable
  requiredIntegrations: ['parkrun'],
  configSchema: [],
  marketingDescription: `
### Official Parkrun Results
Creates activities directly from official Parkrun results when no GPS activity exists. Used for GPS-less participants who want their official times tracked.

### How it works
When you complete a Parkrun but don't have a GPS watch, FitGlue can still track your result by polling parkrun.org for your official time after the event.
  `,
  features: [
    '✅ Track Parkrun results without GPS',
    '✅ Official times from parkrun.org',
    '✅ Position and age grade included',
  ],
  transformations: [],
  useCases: [],
  // UX Organization (internal source - low visibility)
  category: 'manual',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 30,
});

registerSource({
  id: 'file_upload',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'File Upload',
  description: 'Upload FIT files directly from your device',
  icon: '📤',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### Direct FIT File Upload
Upload activity files directly from your device. Perfect for importing data from fitness devices or recovering workouts not captured by connected services.

### How it works
Select a FIT file, optionally add title/description, then upload. The activity enters your pipeline for enrichment and distribution.
  `,
  features: [
    '✅ Upload FIT files from any device',
    '✅ Set custom title and description',
    '✅ Works with all FitGlue boosters',
    '✅ No external service connection required',
  ],
  transformations: [],
  useCases: [
    'Import workouts from Garmin or other standalone GPS devices',
    'Bring in activities from platforms FitGlue doesn\'t integrate with yet',
    'Recover activities from device backups',
  ],
  // UX Organization
  category: 'manual',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 60,
});

registerSource({
  id: 'strava',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Strava',
  description: 'Import activities from Strava via real-time webhooks',
  icon: '🚴',
  enabled: true,
  requiredIntegrations: ['strava'],
  configSchema: [],
  marketingDescription: `
### The World's Leading Fitness Platform
Import your runs, rides, swims, and workouts from Strava into FitGlue. Activities are synced in real-time via webhooks the moment they're uploaded to Strava.

### How it works
Connect your Strava account to FitGlue via OAuth. When you complete an activity on Strava (or sync from your device), FitGlue receives a webhook notification and imports the full activity data. Your activity then flows through your pipeline for enrichment and distribution.

### Seamless Loop Prevention
FitGlue's intelligent loop prevention ensures that activities uploaded TO Strava aren't re-imported FROM Strava. No duplicates, no infinite loops.
  `,
  features: [
    '✅ Real-time sync via Strava webhooks',
    '✅ Import all activity types (runs, rides, swims, workouts)',
    '✅ Heart rate, GPS, and power data included',
    '✅ Automatic sync when activities are uploaded',
    '✅ Smart loop prevention prevents duplicates',
  ],
  transformations: [],
  useCases: [
    'Enhance Strava activities with AI descriptions and muscle heatmaps',
    'Cross-post Strava activities to other platforms like Hevy',
    'Generate public Showcase links for your Strava activities',
  ],
  // UX Organization
  category: 'apps',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 95,
  iconType: 'svg',
  iconPath: '/images/icons/strava.svg',
});

registerSource({
  id: 'garmin',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Garmin',
  description: 'Import activities from Garmin Connect',
  icon: '⌚',
  enabled: true,
  requiredIntegrations: ['garmin'],
  configSchema: [],
  marketingDescription: `
### Garmin Connect Source
Import your runs, rides, swims, and multisport activities from Garmin Connect. FitGlue receives activities in real-time via webhooks with full FIT file data including heart rate, GPS, and power metrics.

### How it works
Connect your Garmin Connect account to FitGlue via OAuth. When you complete an activity on your Garmin device and sync it, FitGlue receives a webhook notification and imports the full activity data into your pipeline for enrichment.
  `,
  features: [
    '✅ Import activities from Garmin devices',
    '✅ Heart rate and GPS data included',
    '✅ Real-time sync via webhooks',
    '✅ Works with all FitGlue boosters',
  ],
  transformations: [],
  useCases: [
    'Enhance Garmin activities with AI descriptions and stats',
    'Cross-post Garmin activities to Strava with boosted content',
    'Generate public Showcase links for your Garmin workouts',
  ],
  // UX Organization
  category: 'wearables',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 88,
  isTemporarilyUnavailable: true,
  iconType: 'svg',
  iconPath: '/images/icons/garmin.svg',
});

registerSource({
  id: 'wahoo',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Wahoo',
  description: 'Import workouts from Wahoo Cloud with full FIT file support',
  icon: '📱',
  enabled: true,
  requiredIntegrations: ['wahoo'],
  configSchema: [],
  marketingDescription: `
### Wahoo Cloud Source
Import workouts from your Wahoo ELEMNT bike computers, KICKR trainers, and other Wahoo devices. Activities are synced in real-time via webhooks with full FIT file data.

### How it works
Connect your Wahoo account to FitGlue via OAuth. When you complete a workout on your Wahoo device, FitGlue receives a webhook notification, downloads the full FIT file, and imports your activity with all sensor data.

### Full Sensor Data
Power meters, heart rate monitors, cadence sensors, and GPS data — all captured in the FIT file and available for your enrichment pipeline.
  `,
  features: [
    '✅ Real-time sync via Wahoo Cloud webhooks',
    '✅ Full FIT file download with complete sensor data',
    '✅ Power, HR, cadence, and GPS data included',
    '✅ Works with ELEMNT, KICKR, and all Wahoo devices',
    '✅ Automatic sync when workouts complete',
  ],
  isTemporarilyUnavailable: true,
  transformations: [],
  useCases: [
    'Enhance Wahoo rides with AI descriptions and stats summaries',
    'Cross-post Wahoo activities to Strava or TrainingPeaks',
    'Generate Showcase links for your cycling workouts',
  ],
  // UX Organization
  category: 'wearables',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 82,
  iconType: 'jpg',
  iconPath: '/images/icons/wahoo.jpg',
});

registerSource({
  id: 'oura',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Oura Ring',
  description: 'Import workouts from Oura Ring',
  icon: '💍',
  enabled: true,
  requiredIntegrations: ['oura'],
  configSchema: [],
  marketingDescription: `
### Smart Ring Workout Source
Import workouts tracked by your Oura Ring into FitGlue. Heart rate data, calories, and workout metrics are captured for enhancement and distribution.

### How it works
When you complete a workout tracked by your Oura Ring, FitGlue receives a webhook notification and imports the workout data. The activity enters your FitGlue pipeline where it can be enriched with AI descriptions and more.
  `,
  features: [
    '✅ Import workouts from Oura Ring',
    '✅ Heart rate data (average and max)',
    '✅ Calories and duration tracking',
    '✅ Real-time sync via webhooks',
    '✅ Works seamlessly with all FitGlue boosters',
  ],
  isTemporarilyUnavailable: true,
  transformations: [],
  useCases: [
    'Track workouts without GPS for indoor activities',
    'Combine sleep/readiness context with workout data',
    'Enhance Oura workouts with AI descriptions',
  ],
  // UX Organization
  category: 'wearables',
  sortOrder: 5,
  isPremium: false,
  popularityScore: 70,
  iconType: 'jpg',
  iconPath: '/images/icons/oura.jpg',
});

registerSource({
  id: 'polar',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Polar Flow',
  description: 'Import activities from Polar wearables',
  icon: '❄️',
  enabled: true,
  requiredIntegrations: ['polar'],
  configSchema: [],
  marketingDescription: `
### Polar Wearable Source
Import your runs, rides, swims, and workouts from Polar devices. Activities are synced via webhooks when you complete them on your Polar watch or sync with Polar Flow.

### How it works
Connect your Polar Flow account to FitGlue via OAuth. When you complete an activity, FitGlue receives a webhook notification and imports the full activity data including heart rate, GPS tracks, and training load metrics.

### Transaction-Based Sync
Polar uses a unique transaction-based API that ensures reliable data delivery. FitGlue handles all the complexity automatically.
  `,
  features: [
    '✅ Import all Polar-tracked activities',
    '✅ Heart rate data with zones',
    '✅ GPS routes for outdoor activities',
    '✅ Training load metrics included',
    '✅ Real-time sync via webhooks',
  ],
  isTemporarilyUnavailable: true,
  transformations: [],
  useCases: [
    'Enhance Polar activities with AI descriptions',
    'Cross-post Polar activities to Strava or Hevy',
    'Generate public Showcase links for your Polar activities',
  ],
  // UX Organization
  category: 'wearables',
  sortOrder: 6,
  isPremium: false,
  popularityScore: 75,
  iconType: 'svg',
  iconPath: '/images/icons/polar.svg',
});

registerSource({
  id: 'intervals',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'Intervals.icu',
  description: 'Import activities from Intervals.icu',
  icon: '📊',
  enabled: true,
  requiredIntegrations: ['intervals'],
  configSchema: [],
  marketingDescription: `
### Advanced Training Analytics Source
Import your activities from Intervals.icu into FitGlue. Power data, heart rate, cadence, GPS, and all performance metrics are captured for enhancement and distribution.

### How it works
FitGlue polls your Intervals.icu account for new activities. When new activities are found, they're imported into your pipeline where they can be enriched with AI descriptions, training load calculations, and more.
  `,
  features: [
    '✅ Import activities with full power data',
    '✅ Heart rate, cadence, and GPS included',
    '✅ Performance metrics and training load',
    '✅ Works seamlessly with all FitGlue boosters',
  ],
  isTemporarilyUnavailable: true,
  transformations: [],
  useCases: [
    'Enhance Intervals.icu activities with AI descriptions',
    'Cross-post to Strava with enriched metadata',
    'Generate public Showcase links for sharing',
  ],
  // UX Organization
  category: 'analytics',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 70,
  iconType: 'png',
  iconPath: '/images/icons/intervals.png',
});

registerSource({
  id: 'github',
  type: PluginType.PLUGIN_TYPE_SOURCE,
  name: 'GitHub',
  description: 'Import activities from a GitHub repository',
  icon: '🐙',
  enabled: true,
  requiredIntegrations: ['github'],
  configSchema: [
    {
      key: 'repo',
      label: 'Repository',
      description: 'Full name of the repository (e.g. your-username/fitness-log)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: true,
      defaultValue: '',
      options: [],
    },
    {
      key: 'folder',
      label: 'Folder Path',
      description: 'Root folder to watch for activity Markdown files (e.g. workouts/)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: 'workouts/',
      options: [],
    },
  ],
  marketingDescription: `
### What is it?
GitHub as a Source lets you store your fitness activities as Markdown files inside a Git repository. Each activity file uses YAML frontmatter for structured metadata (title, type, date) and can optionally reference a \`.fit\` file for full telemetry data.

### How it works
When you push a commit containing new or modified \`.md\` files to your configured folder, a GitHub webhook fires and FitGlue automatically picks up the changes. FitGlue parses the YAML frontmatter — title, activity type, distance, and more — and creates a standardised activity that flows through your pipeline.

If the frontmatter contains a \`fit_file\` field pointing to a \`.fit\` binary in the same repository, FitGlue downloads the file and attaches the full telemetry (heart rate, GPS, cadence, power) so that boosters like Heart Rate Zones and Muscle Heatmap can process the data.

### File Format
Activity files follow a simple convention:
\`\`\`markdown
---
title: "Morning Run"
type: running
date: 2026-02-08
distance_km: 5.2
fit_file: ./morning-run.fit
---
Optional free-text notes here.
\`\`\`

### Safety & Privacy
- FitGlue only reads files inside your configured folder — nothing else in the repo is accessed.
- Webhook payloads are verified via HMAC-SHA256 to prevent spoofing.
- Commits authored by "FitGlue Bot" are automatically ignored to prevent infinite loops when GitHub is also a destination.
- Your OAuth token is stored encrypted and is only used to fetch file contents.
  `,
  features: [
    '✅ Version-controlled activity history',
    '✅ Full FIT file telemetry support',
    '✅ YAML frontmatter for structured metadata',
    '✅ HMAC-SHA256 webhook verification',
    '✅ Automatic loop prevention',
    '✅ Works with any public or private repository',
  ],
  transformations: [],
  useCases: [
    'Maintain a Git-based fitness journal with full history',
    'Trigger enrichment pipelines from CI/CD workflows',
    'Share activity data through pull requests and code review',
    'Archive telemetry data alongside training notes',
  ],
  // UX Organization
  category: 'developer',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 60,
  iconType: 'svg',
  iconPath: '/images/icons/github.svg',
});

// Register lifecycle hooks for GitHub (webhook management)
import { githubHooks } from './hooks/github-hooks';
registerPluginHooks('github', githubHooks);

// ============================================================================
// Register all known destination manifests
// ============================================================================

registerDestination({
  id: 'strava',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'Strava',
  description: 'Upload activities to Strava',
  icon: '🚴',
  enabled: true,
  requiredIntegrations: ['strava'],
  configSchema: [],
  destinationType: 1, // DestinationType.DESTINATION_STRAVA
  marketingDescription: `
### Share Your Boosted Activities
Upload your boosted activities to Strava automatically. Your AI descriptions, muscle heatmaps, and merged heart rate data will appear on your Strava feed.

### How it works
Once activities pass through your Pipeline, FitGlue uploads them to Strava via the official API. They appear as native Strava activities, complete with all the Boosters you've configured.
  `,
  features: [
    '✅ Upload activities to Strava automatically',
    '✅ AI descriptions and muscle heatmaps included',
    '✅ Heart rate and GPS data attached',
    '✅ Secure OAuth connection to your Strava account',
  ],
  transformations: [],
  useCases: [],
  externalUrlTemplate: 'https://www.strava.com/activities/{id}',
  // UX Organization
  category: 'social',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 95,
  iconType: 'svg',
  iconPath: '/images/icons/strava.svg',
});

registerDestination({
  id: 'showcase',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'Showcase',
  description: 'Generate a public, shareable link to your boosted activity',
  icon: '🔗',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  destinationType: 2, // DestinationType.DESTINATION_SHOWCASE
  marketingDescription: `
### Share Your Magic
Create beautiful, public links to your boosted activities. Share your activity data—HR graphs, GPS maps, Boosters applied—with anyone, no login required.

### Tiered Retention
- **Hobbyist**: Showcase links expire after 1 month
- **Athlete**: Showcase links never expire
  `,
  features: [
    '✅ Beautiful public activity pages',
    '✅ Heart rate graphs and GPS maps',
    '✅ Shows all applied FitGlue boosters',
    '✅ Social sharing with rich previews',
    '✅ Mobile-optimized design',
  ],
  transformations: [],
  useCases: [
    'Share activities with friends and coaches',
    'Embed activity summaries in blogs',
    'Create a public fitness portfolio',
  ],
  // Note: externalUrlTemplate is injected by registry-handler based on environment
  externalUrlTemplate: '',
  // UX Organization
  category: 'social',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 85,
});

registerDestination({
  id: 'hevy',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'Hevy',
  description: 'Upload boosted activities to Hevy',
  icon: '🏋️',
  enabled: true,
  requiredIntegrations: ['hevy'],
  configSchema: [],
  destinationType: 3, // DestinationType.DESTINATION_HEVY
  marketingDescription: `
### Complete Activity Target
Upload all your activities to Hevy - from strength training to cardio. Perfect for centralizing your complete fitness history.

### How it works
Activities pass through your Pipeline and are uploaded to Hevy via the official API. Strength sets, reps, and weights are preserved. Cardio activities (runs, rides, walks) are mapped to Hevy's distance-based exercise templates.

### Smart Template Matching
Exercise names are fuzzy-matched to Hevy's library. Unknown exercises automatically create custom templates.
  `,
  features: [
    '✅ Upload all activity types to Hevy',
    '✅ Strength sets, reps, and weights preserved',
    '✅ Cardio mapped to distance-based templates',
    '✅ Smart exercise template matching',
  ],
  transformations: [],
  useCases: [
    'Log Strava runs in Hevy for complete history',
    'Centralize gym workouts from multiple sources',
    'Track Fitbit activities in Hevy',
  ],
  externalUrlTemplate: 'https://hevy.com/workout/{id}',
  // UX Organization
  category: 'logging',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 75,
  iconType: 'svg',
  iconPath: '/images/icons/hevy.svg',
});

registerDestination({
  id: 'trainingpeaks',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'TrainingPeaks',
  description: 'Upload activities to TrainingPeaks',
  icon: '📈',
  enabled: true,
  requiredIntegrations: ['trainingpeaks'],
  configSchema: [],
  destinationType: 4, // DestinationType.DESTINATION_TRAININGPEAKS
  marketingDescription: `
### Professional Training Platform
Upload your boosted activities to TrainingPeaks. Perfect for athletes who use TrainingPeaks for training planning and analysis.

### How it works
Activities pass through your Pipeline and are uploaded to TrainingPeaks via the official API. Duration, distance, heart rate data, and activity type are all preserved.

### Smart Activity Mapping
Activity types are automatically mapped to TrainingPeaks workout types (Run, Bike, Swim, Strength, Other).
  `,
  features: [
    '✅ Upload activities to TrainingPeaks automatically',
    '✅ Duration, distance, and heart rate data included',
    '✅ Activity types mapped intelligently',
    '✅ Secure OAuth connection',
  ],
  isTemporarilyUnavailable: true,
  transformations: [],
  useCases: [
    'Sync gym workouts to your training calendar',
    'Track all activities in one training platform',
    'Analyze training load across sources',
  ],
  externalUrlTemplate: 'https://app.trainingpeaks.com/workout/{id}',
  // UX Organization
  category: 'analytics',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 70,
  iconType: 'png',
  iconPath: '/images/icons/trainingpeaks.png',
});

registerDestination({
  id: 'intervals',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'Intervals.icu',
  description: 'Upload boosted activities to Intervals.icu',
  icon: '📊',
  enabled: true,
  requiredIntegrations: ['intervals'],
  configSchema: [],
  destinationType: 5, // DESTINATION_INTERVALS
  marketingDescription: `
### Advanced Training Analytics Platform
Upload your boosted activities to Intervals.icu for detailed power analysis, training load tracking, and fitness modeling. Perfect for cyclists and triathletes who want advanced analytics.

### How it works
Activities pass through your Pipeline and are uploaded to Intervals.icu via the official API. FIT files are uploaded with full data fidelity, preserving all power, heart rate, cadence, and GPS data. Your AI descriptions and enriched titles appear on your Intervals calendar.

### Complete Data Fidelity
Unlike some platforms, Intervals.icu accepts raw FIT files, meaning all your data—power curves, GPS tracks, metrics—are preserved exactly as recorded.
  `,
  features: [
    '✅ Upload activities to Intervals.icu automatically',
    '✅ FIT files uploaded for full data fidelity',
    '✅ Power, heart rate, cadence, and GPS preserved',
    '✅ AI descriptions and enriched titles included',
    '✅ Simple API key connection',
  ],
  isTemporarilyUnavailable: true,
  transformations: [],
  useCases: [
    'Analyze training load from multiple sources in Intervals.icu',
    'Build fitness/fatigue (CTL/ATL/TSB) from all activities',
    'Plan races with complete training history',
  ],
  externalUrlTemplate: 'https://intervals.icu/activities/{id}',
  // UX Organization
  category: 'analytics',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 70,
  iconType: 'png',
  iconPath: '/images/icons/intervals.png',
});

registerDestination({
  id: 'googlesheets',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'Google Sheets',
  description: 'Log activities to a Google Spreadsheet',
  icon: '📑',
  enabled: true,
  externalUrlTemplate: 'https://docs.google.com/spreadsheets/d/{spreadsheet_id}/edit#gid=0&range=A{row_number}',
  requiredIntegrations: ['google'],
  configSchema: [
    {
      key: 'spreadsheet_id',
      label: 'Spreadsheet ID',
      description: 'The ID from your Google Sheets URL (docs.google.com/spreadsheets/d/{ID}/edit)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: true,
      defaultValue: '',
      options: [],
    },
    {
      key: 'sheet_name',
      label: 'Sheet Name',
      description: 'Which tab to append activities to',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: 'Activities',
      options: [],
    },
    {
      key: 'include_showcase_link',
      label: 'Include Showcase Link',
      description: 'Add a column with the Showcase URL for each activity',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
    {
      key: 'include_visuals',
      label: 'Include Visual Assets',
      description: 'Add IMAGE formulas for muscle heatmaps and route thumbnails',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
  ],
  destinationType: 6, // DESTINATION_GOOGLESHEETS
  marketingDescription: `
### Personal Activity Log
Automatically log all your activities to a Google Sheet for personal tracking, analysis, and visualization. Perfect for creating custom dashboards and long-term fitness tracking.

### How it works
Once you connect your Google account and provide a Spreadsheet ID, FitGlue appends a new row for each activity. Columns include date, type, title, duration, distance, calories, heart rate, and more.

### Visual Assets
If enabled, muscle heatmaps and route thumbnails are embedded using Google Sheets IMAGE formulas, making your spreadsheet a rich visual log of your training.
  `,
  features: [
    '✅ Automatic activity logging to your spreadsheet',
    '✅ Customizable columns (date, type, stats, description)',
    '✅ Visual assets via IMAGE formulas',
    '✅ Showcase links for sharing',
    '✅ Perfect for custom dashboards and analysis',
  ],
  transformations: [],
  useCases: [
    'Create custom fitness dashboards in Google Sheets',
    'Track long-term training trends',
    'Share activity logs with coaches',
    'Build personal analytics with pivot tables',
  ],
  // UX Organization
  category: 'logging',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 65,
  iconType: 'svg',
  iconPath: '/images/icons/googlesheets.svg',
});

registerDestination({
  id: 'github',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'GitHub',
  description: 'Commit enriched activities to a GitHub repository',
  icon: '🐙',
  enabled: true,
  externalUrlTemplate: 'https://github.com/{repo}/blob/main/{file_path}',
  requiredIntegrations: ['github'],
  configSchema: [
    {
      key: 'repo',
      label: 'Repository',
      description: 'Full name of the target repository (e.g. your-username/fitness-log)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: true,
      defaultValue: '',
      options: [],
    },
    {
      key: 'folder',
      label: 'Folder Path',
      description: 'Root folder for committed activity files (e.g. workouts/)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: 'workouts/',
      options: [],
    },
  ],
  destinationType: 7, // DESTINATION_GITHUB
  marketingDescription: `
### What is it?
GitHub as a Destination commits your enriched activities as Markdown files to a Git repository. Each activity becomes a versioned document containing your title, description, booster output, and metadata — all stored in your own repo under your full control.

### How it works
After your activity passes through the FitGlue pipeline and all boosters have run, the enriched result is committed to your chosen repository. The file is placed inside your configured folder, organised by year, month, and date (e.g. \`workouts/2026/02/2026-02-08-morning-run/activity.md\`).

Commits are authored by "FitGlue Bot" so they are clearly distinguishable from your own work. If GitHub is also configured as a source, this author name is used for automatic loop prevention — FitGlue ignores its own commits.

### File Structure
Each committed Markdown file includes:
- **YAML frontmatter** — title, activity type, date, source, pipeline ID, applied enrichments, and tags
- **Heading and description** — the full enriched description with all booster sections
- **\`<!-- fitglue:end -->\` marker** — anything you write below this line is preserved across updates

### Safety & Privacy
- Only the configured folder is written to — FitGlue never modifies other files.
- Updates preserve any content you add below the \`fitglue:end\` marker.
- All commits use the FitGlue Bot identity for traceability.
- Your OAuth token is stored encrypted and scoped to the \`repo\` permission.
  `,
  features: [
    '✅ Full Git history for every activity update',
    '✅ Organised by year/month/date folder structure',
    '✅ User content preserved below fitglue:end marker',
    '✅ FitGlue Bot commits for clear attribution',
    '✅ Automatic loop prevention with source',
    '✅ Works with public and private repositories',
  ],
  transformations: [],
  useCases: [
    'Build a version-controlled fitness journal',
    'Create a public training log on GitHub Pages',
    'Archive enriched activities alongside your code',
    'Review training data through pull request workflows',
  ],
  // UX Organization
  category: 'developer',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 60,
  iconType: 'svg',
  iconPath: '/images/icons/github.svg',
});

// ============================================================================


// Register all known enricher manifests
// These match the Go plugin registrations in enricher_providers/
// ============================================================================

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_WORKOUT_SUMMARY, {
  id: 'workout-summary',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Workout Summary',
  description: 'Generates a text summary of strength training exercises',
  icon: '📋',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'format',
      label: 'Summary Format',
      description: 'How sets should be displayed',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'detailed',
      options: [
        { value: 'compact', label: 'Compact (4×10×100kg)' },
        { value: 'detailed', label: 'Detailed (4 x 10 × 100.0kg)' },
        { value: 'verbose', label: 'Verbose (4 sets of 10 reps at 100.0 kilograms)' },
      ],
    },
    {
      key: 'show_stats',
      label: 'Show Stats',
      description: 'Include headline stats (total volume, reps, etc.)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
  ],
  marketingDescription: `
### What is Workout Summary?
Generates clean, structured summaries of your strength training exercises with sets, reps, and weights. Instead of just raw data, you get a well-formatted breakdown of your session with volume stats and key highlights.

### How it works
FitGlue analyzes your sets, reps, and weight data, identifies your primary muscle groups targeted, and calculates total volume. It then formats a structured summary that highlights your heaviest lifts, total volume, and overall rep count.
  `,
  features: [
    '✅ Narrative summaries of your strength workouts',
    '✅ Highlights key lifts and personal bests',
    '✅ Analyzes volume trends and intensity',
    '✅ Customizable formats (Compact, Detailed, Verbose)',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: '(empty)',
      after: `📋 Workout Summary:
20 sets • 8,240kg volume • 57 reps • Heaviest: 80kg (Bench Press)
• Bench Press: 4 × 8 × 80.0kg
• Overhead Press: 4 × 10 × 40.0kg
• Incline DB Press: 4 × 12 × 24.0kg
• Lateral Raises: 4 × 15 × 10.0kg
• Tricep Pushdowns: 4 × 12 × 25.0kg`,
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Share detailed strength logs on your favorite platforms',
    'Track progressive overload with volume stats',
    'Celebrate personal records automatically',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 90,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_MUSCLE_HEATMAP, {
  id: 'muscle-heatmap',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Muscle Heatmap',
  description: 'Generates an emoji-based heatmap showing muscle group volume',
  icon: '🔥',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'style',
      label: 'Display Style',
      description: 'How the heatmap should be rendered',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'emoji',
      options: [
        { value: 'emoji', label: 'Emoji Bars (🟪🟪🟪⬜⬜)' },
        { value: 'percentage', label: 'Percentage (Chest: 80%)' },
        { value: 'text', label: 'Text Only (High: Chest, Medium: Legs)' },
      ],
    },
    {
      key: 'bar_length',
      label: 'Bar Length',
      description: 'Number of squares in emoji bar',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER,
      required: false,
      defaultValue: '5',
      options: [],
      validation: { minValue: 3, maxValue: 10 },
      dependsOn: { fieldKey: 'style', values: ['emoji'] },
    },
    {
      key: 'preset',
      label: 'Coefficient Preset',
      description: 'Muscle weighting preset',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'standard',
      options: [
        { value: 'standard', label: 'Standard (balanced)' },
        { value: 'powerlifting', label: 'Powerlifting (emphasize compounds)' },
        { value: 'bodybuilding', label: 'Bodybuilding (emphasize isolation)' },
      ],
    },
    {
      key: 'group_by',
      label: 'Group By',
      description: 'Roll up muscles into broader groups (Legs, Back, Arms, etc.)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'muscle',
      options: [
        { value: 'muscle', label: 'Individual Muscles (Quadriceps, Lats, etc.)' },
        { value: 'muscle_group', label: 'Muscle Groups (Legs, Back, Arms, etc.)' },
      ],
    },
  ],
  marketingDescription: `
### Visualize Your Training
The Muscle Heatmap booster generates a visual representation of your training volume by muscle group. Using a heatmap style visualization, you can instantly see which muscles you hit hardest and which ones might be lagging.

### How it works
Every exercise in your workout is mapped to primary and secondary muscle groups using our built-in exercise taxonomy. We calculate the volume load for each muscle and generate a "heatmap" bar or chart that is appended to your activity description.

### Group By Muscle Groups
Too much detail? Enable the **Group By** option to roll up individual muscles (Quadriceps, Hamstrings, Glutes...) into broader categories like **Legs**, **Back**, **Arms**, **Chest**, **Shoulders**, and **Core** for a cleaner overview.

### Smart Exercise Recognition
Our database includes 100+ canonical exercises with fuzzy matching, so even custom-named exercises like "Dave's Bench Press" are correctly identified. Abbreviations (DB, BB, KB) are automatically expanded, and typos are handled gracefully.
  `,
  features: [
    '✅ Visual heatmap of trained muscles',
    '✅ Supports Emoji, Percentage, and Text formats',
    '✅ Group by individual muscles or roll up into categories (Legs, Back, Arms, etc.)',
    '✅ Smart exercise recognition with fuzzy matching',
    '✅ 100+ exercises in canonical database',
    '✅ Adjustable muscle coefficients (Standard, Powerlifting, Bodybuilding)',
    '✅ Works with all strength activities',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Muscle Heatmap (Individual)',
      before: 'Weight Training\n45 min',
      after: '',
      visualType: '',
      afterHtml: '<strong>🔥 Muscle Activation</strong><br><br><span class="heatmap-row">Chest: <span class="heatmap-bar high">🟪🟪🟪🟪🟪</span></span><br><span class="heatmap-row">Shoulders: <span class="heatmap-bar high">🟪🟪🟪🟪⬛</span></span><br><span class="heatmap-row">Triceps: <span class="heatmap-bar med">🟪🟪⬛⬛⬛</span></span><br><span class="heatmap-row">Core: <span class="heatmap-bar low">🟪⬛⬛⬛⬛</span></span>',
    },
    {
      field: 'description',
      label: 'Muscle Heatmap (Grouped)',
      before: 'Weight Training\n45 min',
      after: '',
      visualType: '',
      afterHtml: '<strong>🔥 Muscle Activation</strong><br><br><span class="heatmap-row">Legs: <span class="heatmap-bar high">🟪🟪🟪🟪🟪</span></span><br><span class="heatmap-row">Back: <span class="heatmap-bar high">🟪🟪🟪🟪⬛</span></span><br><span class="heatmap-row">Arms: <span class="heatmap-bar med">🟪🟪🟪⬛⬛</span></span><br><span class="heatmap-row">Core: <span class="heatmap-bar low">🟪⬛⬛⬛⬛</span></span>',
    },
  ],
  useCases: [
    'Visualize muscle balance in your program',
    'Show training focus areas on your activity feed',
    'Identify lagging muscle groups',
    'Track custom exercises with automatic muscle mapping',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 85,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_MUSCLE_HEATMAP_IMAGE, {
  id: 'muscle_heatmap_image',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Muscle Heatmap Image',
  description: 'Generates an anatomical SVG diagram with muscle activation highlighting',
  icon: '🖼️',
  enabled: true,
  requiredIntegrations: [],
  requiredTier: 'athlete',
  configSchema: [
    {
      key: 'preset',
      label: 'Coefficient Preset',
      description: 'Muscle weighting preset (same options as text heatmap)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'standard',
      options: [
        { value: 'standard', label: 'Standard (balanced)' },
        { value: 'powerlifting', label: 'Powerlifting (emphasize compounds)' },
        { value: 'bodybuilding', label: 'Bodybuilding (emphasize isolation)' },
      ],
    },
    {
      key: 'gender',
      label: 'Anatomy Model',
      description: 'Select the anatomical model for the heatmap',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'man',
      options: [
        { value: 'man', label: 'Male' },
        { value: 'woman', label: 'Female' },
      ],
    },
  ],
  marketingDescription: `
### Premium Visual Muscle Activation
The Muscle Heatmap Image booster creates a stunning anatomical diagram showing exactly which muscles you trained and how hard. Unlike the text-based heatmap, this generates a shareable visual asset perfect for social media and the Showcase.

### How it works
Every exercise is analyzed to calculate volume per muscle group. We then generate an SVG image of the human body with muscles colored by intensity - from gray (no activation) through purple gradients to hot pink (maximum activation).

### Athlete-Tier Exclusive
This premium visual enricher is available exclusively to Athlete-tier users. The generated image is stored in Cloud Storage and automatically embedded in your Showcase page.
  `,
  features: [
    '✅ Anatomical SVG diagram with muscle highlighting',
    '✅ Color-coded intensity (gray → purple → hot pink)',
    '✅ Stored as shareable image asset',
    '✅ Automatically embedded in Showcase',
    '✅ Perfect for social media sharing',
    '✅ Athlete-tier exclusive',
  ],
  transformations: [
    {
      field: 'metadata',
      label: 'Muscle Activation Image',
      before: '(no image)',
      after: '',
      visualType: 'image',
      afterHtml: '',
    },
  ],
  useCases: [
    'Share visual muscle activation on social media',
    'Track training balance with anatomical diagrams',
    'Create professional-looking workout summaries',
  ],
  // UX Organization
  category: 'ai_images',
  sortOrder: 4,
  isPremium: true,
  popularityScore: 95,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_ROUTE_THUMBNAIL, {
  id: 'route_thumbnail',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Route Thumbnail',
  description: 'Generates a stunning SVG map image of your GPS route',
  icon: '🗺️',
  enabled: true,
  requiredIntegrations: [],
  requiredTier: 'athlete',
  configSchema: [],
  marketingDescription: `
### Beautiful Route Visualization
The Route Thumbnail booster creates a beautiful stylized SVG map of your GPS route. Perfect for sharing on social media and displaying in your Showcase.

### How it works
FitGlue extracts GPS coordinates from your activity, simplifies the route using the Douglas-Peucker algorithm for optimal rendering, and generates a stunning SVG visualization with FitGlue's signature gradient colors and animated start/finish markers.

### Athlete-Tier Exclusive
This premium visual enricher is available exclusively to Athlete-tier users. The generated SVG is stored in Cloud Storage and automatically embedded in your Showcase page.
  `,
  features: [
    '✅ Stunning SVG route visualization',
    '✅ FitGlue signature gradient colors (pink → purple → green)',
    '✅ Animated start and finish markers',
    '✅ Route simplification for clean rendering',
    '✅ Stored as shareable image asset',
    '✅ Automatically embedded in Showcase',
    '✅ Athlete-tier exclusive',
  ],
  transformations: [
    {
      field: 'metadata',
      label: 'Route Thumbnail URL',
      before: '(no route image)',
      after: 'showcase-assets/{activityId}/route-thumbnail.svg',
      visualType: 'image',
      afterHtml: '',
    },
  ],
  useCases: [
    'Share beautiful route maps on social media',
    'Display route visualization in Showcase header',
    'Create visual summaries of running and cycling routes',
  ],
  // UX Organization
  category: 'ai_images',
  sortOrder: 1,
  isPremium: true,
  popularityScore: 85,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_AI_BANNER, {
  id: 'ai_banner',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'AI Activity Banner',
  description: 'Generates a custom AI header image for your activity',
  icon: '🎨',
  enabled: true,
  requiredIntegrations: [],
  requiredTier: 'athlete',
  configSchema: [
    {
      key: 'style',
      label: 'Image Style',
      description: 'Visual style for the generated banner',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'vibrant',
      options: [
        { value: 'vibrant', label: 'Vibrant & Energetic' },
        { value: 'minimal', label: 'Minimal & Clean' },
        { value: 'dramatic', label: 'Dramatic & Bold' },
      ],
    },
    {
      key: 'subject',
      label: 'Subject Type',
      description: 'What type of subject to show in the banner',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'abstract',
      options: [
        { value: 'male', label: 'Male Athlete' },
        { value: 'female', label: 'Female Athlete' },
        { value: 'abstract', label: 'Abstract (No People)' },
      ],
    },
  ],
  marketingDescription: `
### AI-Generated Activity Banners
Create stunning, custom header images for your activities using Google's Gemini AI. Each banner is uniquely generated based on your activity type, time of day, and conditions.

### How it works
When enabled, FitGlue analyzes your activity context and generates a custom banner image using Gemini 2.0 Flash. The image is stored in Cloud Storage and displayed as the hero background in your Showcase page.

### Athlete-Tier Exclusive
This premium visual enricher is available exclusively to Athlete-tier users. The generated image is also used for OpenGraph/Twitter social sharing cards.
  `,
  features: [
    '✅ AI-generated custom banners for each activity',
    '✅ Context-aware (activity type, time of day)',
    '✅ Multiple style options (Vibrant, Minimal, Dramatic)',
    '✅ Displayed as Showcase hero background',
    '✅ Used for social sharing (OpenGraph/Twitter cards)',
    '✅ Athlete-tier exclusive',
  ],
  transformations: [
    {
      field: 'metadata',
      label: 'AI Banner URL',
      before: '(no banner)',
      after: 'showcase-assets/{activityId}/banner.png',
      visualType: 'image',
      afterHtml: '',
    },
  ],
  useCases: [
    'Create eye-catching social media shares',
    'Professional-looking activity showcases',
    'Unique visual identity for each workout',
  ],
  // UX Organization
  category: 'ai_images',
  sortOrder: 2,
  isPremium: true,
  popularityScore: 75,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_FITBIT_HEART_RATE, {
  id: 'fitbit-heart-rate',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Fitbit Heart Rate',
  description: 'Adds heart rate data from Fitbit to your activity with smart GPS alignment',
  icon: '🩵',
  enabled: true,
  requiredIntegrations: ['fitbit'],
  configSchema: [
    {
      key: 'force',
      label: 'Force Overwrite',
      description: 'Overwrite existing heart rate data if present',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
  ],
  marketingDescription: `
### Unified Heart Data
Sync your heart rate data from your Fitbit device and overlay it onto your imported activities. This is perfect for when you track a workout (like weightlifting) on one app but wear your Fitbit for health monitoring.

### How it works
When an activity is imported (e.g., from a strength training app), FitGlue checks your Fitbit account for heart rate data recorded during that time window. It creates a second-by-second heart rate stream and attaches it to the activity before sending it to your activity feed or other destinations.

### Smart GPS Alignment
When your activity has GPS data (from a phone app or watch), FitGlue uses an "Elastic Match" algorithm to align Fitbit heart rate data with your GPS timestamps. This handles minor clock drift between devices automatically, ensuring your HR matches the correct location points within ±2 seconds accuracy.
  `,
  features: [
    '✅ Merges heart rate from Fitbit to any activity',
    '✅ Smart GPS alignment handles clock drift between devices',
    '✅ Perfect for activities where you don\'t start a GPS watch',
    '✅ Accurate calorie data based on heart rate',
    '✅ Linear interpolation for precise timestamp matching',
    '✅ Seamless background synchronization',
  ],
  transformations: [
    {
      field: 'heartRateStream',
      label: 'Heart Rate Data',
      before: '(no heart rate)',
      after: '',
      visualType: 'hr-graph',
      afterHtml: '',
    },
  ],
  useCases: [
    'Add heart rate to gym workouts',
    'Merge Fitbit HR with phone GPS running data',
    'Complete activity data on Strava',
    'Track training intensity across all activities',
  ],
  // UX Organization
  category: 'data',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 80,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_FIT_FILE_HEART_RATE, {
  id: 'fit-file-heart-rate',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'FIT File Heart Rate',
  description: 'Upload a FIT file to add heart rate data to your activity with smart GPS alignment',
  icon: '💓',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'force',
      label: 'Force Overwrite',
      description: 'Overwrite existing heart rate data if present',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
  ],
  marketingDescription: `
### Merge Heart Rate from FIT Files
Upload a FIT file from another device or activity to merge heart rate data into your current activity. Perfect for activities where HR was recorded on a separate device.

### How it works
When an activity is imported without heart rate data (e.g., treadmill or Pool swim), FitGlue creates a pending input request for a FIT file upload. You can then upload the FIT file containing HR data (e.g., from a chest strap syncing to a different device or from Peloton). FitGlue extracts the heart rate samples and merges them with your activity.

### Smart GPS Alignment
When your activity has GPS data, FitGlue uses an "Elastic Match" algorithm to align the uploaded heart rate data with your GPS timestamps. This handles minor clock drift between devices automatically, ensuring your HR matches the correct location points.
  `,
  features: [
    '✅ Upload FIT files with heart rate data',
    '✅ Smart GPS alignment handles clock drift between devices',
    '✅ Perfect for activities where HR was recorded separately',
    '✅ Supports chest straps, smartwatches, and indoor equipment',
    '✅ Linear interpolation for precise timestamp matching',
  ],
  transformations: [
    {
      field: 'heartRateStream',
      label: 'Heart Rate Data',
      before: '(no heart rate)',
      after: '',
      visualType: 'hr-graph',
      afterHtml: '',
    },
  ],
  useCases: [
    'Add heart rate from indoor bikes to indoor workouts',
    'Merge chest strap HR with phone-tracked activities',
    'Combine HR from one device with GPS from another',
    'Complete heart rate data on pool swims',
  ],
  // UX Organization
  category: 'data',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 75,
});


registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_VIRTUAL_GPS, {
  id: 'virtual-gps',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Virtual GPS',
  description: 'Adds GPS coordinates from a virtual route to indoor activities',
  icon: '🌍',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'route',
      label: 'Route',
      description: 'Virtual route to use for GPS generation',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'london',
      options: [
        { value: 'london', label: 'London Hyde Park (~4km loop)' },
        { value: 'nyc', label: 'NYC Central Park (~10km loop)' },
      ],
    },
    {
      key: 'force',
      label: 'Force Override',
      description: 'Override existing GPS data if present',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
  ],
  marketingDescription: `
### Virtual GPS for Indoor Workouts
Add GPS coordinates to indoor activities so they appear with a map on your activity feed. Choose from preset routes in famous locations like London’s Hyde Park or NYC’s Central Park.

### How it works
When an activity is processed, Virtual GPS overlays a pre-defined GPS route onto the activity. The route is scaled to match your workout duration, giving your indoor session a scenic virtual location.
  `,
  features: [
    '✅ Adds GPS to indoor/gym activities',
    '✅ Activities appear with a map on your activity feed',
    '✅ Choice of scenic routes (London, NYC)',
    '✅ Route scaled to match workout duration',
  ],
  transformations: [
    {
      field: 'gpsData',
      label: 'GPS Routes',
      before: 'Indoor workout\nNo location',
      after: '',
      visualType: 'gps-map',
      afterHtml: '',
    },
    {
      field: 'description',
      label: 'Virtual Tour Message',
      before: '(empty)',
      after: '🗺️ Took a virtual tour of London Hyde Park (GPS generated for this indoor workout)',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Get indoor activities on your activity heatmap',
    'Add visual interest to home gym sessions',
    'Virtual touring while on the treadmill',
  ],
  // UX Organization
  category: 'data',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 70,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_SOURCE_LINK, {
  id: 'source-link',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Source Link',
  description: 'Appends a link to the original activity in the description',
  icon: '↗️',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### Link Back to the Source
Automatically appends a deep link to the original activity in your workout description. Great for cross-referencing your data.

### How it works
When activities are imported from sources, Source Link adds a clickable URL pointing back to the original activity. This makes it easy to see the full details in the source app.
  `,
  features: [
    '✅ Adds a link to the original activity',
    '✅ Easy cross-referencing between apps',
    '✅ Works with all source integrations',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Upper Body Workout',
      after: 'Upper Body Workout\n\n🔗 View in Source App: https://example.com/activity/abc123',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Trace activities back to the source app',
    'Keep links to detailed exercise data',
    'Cross-reference between platforms',
  ],
  // UX Organization
  category: 'links',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 65,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_TYPE_MAPPER, {
  id: 'type-mapper',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Type Mapper',
  description: 'Maps activity types based on title keywords (e.g., title containing "Indoor Bike" → Virtual Ride)',
  icon: '🏷️',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'type_rules',
      label: 'Type Mapping Rules',
      description: 'Map activity titles to desired types. Enter the text to match (case-insensitive), then select the activity type.',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_KEY_VALUE_MAP,
      required: true,
      defaultValue: '',
      options: [],
      valueOptions: [
        { value: 'ACTIVITY_TYPE_ALPINE_SKI', label: 'Alpine Ski' },
        { value: 'ACTIVITY_TYPE_BACKCOUNTRY_SKI', label: 'Backcountry Ski' },
        { value: 'ACTIVITY_TYPE_BADMINTON', label: 'Badminton' },
        { value: 'ACTIVITY_TYPE_CANOEING', label: 'Canoeing' },
        { value: 'ACTIVITY_TYPE_CROSSFIT', label: 'Crossfit' },
        { value: 'ACTIVITY_TYPE_EBIKE_RIDE', label: 'E-Bike Ride' },
        { value: 'ACTIVITY_TYPE_ELLIPTICAL', label: 'Elliptical' },
        { value: 'ACTIVITY_TYPE_EMOUNTAIN_BIKE_RIDE', label: 'E-Mountain Bike Ride' },
        { value: 'ACTIVITY_TYPE_GOLF', label: 'Golf' },
        { value: 'ACTIVITY_TYPE_GRAVEL_RIDE', label: 'Gravel Ride' },
        { value: 'ACTIVITY_TYPE_HANDCYCLE', label: 'Handcycle' },
        { value: 'ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING', label: 'HIIT' },
        { value: 'ACTIVITY_TYPE_HIKE', label: 'Hike' },
        { value: 'ACTIVITY_TYPE_ICE_SKATE', label: 'Ice Skate' },
        { value: 'ACTIVITY_TYPE_INLINE_SKATE', label: 'Inline Skate' },
        { value: 'ACTIVITY_TYPE_KAYAKING', label: 'Kayaking' },
        { value: 'ACTIVITY_TYPE_KITESURF', label: 'Kitesurf' },
        { value: 'ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE', label: 'Mountain Bike Ride' },
        { value: 'ACTIVITY_TYPE_NORDIC_SKI', label: 'Nordic Ski' },
        { value: 'ACTIVITY_TYPE_PICKLEBALL', label: 'Pickleball' },
        { value: 'ACTIVITY_TYPE_PILATES', label: 'Pilates' },
        { value: 'ACTIVITY_TYPE_RACQUETBALL', label: 'Racquetball' },
        { value: 'ACTIVITY_TYPE_RIDE', label: 'Ride' },
        { value: 'ACTIVITY_TYPE_ROCK_CLIMBING', label: 'Rock Climbing' },
        { value: 'ACTIVITY_TYPE_ROLLER_SKI', label: 'Roller Ski' },
        { value: 'ACTIVITY_TYPE_ROWING', label: 'Rowing' },
        { value: 'ACTIVITY_TYPE_RUN', label: 'Run' },
        { value: 'ACTIVITY_TYPE_SAIL', label: 'Sail' },
        { value: 'ACTIVITY_TYPE_SKATEBOARD', label: 'Skateboard' },
        { value: 'ACTIVITY_TYPE_SNOWBOARD', label: 'Snowboard' },
        { value: 'ACTIVITY_TYPE_SNOWSHOE', label: 'Snowshoe' },
        { value: 'ACTIVITY_TYPE_SOCCER', label: 'Soccer' },
        { value: 'ACTIVITY_TYPE_SQUASH', label: 'Squash' },
        { value: 'ACTIVITY_TYPE_STAIR_STEPPER', label: 'Stair Stepper' },
        { value: 'ACTIVITY_TYPE_STAND_UP_PADDLING', label: 'Stand Up Paddling' },
        { value: 'ACTIVITY_TYPE_SURFING', label: 'Surfing' },
        { value: 'ACTIVITY_TYPE_SWIM', label: 'Swim' },
        { value: 'ACTIVITY_TYPE_TABLE_TENNIS', label: 'Table Tennis' },
        { value: 'ACTIVITY_TYPE_TENNIS', label: 'Tennis' },
        { value: 'ACTIVITY_TYPE_TRAIL_RUN', label: 'Trail Run' },
        { value: 'ACTIVITY_TYPE_VELOMOBILE', label: 'Velomobile' },
        { value: 'ACTIVITY_TYPE_VIRTUAL_RIDE', label: 'Virtual Ride' },
        { value: 'ACTIVITY_TYPE_VIRTUAL_ROW', label: 'Virtual Row' },
        { value: 'ACTIVITY_TYPE_VIRTUAL_RUN', label: 'Virtual Run' },
        { value: 'ACTIVITY_TYPE_WALK', label: 'Walk' },
        { value: 'ACTIVITY_TYPE_WEIGHT_TRAINING', label: 'Weight Training' },
        { value: 'ACTIVITY_TYPE_WHEELCHAIR', label: 'Wheelchair' },
        { value: 'ACTIVITY_TYPE_WINDSURF', label: 'Windsurf' },
        { value: 'ACTIVITY_TYPE_WORKOUT', label: 'Workout' },
        { value: 'ACTIVITY_TYPE_YOGA', label: 'Yoga' },
      ],
    },
  ],
  marketingDescription: `
### Remap Activity Types by Title
Automatically change an activity's type based on keywords in the title. Perfect for when your source app doesn't correctly categorize your workouts.

### How it works
Define matching rules like "title contains 'Virtual Ride'" → "Virtual Ride" or "title contains 'Treadmill'" → "Run". When activities are processed, their type is automatically updated if the title matches your pattern.
  `,
  features: [
    '✅ Match activity titles with keywords',
    '✅ Full activity type dropdown',
    '✅ Case-insensitive matching',
    '✅ Multiple rules per enricher',
  ],
  transformations: [
    { field: 'activityType', label: 'Activity Type', before: 'Workout (from source)', after: 'Virtual Ride (matched "Virtual Ride" in title)', visualType: '', afterHtml: '' },
  ],
  useCases: [
    'Categorize indoor cycling sessions',
    'Mark treadmill runs correctly',
    'Fix incorrect activity types from source apps',
  ],
  // UX Organization
  category: 'detection',
  sortOrder: 1,
  isPremium: false,
  popularityScore: 55,
  allowMultipleInstances: true,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_PARKRUN, {
  id: 'parkrun',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Parkrun',
  description: 'Detects Parkrun events, sets title, and enriches with official results',
  icon: '🏃',
  enabled: true,
  requiredIntegrations: ['parkrun'], // Required for official results fetching
  configSchema: [
    {
      key: 'enable_titling',
      label: 'Set Title',
      description: 'Replace activity title with Parkrun event name',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
    {
      key: 'title_pattern',
      label: 'Title Pattern',
      description: 'Template for activity title. Use {location} for event name, {date} for date, {special} for holiday name.',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: '{location}',
      options: [],
      dependsOn: { fieldKey: 'enable_titling', values: ['true'] },
    },
    {
      key: 'special_title_pattern',
      label: 'Special Event Title',
      description: 'Title pattern for Christmas/New Year. Use {location}, {date}, {special}.',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: '{location} - {special} Edition',
      options: [],
      dependsOn: { fieldKey: 'enable_titling', values: ['true'] },
    },
    {
      key: 'fetch_results',
      label: 'Fetch Official Results',
      description: 'Automatically fetch your official position, time, and age grade after the event',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
    {
      key: 'tags',
      label: 'Tags',
      description: 'Comma-separated tags to add when matched (e.g., Parkrun)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: 'Parkrun',
      options: [],
    },
  ],
  marketingDescription: `
### Automatic Parkrun Detection & Results
Automatically detects when your activity is a Parkrun event based on GPS location and time, then boosts it with your official results.

### How it works
1. **Detection**: If your Saturday morning run starts near any of 2,500+ Parkrun locations at the right time, FitGlue recognizes it
2. **Immediate Update**: Your activity title is updated to "Parkrun @ [Location]"
3. **Delayed Results**: ~2 hours after your run, we fetch your **official results** from parkrun.org and update your activity with position, time, age grade, and PB status

### Special Events
Christmas Day and New Year's Day Parkruns get festive titles automatically!
  `,
  features: [
    '✅ Automatic Parkrun detection via GPS',
    '✅ 2,500+ worldwide Parkrun locations supported',
    '✅ Official results fetched automatically',
    '✅ Position, time, and age grade in description',
    '✅ PB celebrations highlighted 🎉',
    '✅ Special Christmas & New Year event titles',
    '✅ Customizable title patterns',
  ],
  transformations: [
    { field: 'title', label: 'Activity Title', before: 'Morning Run', after: 'Parkrun @ Newark', visualType: '', afterHtml: '' },
    { field: 'description', label: 'Official Results', before: '(empty)', after: '🏃 Parkrun Results:\n• Position: 42nd · 🏆 New all-time PB!\n• Time: 24:12\n• Age Grade: 65.2%\n• Location: Newark Parkrun, Parkrun #8 here (42 total)', visualType: '', afterHtml: '' },
  ],
  useCases: [
    'Auto-name Parkrun activities with official results',
    'Track your Parkrun positions and times',
    'Celebrate PBs with automatic highlighting',
    'Special event detection for Christmas & New Year',
  ],
  // UX Organization
  category: 'detection',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 75,
});


registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_CONDITION_MATCHER, {
  id: 'condition-matcher',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Condition Matcher',
  description: 'Applies title/description templates when conditions match (type, day, time, location)',
  icon: '🎯',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'activity_type',
      label: 'Activity Type',
      description: 'Match specific activity type (optional)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: '',
      options: [
        { value: '', label: '(Any)' },
        { value: 'ACTIVITY_TYPE_RUN', label: 'Run' },
        { value: 'ACTIVITY_TYPE_TRAIL_RUN', label: 'Trail Run' },
        { value: 'ACTIVITY_TYPE_WALK', label: 'Walk' },
        { value: 'ACTIVITY_TYPE_HIKE', label: 'Hike' },
        { value: 'ACTIVITY_TYPE_RIDE', label: 'Ride' },
        { value: 'ACTIVITY_TYPE_VIRTUAL_RIDE', label: 'Virtual Ride' },
        { value: 'ACTIVITY_TYPE_WEIGHT_TRAINING', label: 'Weight Training' },
        { value: 'ACTIVITY_TYPE_WORKOUT', label: 'Workout' },
        { value: 'ACTIVITY_TYPE_YOGA', label: 'Yoga' },
        { value: 'ACTIVITY_TYPE_SWIM', label: 'Swim' },
        { value: 'ACTIVITY_TYPE_CROSSFIT', label: 'Crossfit' },
        { value: 'ACTIVITY_TYPE_ELLIPTICAL', label: 'Elliptical' },
        { value: 'ACTIVITY_TYPE_ROWING', label: 'Rowing' },
      ],
    },
    {
      key: 'days_of_week',
      label: 'Days of Week',
      description: 'Match activities on these days (optional)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_MULTI_SELECT,
      required: false,
      defaultValue: '',
      options: [
        { value: 'Mon', label: 'Monday' },
        { value: 'Tue', label: 'Tuesday' },
        { value: 'Wed', label: 'Wednesday' },
        { value: 'Thu', label: 'Thursday' },
        { value: 'Fri', label: 'Friday' },
        { value: 'Sat', label: 'Saturday' },
        { value: 'Sun', label: 'Sunday' },
      ],
    },
    { key: 'start_time', label: 'Start Time', description: 'Earliest time, 24-hour format e.g. 09:00 (optional)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'end_time', label: 'End Time', description: 'Latest time, 24-hour format e.g. 17:00 (optional)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'location_lat', label: 'Location Latitude', description: 'Target latitude (optional)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER, required: false, defaultValue: '', options: [] },
    { key: 'location_long', label: 'Location Longitude', description: 'Target longitude (optional)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER, required: false, defaultValue: '', options: [] },
    { key: 'radius_m', label: 'Radius (meters)', description: 'Match radius around location (optional)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER, required: false, defaultValue: '200', options: [] },
    { key: 'title_template', label: 'Title Template', description: 'New title when conditions match, e.g. "Morning Gym Session"', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'description_template', label: 'Description Template', description: 'New description when conditions match', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
  ],
  marketingDescription: `
### Smart Conditional Templates
Apply custom titles and descriptions based on when, where, and what type of activity you’re doing.

### How it works
Define conditions like "Saturday morning run near the park" and specify a title template. When activities match your conditions, the template is applied automatically. Perfect for recurring workouts like "Morning Gym Session" or "Sunday Long Run".
  `,
  features: [
    '✅ Match by activity type, day, time, or location',
    '✅ Custom title and description templates',
    '✅ Perfect for recurring workout names',
    '✅ Radius-based location matching',
  ],
  transformations: [
    { field: 'title', label: 'Activity Title', before: 'Workout', after: 'Morning Gym Session', visualType: '', afterHtml: '' },
    { field: 'description', label: 'Custom Description', before: '(empty)', after: '(your template text)', visualType: '', afterHtml: '' },
  ],
  useCases: [
    'Auto-title recurring workouts',
    'Name activities by location',
    'Set titles by day or time',
  ],
  // UX Organization
  category: 'detection',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 60,
  allowMultipleInstances: true,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_AUTO_INCREMENT, {
  id: 'auto-increment',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Auto Increment',
  description: 'Appends an incrementing counter number to activity titles',
  icon: '🔢',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'counter_key',
      label: 'Counter Key',
      description: 'Select existing counter or create new one',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_DYNAMIC_SELECT,
      required: true,
      defaultValue: '',
      options: [],
      dynamicSource: 'counters', // Fetches from /users/me/counters
    },
    { key: 'title_contains', label: 'Title Filter', description: 'Only increment if title contains this (optional)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'initial_value', label: 'Initial Value', description: 'Starting number (optional, defaults to 1)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER, required: false, defaultValue: '1', options: [] },
  ],
  marketingDescription: `
### Numbered Activity Series
Automatically add incrementing numbers to your activity titles. Great for tracking workout series.

### How it works
Define a counter key and optional title filter. Activities matching the filter get an incrementing number appended, like "Leg Day #1", "Leg Day #2", etc. Each counter key maintains its own sequence.
  `,
  features: [
    '✅ Automatic sequential numbering',
    '✅ Multiple independent counters',
    '✅ Title filtering for targeted numbering',
    '✅ Configurable starting value',
  ],
  transformations: [
    { field: 'title', label: 'Activity Title', before: 'Leg Day', after: 'Leg Day #42', visualType: '', afterHtml: '' },
  ],
  useCases: [
    'Number workout series',
    'Track session counts',
    'Create numbered runs',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 45,
  allowMultipleInstances: true,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_USER_INPUT, {
  id: 'user-input',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'User Input',
  description: 'Pauses pipeline to wait for user input (title, description, etc.)',
  icon: '✍️',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'fields',
      label: 'Required Fields',
      description: 'Which fields you want to provide (optional, defaults to description)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_MULTI_SELECT,
      required: false,
      defaultValue: 'description',
      options: [
        { value: 'title', label: 'Title' },
        { value: 'description', label: 'Description' },
      ],
    },
  ],
  marketingDescription: `
### Manual Intervention Point
Pauses the pipeline to let you manually add or edit activity details before continuing.

### How it works
When an activity reaches this booster, it’s held pending your input. You receive a notification and can update the title, description, or other fields. Once you confirm, the activity continues through the pipeline.
  `,
  features: [
    '✅ Manual title and description editing',
    '✅ Activity held until you approve',
    '✅ Notification when input is needed',
    '✅ Configurable required fields',
  ],
  transformations: [
    { field: 'title', label: 'Activity Title', before: 'Workout', after: '(your custom title)', visualType: '', afterHtml: '' },
    { field: 'description', label: 'Activity Description', before: '(empty)', after: '(your custom description)', visualType: '', afterHtml: '' },
  ],
  useCases: [
    'Add personal notes to activities',
    'Review before publishing',
    'Custom titles per workout',
  ],
  // UX Organization
  category: 'workflow',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 50,
  allowMultipleInstances: true,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_ACTIVITY_FILTER, {
  id: 'activity-filter',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Activity Filter',
  description: 'Skips activities matching exclude patterns or not matching include patterns',
  icon: '🚫',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'exclude_activity_types',
      label: 'Exclude Activity Types',
      description: 'Skip activities of these types',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_MULTI_SELECT,
      required: false,
      defaultValue: '',
      options: [
        { value: 'ACTIVITY_TYPE_RUN', label: 'Run' },
        { value: 'ACTIVITY_TYPE_TRAIL_RUN', label: 'Trail Run' },
        { value: 'ACTIVITY_TYPE_WALK', label: 'Walk' },
        { value: 'ACTIVITY_TYPE_HIKE', label: 'Hike' },
        { value: 'ACTIVITY_TYPE_RIDE', label: 'Ride' },
        { value: 'ACTIVITY_TYPE_VIRTUAL_RIDE', label: 'Virtual Ride' },
        { value: 'ACTIVITY_TYPE_WEIGHT_TRAINING', label: 'Weight Training' },
        { value: 'ACTIVITY_TYPE_WORKOUT', label: 'Workout' },
        { value: 'ACTIVITY_TYPE_YOGA', label: 'Yoga' },
        { value: 'ACTIVITY_TYPE_SWIM', label: 'Swim' },
        { value: 'ACTIVITY_TYPE_CROSSFIT', label: 'Crossfit' },
        { value: 'ACTIVITY_TYPE_ELLIPTICAL', label: 'Elliptical' },
        { value: 'ACTIVITY_TYPE_ROWING', label: 'Rowing' },
      ],
    },
    { key: 'exclude_title_contains', label: 'Exclude Titles Containing', description: 'Skip if title contains these patterns (comma-separated)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'exclude_description_contains', label: 'Exclude Descriptions Containing', description: 'Skip if description contains these patterns (comma-separated)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    {
      key: 'include_activity_types',
      label: 'Include Only Activity Types',
      description: 'Only allow activities of these types',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_MULTI_SELECT,
      required: false,
      defaultValue: '',
      options: [
        { value: 'ACTIVITY_TYPE_RUN', label: 'Run' },
        { value: 'ACTIVITY_TYPE_TRAIL_RUN', label: 'Trail Run' },
        { value: 'ACTIVITY_TYPE_WALK', label: 'Walk' },
        { value: 'ACTIVITY_TYPE_HIKE', label: 'Hike' },
        { value: 'ACTIVITY_TYPE_RIDE', label: 'Ride' },
        { value: 'ACTIVITY_TYPE_VIRTUAL_RIDE', label: 'Virtual Ride' },
        { value: 'ACTIVITY_TYPE_WEIGHT_TRAINING', label: 'Weight Training' },
        { value: 'ACTIVITY_TYPE_WORKOUT', label: 'Workout' },
        { value: 'ACTIVITY_TYPE_YOGA', label: 'Yoga' },
        { value: 'ACTIVITY_TYPE_SWIM', label: 'Swim' },
        { value: 'ACTIVITY_TYPE_CROSSFIT', label: 'Crossfit' },
        { value: 'ACTIVITY_TYPE_ELLIPTICAL', label: 'Elliptical' },
        { value: 'ACTIVITY_TYPE_ROWING', label: 'Rowing' },
      ],
    },
    { key: 'include_title_contains', label: 'Include Only Titles Containing', description: 'Only allow if title contains one of these (comma-separated)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'include_description_contains', label: 'Include Only Descriptions Containing', description: 'Only allow if description contains one of these (comma-separated)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
  ],
  marketingDescription: `
### Filter Unwanted Activities
Skip activities you don’t want synced based on type or title patterns.

### How it works
Define include or exclude rules by activity type or title keywords. Activities matching exclude patterns (or not matching include patterns) are skipped and won’t be sent to destinations. Perfect for filtering out test workouts or specific activity types.
  `,
  features: [
    '✅ Include or exclude by activity type',
    '✅ Title keyword filtering',
    '✅ Stop unwanted activities from syncing',
    '✅ Flexible pattern matching',
  ],
  transformations: [],
  useCases: [
    'Skip test workouts',
    'Filter by activity type',
    'Only sync strength sessions',
  ],
  // UX Organization
  category: 'workflow',
  sortOrder: 4,
  isPremium: false,
  popularityScore: 50,
  allowMultipleInstances: true,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_LOGIC_GATE, {
  id: 'logic-gate',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Logic Gate',
  description: 'Evaluate rules to conditionally continue or halt the pipeline',
  icon: '🚦',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'match_mode',
      label: 'Match Mode',
      description: 'How rules are combined',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'all',
      options: [
        { value: 'all', label: 'All rules must match (AND)' },
        { value: 'any', label: 'Any rule matches (OR)' },
        { value: 'none', label: 'No rules match (NOR)' },
      ],
    },
    {
      key: 'rules',
      label: 'Rules',
      description: 'JSON array of rules: [{field, op, values, negate}]',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: true,
      defaultValue: '[]',
      options: [],
    },
    {
      key: 'on_match',
      label: 'On Match',
      description: 'Action when rules match',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'continue',
      options: [
        { value: 'continue', label: 'Continue pipeline' },
        { value: 'halt', label: 'Halt pipeline (skip destinations)' },
      ],
    },
    {
      key: 'on_no_match',
      label: 'On No Match',
      description: 'Action when rules do not match',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'continue',
      options: [
        { value: 'continue', label: 'Continue pipeline' },
        { value: 'halt', label: 'Halt pipeline (skip destinations)' },
      ],
    },
  ],
  marketingDescription: `
### Conditional Pipeline Control
Logic Gate lets you create powerful rules to filter, route, or halt activities based on any combination of conditions.

### How it works
Define rules using fields like activity type, day of week, time, location, or title/description content. Combine them with AND/ANY/NONE logic and choose whether to continue or halt the pipeline.

### Supported Rule Fields
- **activity_type**: Match by activity type (Run, Ride, etc.)
- **days**: Match by day of week (Mon, Tue, etc.)
- **time_start / time_end**: Match by time of day (HH:MM)
- **location**: Match by GPS coordinates within radius
- **title_contains / description_contains**: Match text content
  `,
  features: [
    '✅ Match by activity type, day, time, location',
    '✅ Title and description content matching',
    '✅ Flexible AND/OR/NOR logic modes',
    '✅ Continue or halt pipeline on match',
    '✅ Negate individual rules for inverse matching',
  ],
  transformations: [],
  useCases: [
    'Filter out test workouts',
    'Only sync activities from specific locations',
    'Route morning runs to different destinations',
    'Block activities on certain days',
  ],
  // UX Organization
  category: 'workflow',
  sortOrder: 5,
  isPremium: false,
  popularityScore: 55,
  allowMultipleInstances: true,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_HEART_RATE_SUMMARY, {
  id: 'heart-rate-summary',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Heart Rate Summary',
  description: 'Adds min/avg/max heart rate stats with optional drift detection',
  icon: '❤️',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'show_drift',
      label: 'Show Drift',
      description: 'Detect cardiac drift (rising HR) that may indicate dehydration or fatigue',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
  ],
  marketingDescription: `
### Heart Rate Stats at a Glance
Automatically calculates and appends heart rate statistics to your activity description. See your min, average, and max heart rate without diving into charts.

### Cardiac Drift Detection
Enable **Show Drift** to compare your HR at the start vs end of your workout. An upward drift may indicate dehydration or accumulated fatigue.
  `,
  features: [
    '✅ Calculates min/avg/max heart rate',
    '✅ Works with any heart rate source',
    '✅ Optional cardiac drift detection 📈',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: '❤️ Heart Rate: 95 bpm min • 145 bpm avg • 178 bpm max',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Quick HR overview on your activity feed',
    'Track training zones summary',
    'Monitor cardiac drift for long efforts',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 82,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_HEART_RATE_ZONES, {
  id: 'heart-rate-zones',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Heart Rate Zones',
  description: 'Shows time spent in each heart rate training zone with visual bars',
  icon: '💗',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'max_hr',
      label: 'Max Heart Rate',
      description: 'Your maximum heart rate in bpm (default: 190)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER,
      required: false,
      defaultValue: '190',
      options: [],
      validation: { minValue: 120, maxValue: 220 },
    },
    {
      key: 'style',
      label: 'Display Style',
      description: 'How the zones should be rendered',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'emoji',
      options: [
        { value: 'emoji', label: 'Emoji Bars (🟩🟩🟩⬜⬜)' },
        { value: 'percentage', label: 'Percentage (Zone 2: 45%)' },
        { value: 'text', label: 'Text Only (Zone 2: High)' },
      ],
    },
    {
      key: 'bar_length',
      label: 'Bar Length',
      description: 'Number of squares in emoji bar',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER,
      required: false,
      defaultValue: '5',
      options: [],
      validation: { minValue: 3, maxValue: 10 },
      dependsOn: { fieldKey: 'style', values: ['emoji'] },
    },
  ],
  marketingDescription: `
### Visualize Your Training Intensity
See exactly how much time you spent in each heart rate training zone. Perfect for understanding whether your workout hit the intended intensity targets.

### How it works
FitGlue analyzes your heart rate data and calculates time spent in each of the 5 standard training zones:
- **Zone 1 (Recovery)**: 50-60% max HR - Easy warmup/cooldown
- **Zone 2 (Endurance)**: 60-70% max HR - Fat burning, base building
- **Zone 3 (Tempo)**: 70-80% max HR - Aerobic capacity
- **Zone 4 (Threshold)**: 80-90% max HR - Lactate threshold
- **Zone 5 (VO2 Max)**: 90-100% max HR - Maximum effort

### Configurable Display
Choose between emoji bars (visual progress), percentages, or text descriptions. Set your personal max HR for accurate zone calculations.
  `,
  features: [
    '✅ Visual breakdown of time in each zone',
    '✅ Color-coded zones (🟦🟩🟨🟧🟥)',
    '✅ Configurable max heart rate',
    '✅ Multiple display styles (emoji/percentage/text)',
    '✅ Works with any heart rate source',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Heart Rate Zones',
      before: 'Morning Run',
      after: '',
      visualType: '',
      afterHtml: '<strong>❤️ Heart Rate Zones:</strong><br><br><span class="zone-row">Zone 1 (Recovery): <span class="zone-bar">🟦🟦⬜⬜⬜</span> 5 min</span><br><span class="zone-row">Zone 2 (Endurance): <span class="zone-bar">🟩🟩🟩🟩🟩</span> 20 min</span><br><span class="zone-row">Zone 3 (Tempo): <span class="zone-bar">🟨🟨🟨⬜⬜</span> 12 min</span><br><span class="zone-row">Zone 4 (Threshold): <span class="zone-bar">🟧🟧⬜⬜⬜</span> 8 min</span><br><span class="zone-row">Zone 5 (VO2 Max): <span class="zone-bar">🟥⬜⬜⬜⬜</span> 3 min</span>',
    },
  ],
  useCases: [
    'Track training zone distribution',
    'Verify workout intensity targets',
    'Identify over/under training patterns',
    'Share zone breakdown on activity feed',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 84,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_AI_COMPANION, {
  id: 'ai-companion',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'AI Activity Companion',
  description: 'Generates AI-powered titles, descriptions, and summaries for your activities (Athlete tier only)',
  icon: '✨',
  enabled: true,
  requiredIntegrations: [],
  requiredTier: 'athlete',
  configSchema: [
    {
      key: 'mode',
      label: 'Mode',
      description: 'What the AI should generate',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'description',
      options: [
        { value: 'title', label: 'Title only' },
        { value: 'description', label: 'Description only' },
        { value: 'both', label: 'Both title and description' },
      ],
    },
    {
      key: 'section_header',
      label: 'Show Section Header',
      description: 'Prepend "✨ AI Summary:" to the generated description',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
  ],
  marketingDescription: `
### AI-Powered Activity Companion
Let AI craft engaging titles and descriptions for your workouts. Turn raw data into compelling stories.

### How it works
When enabled, this Booster uses a large language model to analyze your activity data—type, duration, distance, heart rate, exercises—and generates a human-like title and/or description. Perfect for making your activity feed more interesting!

### Configurable Summaries
You can choose whether to include a "✨ AI Summary:" header. This is useful if you want to keep your own description and add the AI-generated one below it.

### Athlete Tier Only
This premium feature is exclusively available to Athlete tier subscribers.
  `,
  features: [
    '✅ AI-generated titles and descriptions',
    '✅ Analyzes activity context and data (HR, Cadence, Sets, etc.)',
    '✅ Optional "✨ AI Summary:" section header',
    '✅ Creates engaging workout narratives',
    '✅ Athlete tier exclusive',
  ],
  transformations: [
    {
      field: 'title',
      label: 'Activity Title (mode: title or both)',
      before: 'Weight Training',
      after: 'Iron Therapy: Push Day Edition',
      visualType: '',
      afterHtml: '',
    },
    {
      field: 'description',
      label: 'Activity Description (mode: description or both)',
      before: '45 min session',
      after: '✨ AI Summary:\n💪 Crushed an intense upper body session today! Focused on compound movements with progressive overload. Feeling strong!',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Make activity posts more engaging',
    'Save time writing descriptions',
    'Add personality to workout logs',
  ],
  // UX Organization
  category: 'ai_images',
  sortOrder: 1,
  isPremium: true,
  popularityScore: 95,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_PACE_SUMMARY, {
  id: 'pace-summary',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Pace Summary',
  description: 'Adds avg/best pace stats with optional splits analysis and fatigue detection',
  icon: '🏃‍♂️',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'show_splits',
      label: 'Show Splits',
      description: 'Display pace for each km split with fastest (🏆) and slowest (🐢) markers',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
    {
      key: 'negative_split_alert',
      label: 'Negative Split Alert',
      description: 'Celebrate when your second half is faster than your first',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
    {
      key: 'show_fatigue',
      label: 'Fatigue Analysis',
      description: 'Compare first and last quarter performance to detect pacing fade',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
  ],
  marketingDescription: `
### Pace Stats at a Glance
Automatically calculates and appends pace statistics to your activity description. See your average and best pace without diving into charts.

### How it works
When your activity has speed data (from GPS or sensors), this enricher converts speed to pace and adds a clean summary showing your average and best pace in min/km format.

### Advanced Analysis
Enable **Splits** to see every km pace with fastest/slowest markers. Enable **Negative Split Alert** to celebrate when you finish stronger. Enable **Fatigue Analysis** to understand your pacing strategy.
  `,
  features: [
    '✅ Calculates avg/best pace from speed data',
    '✅ Formats pace as min/km',
    '✅ Optional km-by-km split breakdown',
    '✅ Negative split detection 🔥',
    '✅ Fatigue analysis (first vs last quarter)',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: 'Morning Run\\n\\n⚡ Pace: 5:32/km avg • 4:45/km best',
      visualType: '',
      afterHtml: '',
    },
    {
      field: 'description',
      label: 'With Splits Enabled',
      before: 'Morning Run',
      after: '',
      visualType: '',
      afterHtml: '<strong>⚡ Pace: 5:32/km avg • 4:45/km best</strong><br><br>📊 Splits:<br>- Km 1: 5:45<br>- Km 2: 5:38<br>- Km 3: 5:22 🏆<br>- Km 4: 5:41<br>- Km 5: 5:55 🐢<br><br>🔥 Negative Split! Second half 12s/km faster',
    },
  ],
  useCases: [
    'Quick pace overview on your activity feed',
    'Track running performance and pacing strategy',
    'Identify fatigue patterns in long runs',
    'Celebrate negative splits and strong finishes',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 75,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_CADENCE_SUMMARY, {
  id: 'cadence-summary',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Cadence Summary',
  description: 'Adds avg/max cadence stats to the activity description',
  icon: '🦶',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### Cadence Stats at a Glance
Automatically calculates and appends cadence statistics to your activity description. See your average and max cadence at a glance.

### How it works
When your activity has cadence data (from sensors), this enricher calculates avg/max cadence and adds a clean summary. Uses spm (steps per minute) for running activities and rpm for cycling.
  `,
  features: [
    '✅ Calculates avg/max cadence',
    '✅ Appends clean summary to description',
    '✅ Smart unit detection (spm vs rpm)',
    '✅ No configuration required',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: 'Morning Run\n\n🦶 Cadence: 172 spm avg • 185 spm max',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Track running cadence efficiency',
    'Monitor cycling cadence',
    'Analyze stride consistency',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 4,
  isPremium: false,
  popularityScore: 55,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_POWER_SUMMARY, {
  id: 'power-summary',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Power Summary',
  description: 'Adds avg/max power stats (watts) to the activity description',
  icon: '⚡',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### Power Stats at a Glance
Automatically calculates and appends power statistics to your activity description. Perfect for cyclists with power meters.

### How it works
When your activity has power data (from a power meter), this enricher calculates avg/max power and adds a clean summary in watts.
  `,
  features: [
    '✅ Calculates avg/max power from power meter data',
    '✅ Appends clean summary to description',
    '✅ Works with any power source',
    '✅ No configuration required',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Ride',
      after: 'Morning Ride\n\n⚡ Power: 245W avg • 380W max',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Quick power overview on your activity feed',
    'Track cycling power output',
    'Share power stats easily',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 5,
  isPremium: false,
  popularityScore: 60,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_SPEED_SUMMARY, {
  id: 'speed-summary',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Speed Summary',
  description: 'Adds avg/max speed stats (km/h) to the activity description',
  icon: '🚀',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### Speed Stats at a Glance
Automatically calculates and appends speed statistics to your activity description. See your average and max speed in km/h.

### How it works
When your activity has speed data (from GPS or sensors), this enricher calculates avg/max speed and adds a clean summary in km/h format.
  `,
  features: [
    '✅ Calculates avg/max speed from GPS or sensor data',
    '✅ Formats speed as km/h',
    '✅ Appends clean summary to description',
    '✅ No configuration required',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Ride',
      after: 'Morning Ride\n\n🚀 Speed: 28.5 km/h avg • 42.1 km/h max',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Quick speed overview on your activity feed',
    'Track cycling speed',
    'Monitor activity performance',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 6,
  isPremium: false,
  popularityScore: 50,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_TRAINING_LOAD, {
  id: 'training-load',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Training Load',
  description: 'Calculates Training Impulse (TRIMP) from heart rate data',
  icon: '💪',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'max_hr',
      label: 'Max Heart Rate',
      description: 'Your maximum heart rate (default: 190)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER,
      required: false,
      defaultValue: '190',
      options: [],
    },
    {
      key: 'rest_hr',
      label: 'Resting Heart Rate',
      description: 'Your resting heart rate (default: 60)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER,
      required: false,
      defaultValue: '60',
      options: [],
    },
    {
      key: 'gender',
      label: 'Gender',
      description: 'Used for TRIMP coefficient calculation',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'male',
      options: [
        { value: 'male', label: 'Male' },
        { value: 'female', label: 'Female' },
      ],
    },
  ],
  marketingDescription: `
### Measure Your Training Intensity
The Training Load booster calculates your Training Impulse (TRIMP) using the scientifically validated Banister Formula. This gives you a single number to represent the physiological load of your workout based on heart rate and duration.

### How it works
FitGlue analyzes your heart rate stream throughout the activity. It calculates your Heart Rate Reserve (HRR) and applies the Banister Formula (weighted for gender) to determine total TRIMP. This load is then categorized into Effort Zones from Recovery to Very Hard.

### Know Your Hardest Sessions
TRIMP is cumulative, meaning a long easy session can have the same load as a short intense one. This helps you track total training stimulus across different workout types.
  `,
  features: [
    '✅ Calculates cumulative TRIMP (Training Impulse)',
    '✅ Uses the Banister Formula (weighted for gender)',
    '✅ Categorizes effort into 5 zones',
    '✅ Works with any heart rate data source',
    '✅ Customizable max and resting heart rate',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: 'Morning Run\\n\\n💪 Training Load: 142 (Hard)',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Track total training stress on your activity feed',
    'Compare intensity between different activities',
    'Monitor recovery needs based on load',
    'Celebrate your hardest training sessions',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 7,
  isPremium: false,
  popularityScore: 65,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_PERSONAL_RECORDS, {
  id: 'personal-records',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Personal Records',
  description: 'Detects and celebrates new PRs for cardio and strength activities',
  icon: '🏆',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'cardio_records',
      label: 'Track Cardio PRs',
      description: 'Track PRs for running and cycling (5K, 10K, longest distance, etc.)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
    {
      key: 'strength_records',
      label: 'Track Strength PRs',
      description: 'Track PRs for weightlifting (1RM, volume, max reps)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
    {
      key: 'celebrate_in_title',
      label: 'Celebrate in Title',
      description: 'Add 🎉 emoji to activity title when a PR is achieved',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
  ],
  marketingDescription: `
### Automatic Personal Record Detection
Never miss a PR again! FitGlue automatically detects when you've achieved a new personal record and adds a celebration to your activity.

### Cardio Records Tracked
- **Fastest 5K, 10K, Half Marathon**: Time-based records for running
- **Longest Run**: Your greatest single-run distance
- **Longest Ride**: Your greatest single-ride distance
- **Highest Elevation Gain**: Most climbing in one activity

### Strength Records Tracked (per exercise)
- **1RM**: Uses the Epley formula to estimate your one-rep max
- **Volume**: Most total volume (sets × reps × weight) in one session
- **Reps**: Most reps in a single set

All records are stored in Firestore, so your PRs persist across time.
  `,
  features: [
    '✅ Automatic PR detection for cardio and strength',
    '✅ Epley formula for estimated 1RM',
    '✅ Smart exercise name normalization',
    '✅ Percentage improvement shown',
    '✅ Persistent storage in Firestore',
    '✅ Optional title celebration emoji',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Weight Training',
      after: 'Weight Training\\n\\n🏆 Deadlift 1RM: 140kg (previous: 135kg, +3.7%)',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Celebrate running PRs automatically',
    'Track strength progression over time',
    'Share PR achievements on your activity feed',
    'Know immediately when you hit a new max',
  ],
  // UX Organization
  category: 'detection',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 78,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_ELEVATION_SUMMARY, {
  id: 'elevation-summary',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Elevation Summary',
  description: 'Calculates elevation gain, loss, and max altitude with optional visual profile',
  icon: '⛰️',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'style',
      label: 'Display Style',
      description: 'Choose how to display elevation data',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'text',
      options: [
        { label: 'Text Only', value: 'text' },
        { label: 'With Profile', value: 'profile' },
      ],
    },
  ],
  marketingDescription: `
### Total Ascent & Descent
Automatically calculates and appends elevation statistics to your activity description. Perfect for hilly runs, mountain bike rides, or mountain hikes.

### Visual Profile
Enable **Profile** mode to see an ASCII art elevation chart: \`📈 ▁▂▄▆█▆▃▁\`

### Clean Data Processing
Filters out zero or negative altitude records to ensure accurate calculations even if your GPS occasionally loses altitude data.
  `,
  features: [
    '✅ Calculates total elevation gain (ascent)',
    '✅ Calculates total elevation loss (descent)',
    '✅ Tracks maximum altitude during activity',
    '✅ Optional visual elevation profile 📈',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Mountain Hike',
      after: '⛰️ Elevation: +342m gain • -289m loss • 1,245m max',
      visualType: '',
      afterHtml: '',
    },
    {
      field: 'description',
      label: 'With Profile',
      before: 'Mountain Hike',
      after: '',
      visualType: '',
      afterHtml: '⛰️ Elevation: +342m gain • -289m loss • 1,245m max<br>📈 ▁▂▄▆█▇▅▃▂▁▂▃▅▇█▆▃▁▁',
    },
  ],
  useCases: [
    'Hilly runs and rides',
    'Mountain hiking and climbing',
    'Visualize elevation changes at a glance',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 8,
  isPremium: false,
  popularityScore: 68,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_WEATHER, {
  id: 'weather',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Weather',
  description: 'Adds weather conditions to outdoor activities',
  icon: '🌤️',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'include_wind',
      label: 'Include Wind',
      description: 'Show wind speed and direction in the summary',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
  ],
  marketingDescription: `
### Weather Context for Outdoor Activities
Automatically adds weather conditions to your outdoor activities. See temperature, conditions, and wind at a glance.

### How it works
When your activity has GPS data, this enricher fetches historical weather data from Open-Meteo for the exact time and location of your workout. It adds a clean summary showing temperature, weather conditions, and optionally wind information.
  `,
  features: [
    '✅ Fetches historical weather data',
    '✅ Shows temperature and conditions',
    '✅ Optional wind speed and direction',
    '✅ Works with any GPS-enabled activity',
    '✅ Free API, no authentication required',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: 'Morning Run\\n\\n🌤️ Weather: 18°C, Partly Cloudy • Wind: 12 km/h W',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Track weather conditions for training analysis',
    'Remember what the weather was like',
    'Share outdoor conditions on your activity feed',
  ],
  // UX Organization
  category: 'data',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 72,
});


registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_SPOTIFY_TRACKS, {
  id: 'spotify-tracks',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Spotify Soundtrack',
  description: 'Shows what music you listened to during your activity',
  icon: '🎵',
  enabled: true,
  requiredIntegrations: ['spotify'],
  isTemporarilyUnavailable: true,
  configSchema: [],
  marketingDescription: `
### Your Activity Soundtrack
Automatically track what music you listened to during your workouts. See your top played tracks and workout playlists.

### How it works
When you complete an activity, FitGlue checks your Spotify listening history for tracks played during that time window and adds a summary to your activity description.
  `,
  features: [
    '✅ Track count and top played songs',
    '✅ Playlist identification',
    '✅ Automatic time-window matching',
    '✅ Works with all activity types',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: 'Morning Run\\n\\n🎵 Soundtrack: 12 tracks • Top played: Blinding Lights - The Weeknd • From playlist: Running Hits 2026',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Track your workout music preferences',
    'Share your activity soundtrack on your activity feed',
    'Discover which playlists motivate you most',
  ],
  // UX Organization
  category: 'links',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 58,
  iconType: 'png',
  iconPath: '/images/icons/spotify.png',
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_LOCATION_NAMING, {
  id: 'location_naming',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Location Naming',
  description: 'Auto-generates activity titles from GPS location',
  icon: '📍',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'mode',
      label: 'Mode',
      description: 'How to apply the location name',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'title',
      options: [
        { value: 'title', label: 'Generate Title' },
        { value: 'description', label: 'Add to Description' },
      ],
    },
    {
      key: 'title_template',
      label: 'Title Template',
      description: 'Template for title mode, e.g. "{activity_type} in {location}"',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: '{activity_type} in {location}',
      options: [],
      dependsOn: { fieldKey: 'mode', values: ['title'] },
    },
    {
      key: 'fallback_enabled',
      label: 'Use City Fallback',
      description: 'Use city name if no specific location (park, leisure) found',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'true',
      options: [],
    },
    {
      key: 'time_context',
      label: 'Time of Day Context',
      description: 'Add sunrise/sunset context based on solar position',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'none',
      options: [
        { value: 'none', label: 'None' },
        { value: 'solar', label: 'Solar (Sunrise/Sunset)' },
      ],
    },
  ],
  marketingDescription: `
### Automatic Location-Based Titles
Give your activities meaningful names based on where you exercised. Instead of generic "Morning Run", get "Morning Run in Hyde Park".

### Sunrise/Sunset Context
Enable **Solar Time** to add time-of-day context based on actual sunrise/sunset for your location: "Sunrise Run in Hyde Park" or "Night Ride through Camden".

### How it works
When your activity has GPS data, this enricher uses OpenStreetMap's Nominatim API to reverse geocode your starting location. It prioritizes parks and leisure venues, falling back to suburb or city names when no specific location is found.
  `,
  features: [
    '✅ Automatic location detection from GPS',
    '✅ Prioritizes parks and leisure venues',
    '✅ Customizable title templates',
    '✅ City fallback for urban activities',
    '✅ 🌅 Sunrise/Sunset time context',
    '✅ Free API, no authentication required',
  ],
  transformations: [
    {
      field: 'title',
      label: 'Activity Title',
      before: 'Morning Run',
      after: 'Morning Run in Hyde Park',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Give runs and rides meaningful location names',
    'Track which parks and venues you visit',
    'Share location context on your activity feed',
  ],
  // UX Organization
  category: 'summaries',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 68,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_HYBRID_RACE_TAGGER, {
  id: 'hybrid_race_tagger',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Hybrid Race Tagger',
  description: 'Tag and merge laps for hybrid races like Hyrox, ATHX, and multi-sport events',
  icon: '🏁',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### Hybrid Race Lap Tagging
Properly categorize each segment of your hybrid race. Smartwatches often record multi-sport events (Hyrox, ATHX, etc.) as a single activity type, but each lap is a different exercise.

### How it works
When this enricher is active, your activity pipeline will pause and ask you to tag each lap with the correct exercise (e.g., "SkiErg", "Sled Push", "Running"). You can also merge adjacent laps that belong together (e.g., a 1km run split across multiple watch laps).

### Strength App Integration
Tagged laps can be automatically mapped to exercises in your connected strength apps. If an exact match isn't found, fuzzy matching or custom exercise creation handles it.
  `,
  features: [
    '✅ Tag individual laps with exercise names',
    '✅ Merge adjacent laps into logical segments',
    '✅ Works with Hyrox, ATHX, and custom events',
    '✅ Automatic exercise mapping via fuzzy match',
    '✅ Per-lap distance and duration tracking',
  ],
  transformations: [
    {
      field: 'laps',
      label: 'Lap Exercise Names',
      before: '19 laps (all "Run")',
      after: 'SkiErg (1 lap), 1km Run (2 laps merged), Sled Push (1 lap), ...',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Track Hyrox workouts with correct exercise tagging',
    'Segment ATHX events into individual stations',
    'Create accurate records for multi-sport activities',
    'Properly categorize triathlon or duathlon segments',
  ],
  // UX Organization
  category: 'detection',
  sortOrder: 10,
  isPremium: false,
  popularityScore: 70,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_RUNNING_DYNAMICS, {
  id: 'running-dynamics',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Running Dynamics',
  description: 'Summarize Running Dynamics data (GCT, Stride, Vertical Oscillation)',
  icon: '👟',
  enabled: true,
  requiredIntegrations: [],
  requiredTier: 'athlete',
  configSchema: [],
  marketingDescription: `
### Running Dynamics Booster
Automatically summarizes advanced running telemetry for compatible devices.

### Metrics Included
- **Ground Contact Time (GCT)**: How much time your foot spends on the ground.
- **Stride Length**: The distance between each step.
- **Vertical Oscillation**: How much you "bounce" while running.

### How it works
This booster extracts the telemetry from your activity file and appends a single-line summary to your activity description.
  `,
  features: [
    '✅ Summarize Ground Contact Time',
    '✅ Summarize Stride Length',
    '✅ Summarize Vertical Oscillation',
    '✅ Automatically activates for compatible data',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: 'Morning Run\n\n👟 Running Dynamics:\nGround Contact Time: 242ms • Stride Length: 1.12m • Vertical Oscillation: 8.4cm',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [],
  // UX Organization
  category: 'summaries',
  sortOrder: 10,
  isPremium: true,
  popularityScore: 90,
});

// ============================================
// Calories Burned
// ============================================
registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_CALORIES_BURNED, {
  id: 'calories-burned',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Calories Burned',
  description: 'Estimate calories burned with optional fun food equivalents',
  icon: '🔋',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'fun_mode',
      label: 'Fun Mode',
      description: 'Show food equivalents (e.g., "≈ 2.5 donuts 🍩")',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_BOOLEAN,
      required: false,
      defaultValue: 'false',
      options: [],
    },
    {
      key: 'user_weight',
      label: 'Weight (kg)',
      description: 'Your weight for more accurate calorie calculation',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER,
      required: false,
      defaultValue: '70',
      options: [],
      validation: { minValue: 30, maxValue: 200 },
    },
  ],
  marketingDescription: `
### Know Your Burn
Estimate calories burned based on activity type, duration, and your weight using MET (Metabolic Equivalent) calculations.

### Fun Mode
Enable **Fun Mode** to see your workout translated to food: "🔥 450 kcal ≈ 1.8 slices of pizza 🍕"
  `,
  features: [
    '✅ MET-based calorie estimation',
    '✅ Activity type aware',
    '✅ Optional fun food equivalents 🍕🍩🍫',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: '🔥 Calories: 485 kcal',
      visualType: '',
      afterHtml: '',
    },
    {
      field: 'description',
      label: 'With Fun Mode',
      before: 'Morning Run',
      after: '',
      visualType: '',
      afterHtml: '🔥 Calories: 485 kcal ≈ 1.7 slices of pizza 🍕',
    },
  ],
  useCases: [
    'Track workout energy expenditure',
    'Fun conversation starter',
    'Monitor calorie burn across activities',
  ],
  category: 'summaries',
  sortOrder: 6,
  isPremium: false,
  popularityScore: 75,
});

// ============================================
// Goal Tracker
// ============================================
registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_GOAL_TRACKER, {
  id: 'goal-tracker',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Goal Tracker',
  description: 'Track progress toward weekly, monthly, or yearly goals',
  icon: '📅',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'period',
      label: 'Goal Period',
      description: 'Time period for your goal',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'month',
      options: [
        { label: 'Weekly', value: 'week' },
        { label: 'Monthly', value: 'month' },
        { label: 'Yearly', value: 'year' },
      ],
    },
    {
      key: 'metric',
      label: 'Goal Metric',
      description: 'What to track',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'distance',
      options: [
        { label: 'Distance (km)', value: 'distance' },
        { label: 'Duration (hours)', value: 'duration' },
        { label: 'Activity Count', value: 'activities' },
      ],
    },
    {
      key: 'target',
      label: 'Target',
      description: 'Your goal target (e.g., 100 for 100km)',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_NUMBER,
      required: true,
      defaultValue: '100',
      options: [],
      validation: { minValue: 1 },
    },
  ],
  marketingDescription: `
### Set and Track Your Goals
Configure weekly, monthly, or yearly targets for distance, duration, or activity count. Each activity shows your progress!

### Smart Guidance
Get daily target suggestions to stay on track: "💡 8.5 km/day to hit your goal"
  `,
  features: [
    '✅ Configurable period (week/month/year)',
    '✅ Multiple metrics (distance, duration, count)',
    '✅ Daily pace guidance',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: '',
      visualType: '',
      afterHtml: '📅 February Goal: 100 km<br>➕ This activity: +5.2 km<br>💡 Target: 3.6 km/day',
    },
  ],
  useCases: [
    'Monthly distance challenges',
    'Yearly mileage goals',
    'Weekly activity targets',
  ],
  category: 'data',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 70,
});

// ============================================
// Streak Tracker
// ============================================
registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_STREAK_TRACKER, {
  id: 'streak-tracker',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Streak Tracker',
  description: 'Celebrate consecutive activity days and streaks',
  icon: '⭐',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'activity_types',
      label: 'Activity Type Filter',
      description: 'Track streaks for specific activity types',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'any',
      options: [
        { label: 'Any Activity', value: 'any' },
        { label: 'Running Only', value: 'running' },
        { label: 'Cycling Only', value: 'cycling' },
        { label: 'Swimming Only', value: 'swimming' },
        { label: 'Strength Only', value: 'strength' },
      ],
    },
  ],
  marketingDescription: `
### Keep the Fire Burning
Track consecutive days of activity to maintain motivation. Receive milestone celebrations at 7, 14, 30, and 100 days!

### Activity Type Filtering
Focus on specific activities like running or strength training to build targeted habits.
  `,
  features: [
    '✅ Track consecutive activity days',
    '✅ Filter by activity type',
    '✅ Milestone celebrations 🎉',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Activity Description',
      before: 'Morning Run',
      after: '🔥 Keep the running streak alive!',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Build daily exercise habits',
    'Track running streak',
    'Celebrate consistency milestones',
  ],
  category: 'data',
  sortOrder: 2,
  isPremium: false,
  popularityScore: 65,
});

// ============================================
// Distance Milestones
// ============================================
registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_DISTANCE_MILESTONES, {
  id: 'distance-milestones',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Distance Milestones',
  description: 'Celebrate lifetime distance achievements per sport',
  icon: '🏅',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'sport',
      label: 'Sport Filter',
      description: 'Track milestones for specific sports',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'any',
      options: [
        { label: 'All Activities', value: 'any' },
        { label: 'Running Only', value: 'running' },
        { label: 'Cycling Only', value: 'cycling' },
        { label: 'Swimming Only', value: 'swimming' },
      ],
    },
  ],
  marketingDescription: `
### Celebrate Your Journey
Track your lifetime distance and get celebrated when you hit major milestones — 100km, 500km, 1,000km, and beyond!

### Per-Sport Tracking
Focus on a specific sport to track your running, cycling, or swimming journey separately.
  `,
  features: [
    '✅ Lifetime distance tracking',
    '✅ Milestone celebrations at key thresholds',
    '✅ Per-sport filtering',
    '✅ Progress toward next milestone',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Milestone Celebration',
      before: 'Morning Run',
      after: '',
      visualType: '',
      afterHtml: '🎉 MILESTONE: 1,000 km lifetime!<br>• Total: 1,005.2 km<br>• This run: +5.2 km',
    },
  ],
  useCases: [
    'Track your lifetime running distance',
    'Celebrate major cycling milestones',
    'Monitor progress toward yearly goals',
  ],
  category: 'data',
  sortOrder: 3,
  isPremium: false,
  popularityScore: 60,
});

// ============================================
// Recovery Advisor
// ============================================
registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_RECOVERY_ADVISOR, {
  id: 'recovery-advisor',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Recovery Advisor',
  description: 'Calculate training load and suggest recovery time',
  icon: '💤',
  enabled: true,
  requiredIntegrations: [],
  requiredTier: 'athlete',
  configSchema: [],
  marketingDescription: `
### Know When to Rest
Uses TRIMP (Training Impulse) to estimate your training load and suggest optimal recovery time.

### 7-Day Load Tracking
Monitors your accumulated training stress over the past week to give context-aware recovery recommendations.
  `,
  features: [
    '✅ TRIMP-based training load calculation',
    '✅ 7-day rolling load history',
    '✅ Smart recovery recommendations',
    '✅ Intensity classification',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Recovery Recommendation',
      before: 'Hard Interval Session',
      after: '',
      visualType: '',
      afterHtml: '💤 Recovery Advisor<br>• Session load: 150 TRIMP (Hard)<br>• 7-day load: 520 TRIMP<br>• 💡 Suggested recovery: 36 hours',
    },
  ],
  useCases: [
    'Prevent overtraining',
    'Optimize recovery between sessions',
    'Balance training load across the week',
  ],
  category: 'summaries',
  sortOrder: 9,
  isPremium: true,
  popularityScore: 55,
});

// ============================================
// Effort Score
// ============================================
registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_EFFORT_SCORE, {
  id: 'effort-score',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Effort Score',
  description: 'Rate every workout on a 0-100 scale relative to your personal norms',
  icon: '💥',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### How Hard Was That — For You?
Every workout gets a personal difficulty score from 0 to 100, based on how it compares to your recent 14-day activity history.

### Multi-Factor Analysis
Your score is calculated from heart rate, pace, duration, elevation gain, and training load — weighted and combined into a single number. If a signal is unavailable (e.g., no HR data), the remaining factors are weighted proportionally.

### Adaptive Over Time
As your fitness changes, so does your baseline. A run that scored 75 three months ago might score 55 today — because you've gotten stronger.
  `,
  features: [
    '✅ Normalized 0-100 effort score',
    '✅ Multi-factor: HR, pace, duration, elevation, TRIMP',
    '✅ Adapts to your personal rolling averages',
    '✅ Trend indicators (harder/easier than usual)',
    '✅ Fully automatic — no configuration needed',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Effort Score',
      before: 'Morning Run',
      after: '',
      visualType: '',
      afterHtml: '💥 Effort Score: 72/100 (Hard)<br>• ❤️ HR: 1.15× avg<br>• 🏃 Pace: 1.08× avg<br>• 📈 Harder than usual',
    },
  ],
  useCases: [
    'Understand relative workout intensity',
    'Track fitness progression over time',
    'Balance easy and hard training days',
    'See which workouts push your limits',
  ],
  category: 'summaries',
  sortOrder: 8,
  isPremium: false,
  popularityScore: 80,
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_MOCK, {
  id: 'mock',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Mock',
  description: 'Testing enricher that simulates various behaviors',
  icon: '🧪',
  enabled: false, // Testing only
  requiredIntegrations: [],
  configSchema: [
    {
      key: 'behavior',
      label: 'Behavior',
      description: 'How the mock should behave',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_SELECT,
      required: false,
      defaultValue: 'success',
      options: [
        { value: 'success', label: 'Success' },
        { value: 'lag', label: 'Simulate Lag' },
        { value: 'fail', label: 'Fail' },
      ],
    },
    { key: 'name', label: 'Activity Name', description: 'Name to set', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'description', label: 'Description', description: 'Description to add', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
  ],
  marketingDescription: '',
  features: [],
  transformations: [],
  useCases: [],
  // UX Organization (internal testing only)
  category: 'workflow',
  sortOrder: 99,
  isPremium: false,
  popularityScore: 0,
});

// ============================================================================
// Register all known integrations
// NOTE: Keep in sync with web/skier.tasks.mjs for marketing site
// ============================================================================

registerIntegration({
  id: 'hevy',
  name: 'Hevy',
  description: 'Import strength training workouts',
  icon: '🏋️',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_API_KEY,
  enabled: true,
  docsUrl: 'https://docs.hevy.com',
  setupTitle: 'Connect Hevy',
  setupInstructions: `To connect Hevy, you'll need a **Hevy Pro** subscription and an API key:

1. Open the **Hevy app** on your phone
2. Go to **Settings** (gear icon)
3. Scroll down and tap **Developer** (requires Hevy Pro)
4. Tap **Generate API Key**
5. Copy the key and enter it in your **FitGlue Dashboard**

**Note:** The Developer API requires **Hevy Pro**. Free tier accounts cannot access this feature.`,
  apiKeyLabel: 'Hevy API Key',
  apiKeyHelpUrl: 'https://docs.hevy.com/developer-api',
  marketingDescription: `
### What is Hevy?
Hevy is a popular workout tracking app designed for strength training enthusiasts. It lets you log exercises, sets, reps, and weights with a clean, intuitive interface.

### What FitGlue Does
FitGlue connects to your Hevy account via API key, allowing your logged activities to flow into the FitGlue Pipeline. From there, you can boost them with AI summaries, muscle heatmaps, and more — then sync them to your Targets.
  `,
  features: [
    '✅ Import all your strength workouts automatically',
    '✅ Exercises, sets, reps, and weights included',
    '✅ Real-time sync when you finish a workout',
    '✅ Simple API key setup — no OAuth required',
  ],
  iconType: 'svg',
  iconPath: '/images/icons/hevy.svg',
  actions: [
    {
      id: 'import_strength_prs',
      label: 'Import Strength PRs',
      description: 'Import 1RM and volume records from your last 12 months of Hevy workouts',
      icon: '💪',
    },
  ],
});

registerIntegration({
  id: 'fitbit',
  name: 'Fitbit',
  description: 'Sync activities and health data from your Fitbit device',
  icon: '⌚',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  docsUrl: '',
  setupTitle: 'Connect Fitbit',
  setupInstructions: `Connect your Fitbit account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on Fitbit
3. Sign in to your **Fitbit account** when redirected
4. Review and **Accept Permissions** to allow FitGlue access
5. You're connected! Activities will sync automatically

FitGlue uses secure OAuth — your Fitbit password is never stored.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Fitbit?
Fitbit is a leading wearable fitness tracker that monitors your activity, heart rate, sleep, and more. Millions of users rely on Fitbit devices to track their daily health metrics.

### What FitGlue Does
FitGlue connects to your Fitbit account via OAuth, enabling you to import activities and heart rate data. Use Fitbit as a Source for activities, or overlay heart rate data onto activities from other Sources like Hevy.
  `,
  features: [
    '✅ Import activities tracked by your Fitbit device',
    '✅ Use heart rate data to boost activities from other Sources',
    '✅ Secure OAuth connection — no passwords stored',
    '✅ Automatic sync of new activities',
  ],
  iconType: 'svg',
  iconPath: '/images/icons/fitbit.svg',
  actions: [],
});

registerIntegration({
  id: 'strava',
  name: 'Strava',
  description: 'Import activities from and upload to Strava',
  icon: '🚴',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  docsUrl: '',
  setupTitle: 'Connect Strava',
  setupInstructions: `Connect your Strava account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on Strava
3. Sign in to your **Strava account** when redirected
4. Review and **Accept Permissions** to allow FitGlue to upload activities
5. You're connected! Boosted activities will appear on your Strava feed

FitGlue uses secure OAuth — your Strava password is never stored.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Strava?
Strava is the social network for athletes. Share your activities with friends, compete on segments, and track your training progress over time.

### What FitGlue Does
FitGlue connects to your Strava account via OAuth and can upload your boosted activities directly. Activities from Hevy or Fitbit — boosted with AI descriptions, muscle heatmaps, and heart rate data — appear on your Strava feed automatically.
  `,
  features: [
    '✅ Upload boosted activities to Strava automatically',
    '✅ AI-generated descriptions appear in your feed',
    '✅ Muscle heatmaps and stats included',
    '✅ Secure OAuth connection',
  ],
  iconType: 'svg',
  iconPath: '/images/icons/strava.svg',
  actions: [
    {
      id: 'import_cardio_prs',
      label: 'Import Cardio PRs',
      description: 'Fetch your fastest 5K, 10K, and half marathon times from Strava history',
      icon: '🏃',
    },
  ],
});

registerIntegration({
  id: 'apple-health',
  name: 'Apple Health',
  description: 'Sync workouts and health data from your iOS device',
  icon: '🍎',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_APP_SYNC,
  enabled: true,
  docsUrl: '',
  setupTitle: 'Connect Apple Health',
  setupInstructions: `To connect Apple Health, install the **FitGlue mobile app** on your iOS device:

1. Download **FitGlue** from the App Store
2. Sign in with your FitGlue account
3. Grant access to Apple Health when prompted
4. Your workouts will sync automatically in the background

**Note:** Apple Health data can only be accessed from the mobile app running on your iOS device.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Apple Health?
Apple Health is the centralized health data repository on iOS devices. It aggregates data from your Apple Watch, iPhone sensors, and third-party fitness apps.

### What FitGlue Does
FitGlue's mobile app reads your workout data from Apple Health and syncs it to the cloud. Workouts complete with heart rate data and GPS routes flow through your FitGlue pipeline for enhancement and distribution to destinations like Strava.
  `,
  features: [
    '✅ Import workouts from Apple Watch and iPhone',
    '✅ Heart rate data from wrist sensors',
    '✅ GPS routes for outdoor activities',
    '✅ Background sync via FitGlue mobile app',
  ],
  iconType: 'png',
  iconPath: '/images/icons/healthkit.png',
  actions: [],
});

registerIntegration({
  id: 'health-connect',
  name: 'Health Connect',
  description: 'Sync workouts and health data from your Android device',
  icon: '🤖',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_APP_SYNC,
  enabled: true,
  docsUrl: '',
  setupTitle: 'Connect Health Connect',
  setupInstructions: `To connect Health Connect, install the **FitGlue mobile app** on your Android device:

1. Download **FitGlue** from the Google Play Store
2. Sign in with your FitGlue account
3. Grant access to Health Connect when prompted
4. Your workouts will sync automatically in the background

**Note:** Health Connect data can only be accessed from the mobile app running on your Android device.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Health Connect?
Health Connect is Android's unified health data platform. It allows apps and wearables to share health and fitness data in a standardized way.

### What FitGlue Does
FitGlue's mobile app reads your workout data from Health Connect and syncs it to the cloud. Workouts from Garmin, Samsung, Fitbit, and other compatible devices flow through your FitGlue pipeline for enhancement and distribution.
  `,
  features: [
    '✅ Import workouts from any Health Connect-compatible device',
    '✅ Works with Garmin, Samsung, Fitbit, and more',
    '✅ Heart rate and GPS data included',
    '✅ Background sync via FitGlue mobile app',
  ],
  iconType: 'svg',
  iconPath: '/images/icons/healthconnect.svg',
  actions: [],
});

registerIntegration({
  id: 'parkrun',
  name: 'Parkrun',
  description: 'Enhanced Parkrun detection with official results',
  icon: '🏃',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_PUBLIC_ID,
  enabled: true,
  docsUrl: 'https://www.parkrun.com',
  setupTitle: 'Connect Parkrun',
  setupInstructions: `To connect Parkrun, you'll need your athlete barcode number:

1. **Find your barcode** — Your Parkrun barcode starts with "A" followed by numbers (e.g. A12345678)
2. **Check your barcode card** — This is printed on your physical barcode card, or find it on the Parkrun website in your profile
3. **Enter your barcode** — Enter your barcode number (including the A) in the form below

Once connected, FitGlue can fetch your official results and update your activities automatically.`,
  apiKeyLabel: 'Barcode Number',
  apiKeyHelpUrl: 'https://www.parkrun.com/register/',
  marketingDescription: `
### What is Parkrun?
Parkrun is a free, community-organized 5K running event held every Saturday morning at locations worldwide. Millions of people participate, from first-timers to elite runners.

### What FitGlue Does
FitGlue automatically detects when your run is a Parkrun based on GPS location and time. With your barcode connected, we can fetch your **official results** — position, time, age grade, and PB status — and update your activity with all the details.
  `,
  features: [
    '✅ Automatic Parkrun event detection via GPS',
    '✅ Official results fetched after the event',
    '✅ Position, time, and age grade added to description',
    '✅ PB celebrations highlighted',
    '✅ Special event detection (Christmas, New Year)',
    '✅ 2,500+ worldwide Parkrun locations supported',
  ],
  iconType: 'png',
  iconPath: '/images/icons/parkrun.png',
  actions: [],
});

registerIntegration({
  id: 'trainingpeaks',
  name: 'TrainingPeaks',
  description: 'Upload activities to TrainingPeaks',
  icon: '📈',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  isTemporarilyUnavailable: true,
  docsUrl: 'https://developer.trainingpeaks.com',
  setupTitle: 'Connect TrainingPeaks',
  setupInstructions: `Connect your TrainingPeaks account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on TrainingPeaks
3. Sign in to your **TrainingPeaks account** when redirected
4. Review and **Accept Permissions** to allow FitGlue to upload workouts
5. You're connected! Boosted activities will appear in your TrainingPeaks calendar

FitGlue uses secure OAuth — your TrainingPeaks password is never stored.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is TrainingPeaks?
TrainingPeaks is a professional training platform used by athletes and coaches worldwide. It provides structured training plans, workout analysis, and performance tracking.

### What FitGlue Does
FitGlue connects to your TrainingPeaks account via OAuth and uploads your boosted activities. Activities from your connected sources — enhanced with AI descriptions, muscle heatmaps, and heart rate data — appear in your TrainingPeaks calendar automatically.
  `,
  features: [
    '✅ Upload boosted activities to TrainingPeaks automatically',
    '✅ Duration, distance, and heart rate data included',
    '✅ Activity types mapped to TrainingPeaks workout types',
    '✅ Secure OAuth connection',
  ],
  iconType: 'png',
  iconPath: '/images/icons/trainingpeaks.png',
  actions: [],
});

registerIntegration({
  id: 'spotify',
  name: 'Spotify',
  description: 'Connect your Spotify account to track music during activities',
  icon: '🎵',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  isTemporarilyUnavailable: true,
  docsUrl: 'https://www.spotify.com',
  setupTitle: 'Connect Spotify',
  setupInstructions: `To connect Spotify, you'll authorize FitGlue to access your listening history:

1. **Click Connect** — You'll be redirected to Spotify's authorization page
2. **Sign in to Spotify** — Use your Spotify account credentials
3. **Authorize FitGlue** — Grant permission to read your recently played tracks
4. **Done!** — You'll be redirected back to FitGlue

Once connected, FitGlue can track what music you listened to during your activities.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Spotify?
Spotify is the world's leading music streaming platform with millions of songs, podcasts, and playlists.

### What FitGlue Does
FitGlue connects to your Spotify account and tracks what music you listened to during your workouts. See your top played tracks, discover your workout playlists, and share your activity soundtrack.
  `,
  features: [
    '✅ Track music played during activities',
    '✅ See top played tracks',
    '✅ Identify workout playlists',
    '✅ Automatic time-window matching',
  ],
  iconType: 'png',
  iconPath: '/images/icons/spotify.png',
  actions: [],
});

registerIntegration({
  id: 'google',
  name: 'Google',
  description: 'Connect your Google account',
  icon: '🔵',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  docsUrl: 'https://developers.google.com/sheets',
  setupTitle: 'Connect Google',
  setupInstructions: `To connect Google, you'll authorize FitGlue to access your Google Sheets:

1. **Click Connect** — You'll be redirected to Google's authorization page
2. **Sign in to Google** — Use your Google account credentials
3. **Authorize FitGlue** — Grant permission to read and write to your spreadsheets
4. **Done!** — You'll be redirected back to FitGlue

Once connected, FitGlue can log your activities to any Google Sheet you specify.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Google Sheets?
Google Sheets is a powerful, cloud-based spreadsheet application that's part of Google Workspace. It's perfect for tracking, analyzing, and visualizing your fitness data.

### What FitGlue Does
FitGlue connects to your Google account and automatically logs your activities to a spreadsheet of your choice. Each activity becomes a row with customizable columns including stats, visual assets, and showcase links.
  `,
  features: [
    '✅ Automatic activity logging to Google Sheets',
    '✅ Customizable columns and data fields',
    '✅ Visual assets via IMAGE formulas',
    '✅ Secure OAuth connection',
  ],
  iconType: 'png',
  iconPath: '/images/icons/google.png',
  actions: [],
});

registerIntegration({
  id: 'oura',
  name: 'Oura Ring',
  description: 'Import workouts from your Oura Ring',
  icon: '💍',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  isTemporarilyUnavailable: true,
  docsUrl: 'https://cloud.ouraring.com/docs',
  setupTitle: 'Connect Oura Ring',
  setupInstructions: `Connect your Oura account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on Oura Ring
3. Sign in to your **Oura account** when redirected
4. Review and **Accept Permissions** to allow FitGlue to access your workouts
5. You're connected! Workouts will sync automatically

FitGlue uses secure OAuth — your Oura password is never stored.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Oura Ring?
Oura Ring is a smart ring that tracks sleep, readiness, and activity. It provides detailed insights into your recovery and overall health, with workout tracking capabilities.

### What FitGlue Does
FitGlue connects to your Oura account via OAuth and imports your tracked workouts. Heart rate data, calories, and workout duration flow through your FitGlue pipeline for enhancement and distribution to destinations like Strava.
  `,
  features: [
    '✅ Import workouts tracked by your Oura Ring',
    '✅ Heart rate data (average and max) included',
    '✅ Automatic sync of new workouts',
    '✅ Secure OAuth connection',
  ],
  iconType: 'jpg',
  iconPath: '/images/icons/oura.jpg',
  actions: [],
});

registerIntegration({
  id: 'polar',
  name: 'Polar Flow',
  description: 'Import activities from Polar wearables',
  icon: '❄️',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  isTemporarilyUnavailable: true,
  docsUrl: 'https://www.polar.com/accesslink-api',
  setupTitle: 'Connect Polar Flow',
  setupInstructions: `Connect your Polar Flow account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on Polar Flow
3. Sign in to your **Polar account** when redirected
4. Review and **Accept Permissions** to allow FitGlue to access your activities
5. You're connected! Activities will sync automatically

FitGlue uses secure OAuth — your Polar password is never stored.

**Note:** App registration at admin.polaraccesslink.com is required.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Polar Flow?
Polar Flow is the cloud platform for Polar wearables and sports watches. It stores your training data, provides analytics, and syncs across devices.

### What FitGlue Does
FitGlue connects to your Polar Flow account via OAuth and imports your activities via webhooks. Heart rate data, GPS routes, and training metrics flow through your FitGlue pipeline for enhancement and distribution to destinations like Strava.

### Transaction-Based Reliability
Polar uses a unique transaction-based API that ensures no activities are lost. FitGlue handles the transaction lifecycle automatically.
  `,
  features: [
    '✅ Import activities from Polar watches',
    '✅ Heart rate data with zones included',
    '✅ GPS routes for outdoor activities',
    '✅ Training load metrics',
    '✅ Real-time sync via webhooks',
    '✅ Secure OAuth connection',
  ],
  iconType: 'svg',
  iconPath: '/images/icons/polar.svg',
  actions: [],
});

registerIntegration({
  id: 'intervals',
  name: 'Intervals.icu',
  description: 'Advanced training analytics with power and performance metrics',
  icon: '📊',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_API_KEY,
  enabled: true,
  isTemporarilyUnavailable: true,
  docsUrl: 'https://intervals.icu/api',
  setupTitle: 'Connect Intervals.icu',
  setupInstructions: `To connect Intervals.icu, you'll need your **API Key** and **Athlete ID**:

1. Go to **Intervals.icu** and log in
2. Navigate to **Settings** → **Developer Settings**
3. Click **Generate API Key** and copy the key
4. Find your **Athlete ID** in the URL: intervals.icu/athlete/**{your-id}**/calendar
5. Enter both values in your **FitGlue Dashboard**

**Note:** The API key and athlete ID are unique to your account. Keep them secure.`,
  apiKeyLabel: 'API Key',
  apiKeyHelpUrl: 'https://intervals.icu/api',
  marketingDescription: `
### What is Intervals.icu?
Intervals.icu is a free, advanced training analytics platform popular with cyclists and triathletes. It provides detailed power analysis, training load tracking, fitness modeling (CTL/ATL/TSB), and race planning tools.

### What FitGlue Does
FitGlue connects to your Intervals.icu account via API key and can upload your boosted activities. Your enriched workouts — with AI descriptions, heart rate summaries, and performance metrics — appear in your Intervals.icu calendar for detailed analysis.
  `,
  features: [
    '✅ Upload boosted activities to Intervals.icu automatically',
    '✅ Power, heart rate, and GPS data included',
    '✅ FIT files uploaded for full data fidelity',
    '✅ Simple API key setup — no OAuth required',
    '✅ Works with all FitGlue boosters',
  ],
  iconType: 'png',
  iconPath: '/images/icons/intervals.png',
  actions: [],
});

registerIntegration({
  id: 'wahoo',
  name: 'Wahoo',
  description: 'Import workouts from Wahoo Cloud',
  icon: '📱',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  isTemporarilyUnavailable: true,
  docsUrl: 'https://developers.wahooligan.com',
  setupTitle: 'Connect Wahoo',
  setupInstructions: `Connect your Wahoo account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on Wahoo
3. Sign in to your **Wahoo account** when redirected
4. Review and **Accept Permissions** to allow FitGlue to access your workouts
5. You're connected! Activities will sync automatically

FitGlue uses secure OAuth — your Wahoo password is never stored.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Wahoo?
Wahoo is a leading cycling technology company known for ELEMNT bike computers, KICKR smart trainers, and TICKR heart rate monitors. Wahoo Cloud stores your workout data from all Wahoo devices.

### What FitGlue Does
FitGlue connects to your Wahoo account via OAuth and imports your workouts with full FIT file data. Power, heart rate, cadence, and GPS data flow through your FitGlue pipeline for enhancement and distribution to destinations like Strava or TrainingPeaks.
  `,
  features: [
    '✅ Import workouts from ELEMNT, KICKR, and all Wahoo devices',
    '✅ Full FIT file support with complete sensor data',
    '✅ Power, HR, cadence, and GPS data included',
    '✅ Real-time sync via webhooks',
    '✅ Secure OAuth connection',
  ],
  iconType: 'jpg',
  iconPath: '/images/icons/wahoo.jpg',
  actions: [],
});

registerIntegration({
  id: 'garmin',
  name: 'Garmin',
  description: 'Import activities from Garmin Connect',
  icon: '⌚',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  isTemporarilyUnavailable: true,
  docsUrl: '',
  setupTitle: 'Connect Garmin',
  setupInstructions: `Connect your Garmin Connect account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on Garmin
3. Sign in to your **Garmin account** when redirected
4. Review and **Accept Permissions** to allow FitGlue to access your activities
5. You're connected! Activities will sync automatically

FitGlue uses secure OAuth — your Garmin password is never stored.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Garmin Connect?
Garmin Connect is the tool for tracking, analyzing and sharing health and fitness activities from your Garmin device.

### What FitGlue Does
FitGlue connects to your Garmin Connect account via OAuth and imports your activities. Heart rate data, GPS routes, and training metrics flow through your FitGlue pipeline for enhancement and distribution to destinations like Strava.
  `,
  features: [
    '✅ Import activities from Garmin devices',
    '✅ Heart rate and GPS data included',
    '✅ Secure OAuth connection',
  ],
  iconType: 'svg',
  iconPath: '/images/icons/garmin.svg',
  actions: [],
});

registerIntegration({
  id: 'github',
  name: 'GitHub',
  description: 'Connect your GitHub account for repository-based activity tracking',
  icon: '🐙',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH,
  enabled: true,
  docsUrl: 'https://docs.github.com/en/apps/oauth-apps/building-oauth-apps',
  setupTitle: 'Connect GitHub',
  setupInstructions: `Connect your GitHub account to FitGlue with secure OAuth:

1. Open the **FitGlue Dashboard**
2. Navigate to **Connections** and click **Connect** on GitHub
3. Sign in to your **GitHub account** when redirected
4. Review the requested permissions and click **Authorize FitGlue**
5. You're connected! Choose a repository and folder to get started

**Permissions:** FitGlue requests the \`repo\` scope, which allows reading and writing repository contents. This is required for both source (reading your activity files) and destination (committing enriched activities). FitGlue only accesses the specific repository and folder you configure.

**Webhook Setup (Source only):** If you want to use GitHub as a source, you'll need to add a webhook to your repository:
1. Go to your repository's **Settings → Webhooks → Add webhook**
2. Set **Payload URL** to \`https://api.fitglue.com/github/webhook\`
3. Set **Content type** to \`application/json\`
4. Set **Secret** to the shared webhook secret shown in your FitGlue dashboard
5. Select **Just the push event**
6. Click **Add webhook**`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is GitHub?
GitHub is the world's largest platform for software development and version control. With Git-based repositories, every change is tracked, versioned, and recoverable — making it an ideal platform for maintaining structured data like fitness activity logs.

### What FitGlue Does
FitGlue connects to your GitHub account via OAuth, enabling a bidirectional integration. As a **source**, FitGlue watches a repository folder for new Markdown activity files and pulls them into your pipeline. As a **destination**, FitGlue commits enriched activities back to a repository as version-controlled Markdown documents.

### How it works
- **Source flow:** You push a Markdown file with YAML frontmatter to your repo → GitHub fires a webhook → FitGlue parses the metadata and optional FIT file → your activity enters the pipeline for enrichment.
- **Destination flow:** Your activity passes through the pipeline → boosters add descriptions, metrics, and tags → FitGlue commits the enriched result to your repo as a Markdown file organised by date.

### Safety & Privacy
- All webhook payloads are verified with HMAC-SHA256 signature validation.
- OAuth tokens are stored encrypted and never exposed.
- Automatic loop prevention ensures FitGlue's own commits are never re-processed.
- FitGlue only accesses the specific repository and folder you configure.
  `,
  features: [
    '✅ Bidirectional — use as source, destination, or both',
    '✅ Secure OAuth connection — no passwords stored',
    '✅ HMAC-SHA256 webhook verification',
    '✅ Works with public and private repositories',
    '✅ Automatic loop prevention',
    '✅ Version-controlled activity history',
  ],
  iconType: 'svg',
  iconPath: '/images/icons/github.svg',
  actions: [],
});
