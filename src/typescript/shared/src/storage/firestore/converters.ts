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
    return {
      email: model.email,
      source: model.source,
      created_at: model.createdAt,
      user_agent: model.userAgent,
      ip: model.ip
    };
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
    return {
      user_id: model.userId,
      label: model.label,
      scopes: model.scopes,
      created_at: model.createdAt,
      last_used_at: model.lastUsedAt
    };
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
    return {
      user_id: model.userId,
      created_at: model.createdAt
    };
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
    return {
      execution_id: model.executionId,
      service: model.service,
      status: model.status,
      timestamp: model.timestamp,
      user_id: model.userId,
      test_run_id: model.testRunId,
      trigger_type: model.triggerType,
      start_time: model.startTime,
      end_time: model.endTime,
      error_message: model.errorMessage,
      inputs_json: model.inputsJson,
      outputs_json: model.outputsJson,
      parent_execution_id: model.parentExecutionId
    };
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
      inputsJson: data.inputs_json,
      outputsJson: data.outputs_json,
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
      user_id: i.hevy.userId
    };
  }
  if (i.fitbit) {
    out.fitbit = {
      enabled: i.fitbit.enabled,
      access_token: i.fitbit.accessToken,
      refresh_token: i.fitbit.refreshToken,
      expires_at: i.fitbit.expiresAt,
      fitbit_user_id: i.fitbit.fitbitUserId
    };
  }
  if (i.strava) {
    out.strava = {
      enabled: i.strava.enabled,
      access_token: i.strava.accessToken,
      refresh_token: i.strava.refreshToken,
      expires_at: i.strava.expiresAt,
      athlete_id: i.strava.athleteId
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
      userId: data.hevy.user_id || data.hevy.userId
    } : undefined,
    fitbit: data.fitbit ? {
      enabled: !!data.fitbit.enabled,
      accessToken: data.fitbit.access_token,
      refreshToken: data.fitbit.refresh_token,
      expiresAt: toDate(data.fitbit.expires_at),
      fitbitUserId: data.fitbit.fitbit_user_id
    } : undefined,
    strava: data.strava ? {
      enabled: !!data.strava.enabled,
      accessToken: data.strava.access_token,
      refreshToken: data.strava.refresh_token,
      expiresAt: toDate(data.strava.expires_at),
      athleteId: data.strava.athlete_id
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
    return {
      user_id: model.userId,
      created_at: model.createdAt,
      integrations: mapUserIntegrationsToFirestore(model.integrations),
      pipelines: model.pipelines?.map(mapPipelineToFirestore)
    };
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
    return {
      source: model.source,
      external_id: model.externalId,
      processed_at: model.processedAt
    };
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
