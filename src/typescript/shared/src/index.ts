export * from './errors';
export * from './config';
export * from './infrastructure/secrets';
export * from './infrastructure/crypto';
export * from './framework/index';
export * from './framework/auth';
export * from './framework/auth-strategies';
export * from './routing';

// Types
export { ActivityPayload, ActivitySource } from './types/pb/activity';
export { StandardizedActivity, Session, Lap, StrengthSet, MuscleGroup, Record, ActivityType } from './types/pb/standardized_activity';
export { ExecutionRecord, ExecutionStatus } from './types/pb/execution';
export { CloudEventType, CloudEventSource, Destination } from './types/pb/events';
export * from './types/events-helper';
export { ApiKeyRecord } from './types/pb/auth';
export { UserRecord, UserTier, UserIntegrations, HevyIntegration, EnricherProviderType, EnricherConfig, ProcessedActivityRecord, PipelineConfig, SynchronizedActivity, ShowcasedActivity } from './types/pb/user';
export { FitbitNotification } from './types/pb/fitbit';
export { PendingInput, PendingInput_Status } from './types/pb/pending_input';
export * from './types/integrations';

// Plugin Registry
export * from './plugin/registry';
export * from './plugin/categories';
export { PluginManifest, PluginRegistryResponse, PluginType, ConfigFieldType, ConfigFieldSchema, ConfigFieldOption, IntegrationAuthType, IntegrationManifest } from './types/pb/plugin';

// Services
export * from './domain/services/user';
export * from './domain/services/execution';
export * from './domain/services/apikey';
export * from './domain/services/inputs';
export * from './domain/services/authorization';

// Domain Logic
export * from './domain/tier';

// Integrations
export * from './integrations/hevy/client';
export * from './integrations/fitbit/client';
export * from './integrations/strava/client';
export * from './integrations/oura/client';
export * from './integrations/polar/client';
export * from './integrations/spotify/client';
export * from './integrations/wahoo/client';
export * from './integrations/trainingpeaks/client';
export * from './infrastructure/oauth';

// Infrastructure
export * from './infrastructure/pubsub/cloud-event-publisher';
export * as storage from './storage/firestore';
export { UserStore, ActivityStore, ApiKeyStore, ExecutionStore, IntegrationIdentityStore, InputStore, ShowcaseStore } from './storage/firestore';
export { mapTCXToStandardized } from './domain/file-parsers/tcx';
export * from './execution/logger';

// Converters
export * from './storage/firestore/converters';

// Enum Formatters
export * from './types/pb/enum-formatters';

// Activity Counters (Phase 2 performance optimization)
export * from './services/activity-counter-service';
export * from './types/activity-counters';
