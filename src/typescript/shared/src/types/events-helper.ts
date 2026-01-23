import { CloudEventType, CloudEventSource, Destination } from './pb/events';
import { formatDestination } from './pb/enum-formatters';

// Map CloudEventType enum to string URN
export const CloudEventTypeURN: Record<number, string> = {
  [CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_CREATED]: 'com.fitglue.activity.created',
  [CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_ENRICHED]: 'com.fitglue.activity.enriched',
  [CloudEventType.CLOUD_EVENT_TYPE_JOB_ROUTED]: 'com.fitglue.job.routed',
  [CloudEventType.CLOUD_EVENT_TYPE_FITBIT_NOTIFICATION]: 'com.fitglue.fitbit.notification',
  [CloudEventType.CLOUD_EVENT_TYPE_ENRICHMENT_LAG]: 'com.fitglue.enrichment.lag',
  [CloudEventType.CLOUD_EVENT_TYPE_INPUT_RESOLVED]: 'com.fitglue.input.resolved',
};

// Map CloudEventSource enum to string URN
export const CloudEventSourceURN: Record<number, string> = {
  [CloudEventSource.CLOUD_EVENT_SOURCE_HEVY]: '/integrations/hevy',
  [CloudEventSource.CLOUD_EVENT_SOURCE_FITBIT_WEBHOOK]: '/integrations/fitbit/webhook',
  [CloudEventSource.CLOUD_EVENT_SOURCE_FITBIT_INGEST]: '/integrations/fitbit/ingest',
  [CloudEventSource.CLOUD_EVENT_SOURCE_ENRICHER]: '/core/enricher',
  [CloudEventSource.CLOUD_EVENT_SOURCE_ROUTER]: '/core/router',
  [CloudEventSource.CLOUD_EVENT_SOURCE_INPUTS_HANDLER]: '/core/inputs-handler',
};

// Map Destination enum to Pub/Sub topic
// These mappings match the dest_topic extensions in events.proto
export const DestinationTopics: Record<number, string> = {
  [Destination.DESTINATION_STRAVA]: 'topic-job-upload-strava',
  [Destination.DESTINATION_SHOWCASE]: 'topic-job-upload-showcase',
  [Destination.DESTINATION_MOCK]: 'topic-job-upload-mock',
  [Destination.DESTINATION_HEVY]: 'topic-job-upload-hevy',
  [Destination.DESTINATION_TRAININGPEAKS]: 'topic-job-upload-trainingpeaks',
  [Destination.DESTINATION_INTERVALS]: 'topic-job-upload-intervals',
  [Destination.DESTINATION_GOOGLESHEETS]: 'topic-job-upload-googlesheets',
};

export function getCloudEventType(t: CloudEventType): string {
  return CloudEventTypeURN[t] || 'unknown';
}

export function getCloudEventSource(s: CloudEventSource): string {
  return CloudEventSourceURN[s] || 'unknown';
}

/**
 * Get the Pub/Sub topic for a destination.
 * @param destination - Destination enum value or string name (e.g., "strava", "DESTINATION_STRAVA")
 * @returns The topic name, or undefined if not found
 */
export function getDestinationTopic(destination: Destination | string): string | undefined {
  // Handle enum value directly
  if (typeof destination === 'number') {
    return DestinationTopics[destination] || undefined;
  }

  // Handle string input (e.g., "strava", "DESTINATION_STRAVA")
  const normalized = destination.toUpperCase();
  const key = normalized.startsWith('DESTINATION_')
    ? normalized
    : `DESTINATION_${normalized}`;

  // Look up enum value from string
  const enumValue = Destination[key as keyof typeof Destination];
  if (typeof enumValue === 'number') {
    return DestinationTopics[enumValue] || undefined;
  }

  return undefined;
}

/**
 * Get the destination key name from enum value.
 * Uses generated formatter for display-friendly names.
 * @param destination - Destination enum value
 * @returns Lowercase destination name (e.g., "strava", "showcase")
 */
export function getDestinationName(destination: Destination): string {
  return formatDestination(destination).toLowerCase();
}

/**
 * Parse a destination string to its enum value.
 * @param destination - String like "strava", "showcase", or "DESTINATION_STRAVA"
 * @returns Destination enum value, or undefined if not found
 */
export function parseDestination(destination: string): Destination | undefined {
  const normalized = destination.toUpperCase();
  const key = normalized.startsWith('DESTINATION_')
    ? normalized
    : `DESTINATION_${normalized}`;

  const enumValue = Destination[key as keyof typeof Destination];
  return typeof enumValue === 'number' ? enumValue : undefined;
}

