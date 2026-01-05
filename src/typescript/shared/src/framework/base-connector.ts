import { Connector, ConnectorConfig, IngestStrategy } from './connector';
import { StandardizedActivity } from '../types/pb/standardized_activity';
import { CloudEventSource } from '../types/pb/events';
import { ActivitySource } from '../types/pb/activity';
import { FrameworkContext } from './index';

/**
 * BaseConnector provides a default implementation for common connector tasks.
 * It enforces configuration validation structure.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export abstract class BaseConnector<TConfig extends ConnectorConfig = ConnectorConfig, TRawPayload = any>
  implements Connector<TConfig, TRawPayload> {
  protected _context: FrameworkContext;

  protected get context(): FrameworkContext {
    if (!this._context) {
      throw new Error(`Connector ${this.name}: FrameworkContext not set. Ensure context is passed in constructor.`);
    }
    return this._context;
  }

  abstract readonly name: string;
  abstract readonly strategy: IngestStrategy;
  abstract readonly cloudEventSource: CloudEventSource;
  abstract readonly activitySource: ActivitySource;

  constructor(context: FrameworkContext) {
    this._context = context;
  }

  /**
   * Default validation: checks if 'enabled' is present.
   * Override this to add specific config validation (e.g. API keys).
   * Always call super.validateConfig(config) when overriding.
   */
  validateConfig(config: TConfig): void {
    if (config.enabled === undefined) {
      throw new Error(`Connector ${this.name}: 'enabled' flag is missing`);
    }
    this.context.logger.info(`Connector ${this.name}: validated config`, {
      config,
      name: this.name,
      strategy: this.strategy,
      cloudEventSource: this.cloudEventSource,
      activitySource: this.activitySource,
    });
  }

  /**
   * Verify a request. Defaults to no verification.
   * Override to add vendor-specific verification.
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async verifyRequest(_req: any, _res: any, _context: any): Promise<{ handled: boolean; response?: any } | undefined> {
    return undefined;
  }

  /**
   * Extract an ID from the raw payload. Defaults to null.
   */
  abstract extractId(payload: TRawPayload): string | null;

  /**
   * Map a raw payload to a StandardizedActivity.
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  abstract mapActivity(rawPayload: TRawPayload, context?: any): Promise<StandardizedActivity>;

  /**
   * Fetch and map activities for a given ID.
   */
  abstract fetchAndMap(activityId: string, config: TConfig): Promise<StandardizedActivity[]>;

  /**
   * Health check. Defaults to true (stateless/assumed healthy).
   * Override to add vendor-specific health checks.
   * Default implementations for optional methods
   */
  async healthCheck(): Promise<boolean> {
    return true; // Stateless connectors are assumed healthy
  }
}
