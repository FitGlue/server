import { Connector, ConnectorConfig, IngestStrategy } from './connector';
import { StandardizedActivity } from '../types/pb/standardized_activity';
import { CloudEventSource } from '../types/pb/events';
import { ActivitySource } from '../types/pb/activity';

/**
 * BaseConnector provides a default implementation for common connector tasks.
 * It enforces configuration validation structure.
 */
export abstract class BaseConnector<TConfig extends ConnectorConfig = ConnectorConfig, TRaw = any>
  implements Connector<TConfig, TRaw> {

  abstract readonly name: string;
  abstract readonly strategy: IngestStrategy;
  abstract readonly cloudEventSource: CloudEventSource;
  abstract readonly activitySource: ActivitySource;

  constructor() { }

  /**
   * Default validation: checks if 'enabled' is present.
   * Override this to add specific config validation (e.g. API keys).
   * Always call super.validateConfig(config) when overriding.
   */
  validateConfig(config: TConfig): void {
    if (config.enabled === undefined) {
      throw new Error(`Connector ${this.name}: 'enabled' flag is missing`);
    }
  }

  /**
   * Abstract mapping function that must be implemented by the concrete connector.
   */
  abstract mapActivity(rawPayload: TRaw, context?: any): Promise<StandardizedActivity>;

  abstract extractId(payload: any): string | null;

  abstract fetchAndMap(activityId: string, config: TConfig): Promise<StandardizedActivity[]>;

  /**
   * Health check. Defaults to true (stateless/assumed healthy).
   * Override to add vendor-specific health checks.
   */
  async healthCheck(): Promise<boolean> {
    return true;
  }

  /**
   * Request verification. Defaults to no custom verification.
   * Override to add vendor-specific verification (e.g. Fitbit's GET endpoint, signature checks).
   */
  async verifyRequest(req: any, res: any, context: any): Promise<{ handled: boolean; response?: any } | undefined> {
    return undefined;
  }
}
