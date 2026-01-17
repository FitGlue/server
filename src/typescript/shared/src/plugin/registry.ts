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
  useCases: [],
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
  useCases: [],
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
});

registerSource({
  id: 'apple-health',
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
  useCases: [],
});

registerSource({
  id: 'health-connect',
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
  useCases: [],
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
});

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
### Share Your Enhanced Activities
Upload your enriched activities to Strava automatically. Your AI descriptions, muscle heatmaps, and merged heart rate data will appear on your Strava feed.

### How it works
Once activities pass through your enrichment pipeline, FitGlue uploads them to Strava via the official API. They appear as native Strava activities, complete with all the enhancements you've configured.
  `,
  features: [
    '✅ Upload activities to Strava automatically',
    '✅ AI descriptions and muscle heatmaps included',
    '✅ Heart rate and GPS data attached',
    '✅ Secure OAuth connection to your Strava account',
  ],
  transformations: [],
  useCases: [],
});

registerDestination({
  id: 'showcase',
  type: PluginType.PLUGIN_TYPE_DESTINATION,
  name: 'Showcase',
  description: 'Generate a public, shareable link to your enriched activity',
  icon: '🔗',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  destinationType: 2, // DestinationType.DESTINATION_SHOWCASE
  marketingDescription: `
### Share Your Magic
Create beautiful, public links to your enriched activities. Share your workout data—HR graphs, GPS maps, boosters applied—with anyone, no login required.

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
    'Share workouts with friends and coaches',
    'Embed activity summaries in blogs',
    'Create a public fitness portfolio',
  ],
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
This booster uses advanced AI to analyze your strength training data and generate engaging, human-readable summaries of your sessions. Instead of just a list of numbers, you get a narrative description of your workout intensity, volume, and focus areas.

### How it works
FitGlue analyzes your sets, reps, and weight data, identifies your primary muscle groups targeted, and calculates total volume. It then uses a Large Language Model (LLM) to craft a summary that highlights your achievements, personal bests, and overall effort.
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
      after: `Workout Summary:
📊 20 sets • 8,240kg volume • 57 reps • Heaviest: 80kg (Bench Press)

- Bench Press: 4 x 8 × 80.0kg
- Overhead Press: 4 x 10 × 40.0kg
- Incline DB Press: 4 x 12 × 24.0kg
- Lateral Raises: 4 x 15 × 10.0kg
- Tricep Pushdowns: 4 x 12 × 25.0kg`,
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Share detailed strength logs on Strava',
    'Track progressive overload with volume stats',
    'Celebrate personal records automatically',
  ],
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
  ],
  marketingDescription: `
### Visualize Your Training
The Muscle Heatmap booster generates a visual representation of your training volume by muscle group. Using a heatmap style visualization, you can instantly see which muscles you hit hardest and which ones might be lagging.

### How it works
Every exercise in your workout is mapped to primary and secondary muscle groups using our built-in exercise taxonomy. We calculate the volume load for each muscle and generate a "heatmap" bar or chart that is appended to your activity description.

### Smart Exercise Recognition
Our database includes 100+ canonical exercises with fuzzy matching, so even custom-named exercises like "Dave's Bench Press" are correctly identified. Abbreviations (DB, BB, KB) are automatically expanded, and typos are handled gracefully.
  `,
  features: [
    '✅ Visual heatmap of trained muscles',
    '✅ Supports Emoji, Percentage, and Text formats',
    '✅ Smart exercise recognition with fuzzy matching',
    '✅ 100+ exercises in canonical database',
    '✅ Adjustable muscle coefficients (Standard, Powerlifting, Bodybuilding)',
    '✅ Works with all strength activities',
  ],
  transformations: [
    {
      field: 'description',
      label: 'Muscle Heatmap',
      before: 'Weight Training\n45 min',
      after: '',
      visualType: '',
      afterHtml: '<strong>🔥 Muscle Activation</strong><br><br><span class="heatmap-row">Chest: <span class="heatmap-bar high">🟪🟪🟪🟪🟪</span></span><br><span class="heatmap-row">Shoulders: <span class="heatmap-bar high">🟪🟪🟪🟪⬛</span></span><br><span class="heatmap-row">Triceps: <span class="heatmap-bar med">🟪🟪⬛⬛⬛</span></span><br><span class="heatmap-row">Core: <span class="heatmap-bar low">🟪⬛⬛⬛⬛</span></span>',
    },
  ],
  useCases: [
    'Visualize muscle balance in your program',
    'Show training focus areas on Strava',
    'Identify lagging muscle groups',
    'Track custom exercises with automatic muscle mapping',
  ],
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_FITBIT_HEART_RATE, {
  id: 'fitbit-heart-rate',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Fitbit Heart Rate',
  description: 'Adds heart rate data from Fitbit to your activity with smart GPS alignment',
  icon: '❤️',
  enabled: true,
  requiredIntegrations: ['fitbit'],
  configSchema: [],
  marketingDescription: `
### Unified Heart Data
Sync your heart rate data from your Fitbit device and overlay it onto your imported activities. This is perfect for when you track a workout (like weightlifting) on one app but wear your Fitbit for health monitoring.

### How it works
When an activity is imported (e.g., from Hevy), FitGlue checks your Fitbit account for heart rate data recorded during that time window. It creates a second-by-second heart rate stream and attaches it to the activity before sending it to Strava or other destinations.

### Smart GPS Alignment
When your activity has GPS data (from a phone app or watch), FitGlue uses an "Elastic Match" algorithm to align Fitbit heart rate data with your GPS timestamps. This handles minor clock drift between devices automatically, ensuring your HR matches the correct location points within ±2 seconds accuracy.
  `,
  features: [
    '✅ Merges heart rate from Fitbit to any activity',
    '✅ Smart GPS alignment handles clock drift between devices',
    '✅ Perfect for gym workouts where you don\'t start a GPS watch',
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
    'Add heart rate to Hevy gym workouts',
    'Merge Fitbit HR with phone GPS running data',
    'Complete activity data on Strava',
    'Track training intensity across all activities',
  ],
});


registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_VIRTUAL_GPS, {
  id: 'virtual-gps',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Virtual GPS',
  description: 'Adds GPS coordinates from a virtual route to indoor activities',
  icon: '🗺️',
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
Add GPS coordinates to indoor activities so they appear with a map on Strava. Choose from preset routes in famous locations like London’s Hyde Park or NYC’s Central Park.

### How it works
When an activity is processed, Virtual GPS overlays a pre-defined GPS route onto the activity. The route is scaled to match your workout duration, giving your indoor session a scenic virtual location.
  `,
  features: [
    '✅ Adds GPS to indoor/gym activities',
    '✅ Activities appear with a map on Strava',
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
  ],
  useCases: [
    'Get indoor activities on your Strava heatmap',
    'Add visual interest to home gym sessions',
    'Virtual touring while on the treadmill',
  ],
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_SOURCE_LINK, {
  id: 'source-link',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Source Link',
  description: 'Appends a link to the original activity in the description',
  icon: '🔗',
  enabled: true,
  requiredIntegrations: [],
  configSchema: [],
  marketingDescription: `
### Link Back to the Source
Automatically appends a deep link to the original activity in your workout description. Great for cross-referencing your data.

### How it works
When activities are imported from sources like Hevy or Fitbit, Source Link adds a clickable URL pointing back to the original activity. This makes it easy to see the full details in the source app.
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
      after: 'Upper Body Workout\n\n🔗 View in Hevy: https://hevy.app/workout/abc123',
      visualType: '',
      afterHtml: '',
    },
  ],
  useCases: [
    'Trace activities back to the source app',
    'Keep links to detailed exercise data',
    'Cross-reference between platforms',
  ],
});

