import { FirestoreDataConverter, QueryDocumentSnapshot, Timestamp } from 'firebase-admin/firestore';
import { UserRecord, UserIntegrations, PipelineConfig, ProcessedActivityRecord } from '../../types/pb/user';
import { WaitlistEntry } from '../../types/pb/waitlist';
import { ApiKeyRecord, IntegrationIdentity } from '../../types/pb/auth';
import { ExecutionRecord, ExecutionStatus } from '../../types/pb/execution';
import { PendingInput, PendingInput_Status } from '../../types/pb/pending_input';

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
    if (model.parentExecutionId !== undefined) data.parent_execution_id = model.parentExecutionId;
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
      expireAt: toDate(data.expire_at)
    };
  }
};

// --- User Record Mapping Complex Logic ---

export const mapHevyToFirestore = (i: NonNullable<UserIntegrations['hevy']>): Record<string, unknown> => ({
  enabled: i.enabled,
  api_key: i.apiKey,
  user_id: i.userId,
  created_at: i.createdAt,
  last_used_at: i.lastUsedAt
});

export const mapFitbitToFirestore = (i: NonNullable<UserIntegrations['fitbit']>): Record<string, unknown> => ({
  enabled: i.enabled,
  access_token: i.accessToken,
  refresh_token: i.refreshToken,
  expires_at: i.expiresAt,
  fitbit_user_id: i.fitbitUserId,
  created_at: i.createdAt,
  last_used_at: i.lastUsedAt
});

export const mapStravaToFirestore = (i: NonNullable<UserIntegrations['strava']>): Record<string, unknown> => ({
  enabled: i.enabled,
  access_token: i.accessToken,
  refresh_token: i.refreshToken,
  expires_at: i.expiresAt,
  athlete_id: i.athleteId,
  created_at: i.createdAt,
  last_used_at: i.lastUsedAt
});

const mapUserIntegrationsToFirestore = (i?: UserIntegrations): Record<string, unknown> | undefined => {
  if (!i) return undefined;
  const out: Record<string, unknown> = {};
  if (i.hevy) out.hevy = mapHevyToFirestore(i.hevy);
  if (i.fitbit) out.fitbit = mapFitbitToFirestore(i.fitbit);
  if (i.strava) out.strava = mapStravaToFirestore(i.strava);
  return out;
};


interface FirestoreIntegrationData {
  enabled?: boolean;
  api_key?: string;
  apiKey?: string; // Legacy
  user_id?: string;
  userId?: string; // Legacy
  created_at?: unknown;
  createdAt?: unknown; // Legacy
  last_used_at?: unknown;
  access_token?: string;
  refresh_token?: string;
  expires_at?: unknown;
  fitbit_user_id?: string;
  athlete_id?: string;
}

const mapUserIntegrationsFromFirestore = (data: Record<string, unknown> | undefined): UserIntegrations | undefined => {
  if (!data) return undefined;
  return {
    hevy: data.hevy ? {
      enabled: !!(data.hevy as FirestoreIntegrationData).enabled,
      apiKey: (data.hevy as FirestoreIntegrationData).api_key || (data.hevy as FirestoreIntegrationData).apiKey || '',
      userId: (data.hevy as FirestoreIntegrationData).user_id || (data.hevy as FirestoreIntegrationData).userId || '',
      createdAt: toDate((data.hevy as FirestoreIntegrationData).created_at),
      lastUsedAt: toDate((data.hevy as FirestoreIntegrationData).last_used_at)
    } : undefined,
    fitbit: data.fitbit ? {
      enabled: !!(data.fitbit as FirestoreIntegrationData).enabled,
      accessToken: (data.fitbit as FirestoreIntegrationData).access_token || '',
      refreshToken: (data.fitbit as FirestoreIntegrationData).refresh_token || '',
      expiresAt: toDate((data.fitbit as FirestoreIntegrationData).expires_at),
      fitbitUserId: (data.fitbit as FirestoreIntegrationData).fitbit_user_id || '',
      createdAt: toDate((data.fitbit as FirestoreIntegrationData).created_at),
      lastUsedAt: toDate((data.fitbit as FirestoreIntegrationData).last_used_at)
    } : undefined,
    strava: data.strava ? {
      enabled: !!(data.strava as FirestoreIntegrationData).enabled,
      accessToken: (data.strava as FirestoreIntegrationData).access_token || '',
      refreshToken: (data.strava as FirestoreIntegrationData).refresh_token || '',
      expiresAt: toDate((data.strava as FirestoreIntegrationData).expires_at),
      athleteId: parseInt((data.strava as FirestoreIntegrationData).athlete_id || '0', 10),
      createdAt: toDate((data.strava as FirestoreIntegrationData).created_at),
      lastUsedAt: toDate((data.strava as FirestoreIntegrationData).last_used_at)
    } : undefined
  };
};

// Pipelines Mapping
// Pipelines Mapping
export const mapPipelineToFirestore = (p: PipelineConfig): Record<string, unknown> => ({
  id: p.id,
  source: p.source,
  destinations: p.destinations,
  enrichers: p.enrichers?.map(e => ({
    provider_type: e.providerType,
    inputs: e.inputs
  }))
});

export const mapPipelineFromFirestore = (p: Record<string, unknown>): PipelineConfig => ({
  id: p.id as string,
  source: p.source as string,
  destinations: (p.destinations as string[]) || [],
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  enrichers: ((p.enrichers as any[]) || []).map((e: any) => ({
    providerType: e.provider_type || e.providerType,
    inputs: e.inputs || {}
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
  if (data.parentExecutionId !== undefined) out.parent_execution_id = data.parentExecutionId;
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
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): UserRecord {
    const data = snapshot.data();
    return {
      userId: data.user_id || data.userId,
      createdAt: toDate(data.created_at || data.createdAt),
      integrations: mapUserIntegrationsFromFirestore(data.integrations),
      pipelines: (data.pipelines || []).map(mapPipelineFromFirestore),
      fcmTokens: data.fcm_tokens || data.fcmTokens || []
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
    completedAt: toDate(data.completed_at)
  };
};
