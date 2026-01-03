import { FirestoreDataConverter, QueryDocumentSnapshot, Timestamp } from 'firebase-admin/firestore';
import { UserRecord, UserIntegrations, PipelineConfig, ProcessedActivityRecord } from '../../types/pb/user';
import { WaitlistEntry } from '../../types/pb/waitlist';
import { ApiKeyRecord, IntegrationIdentity } from '../../types/pb/auth';
import { ExecutionRecord, ExecutionStatus } from '../../types/pb/execution';

// Helper to convert Firestore Timestamp to Date
const toDate = (val: any): Date | undefined => {
  if (!val) return undefined;
  if (val instanceof Timestamp) return val.toDate();
  if (val.toDate) return val.toDate(); // Duck typing
  return new Date(val); // Fallback string/number
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
      parentExecutionId: data.parent_execution_id
    };
  }
};

// --- User Record Mapping Complex Logic ---

const mapUserIntegrationsToFirestore = (i?: UserIntegrations): any => {
  if (!i) return undefined;
  const out: any = {};
  if (i.hevy) {
    out.hevy = {
      enabled: i.hevy.enabled,
      api_key: i.hevy.apiKey,
      user_id: i.hevy.userId,
      created_at: i.hevy.createdAt,
      last_used_at: i.hevy.lastUsedAt
    };
  }
  if (i.fitbit) {
    out.fitbit = {
      enabled: i.fitbit.enabled,
      access_token: i.fitbit.accessToken,
      refresh_token: i.fitbit.refreshToken,
      expires_at: i.fitbit.expiresAt,
      fitbit_user_id: i.fitbit.fitbitUserId,
      created_at: i.fitbit.createdAt,
      last_used_at: i.fitbit.lastUsedAt
    };
  }
  if (i.strava) {
    out.strava = {
      enabled: i.strava.enabled,
      access_token: i.strava.accessToken,
      refresh_token: i.strava.refreshToken,
      expires_at: i.strava.expiresAt,
      athlete_id: i.strava.athleteId,
      created_at: i.strava.createdAt,
      last_used_at: i.strava.lastUsedAt
    };
  }
  return out;
};

const mapUserIntegrationsFromFirestore = (data: any): UserIntegrations | undefined => {
  if (!data) return undefined;
  return {
    hevy: data.hevy ? {
      enabled: !!data.hevy.enabled,
      apiKey: data.hevy.api_key || data.hevy.apiKey,
      userId: data.hevy.user_id || data.hevy.userId,
      createdAt: toDate(data.hevy.created_at),
      lastUsedAt: toDate(data.hevy.last_used_at)
    } : undefined,
    fitbit: data.fitbit ? {
      enabled: !!data.fitbit.enabled,
      accessToken: data.fitbit.access_token,
      refreshToken: data.fitbit.refresh_token,
      expiresAt: toDate(data.fitbit.expires_at),
      fitbitUserId: data.fitbit.fitbit_user_id,
      createdAt: toDate(data.fitbit.created_at),
      lastUsedAt: toDate(data.fitbit.last_used_at)
    } : undefined,
    strava: data.strava ? {
      enabled: !!data.strava.enabled,
      accessToken: data.strava.access_token,
      refreshToken: data.strava.refresh_token,
      expiresAt: toDate(data.strava.expires_at),
      athleteId: data.strava.athlete_id,
      createdAt: toDate(data.strava.created_at),
      lastUsedAt: toDate(data.strava.last_used_at)
    } : undefined
  };
};

// Pipelines Mapping
const mapPipelineToFirestore = (p: PipelineConfig): any => ({
  id: p.id,
  source: p.source,
  destinations: p.destinations,
  enrichers: p.enrichers?.map(e => ({
    provider_type: e.providerType,
    inputs: e.inputs
  }))
});

const mapPipelineFromFirestore = (p: any): PipelineConfig => ({
  id: p.id,
  source: p.source,
  destinations: p.destinations || [],
  enrichers: (p.enrichers || []).map((e: any) => ({
    providerType: e.provider_type || e.providerType,
    inputs: e.inputs || {}
  }))
});

export const userConverter: FirestoreDataConverter<UserRecord> = {
  toFirestore(model: UserRecord): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {};
    if (model.userId !== undefined) data.user_id = model.userId;
    if (model.createdAt !== undefined) data.created_at = model.createdAt;
    if (model.integrations !== undefined) data.integrations = mapUserIntegrationsToFirestore(model.integrations);
    if (model.pipelines !== undefined) data.pipelines = model.pipelines?.map(mapPipelineToFirestore);
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): UserRecord {
    const data = snapshot.data();
    return {
      userId: data.user_id || data.userId,
      createdAt: toDate(data.created_at || data.createdAt),
      integrations: mapUserIntegrationsFromFirestore(data.integrations),
      pipelines: (data.pipelines || []).map(mapPipelineFromFirestore)
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
