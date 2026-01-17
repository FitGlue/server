import { FirestoreDataConverter, QueryDocumentSnapshot, Timestamp } from 'firebase-admin/firestore';
import { UserRecord, UserIntegrations, PipelineConfig, ProcessedActivityRecord } from '../../types/pb/user';
import { WaitlistEntry } from '../../types/pb/waitlist';
import { ApiKeyRecord, IntegrationIdentity } from '../../types/pb/auth';
import { ExecutionRecord, ExecutionStatus } from '../../types/pb/execution';
import { PendingInput, PendingInput_Status } from '../../types/pb/pending_input';
import { Destination } from '../../types/pb/events';
import { INTEGRATIONS, OAuthIntegrationDefinition } from '../../types/integrations';



// Helper to convert Firestore Timestamp to Date
const toDate = (val: unknown): Date | undefined => {
  if (!val) return undefined;
  if (val instanceof Timestamp) return val.toDate();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  if ((val as any).toDate) return (val as any).toDate(); // Duck typing
  return new Date(val as string | number); // Fallback string/number
};

// Helper for generic recursive snake->camel for simple objects if strictly needed,
// but manual mapping is safer for refactoring.

export const waitlistConverter: FirestoreDataConverter<WaitlistEntry> = {
  toFirestore(model: WaitlistEntry): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {};
    if (model.email !== undefined) data.email = model.email;
    if (model.source !== undefined) data.source = model.source;
    if (model.createdAt !== undefined) data.created_at = model.createdAt;
    if (model.userAgent !== undefined) data.user_agent = model.userAgent;
    if (model.ip !== undefined) data.ip = model.ip;
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): WaitlistEntry {
    const data = snapshot.data();
    return {
      email: data.email,
      source: data.source,
      createdAt: toDate(data.created_at),
      userAgent: data.user_agent,
      ip: data.ip
    };
  }
};

export const apiKeyConverter: FirestoreDataConverter<ApiKeyRecord> = {
  toFirestore(model: ApiKeyRecord): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {};
    if (model.userId !== undefined) data.user_id = model.userId;
    if (model.label !== undefined) data.label = model.label;
    if (model.scopes !== undefined) data.scopes = model.scopes;
    if (model.createdAt !== undefined) data.created_at = model.createdAt;
    if (model.lastUsedAt !== undefined) data.last_used_at = model.lastUsedAt;
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): ApiKeyRecord {
    const data = snapshot.data();
    return {
      userId: data.user_id,
      label: data.label,
      scopes: data.scopes || [],
      createdAt: toDate(data.created_at),
      lastUsedAt: toDate(data.last_used_at)
    };
  }
};

export const integrationIdentityConverter: FirestoreDataConverter<IntegrationIdentity> = {
  toFirestore(model: IntegrationIdentity): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {};
    if (model.userId !== undefined) data.user_id = model.userId;
    if (model.createdAt !== undefined) data.created_at = model.createdAt;
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): IntegrationIdentity {
    const data = snapshot.data();
    return {
      userId: data.user_id,
      createdAt: toDate(data.created_at)
    };
  }
};

export const executionConverter: FirestoreDataConverter<ExecutionRecord> = {
  toFirestore(model: ExecutionRecord): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {};

    // Only include fields that are actually defined
    if (model.executionId !== undefined) data.execution_id = model.executionId;
    if (model.service !== undefined) data.service = model.service;
    if (model.status !== undefined) data.status = model.status;
    if (model.timestamp !== undefined) data.timestamp = model.timestamp;
    if (model.userId !== undefined) data.user_id = model.userId;
    if (model.testRunId !== undefined) data.test_run_id = model.testRunId;
    if (model.triggerType !== undefined) data.trigger_type = model.triggerType;
    if (model.startTime !== undefined) data.start_time = model.startTime;
    if (model.endTime !== undefined) data.end_time = model.endTime;
    if (model.errorMessage !== undefined) data.error_message = model.errorMessage;
    if (model.inputsJson !== undefined) data.inputs_json = model.inputsJson;
    if (model.outputsJson !== undefined) data.outputs_json = model.outputsJson;
    if (model.pipelineExecutionId !== undefined) data.pipeline_execution_id = model.pipelineExecutionId;
    if (model.expireAt !== undefined) data.expire_at = model.expireAt;

    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): ExecutionRecord {
    const data = snapshot.data();
    return {
      executionId: data.execution_id,
      service: data.service,
      status: data.status || ExecutionStatus.STATUS_UNKNOWN,
      timestamp: toDate(data.timestamp),
      userId: data.user_id,
      testRunId: data.test_run_id,
      triggerType: data.trigger_type,
      startTime: toDate(data.start_time),
      endTime: toDate(data.end_time),
      errorMessage: data.error_message,
      inputsJson: data.inputs_json || data.inputsJson,
      outputsJson: data.outputs_json || data.outputsJson,
      pipelineExecutionId: data.pipeline_execution_id,
      expireAt: toDate(data.expire_at)
    };
  }
};

