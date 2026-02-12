// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext, FrameworkResponse, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest } from '@fitglue/shared/routing';
import { ShowcaseStore, ShowcaseProfileStore } from '@fitglue/shared/storage';
import { getEnricherManifest } from '@fitglue/shared/plugin';
import { getEffectiveTier } from '@fitglue/shared/domain';
import type { StandardizedActivity, ActivityType, ActivitySource } from '@fitglue/shared/types';
import { EnricherProviderType, UserRecord } from '@fitglue/shared/types';
import * as admin from 'firebase-admin';
import { Request } from 'express';

// Initialize Firebase if not already done (idempotent)
if (admin.apps.length === 0) {
  admin.initializeApp();
}

/**
 * Public showcase handler - serves activity data for shareable URLs.
 * Routes:
 *   GET /api/showcase/{id} - Returns JSON activity data
 *   GET /showcase/{id}     - Redirects to static viewer page
 */
export const showcaseHandler = createCloudFunction(async (req: Request, ctx: FrameworkContext) => {
  const showcaseStore = new ShowcaseStore(db);
  const showcaseProfileStore = new ShowcaseProfileStore(db);


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
        return await handleApiShowcase(match.params.id, showcaseStore, db, corsHeaders, ctx.logger);
      }
    },
    {
      method: 'GET',
      pattern: '/api/showcase/profile/:slug',
      handler: async (match) => {
        return await handleProfileApi(match.params.slug, showcaseProfileStore, corsHeaders);
      }
    },
    {
      method: 'GET',
      pattern: '/u/:slug',
      handler: async (match) => {
        return await handleHtmlProfile(match.params.slug, showcaseProfileStore, corsHeaders);
      }
    },
    {
      method: 'GET',
      pattern: '/u/:slug/:id',
      handler: async (match) => {
        return await handleHtmlShowcase(match.params.id, showcaseStore, corsHeaders);
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

/**
 * Fetch activity data from GCS URI.
 * Handles both old format (direct StandardizedActivity) and new format (EnrichedActivityEvent wrapper).
 */
async function fetchActivityDataFromGcs(
  uri: string | undefined,
  logger: import('winston').Logger
): Promise<StandardizedActivity | undefined> {
  if (!uri) return undefined;

  try {
    // Parse GCS URI: gs://bucket-name/path/to/file
    const gcsMatch = uri.match(/^gs:\/\/([^/]+)\/(.+)$/);
    if (!gcsMatch) return undefined;

    const [, bucketName, filePath] = gcsMatch;
    const bucket = admin.storage().bucket(bucketName);
    const [contents] = await bucket.file(filePath).download();
    const parsed = JSON.parse(contents.toString());

    // New format (EnrichedActivityEvent): extract activity_data from nested field
    if (parsed.activity_data || parsed.activityData) {
      return parsed.activity_data || parsed.activityData;
    }
    // Old format: the file IS the StandardizedActivity
    if (parsed.sessions) {
      return parsed as StandardizedActivity;
    }
    return undefined;
  } catch (err) {
    logger.error('Failed to fetch activity data from GCS', { error: err, uri });
    return undefined;
  }
}
async function handleApiShowcase(
  showcaseId: string,
  showcaseStore: ShowcaseStore,
  db: admin.firestore.Firestore,
  corsHeaders: Record<string, string>,
  logger: import('winston').Logger
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

  // Fetch activity data from GCS if not inline
  const activityData = data.activityData ?? await fetchActivityDataFromGcs(data.activityDataUri, logger);

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
      'Cache-Control': 'public, max-age=300'
    }
  });
}


const OG_FALLBACK_IMAGE = 'https://fitglue.com/images/screenshots/boosted-activity.png';
const SITE_URL = 'https://fitglue.com';

/**
 * Generate a minimal HTML page with dynamic OG meta tags and a client-side redirect.
 * Social media crawlers read the OG tags; browsers execute the JS redirect to the full page.
 */
