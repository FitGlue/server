import * as functions from '@google-cloud/functions-framework';
import * as admin from 'firebase-admin';
import { ShowcaseStore, type StandardizedActivity, type ActivityType, type ActivitySource, getEnricherManifest, getEffectiveTier } from '@fitglue/shared';
import { EnricherProviderType, UserRecord } from '@fitglue/shared/dist/types/pb/user';

// Initialize Firebase Admin (only once)
if (!admin.apps.length) {
  admin.initializeApp();
}

const db = admin.firestore();
const showcaseStore = new ShowcaseStore(db);

/**
 * Public showcase handler - serves activity data for shareable URLs.
 * Routes:
 *   GET /api/showcase/{id} - Returns JSON activity data
 *   GET /showcase/{id}     - Redirects to static viewer page
 */
export const showcaseHandler = async (req: functions.Request, res: functions.Response) => {
  // CORS Headers - public, allow all origins
  res.set('Access-Control-Allow-Origin', '*');
  res.set('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.set('Access-Control-Allow-Headers', 'Content-Type');
  res.set('Access-Control-Max-Age', '3600');

  if (req.method === 'OPTIONS') {
    res.status(204).send('');
    return;
  }

  if (req.method !== 'GET') {
    res.status(405).json({ error: 'Method Not Allowed' });
    return;
  }

  // Extract showcase ID from path
  // Paths: /api/showcase/{id} or /showcase/{id}
  const pathParts = req.path.split('/').filter(Boolean);
  let showcaseId: string | undefined;

  if (pathParts[0] === 'api' && pathParts[1] === 'showcase') {
    showcaseId = pathParts[2];
  } else if (pathParts[0] === 'showcase') {
    showcaseId = pathParts[1];
  }

  if (!showcaseId) {
    res.status(400).json({ error: 'Missing showcase ID' });
    return;
  }

  try {
    // Fetch from Firestore using ShowcaseStore
    const data = await showcaseStore.get(showcaseId);

    if (!data) {
      res.status(404).json({ error: 'Showcase not found' });
      return;
    }

    // Check expiration
    if (data.expiresAt && data.expiresAt < new Date()) {
      res.status(410).json({ error: 'This showcase has expired' });
      return;
    }

    // For /showcase/{id} (HTML page request) - redirect to static page
    const isHtmlRequest = pathParts[0] === 'showcase';
    if (isHtmlRequest) {
      // Serve the static page, which will fetch data via /api/showcase/{id}
      res.redirect(302, `/showcase.html?id=${showcaseId}`);
      return;
    }

    // For /api/showcase/{id} - return JSON
    // Apply heavy caching (showcased activities are immutable)
    res.set('Cache-Control', 'public, max-age=31536000, immutable');

    // Fetch user to determine tier
    const user = await db.collection('users').doc(data.userId).get();
    const userData = user.data() as UserRecord;
    const effectiveTier = getEffectiveTier(userData);

    // Build the public API response, stripping sensitive fields
    const response: ShowcaseResponse = {
      isAthlete: effectiveTier === 'athlete',
      showcaseId: data.showcaseId,
      title: data.title,
      description: data.description,
      activityType: data.activityType,
      source: data.source,
      startTime: data.startTime?.toISOString(),
      activityData: data.activityData,
      appliedEnrichments: data.appliedEnrichments || [],
      enrichmentMetadata: data.enrichmentMetadata || {},
      registry: (data.appliedEnrichments || []).reduce((acc, e) => {
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
      }, {} as { [key: string]: { name: string; icon: string; description: string } }),
      tags: data.tags || [],
      createdAt: data.createdAt?.toISOString(),
      ownerDisplayName: data.ownerDisplayName,
      // Don't expose: userId, activityId, fitFileUri, pipelineExecutionId, expiresAt
    };

    res.status(200).json(response);
  } catch (error) {
    console.error('Error fetching showcase:', error);
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

// Register the function
functions.http('showcaseHandler', showcaseHandler);

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
