import {
  ShowcaseStore,
  type StandardizedActivity,
  type ActivityType,
  type ActivitySource,
  getEnricherManifest,
  getEffectiveTier,
  createCloudFunction,
  FrameworkResponse,
  HttpError,
  FrameworkContext,
  routeRequest,
  db
} from '@fitglue/shared';
import * as admin from 'firebase-admin';
import { EnricherProviderType, UserRecord } from '@fitglue/shared/dist/types/pb/user';
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

  // Fetch activity data from GCS if stored there, otherwise use inline data (legacy)
  let activityData: StandardizedActivity | undefined = data.activityData;
  if (!activityData && data.activityDataUri) {
    try {
      // Parse GCS URI: gs://bucket-name/path/to/file
      const gcsMatch = data.activityDataUri.match(/^gs:\/\/([^/]+)\/(.+)$/);
      if (gcsMatch) {
        const [, bucketName, filePath] = gcsMatch;
        const bucket = admin.storage().bucket(bucketName);
        const [contents] = await bucket.file(filePath).download();
        activityData = JSON.parse(contents.toString()) as StandardizedActivity;
      }
    } catch (err) {
      console.error('Failed to fetch activity data from GCS', { error: err, uri: data.activityDataUri });
      // Continue without activity data - page will show partial content
    }
  }

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
  appliedEnrichments: string[];
  enrichmentMetadata: { [key: string]: string };
  registry: { [key: string]: { name: string; icon: string; description: string } };
  tags: string[];
  createdAt?: string;
  ownerDisplayName?: string;  // Public attribution - owner's display name or email prefix
}
