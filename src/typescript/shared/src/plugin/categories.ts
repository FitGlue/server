/**
 * Plugin Category Constants
 *
 * Canonical category IDs used in registry.ts and exposed via the API.
 * Frontend components should use these same IDs for filtering/grouping.
 */

// Source categories
export const CATEGORY_WEARABLES = 'wearables';
export const CATEGORY_APPS = 'apps';
export const CATEGORY_MANUAL = 'manual';

// Enricher categories
export const CATEGORY_AI_CONTENT = 'ai_content';
export const CATEGORY_STATS = 'stats';
export const CATEGORY_DETECTION = 'detection';
export const CATEGORY_TRANSFORMATION = 'transformation';
export const CATEGORY_VISUAL = 'visual';
export const CATEGORY_LOCATION = 'location';
export const CATEGORY_LOGIC = 'logic';
export const CATEGORY_REFERENCES = 'references';

// Destination categories
export const CATEGORY_SOCIAL = 'social';
export const CATEGORY_ANALYTICS = 'analytics';
export const CATEGORY_LOGGING = 'logging';

/**
 * Category metadata for UI rendering
 */
export interface CategoryMeta {
  id: string;
  name: string;
  emoji: string;
}

export const SOURCE_CATEGORIES: CategoryMeta[] = [
  { id: CATEGORY_WEARABLES, name: 'Wearables', emoji: '⌚' },
  { id: CATEGORY_APPS, name: 'Apps', emoji: '📱' },
  { id: CATEGORY_MANUAL, name: 'Manual', emoji: '📄' },
];

export const ENRICHER_CATEGORIES: CategoryMeta[] = [
  { id: CATEGORY_AI_CONTENT, name: 'AI & Content', emoji: '✨' },
  { id: CATEGORY_STATS, name: 'Stats', emoji: '📊' },
  { id: CATEGORY_DETECTION, name: 'Detection', emoji: '🎯' },
  { id: CATEGORY_TRANSFORMATION, name: 'Transformation', emoji: '🔧' },
  { id: CATEGORY_VISUAL, name: 'Visual', emoji: '🎨' },
  { id: CATEGORY_LOCATION, name: 'Location', emoji: '🗺️' },
  { id: CATEGORY_LOGIC, name: 'Logic', emoji: '⚙️' },
  { id: CATEGORY_REFERENCES, name: 'References', emoji: '🔗' },
];

export const DESTINATION_CATEGORIES: CategoryMeta[] = [
  { id: CATEGORY_SOCIAL, name: 'Social', emoji: '🌐' },
  { id: CATEGORY_ANALYTICS, name: 'Analytics', emoji: '📈' },
  { id: CATEGORY_LOGGING, name: 'Logging', emoji: '📊' },
];

/**
 * Type-safe category unions for compile-time validation
 */
export type SourceCategory = typeof CATEGORY_WEARABLES | typeof CATEGORY_APPS | typeof CATEGORY_MANUAL;
export type EnricherCategory = typeof CATEGORY_AI_CONTENT | typeof CATEGORY_STATS | typeof CATEGORY_DETECTION | typeof CATEGORY_TRANSFORMATION | typeof CATEGORY_VISUAL | typeof CATEGORY_LOCATION | typeof CATEGORY_LOGIC | typeof CATEGORY_REFERENCES;
export type DestinationCategory = typeof CATEGORY_SOCIAL | typeof CATEGORY_ANALYTICS | typeof CATEGORY_LOGGING;