// --- User Record Mapping Complex Logic ---

// --- User Record Mapping Generic Logic ---

interface GenericIntegrationData {
  enabled?: boolean;
  apiKey?: string;
  api_key?: string;
  userId?: string;
  user_id?: string;
  fitbitUserId?: string;
  fitbit_user_id?: string;
  athleteId?: number | string;
  athlete_id?: string;
  accessToken?: string;
  access_token?: string;
  refreshToken?: string;
  refresh_token?: string;
  expiresAt?: Date | Timestamp;
  expires_at?: Date | Timestamp;
  createdAt?: Date | Timestamp;
  created_at?: Date | Timestamp;
  lastUsedAt?: Date | Timestamp;
  last_used_at?: Date | Timestamp;
  [key: string]: unknown;
}

export const mapGenericIntegrationToFirestore = (i: Record<string, unknown>, key: string): Record<string, unknown> => {
  const def = INTEGRATIONS[key as keyof UserIntegrations];
  if (!def) return {}; // Should not happen if calling safely

  const out: Record<string, unknown> = {
    enabled: i.enabled
  };

  if ('createdAt' in i) out.created_at = i.createdAt;
  if ('lastUsedAt' in i) out.last_used_at = i.lastUsedAt;

  // Handle OAuth specific fields
  if (def.type === 'oauth') {
    const oauthDef = def as OAuthIntegrationDefinition;
    out.access_token = i.accessToken;
    out.refresh_token = i.refreshToken;

    out.expires_at = i.expiresAt;

    // Map generic externalUserId internal field to the database field (snake_case)
    // Actually our Proto defines them as athleteId / fitbitUserId.
    // We need to map from the Proto keys to the DB keys (snake_case).
    // The definition has externalUserIdField like 'athleteId' -> snake would be 'athlete_id'

    // Simple heuristic: if the object has the property, map it to snake_case
    // Or check the specific definition field
    const extId = i[oauthDef.externalUserIdField];
    if (extId) {
      // Convention: camelCase -> snake_case
      const dbKey = oauthDef.externalUserIdField.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);
      out[dbKey] = extId;
    }
  } else {
    // API Key based or others
    // Check specific fields from registry? Or just hardcode common ones like apiKey/userId?
    // Current usage for Hevy: apiKey, userId
    if ('apiKey' in i) out.api_key = i.apiKey;
    if ('userId' in i) out.user_id = i.userId;
  }

  return out;
};

const mapUserIntegrationsToFirestore = (i?: UserIntegrations): Record<string, unknown> | undefined => {
  if (!i) return undefined;
  const out: Record<string, unknown> = {};

  // Dynamic iteration based on registry, but we have to check actual data presence
  for (const key of Object.keys(INTEGRATIONS)) {
    const k = key as keyof UserIntegrations;
    if (i[k]) {
      out[key] = mapGenericIntegrationToFirestore(i[k] as unknown as Record<string, unknown>, key);
    }
  }

  return out;
};

const mapGenericIntegrationFromFirestore = (data: GenericIntegrationData, key: string): Record<string, unknown> | undefined => {
  if (!data) return undefined;

  const def = INTEGRATIONS[key as keyof UserIntegrations];
  if (!def) return undefined;

  const out: Record<string, unknown> = {
    enabled: !!data.enabled
  };

  // Standard timestamps
  out.createdAt = toDate(data.created_at || data.createdAt);
  out.lastUsedAt = toDate(data.last_used_at || data.lastUsedAt);

  if (def.type === 'oauth') {
    out.accessToken = data.access_token || data.accessToken || '';
    out.refreshToken = data.refresh_token || data.refreshToken || '';

    out.expiresAt = toDate(data.expires_at || data.expiresAt);

    const extField = (def as OAuthIntegrationDefinition).externalUserIdField;
    // camel -> snake for lookup
    const dbKey = extField.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);

    // Special handling for number mapping (Strava athleteId)
    // Current logic was: parseInt(data.athlete_id || '0', 10)
    // We need to know if the target field expects a number.
    // Typescript reflection isn't perfect here.
    // For now we can stick to string for externalIds unless verified.
    // Strava athleteId is indeed a number in Proto (int64 -> number/long).

    const val = data[dbKey] || data[extField];
    if (key === 'strava' && val) {
      out[extField] = parseInt(String(val), 10);
    } else {
      out[extField] = val || '';
    }
  } else {
    // API Key / User ID
    // Hevy logic: apiKey, userId
    if (key === 'hevy') {
      out.apiKey = data.api_key || data.apiKey || '';
      out.userId = data.user_id || data.userId || '';
    }
  }

  return out;
};

