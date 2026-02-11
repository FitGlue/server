// Types - re-export commonly used types
// Note: We use selective exports to avoid name collisions (e.g., protobufPackage)

// Activity types
export { ActivityPayload, ActivitySource } from './pb/activity';

// Standardized activity types
export {
  StandardizedActivity,
  Session,
  Lap,
  StrengthSet,
  MuscleGroup,
  Record,
  ActivityType
} from './pb/standardized_activity';

// Execution types
export { ExecutionRecord, ExecutionStatus } from './pb/execution';

// Event types
export { CloudEventType, CloudEventSource, Destination } from './pb/events';

// Auth types
export { ApiKeyRecord } from './pb/auth';

// User types
export {
  UserRecord,
  UserTier,
  UserIntegrations,
  HevyIntegration,
  GitHubIntegration,
  EnricherProviderType,
  EnricherConfig,
  DestinationConfig,
  ProcessedActivityRecord,
  PipelineConfig,
  ShowcasedActivity,
  PipelineRun,
  PipelineRunStatus,
  PluginDefault
} from './pb/user';

// Fitbit types
export { FitbitNotification } from './pb/fitbit';

// Pending input types
export { PendingInput, PendingInput_Status } from './pb/pending_input';

// Plugin types
export {
  PluginManifest,
  PluginRegistryResponse,
  PluginType,
  ConfigFieldType,
  ConfigFieldSchema,
  ConfigFieldOption,
  IntegrationAuthType,
  IntegrationManifest
} from './pb/plugin';

// Enum formatters
export * from './pb/enum-formatters';

// Helper types (named exports to avoid collision with enum-formatters parseDestination)
export {
  SourceToDestinationMap,
  getCorrespondingDestination,
  CloudEventTypeURN,
  CloudEventSourceURN,
  DestinationTopics,
  getCloudEventType,
  getCloudEventSource,
  getDestinationTopic,
  getDestinationName,
} from './events-helper';
export * from './integrations';
export * from './activity-counters';
