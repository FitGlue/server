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

// Enricher categories (Output-based taxonomy)
export const CATEGORY_AI_IMAGES = 'ai_images';
export const CATEGORY_SUMMARIES = 'summaries';
export const CATEGORY_DATA = 'data';
export const CATEGORY_DETECTION = 'detection';
export const CATEGORY_LINKS = 'links';
export const CATEGORY_WORKFLOW = 'workflow';

// Destination categories
export const CATEGORY_SOCIAL = 'social';
export const CATEGORY_ANALYTICS = 'analytics';
export const CATEGORY_LOGGING = 'logging';
export const CATEGORY_DEVELOPER = 'developer';

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
  { id: CATEGORY_AI_IMAGES, name: 'AI & Images', emoji: '✨' },
  { id: CATEGORY_SUMMARIES, name: 'Summaries', emoji: '📝' },
  { id: CATEGORY_DATA, name: 'Data & Stats', emoji: '📊' },
  { id: CATEGORY_DETECTION, name: 'Smart Detection', emoji: '🎯' },
  { id: CATEGORY_LINKS, name: 'Links & References', emoji: '🔗' },
  { id: CATEGORY_WORKFLOW, name: 'Workflow', emoji: '⚙️' },
];

export const DESTINATION_CATEGORIES: CategoryMeta[] = [
  { id: CATEGORY_SOCIAL, name: 'Social', emoji: '🌐' },
  { id: CATEGORY_ANALYTICS, name: 'Analytics', emoji: '📈' },
  { id: CATEGORY_LOGGING, name: 'Logging', emoji: '📊' },
  { id: CATEGORY_DEVELOPER, name: 'Developer', emoji: '🛠️' },
];

/**
 * Type-safe category unions for compile-time validation
 */
export type SourceCategory = typeof CATEGORY_WEARABLES | typeof CATEGORY_APPS | typeof CATEGORY_MANUAL;
export type EnricherCategory = typeof CATEGORY_AI_IMAGES | typeof CATEGORY_SUMMARIES | typeof CATEGORY_DATA | typeof CATEGORY_DETECTION | typeof CATEGORY_LINKS | typeof CATEGORY_WORKFLOW;
export type DestinationCategory = typeof CATEGORY_SOCIAL | typeof CATEGORY_ANALYTICS | typeof CATEGORY_LOGGING | typeof CATEGORY_DEVELOPER;
