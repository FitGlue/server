// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext, FrameworkResponse, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest } from '@fitglue/shared/routing';
import { ShowcaseStore } from '@fitglue/shared/storage';
import { getEnricherManifest } from '@fitglue/shared/plugin';
import { getEffectiveTier } from '@fitglue/shared/domain';
import type { StandardizedActivity, ActivityType, ActivitySource } from '@fitglue/shared/types';
import { EnricherProviderType, UserRecord } from '@fitglue/shared/types';
import * as admin from 'firebase-admin';
import { Request } from 'express';

/**
 * Public showcase handler - serves activity data for shareable URLs.
 * Routes:
 *   GET /api/showcase/{id} - Returns JSON activity data
 *   GET /showcase/{id}     - Redirects to static viewer page
 */
export const showcaseHandler = createCloudFunction(async (req: Request, ctx: FrameworkContext) => {
  const showcaseStore = new ShowcaseStore(db);


  const corsHeaders = {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
    'Access-Control-Max-Age': '3600',
  };

  if (req.method === 'OPTIONS') {
    return new FrameworkResponse({
      status: 204,
      body: '',
      headers: corsHeaders
    });
  }

  return await routeRequest(req, ctx, [
    {
      method: 'GET',
      pattern: '/api/showcase/:id',
      handler: async (match) => {
        return await handleApiShowcase(match.params.id, showcaseStore, db, corsHeaders);
      }
    },
    {
      method: 'GET',
      pattern: '/showcase/:id',
      handler: async (match) => {
        return await handleHtmlShowcase(match.params.id, showcaseStore, corsHeaders);
      }
    }
  ]);
}, {
  allowUnauthenticated: true,
  skipExecutionLogging: true
});

async function handleApiShowcase(
  showcaseId: string,
  showcaseStore: ShowcaseStore,
  db: admin.firestore.Firestore,
  corsHeaders: Record<string, string>
): Promise<FrameworkResponse> {
  // Fetch from Firestore using ShowcaseStore
  const data = await showcaseStore.get(showcaseId);

  if (!data) {
    throw new HttpError(404, 'Showcase not found');
  }

  // Check expiration
  if (data.expiresAt && data.expiresAt < new Date()) {
    throw new HttpError(410, 'This showcase has expired');
  }

  // Fetch user to determine tier
  const user = await db.collection('users').doc(data.userId).get();
  const userData = user.data() as UserRecord;
  const effectiveTier = getEffectiveTier(userData);

  // Fetch activity data from GCS
  // New format: URI points to enriched_events/{userId}/{pipelineExecutionId}.json (EnrichedActivityEvent)
  // Old format: URI points to showcase-assets/{showcaseId}/activity.json (just StandardizedActivity)
  let activityData: StandardizedActivity | undefined = data.activityData;
  if (!activityData && data.activityDataUri) {
    try {
      // Parse GCS URI: gs://bucket-name/path/to/file
      const gcsMatch = data.activityDataUri.match(/^gs:\/\/([^/]+)\/(.+)$/);
      if (gcsMatch) {
        const [, bucketName, filePath] = gcsMatch;
        const bucket = admin.storage().bucket(bucketName);
        const [contents] = await bucket.file(filePath).download();
        const parsed = JSON.parse(contents.toString());
        
        // Handle both formats:
        // 1. New format (EnrichedActivityEvent): extract activity_data from nested field
        // 2. Old format (StandardizedActivity): use parsed object directly (has sessions field)
        if (parsed.activity_data || parsed.activityData) {
          // New format - extract nested activity_data
          activityData = parsed.activity_data || parsed.activityData;
        } else if (parsed.sessions) {
          // Old format - the file IS the StandardizedActivity
          activityData = parsed as StandardizedActivity;
        }
      }
    } catch (err) {
      console.error('Failed to fetch activity data from GCS', { error: err, uri: data.activityDataUri });
      // Continue without activity data - page will show partial content
    }
  }

  // Compute summary from activity data
  const summary = activityData ? computeSummary(activityData) : undefined;
  const laps = activityData ? extractLaps(activityData) : undefined;
  const timeMarkers = activityData ? extractTimeMarkers(activityData) : undefined;

  // Build the public API response, stripping sensitive fields
  const response: ShowcaseResponse = {
    isAthlete: effectiveTier === 'athlete',
    showcaseId: data.showcaseId,
    title: data.title,
    description: data.description,
    activityType: data.activityType,
    source: data.source,
    startTime: data.startTime?.toISOString(),
    activityData: activityData,
    summary,
    laps,
    timeMarkers,
    appliedEnrichments: data.appliedEnrichments || [],
    enrichmentMetadata: data.enrichmentMetadata || {},
    registry: (data.appliedEnrichments || []).reduce((acc: Record<string, { name: string; icon: string; description: string }>, e: string) => {
      // Try to find in registry
      if (e in EnricherProviderType) {
        const providerType = EnricherProviderType[e as keyof typeof EnricherProviderType] as EnricherProviderType;
        const manifest = getEnricherManifest(providerType);
        if (manifest) {
          acc[e] = {
            name: manifest.name,
            icon: manifest.icon,
            description: manifest.description
          };
        }
      }
      return acc;
    }, {} as Record<string, { name: string; icon: string; description: string }>),
    tags: data.tags || [],
    createdAt: data.createdAt?.toISOString(),
    ownerDisplayName: data.ownerDisplayName,
    // Don't expose: userId, activityId, fitFileUri, pipelineExecutionId, expiresAt
  };

  return new FrameworkResponse({
    status: 200,
    body: response,
    headers: {
      ...corsHeaders,
      'Cache-Control': 'public, max-age=31536000, immutable'
    }
  });
}