registerEnricher(EnricherProviderType.ENRICHER_PROVIDER_TYPE_MAPPER, {
  id: 'type-mapper',
  type: PluginType.PLUGIN_TYPE_ENRICHER,
  name: 'Type Mapper',
  description: 'Maps activity types based on title keywords (e.g., title containing "Zwift" → Virtual Ride)',
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
Define matching rules like "title contains 'Zwift'" → "Virtual Ride" or "title contains 'Treadmill'" → "Run". When activities are processed, their type is automatically updated if the title matches your pattern.
  `,
  features: [
    '✅ Match activity titles with keywords',
    '✅ Full Strava activity type dropdown',
    '✅ Case-insensitive matching',
    '✅ Multiple rules per enricher',
  ],
  transformations: [
    { field: 'activityType', label: 'Activity Type', before: 'Workout (from source)', after: 'Virtual Ride (matched "Zwift" in title)', visualType: '', afterHtml: '' },
  ],
  useCases: [
    'Categorize indoor cycling as Virtual Ride',
    'Mark treadmill runs correctly',
    'Fix incorrect activity types from source apps',
  ],
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
      description: 'Template for activity title. Use {event} for event name.',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: 'Parkrun @ {event}',
      options: [],
    },
    {
      key: 'special_title_pattern',
      label: 'Special Event Title',
      description: 'Title pattern for Christmas/New Year events (e.g., 🎄 {event})',
      fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING,
      required: false,
      defaultValue: '🎄 {event} Parkrun',
      options: [],
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
Automatically detects when your activity is a Parkrun event based on GPS location and time, then enriches it with your official results.

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
    { field: 'description', label: 'Official Results', before: '(empty)', after: '🏃 **Official Parkrun Results**\n\n📍 Newark Parkrun\n🏁 Position: 42\n⏱️ Official Time: 24:12\n📊 Age Grade: 65.2%', visualType: '', afterHtml: '' },
  ],
  useCases: [
    'Auto-name Parkrun activities with official results',
    'Track your Parkrun positions and times',
    'Celebrate PBs with automatic highlighting',
    'Special event detection for Christmas & New Year',
  ],
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
    { key: 'activity_type', label: 'Activity Type', description: 'Match specific activity type', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'days_of_week', label: 'Days of Week', description: 'Comma-separated days (Mon,Wed,Sat)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'start_time', label: 'Start Time', description: 'Earliest time (HH:MM)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'end_time', label: 'End Time', description: 'Latest time (HH:MM)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'location_lat', label: 'Location Latitude', description: 'Target latitude', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'location_long', label: 'Location Longitude', description: 'Target longitude', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'radius_m', label: 'Radius (meters)', description: 'Match radius', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '200', options: [] },
    { key: 'title_template', label: 'Title Template', description: 'Title when matched', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'description_template', label: 'Description Template', description: 'Description when matched', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
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
  ],
  useCases: [
    'Auto-title recurring workouts',
    'Name activities by location',
    'Set titles by day or time',
  ],
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
    { key: 'counter_key', label: 'Counter Key', description: 'Unique identifier for this counter', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: true, defaultValue: '', options: [] },
    { key: 'title_contains', label: 'Title Filter', description: 'Only increment if title contains this', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'initial_value', label: 'Initial Value', description: 'Starting number', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '1', options: [] },
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
    { key: 'fields', label: 'Required Fields', description: 'Comma-separated fields (title,description)', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: 'description', options: [] },
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
    { key: 'exclude_activity_types', label: 'Exclude Activity Types', description: 'Comma-separated types to exclude', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'exclude_title_contains', label: 'Exclude Titles Containing', description: 'Patterns to exclude', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'include_activity_types', label: 'Include Only Activity Types', description: 'Only include these types', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
    { key: 'include_title_contains', label: 'Include Only Titles Containing', description: 'Must contain one of these', fieldType: ConfigFieldType.CONFIG_FIELD_TYPE_STRING, required: false, defaultValue: '', options: [] },
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
FitGlue connects to your Hevy account via API key, allowing your logged workouts to flow into the FitGlue pipeline. From there, you can enhance them with AI summaries, muscle heatmaps, and more — then sync them to Strava or other destinations.
  `,
  features: [
    '✅ Import all your strength workouts automatically',
    '✅ Exercises, sets, reps, and weights included',
    '✅ Real-time sync when you finish a workout',
    '✅ Simple API key setup — no OAuth required',
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
FitGlue connects to your Fitbit account via OAuth, enabling you to import activities and heart rate data. Use Fitbit as a source for activities, or overlay heart rate data onto workouts from other sources like Hevy.
  `,
  features: [
    '✅ Import activities tracked by your Fitbit device',
    '✅ Use heart rate data to enrich workouts from other sources',
    '✅ Secure OAuth connection — no passwords stored',
    '✅ Automatic sync of new activities',
  ],
});

registerIntegration({
  id: 'strava',
  name: 'Strava',
  description: 'Upload activities to Strava',
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
5. You're connected! Enhanced activities will appear on your Strava feed

FitGlue uses secure OAuth — your Strava password is never stored.`,
  apiKeyLabel: '',
  apiKeyHelpUrl: '',
  marketingDescription: `
### What is Strava?
Strava is the social network for athletes. Share your activities with friends, compete on segments, and track your training progress over time.

### What FitGlue Does
FitGlue connects to your Strava account via OAuth and can upload your enriched activities directly. Workouts from Hevy or Fitbit — enhanced with AI descriptions, muscle heatmaps, and heart rate data — appear on your Strava feed automatically.
  `,
  features: [
    '✅ Upload enriched activities to Strava automatically',
    '✅ AI-generated descriptions appear in your feed',
    '✅ Muscle heatmaps and stats included',
    '✅ Secure OAuth connection',
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
});

registerIntegration({
  id: 'parkrun',
  name: 'Parkrun',
  description: 'Enhanced Parkrun detection with official results',
  icon: '🏃',
  authType: IntegrationAuthType.INTEGRATION_AUTH_TYPE_API_KEY,
  enabled: true,
  docsUrl: 'https://www.parkrun.com',
  setupTitle: 'Connect Parkrun',
  setupInstructions: `To connect Parkrun, you'll need your athlete barcode number:

1. Find your **Parkrun barcode** — it starts with **A** followed by numbers (e.g. A12345678)
2. This is printed on your barcode card or available on the Parkrun website
3. Enter your barcode number below

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
});