function generateOgHtml(options: {
  title: string;
  description: string;
  url: string;
  redirectUrl: string;
  image?: string;
}): string {
  const escape = (s: string) => s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const title = escape(options.title);
  const description = escape(options.description);
  const url = escape(options.url);
  const image = escape(options.image || OG_FALLBACK_IMAGE);
  const redirectUrl = escape(options.redirectUrl);

  return [
    '<!DOCTYPE html>',
    '<html lang="en">',
    '<head>',
    '  <meta charset="UTF-8">',
    '  <meta name="viewport" content="width=device-width, initial-scale=1.0">',
    `  <title>${title}</title>`,
    `  <meta name="description" content="${description}">`,
    '',
    '  <!-- Open Graph -->',
    `  <meta property="og:title" content="${title}">`,
    `  <meta property="og:description" content="${description}">`,
    '  <meta property="og:type" content="website">',
    `  <meta property="og:url" content="${url}">`,
    `  <meta property="og:image" content="${image}">`,
    '  <meta property="og:image:width" content="1200">',
    '  <meta property="og:image:height" content="630">',
    '  <meta property="og:site_name" content="FitGlue">',
    '  <meta property="og:locale" content="en_GB">',
    '',
    '  <!-- Twitter Card -->',
    '  <meta name="twitter:card" content="summary_large_image">',
    `  <meta name="twitter:title" content="${title}">`,
    `  <meta name="twitter:description" content="${description}">`,
    `  <meta name="twitter:image" content="${image}">`,
    '',
    `  <meta http-equiv="refresh" content="0;url=${redirectUrl}">`,
    '  <style>',
    '    *{margin:0;padding:0;box-sizing:border-box}',
    "    body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0A0A0A;color:#fff;min-height:100vh;display:flex;align-items:center;justify-content:center;overflow:hidden}",
    '    .bg{position:fixed;inset:0;background:radial-gradient(ellipse at 20% 20%,rgba(255,27,141,.08) 0%,transparent 50%),radial-gradient(ellipse at 80% 80%,rgba(157,78,221,.08) 0%,transparent 50%),radial-gradient(ellipse at 50% 50%,rgba(76,201,240,.03) 0%,transparent 70%);animation:bp 4s ease-in-out infinite}',
    '    @keyframes bp{0%,100%{opacity:.6;transform:scale(1)}50%{opacity:1;transform:scale(1.05)}}',
    '    .c{display:flex;flex-direction:column;align-items:center;gap:2rem;z-index:1;animation:fi .6s ease-out}',
    '    @keyframes fi{from{opacity:0;transform:translateY(10px)}to{opacity:1;transform:translateY(0)}}',
    '    .logo{font-size:clamp(2.5rem,8vw,4rem);font-weight:900;letter-spacing:-.02em}',
    '    .f{background:linear-gradient(135deg,#FF1B8D,#FF6BB3);background-clip:text;-webkit-background-clip:text;-webkit-text-fill-color:transparent;animation:sh 2s ease-in-out infinite}',
    '    .g{background:linear-gradient(135deg,#9D4EDD,#C77DFF);background-clip:text;-webkit-background-clip:text;-webkit-text-fill-color:transparent;animation:sh 2s ease-in-out infinite .15s}',
    '    @keyframes sh{0%,100%{opacity:.9;filter:brightness(1)}50%{opacity:1;filter:brightness(1.2)}}',
    '    .sc{position:relative;width:60px;height:60px}',
    '    .sr{position:absolute;inset:0;border-radius:50%;border:3px solid transparent;border-top-color:#FF1B8D;animation:sp 1.2s cubic-bezier(.5,.1,.5,.9) infinite}',
    '    .s2{inset:6px;border-top-color:#9D4EDD;animation-duration:1.8s;animation-direction:reverse}',
    '    .s3{inset:12px;border-top-color:#4CC9F0;animation-duration:2.4s}',
    '    @keyframes sp{to{transform:rotate(360deg)}}',
    '    .msg{font-size:1rem;color:rgba(255,255,255,.5);font-weight:500;min-height:1.5em;transition:opacity .3s ease}',
    '    .msg.fo{opacity:0}',
    '  </style>',
    '</head>',
    '<body>',
    '  <div class="bg"></div>',
    '  <div class="c">',
    '    <div class="logo"><span class="f">Fit</span><span class="g">Glue</span></div>',
    '    <div class="sc"><div class="sr"></div><div class="sr s2"></div><div class="sr s3"></div></div>',
    '    <p id="m" class="msg">Loading activity...</p>',
    '  </div>',
    '  <script>',
    `    window.location.replace("${redirectUrl.replace(/"/g, '\\"')}");`,
    "    var ms=['Reticulating muscle fibers...','Calibrating sweat glands...','Polishing your running shoes...','Stretching the pixels...','Syncing your chakras...','Buffering endorphins...','Warming up the algorithms...','Hydrating the database...','Massaging the data points...',\"Doing some light cardio...\",\"Flexing the API...\",\"Foam rolling the server...\",\"Untangling your headphones...\",\"Motivating the backend...\",\"Loading protein shakes...\",\"Activating beast mode...\"];",
    "    var el=document.getElementById('m'),i=Math.floor(Math.random()*ms.length);",
    '    if(el)el.textContent=ms[i];',
    "    setInterval(function(){if(!el)return;el.classList.add('fo');setTimeout(function(){i=(i+1)%ms.length;el.textContent=ms[i];el.classList.remove('fo')},300)},2000);",
    '  </script>',
    '</body>',
    '</html>',
  ].join('\n');
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

  const ownerAttribution = data.ownerDisplayName ? ` by ${data.ownerDisplayName}` : '';
  const ogTitle = `${data.title || 'Activity'}${ownerAttribution} — FitGlue`;
  const ogDescription = data.description || 'Check out this activity on FitGlue — Watch your workout become extraordinary.';
  const canonicalUrl = `${SITE_URL}/showcase/${showcaseId}`;

  const html = generateOgHtml({
    title: ogTitle,
    description: ogDescription,
    url: canonicalUrl,
    redirectUrl: `/showcase.html?id=${showcaseId}`,
  });

  return new FrameworkResponse({
    status: 200,
    body: html,
    headers: {
      ...corsHeaders,
      'Content-Type': 'text/html; charset=utf-8',
      'Cache-Control': 'public, max-age=300',
    }
  });
}