const mapUserIntegrationsFromFirestore = (data: Record<string, unknown> | undefined): UserIntegrations | undefined => {
  if (!data) return undefined;
  const out: Partial<UserIntegrations> = {};

  for (const key of Object.keys(INTEGRATIONS)) {
    const k = key as keyof UserIntegrations;
    if (data[key]) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      out[k] = mapGenericIntegrationFromFirestore(data[key] as GenericIntegrationData, key) as any;
    }
  }

  return out as UserIntegrations;
};

// Pipelines Mapping

export const mapPipelineToFirestore = (p: PipelineConfig): Record<string, unknown> => ({
  id: p.id,
  source: p.source,
  destinations: p.destinations, // Stored as numbers (enum values)
  enrichers: p.enrichers?.map(e => ({
    provider_type: e.providerType,
    typed_config: e.typedConfig
  }))
});

export const mapPipelineFromFirestore = (p: Record<string, unknown>): PipelineConfig => ({
  id: p.id as string,
  source: p.source as string,
  destinations: ((p.destinations as unknown[]) || []).map(d => {
    if (typeof d === 'number') return d as Destination;
    if (typeof d === 'string') {
      // Legacy string support
      if (d === 'strava' || d === 'DESTINATION_STRAVA') return Destination.DESTINATION_STRAVA;
      if (d === 'mock' || d === 'DESTINATION_MOCK') return Destination.DESTINATION_MOCK;
    }
    return Destination.DESTINATION_UNSPECIFIED;
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  enrichers: ((p.enrichers as any[]) || []).map((e: any) => ({
    providerType: e.provider_type || e.providerType,
    typedConfig: e.typed_config || e.typedConfig || {}
  }))
});

// Helper for partial execution updates
export const mapExecutionPartialToFirestore = (data: Partial<ExecutionRecord>): Record<string, unknown> => {
  const out: Record<string, unknown> = {};
  if (data.executionId !== undefined) out.execution_id = data.executionId;
  if (data.service !== undefined) out.service = data.service;
  if (data.status !== undefined) out.status = data.status;
  if (data.timestamp !== undefined) out.timestamp = data.timestamp;
  if (data.userId !== undefined) out.user_id = data.userId;
  if (data.testRunId !== undefined) out.test_run_id = data.testRunId;
  if (data.triggerType !== undefined) out.trigger_type = data.triggerType;
  if (data.startTime !== undefined) out.start_time = data.startTime;
  if (data.endTime !== undefined) out.end_time = data.endTime;
  if (data.errorMessage !== undefined) out.error_message = data.errorMessage;
  if (data.inputsJson !== undefined) out.inputs_json = data.inputsJson;
  if (data.outputsJson !== undefined) out.outputs_json = data.outputsJson;
  if (data.pipelineExecutionId !== undefined) out.pipeline_execution_id = data.pipelineExecutionId;
  if (data.expireAt !== undefined) out.expire_at = data.expireAt;
  return out;
};

export const userConverter: FirestoreDataConverter<UserRecord> = {
  toFirestore(model: UserRecord): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {};
    if (model.userId !== undefined) data.user_id = model.userId;
    if (model.createdAt !== undefined) data.created_at = model.createdAt;
    if (model.integrations !== undefined) data.integrations = mapUserIntegrationsToFirestore(model.integrations);
    if (model.pipelines !== undefined) data.pipelines = model.pipelines?.map(mapPipelineToFirestore);
    if (model.fcmTokens !== undefined) data.fcm_tokens = model.fcmTokens;
    // Tier management fields
    if (model.tier !== undefined) data.tier = model.tier;
    if (model.trialEndsAt !== undefined) data.trial_ends_at = model.trialEndsAt;
    if (model.isAdmin !== undefined) data.is_admin = model.isAdmin;
    if (model.syncCountThisMonth !== undefined) data.sync_count_this_month = model.syncCountThisMonth;
    if (model.syncCountResetAt !== undefined) data.sync_count_reset_at = model.syncCountResetAt;
    if (model.stripeCustomerId !== undefined) data.stripe_customer_id = model.stripeCustomerId;
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): UserRecord {
    const data = snapshot.data();
    return {
      userId: data.user_id || data.userId,
      createdAt: toDate(data.created_at || data.createdAt),
      integrations: mapUserIntegrationsFromFirestore(data.integrations),
      pipelines: (data.pipelines || []).map(mapPipelineFromFirestore),
      fcmTokens: data.fcm_tokens || data.fcmTokens || [],
      // Tier management fields (with backwards-compatible defaults)
      tier: data.tier || 'free',
      trialEndsAt: toDate(data.trial_ends_at),
      isAdmin: data.is_admin || false,
      syncCountThisMonth: data.sync_count_this_month || 0,
      syncCountResetAt: toDate(data.sync_count_reset_at),
      stripeCustomerId: data.stripe_customer_id || undefined,
    };
  }
};


export const processedActivityConverter: FirestoreDataConverter<ProcessedActivityRecord> = {
  toFirestore(model: ProcessedActivityRecord): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {};
    if (model.source !== undefined) data.source = model.source;
    if (model.externalId !== undefined) data.external_id = model.externalId;
    if (model.processedAt !== undefined) data.processed_at = model.processedAt;
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): ProcessedActivityRecord {
    const data = snapshot.data();
    return {
      source: data.source,
      externalId: data.external_id,
      processedAt: toDate(data.processed_at)
    };
  }
};

