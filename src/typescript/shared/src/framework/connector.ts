import { StandardizedActivity } from '../types/pb/standardized_activity';
import { CloudEventSource } from '../types/pb/events';
import { ActivitySource } from '../types/pb/activity';


/**
 * ConnectorConfig defines the base configuration required by all connectors.
 */
export interface ConnectorConfig {
  enabled: boolean;
  /**
   * Secret keys or tokens required by the connector.
   * Can be mapped from Secret Manager or Env Vars.
   */
  secrets?: Record<string, string>;
}

/**
 * IngestStrategy defines how the connector receives data.
 * - 'webhook': Passive listener (push)
 * - 'polling': Active fetcher (pull)
 * - 'hybrid': Both (e.g. Fitbit uses webhook for notification, then polling for data)
 */
export type IngestStrategy = 'webhook' | 'polling' | 'hybrid';

/**
 * ActivityMapper defines the function signature for converting raw vendor payloads
 * into the FitGlue StandardizedActivity format.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export interface ActivityMapper<T = any> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (rawPayload: T, context?: any): Promise<StandardizedActivity>;
}

/**
 * Connector defines the contract that all third-party integrations must implement.
 * TConfig: The specific configuration type for this connector.
 * TRaw: The type of the raw payload from the vendor.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export interface Connector<TConfig extends ConnectorConfig = ConnectorConfig, TRaw = any> {
  /**
   * Unique identifier for the connector (e.g., 'hevy', 'fitbit').
   * This should match the directory name in `integrations/`.
   */
  readonly name: string;

  /**
   * The strategy used to ingest data.
   */
  readonly strategy: IngestStrategy;

  /**
   * The CloudEvent Source URI to use when publishing events.
   * e.g. CloudEventSource.CLOUD_EVENT_SOURCE_HEVY
   */
  readonly cloudEventSource: CloudEventSource;

  /**
   * The ActivitySource enum value for this connector.
   * e.g. ActivitySource.SOURCE_HEVY
   */
  readonly activitySource: ActivitySource;

  /**
   * Validates the incoming configuration.
   * Throws if required secrets or config values are missing.
   */
  validateConfig(config: TConfig): void;

  /**
   * Core logic to map the vendor-specific payload to a standardized format.
   * This is the "glue" logic.
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  mapActivity(rawPayload: TRaw, context?: any): Promise<StandardizedActivity>;

  /**
   * Extracts the unique external ID from a webhook payload.
   * Useful for deduplication before processing.
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  extractId(payload: any): string | null;

  /**
   * Full retrieval lifecycle:
   * 1. (Optional) Fetch full data from vendor API using ID.
   * 2. (Optional) Fetch dependencies (e.g. templates).
   * 3. Map to StandardizedActivity.
   *
   * Always returns an array to support batch processing (e.g. Fitbit date-based webhooks).
   * Single-activity connectors should wrap the result in an array.
   */
  fetchAndMap(activityId: string, config: TConfig): Promise<StandardizedActivity[]>;

  /**
   * Health check to verify connectivity/auth with the vendor.
   * Default implementation returns true (stateless/assumed healthy).
   */
  healthCheck(): Promise<boolean>;

  /**
   * Custom request verification for vendor-specific requirements.
   * Examples: Fitbit's GET verification endpoint, HMAC signature validation, etc.
   *
   * If this method returns a response object, the webhook processor will skip normal processing
   * and return that response immediately.
   *
   * Default implementation returns undefined (no custom verification).
   *
   * @returns undefined to continue normal processing, or a response object to short-circuit
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  verifyRequest(req: any, context: import('./index').FrameworkContext): Promise<{ handled: boolean; response?: any } | undefined>;

  /**
   * Resolve user ID from webhook payload.
   * Used for webhooks that don't include per-user authentication (like Fitbit).
   *
   * Default implementation returns null (uses auth-provided userId).
   *
   * @returns userId if resolved from payload, null to use auth-provided userId
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  resolveUser?(payload: any, context: import('./index').FrameworkContext): Promise<string | null>;
}