/**
 * Handle GET /u/:slug
 * Serves an HTML page with dynamic OG tags for the profile, then redirects to the full profile page.
 */
async function handleHtmlProfile(
  slug: string,
  profileStore: ShowcaseProfileStore,
  corsHeaders: Record<string, string>
): Promise<FrameworkResponse> {
  const profile = await profileStore.get(slug);
  if (!profile || profile.visible === false) {
    throw new HttpError(404, 'Profile not found');
  }

  const ogTitle = profile.subtitle
    ? `${profile.displayName} — ${profile.subtitle}`
    : `${profile.displayName} — FitGlue Athlete`;
  const statsParts: string[] = [];
  if (profile.totalActivities > 0) statsParts.push(`${profile.totalActivities} activities`);
  if (profile.totalDistanceMeters > 0) statsParts.push(`${(profile.totalDistanceMeters / 1000).toFixed(1)} km`);
  if (profile.totalDurationSeconds > 0) {
    const hours = Math.floor(profile.totalDurationSeconds / 3600);
    if (hours > 0) statsParts.push(`${hours}h active`);
  }
  const ogDescription = profile.bio
    ? profile.bio
    : statsParts.length > 0
      ? `${profile.displayName}'s showcase: ${statsParts.join(' · ')}. Powered by FitGlue.`
      : `${profile.displayName}'s athlete showcase on FitGlue.`;

  const canonicalUrl = `${SITE_URL}/u/${encodeURIComponent(slug)}`;

  const html = generateOgHtml({
    title: ogTitle,
    description: ogDescription,
    url: canonicalUrl,
    redirectUrl: `/showcase-profile.html?slug=${encodeURIComponent(slug)}`,
    image: profile.profilePictureUrl || undefined,
  });

  return new FrameworkResponse({
    status: 200,
    body: html,
    headers: {
      ...corsHeaders,
      'Content-Type': 'text/html; charset=utf-8',
      'Cache-Control': 'public, max-age=300',
    }
  });
}

/**
 * Handle GET /api/showcase/profile/:slug
 * Returns public profile data for the showcase homepage.
 */
async function handleProfileApi(
  slug: string,
  profileStore: ShowcaseProfileStore,
  corsHeaders: Record<string, string>
): Promise<FrameworkResponse> {
  const profile = await profileStore.get(slug);
  if (!profile || profile.visible === false) {
    throw new HttpError(404, 'Profile not found');
  }

  // Map entries to public-safe format with formatted fields
  const entries = profile.entries.map(entry => ({
    showcaseId: entry.showcaseId,
    title: entry.title,
    activityType: entry.activityType,
    source: entry.source,
    startTime: entry.startTime?.toISOString(),
    routeThumbnailUrl: entry.routeThumbnailUrl || undefined,
    distanceMeters: entry.distanceMeters,
    durationSeconds: entry.durationSeconds,
    totalSets: entry.totalSets,
    totalReps: entry.totalReps,
    totalWeightKg: entry.totalWeightKg,
  }));

  const response = {
    slug: profile.slug,
    displayName: profile.displayName,
    subtitle: profile.subtitle || '',
    bio: profile.bio || '',
    profilePictureUrl: profile.profilePictureUrl || '',
    entries,
    totalActivities: profile.totalActivities,
    totalDistanceMeters: profile.totalDistanceMeters,
    totalDurationSeconds: profile.totalDurationSeconds,
    totalSets: profile.totalSets,
    totalReps: profile.totalReps,
    totalWeightKg: profile.totalWeightKg,
    latestActivityAt: profile.latestActivityAt?.toISOString(),
  };

  return new FrameworkResponse({
    status: 200,
    body: response,
    headers: {
      ...corsHeaders,
      'Cache-Control': 'public, max-age=300',
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
 * Collect heart rates from activity sessions
 */
function collectHeartRates(activity: StandardizedActivity): number[] {
  const heartRates: number[] = [];

  for (const session of activity.sessions || []) {
    // Collect from lap records
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

  return heartRates;
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

  for (const session of activity.sessions || []) {
    totalDuration += session.totalElapsedTime || 0;
    totalDistance += session.totalDistance || 0;
    totalCalories += session.totalCalories || 0;
    lapCount += (session.laps || []).length;
    strengthSetCount += (session.strengthSets || []).length;
  }

  const heartRates = collectHeartRates(activity);
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
