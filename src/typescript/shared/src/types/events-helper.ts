import { CloudEventType, CloudEventSource } from './pb/events';

// Map CloudEventType enum to string URN
export const CloudEventTypeURN: Record<number, string> = {
  [CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_CREATED]: "com.fitglue.activity.created",
  [CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_ENRICHED]: "com.fitglue.activity.enriched",
  [CloudEventType.CLOUD_EVENT_TYPE_JOB_ROUTED]: "com.fitglue.job.routed",
  [CloudEventType.CLOUD_EVENT_TYPE_FITBIT_NOTIFICATION]: "com.fitglue.fitbit.notification",
  [CloudEventType.CLOUD_EVENT_TYPE_ENRICHMENT_LAG]: "com.fitglue.enrichment.lag",
  [CloudEventType.CLOUD_EVENT_TYPE_INPUT_RESOLVED]: "com.fitglue.input.resolved",
};

// Map CloudEventSource enum to string URN
export const CloudEventSourceURN: Record<number, string> = {
  [CloudEventSource.CLOUD_EVENT_SOURCE_HEVY]: "/integrations/hevy",
  [CloudEventSource.CLOUD_EVENT_SOURCE_FITBIT_WEBHOOK]: "/integrations/fitbit/webhook",
  [CloudEventSource.CLOUD_EVENT_SOURCE_FITBIT_INGEST]: "/integrations/fitbit/ingest",
  [CloudEventSource.CLOUD_EVENT_SOURCE_ENRICHER]: "/core/enricher",
  [CloudEventSource.CLOUD_EVENT_SOURCE_ROUTER]: "/core/router",
  [CloudEventSource.CLOUD_EVENT_SOURCE_INPUTS_HANDLER]: "/core/inputs-handler",
};

export function getCloudEventType(t: CloudEventType): string {
  return CloudEventTypeURN[t] || "unknown";
}

export function getCloudEventSource(s: CloudEventSource): string {
  return CloudEventSourceURN[s] || "unknown";
}
