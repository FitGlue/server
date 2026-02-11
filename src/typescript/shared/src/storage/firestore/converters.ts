import { FirestoreDataConverter, QueryDocumentSnapshot, Timestamp } from 'firebase-admin/firestore';
import { UserRecord, UserTier, UserIntegrations, ProcessedActivityRecord } from '../../types/pb/user';
import { ApiKeyRecord, IntegrationIdentity } from '../../types/pb/auth';
import { ExecutionRecord, ExecutionStatus } from '../../types/pb/execution';
import { PendingInput, PendingInput_Status } from '../../types/pb/pending_input';
import { INTEGRATIONS, isOAuthIntegration } from '../../types/integrations';



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
  if (isOAuthIntegration(def)) {
    out.access_token = i.accessToken;
    out.refresh_token = i.refreshToken;

    out.expires_at = i.expiresAt;

    // Map the externalUserIdField from proto key to snake_case DB key
    const extId = def.externalUserIdField ? i[def.externalUserIdField] : undefined;
    if (extId && def.externalUserIdField) {
      // Convention: camelCase -> snake_case
      const dbKey = def.externalUserIdField.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);
      out[dbKey] = extId;
    }
  } else {
    // API Key based (Hevy) or Public ID (Parkrun)
    if ('apiKey' in i) out.api_key = i.apiKey;
    if ('userId' in i) out.user_id = i.userId;
    // Parkrun fields
    if ('athleteId' in i) out.athlete_id = i.athleteId;
    if ('countryUrl' in i) out.country_url = i.countryUrl;
    if ('consentGiven' in i) out.consent_given = i.consentGiven;
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

// eslint-disable-next-line complexity
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

  if (isOAuthIntegration(def)) {
    out.accessToken = data.access_token || data.accessToken || '';
    out.refreshToken = data.refresh_token || data.refreshToken || '';

    out.expiresAt = toDate(data.expires_at || data.expiresAt);

    const extField = def.externalUserIdField;
    if (extField) {
      // camel -> snake for lookup
      const dbKey = extField.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);

      // Special handling for number mapping (Strava athleteId)
      // Strava athleteId is a number in Proto (int64 -> number/long).
      const val = data[dbKey] || data[extField];
      if (key === 'strava' && val) {
        out[extField] = parseInt(String(val), 10);
      } else {
        out[extField] = val || '';
      }
    }
  } else {
    // API Key (Hevy) or Public ID (Parkrun)
    if (key === 'hevy') {
      out.apiKey = data.api_key || data.apiKey || '';
      out.userId = data.user_id || data.userId || '';
    } else if (key === 'parkrun') {
      out.athleteId = data.athlete_id || data.athleteId || '';
      out.countryUrl = data.country_url || data.countryUrl || '';
      out.consentGiven = data.consent_given || data.consentGiven || false;
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

// Pipeline converters moved to pipeline-store.ts

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
    if (model.fcmTokens !== undefined) data.fcm_tokens = model.fcmTokens;
    // Tier management fields
    if (model.tier !== undefined) {
      if (model.tier === UserTier.USER_TIER_ATHLETE) {
        data.tier = 'athlete';
      } else {
        data.tier = 'hobbyist';
      }
    }
    if (model.trialEndsAt !== undefined) data.trial_ends_at = model.trialEndsAt;
    if (model.isAdmin !== undefined) data.is_admin = model.isAdmin;
    if (model.syncCountThisMonth !== undefined) data.sync_count_this_month = model.syncCountThisMonth;
    if (model.preventedSyncCount !== undefined) data.prevented_sync_count = model.preventedSyncCount;
    if (model.syncCountResetAt !== undefined) data.sync_count_reset_at = model.syncCountResetAt;
    if (model.stripeCustomerId !== undefined) data.stripe_customer_id = model.stripeCustomerId;
    if (model.accessEnabled !== undefined) data.access_enabled = model.accessEnabled;
    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): UserRecord {
    const data = snapshot.data();

    return {
      userId: data.user_id || data.userId,
      createdAt: toDate(data.created_at || data.createdAt),
      integrations: mapUserIntegrationsFromFirestore(data.integrations),
      fcmTokens: data.fcm_tokens || data.fcmTokens || [],
      // Tier management fields (with backwards-compatible defaults)
      tier: ((): UserTier => {
        const t = data.tier;
        if (t === 'athlete' || t === 'pro' || t === 2 || t === '2') {
          return UserTier.USER_TIER_ATHLETE;
        }
        return UserTier.USER_TIER_HOBBYIST;
      })(),
      trialEndsAt: toDate(data.trial_ends_at),
      isAdmin: data.is_admin || false,
      syncCountThisMonth: data.sync_count_this_month || 0,
      preventedSyncCount: data.prevented_sync_count || 0,
      syncCountResetAt: toDate(data.sync_count_reset_at),
      stripeCustomerId: data.stripe_customer_id || undefined,
      accessEnabled: data.access_enabled || false,
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
    // Resume pattern fields
    continued_without_resolution: model.continuedWithoutResolution,
    linked_activity_id: model.linkedActivityId,
    pipeline_id: model.pipelineId,
    enricher_provider_id: model.enricherProviderId,
    auto_populated: model.autoPopulated,
    auto_deadline: model.autoDeadline,
    provider_metadata: model.providerMetadata,
  };
  // GCS URI for large payloads
  if (model.originalPayloadUri) {
    data.original_payload_uri = model.originalPayloadUri;
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
    // GCS URI for large payloads (original_payload is now stored in GCS)
    originalPayloadUri: data.original_payload_uri as string || '',
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
    providerMetadata: (data.provider_metadata as Record<string, string>) || {},
  };
};


export const FirestoreToShowcasedActivity = (data: Record<string, unknown>): import('../../types/pb/user').ShowcasedActivity => {
  return {
    showcaseId: data.showcase_id as string || '',
    activityId: data.activity_id as string || '',
    userId: data.user_id as string || '',
    title: data.title as string || '',
    description: data.description as string || '',
    activityType: (data.activity_type as number) || 0,
    source: (data.source as number) || 0,
    startTime: toDate(data.start_time),
    // Legacy: inline activity_data (will be removed after migration)
    activityData: data.activity_data ? JSON.parse(data.activity_data as string) : undefined,
    // New: GCS URI for activity JSON (avoids Firestore 1MB limit)
    activityDataUri: data.activity_data_uri as string || '',
    fitFileUri: data.fit_file_uri as string || '',
    appliedEnrichments: (data.applied_enrichments as string[]) || [],
    enrichmentMetadata: (data.enrichment_metadata as Record<string, string>) || {},
    tags: (data.tags as string[]) || [],
    pipelineExecutionId: data.pipeline_execution_id as string,
    createdAt: toDate(data.created_at),
    expiresAt: toDate(data.expires_at),
    ownerDisplayName: data.owner_display_name as string || '',
  };
};

export const FirestoreToShowcaseProfileEntry = (data: Record<string, unknown>): import('../../types/pb/user').ShowcaseProfileEntry => {
  return {
    showcaseId: data.showcase_id as string || '',
    title: data.title as string || '',
    activityType: (data.activity_type as number) || 0,
    source: (data.source as number) || 0,
    startTime: toDate(data.start_time),
    routeThumbnailUrl: data.route_thumbnail_url as string || '',
    distanceMeters: (data.distance_meters as number) || 0,
    durationSeconds: (data.duration_seconds as number) || 0,
    totalSets: (data.total_sets as number) || 0,
    totalReps: (data.total_reps as number) || 0,
    totalWeightKg: (data.total_weight_kg as number) || 0,
  };
};

export const FirestoreToShowcaseProfile = (data: Record<string, unknown>): import('../../types/pb/user').ShowcaseProfile => {
  const entriesRaw = (data.entries as Record<string, unknown>[]) || [];
  return {
    slug: data.slug as string || '',
    userId: data.user_id as string || '',
    displayName: data.display_name as string || '',
    entries: entriesRaw.map(e => FirestoreToShowcaseProfileEntry(e)),
    totalActivities: (data.total_activities as number) || 0,
    totalDistanceMeters: (data.total_distance_meters as number) || 0,
    totalDurationSeconds: (data.total_duration_seconds as number) || 0,
    totalSets: (data.total_sets as number) || 0,
    totalReps: (data.total_reps as number) || 0,
    totalWeightKg: (data.total_weight_kg as number) || 0,
    latestActivityAt: toDate(data.latest_activity_at),
    createdAt: toDate(data.created_at),
    updatedAt: toDate(data.updated_at),
  };
};


// --- PipelineRun Converter (lifecycle tracking) ---

import { PipelineRun, PipelineRunStatus, BoosterExecution, DestinationOutcome, DestinationStatus } from '../../types/pb/user';
import { Destination } from '../../types/pb/events';

export const pipelineRunConverter: FirestoreDataConverter<PipelineRun> = {
  toFirestore(model: PipelineRun): FirebaseFirestore.DocumentData {
    const data: FirebaseFirestore.DocumentData = {
      id: model.id,
      pipeline_id: model.pipelineId,
      activity_id: model.activityId,
      source: model.source,
      source_activity_id: model.sourceActivityId,
      title: model.title,
      description: model.description,
      type: model.type,
      status: model.status,
    };

    if (model.startTime) data.start_time = model.startTime;
    if (model.createdAt) data.created_at = model.createdAt;
    if (model.updatedAt) data.updated_at = model.updatedAt;
    if (model.statusMessage) data.status_message = model.statusMessage;
    if (model.pendingInputId) data.pending_input_id = model.pendingInputId;
    if (model.originalPayloadUri) data.original_payload_uri = model.originalPayloadUri;

    // Serialize boosters
    if (model.boosters?.length > 0) {
      data.boosters = model.boosters.map(b => ({
        provider_name: b.providerName,
        status: b.status,
        duration_ms: b.durationMs,
        metadata: b.metadata,
        error: b.error,
      }));
    }

    // Serialize destinations
    if (model.destinations?.length > 0) {
      data.destinations = model.destinations.map(d => ({
        destination: d.destination,
        status: d.status,
        external_id: d.externalId,
        error: d.error,
        completed_at: d.completedAt,
      }));
    }

    // Note: enriched_event is now stored in GCS via enriched_event_uri
    if (model.enrichedEventUri) {
      data.enriched_event_uri = model.enrichedEventUri;
    }

    return data;
  },
  fromFirestore(snapshot: QueryDocumentSnapshot): PipelineRun {
    const data = snapshot.data();

    // Parse boosters
    const boosters: BoosterExecution[] = [];
    if (Array.isArray(data.boosters)) {
      for (const b of data.boosters) {
        boosters.push({
          providerName: b.provider_name || '',
          status: b.status || '',
          durationMs: b.duration_ms || 0,
          metadata: b.metadata || {},
          error: b.error,
        });
      }
    }

    // Parse destinations
    const destinations: DestinationOutcome[] = [];
    if (Array.isArray(data.destinations)) {
      for (const d of data.destinations) {
        destinations.push({
          destination: d.destination as Destination || 0,
          status: d.status as DestinationStatus || 0,
          externalId: d.external_id,
          error: d.error,
          completedAt: toDate(d.completed_at),
        });
      }
    }

    return {
      id: data.id || '',
      pipelineId: data.pipeline_id || '',
      activityId: data.activity_id || '',
      source: data.source || '',
      sourceActivityId: data.source_activity_id || '',
      title: data.title || '',
      description: data.description || '',
      type: data.type || 0,
      status: data.status as PipelineRunStatus || 0,
      startTime: toDate(data.start_time),
      createdAt: toDate(data.created_at),
      updatedAt: toDate(data.updated_at),
      statusMessage: data.status_message as string | undefined,
      pendingInputId: data.pending_input_id as string | undefined,
      originalPayloadUri: data.original_payload_uri as string || '',
      enrichedEventUri: data.enriched_event_uri as string || '',
      boosters,
      destinations,
    };
  }
};