export const PendingInputToFirestore = (model: PendingInput): Record<string, unknown> => {
  const data: Record<string, unknown> = {
    activity_id: model.activityId,
    user_id: model.userId,
    status: model.status,
    required_fields: model.requiredFields,
    input_data: model.inputData,
    created_at: model.createdAt,
    updated_at: model.updatedAt,
    completed_at: model.completedAt,
  };
  // If we had original_payload exposed in TS, we'd map it here, but it's often binary or complex structure
  // For now, allow passthrough if it exists on model (duck typing)
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  if ((model as any).originalPayload) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    data.original_payload = (model as any).originalPayload;
  }
  return data;
};

export const FirestoreToPendingInput = (data: Record<string, unknown>): PendingInput => {
  return {
    activityId: data.activity_id as string,
    userId: data.user_id as string,
    status: data.status as PendingInput_Status,
    requiredFields: (data.required_fields as string[]) || [],
    inputData: (data.input_data as Record<string, string>) || {},
    originalPayload: typeof data.original_payload === 'string'
      ? JSON.parse(data.original_payload)
      : data.original_payload, // Handle both JSON string and object
    createdAt: toDate(data.created_at),
    updatedAt: toDate(data.updated_at),
    completedAt: toDate(data.completed_at),
    // Resume pattern fields
    continuedWithoutResolution: data.continued_without_resolution as boolean ?? false,
    linkedActivityId: data.linked_activity_id as string ?? '',
    pipelineId: data.pipeline_id as string ?? '',
    enricherProviderId: data.enricher_provider_id as string ?? '',
    autoPopulated: data.auto_populated as boolean ?? false,
    autoDeadline: toDate(data.auto_deadline),
  };
};


export const synchronizedActivityConverter: FirestoreDataConverter<import('../../types/pb/user').SynchronizedActivity> = {
  toFirestore(model: import('../../types/pb/user').SynchronizedActivity): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {
      activity_id: model.activityId,
      title: model.title,
      description: model.description,
      type: model.type,
      source: model.source,
      start_time: model.startTime,
      synced_at: model.syncedAt,
      pipeline_id: model.pipelineId,
      destinations: model.destinations,
      pipeline_execution_id: model.pipelineExecutionId,
      // Parkrun tracking fields
      parkrun_results_state: model.parkrunResultsState,
      parkrun_event_name: model.parkrunEventName,
      parkrun_event_slug: model.parkrunEventSlug,
      parkrun_polling_deadline: model.parkrunPollingDeadline
    };
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): import('../../types/pb/user').SynchronizedActivity {
    const data = snapshot.data();
    return {
      activityId: data.activity_id,
      title: data.title,
      description: data.description,
      type: data.type,
      source: data.source,
      startTime: toDate(data.start_time),
      syncedAt: toDate(data.synced_at),
      pipelineId: data.pipeline_id,
      destinations: data.destinations || {},
      pipelineExecutionId: data.pipeline_execution_id,
      // Parkrun tracking fields
      parkrunResultsState: data.parkrun_results_state || 0,
      parkrunEventName: data.parkrun_event_name || '',
      parkrunEventSlug: data.parkrun_event_slug || '',
      parkrunPollingDeadline: toDate(data.parkrun_polling_deadline)
    };
  }
};
