import * as functions from '@google-cloud/functions-framework';
import * as admin from 'firebase-admin';
import type { ShowcasedActivity, StandardizedActivity, ActivityType, ActivitySource } from '@fitglue/shared';

// Initialize Firebase Admin (only once)
if (!admin.apps.length) {
  admin.initializeApp();
}

const db = admin.firestore();

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
    // Fetch from Firestore
    const docRef = db.collection('showcased_activities').doc(showcaseId);
    const doc = await docRef.get();

    if (!doc.exists) {
      res.status(404).json({ error: 'Showcase not found' });
      return;
    }

    const data = doc.data() as ShowcasedActivityFirestore;

    // Check expiration
    if (data.expiresAt && data.expiresAt.toDate() < new Date()) {
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

    // Convert Firestore timestamps and strip sensitive data
    const response: ShowcaseResponse = {
      showcaseId: data.showcaseId,
      title: data.title,
      description: data.description,
      activityType: data.activityType,
      source: data.source,
      startTime: data.startTime?.toDate().toISOString(),
      activityData: data.activityData,
      appliedEnrichments: data.appliedEnrichments || [],
      enrichmentMetadata: data.enrichmentMetadata || {},
      tags: data.tags || [],
      createdAt: data.createdAt?.toDate().toISOString(),
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

// Types for Firestore data (with Timestamps)
interface ShowcasedActivityFirestore {
  showcaseId: string;
  activityId: string;
  userId: string;
  title: string;
  description: string;
  activityType: ActivityType;
  source: ActivitySource;
  startTime?: admin.firestore.Timestamp;
  activityData?: StandardizedActivity;
  fitFileUri: string;
  appliedEnrichments: string[];
  enrichmentMetadata: { [key: string]: string };
  tags: string[];
  pipelineExecutionId?: string;
  createdAt?: admin.firestore.Timestamp;
  expiresAt?: admin.firestore.Timestamp;
}

// Public API response (sanitized, no sensitive data)
interface ShowcaseResponse {
  showcaseId: string;
  title: string;
  description: string;
  activityType: ActivityType;
  source: ActivitySource;
  startTime?: string;
  activityData?: StandardizedActivity;
  appliedEnrichments: string[];
  enrichmentMetadata: { [key: string]: string };
  tags: string[];
  createdAt?: string;
}