async function handleHtmlShowcase(
  showcaseId: string,
  showcaseStore: ShowcaseStore,
  corsHeaders: Record<string, string>
): Promise<FrameworkResponse> {
  // Fetch from Firestore using ShowcaseStore
  const data = await showcaseStore.get(showcaseId);

  if (!data) {
    throw new HttpError(404, 'Showcase not found');
  }

  // Check expiration
  if (data.expiresAt && data.expiresAt < new Date()) {
    throw new HttpError(410, 'This showcase has expired');
  }

  // Serve the static page, which will fetch data via /api/showcase/{id}
  return new FrameworkResponse({
    status: 302,
    headers: {
      ...corsHeaders,
      'Location': `/showcase.html?id=${showcaseId}`
    }
  });
}

// Register the function
// Note: functions-framework .http() registration might still be needed if createCloudFunction
// doesn't handle the registration itself, but usually it returns the function to be exported.

// Summary computed from activity data
interface ActivitySummary {
  totalDurationSeconds: number;
  totalDistanceMeters: number;
  totalCalories?: number;
  avgHeartRate?: number;
  maxHeartRate?: number;
  lapCount: number;
  strengthSetCount: number;
}

// Lap summary for display
interface LapSummary {
  exerciseName: string;
  durationSeconds: number;
  distanceMeters: number;
}

// Time marker for charts
interface TimeMarkerSummary {
  timestamp?: string;
  label: string;
  markerType: string;
}

// Public API response (sanitized, no sensitive data)
interface ShowcaseResponse {
  isAthlete: boolean;
  showcaseId: string;
  title: string;
  description: string;
  activityType: ActivityType;
  source: ActivitySource;
  startTime?: string;
  activityData?: StandardizedActivity;
  summary?: ActivitySummary;
  laps?: LapSummary[];
  timeMarkers?: TimeMarkerSummary[];
  appliedEnrichments: string[];
  enrichmentMetadata: { [key: string]: string };
  registry: { [key: string]: { name: string; icon: string; description: string } };
  tags: string[];
  createdAt?: string;
  ownerDisplayName?: string;  // Public attribution - owner's display name or email prefix
}

/**
 * Compute summary statistics from activity data
 */
function computeSummary(activity: StandardizedActivity): ActivitySummary {
  let totalDuration = 0;
  let totalDistance = 0;
  let totalCalories = 0;
  let lapCount = 0;
  let strengthSetCount = 0;
  const heartRates: number[] = [];

  for (const session of activity.sessions || []) {
    totalDuration += session.totalElapsedTime || 0;
    totalDistance += session.totalDistance || 0;
    totalCalories += session.totalCalories || 0;
    lapCount += (session.laps || []).length;
    strengthSetCount += (session.strengthSets || []).length;

    // Collect heart rates from records
    for (const lap of session.laps || []) {
      for (const record of lap.records || []) {
        if (record.heartRate && record.heartRate > 0) {
          heartRates.push(record.heartRate);
        }
      }
    }

    // Also check session-level HR
    if (session.avgHeartRate && session.avgHeartRate > 0) {
      heartRates.push(session.avgHeartRate);
    }
  }

  const avgHeartRate = heartRates.length > 0
    ? Math.round(heartRates.reduce((a, b) => a + b, 0) / heartRates.length)
    : undefined;
  const maxHeartRate = heartRates.length > 0
    ? Math.max(...heartRates)
    : undefined;

  return {
    totalDurationSeconds: totalDuration,
    totalDistanceMeters: totalDistance,
    totalCalories: totalCalories > 0 ? totalCalories : undefined,
    avgHeartRate,
    maxHeartRate,
    lapCount,
    strengthSetCount,
  };
}

/**
 * Extract lap summaries for display
 */
function extractLaps(activity: StandardizedActivity): LapSummary[] {
  const laps: LapSummary[] = [];

  for (const session of activity.sessions || []) {
    for (const lap of session.laps || []) {
      laps.push({
        exerciseName: lap.exerciseName || 'Lap',
        durationSeconds: lap.totalElapsedTime || 0,
        distanceMeters: lap.totalDistance || 0,
      });
    }
  }

  return laps;
}

/**
 * Extract time markers for chart visualization
 */
function extractTimeMarkers(activity: StandardizedActivity): TimeMarkerSummary[] {
  return (activity.timeMarkers || []).map(marker => ({
    timestamp: marker.timestamp instanceof Date
      ? marker.timestamp.toISOString()
      : (marker.timestamp as string | undefined),
    label: marker.label || '',
    markerType: marker.markerType || '',
  }));
}
