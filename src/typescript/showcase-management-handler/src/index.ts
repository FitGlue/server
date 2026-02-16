// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch, RoutableRequest } from '@fitglue/shared/routing';
import { ShowcaseProfileStore, ShowcaseStore, UserStore } from '@fitglue/shared/storage';
import { requireAthleteTier } from '@fitglue/shared/domain';
import { Destination } from '@fitglue/shared/types';
import { getStorage } from 'firebase-admin/storage';
import { Timestamp as FirestoreTimestamp } from 'firebase-admin/firestore';

/**
 * Showcase Management Handler - Authenticated CRUD for showcase profiles
 *
 * Endpoints:
 * - GET    /profile              - Get user's profile + all showcased activities
 * - PATCH  /profile              - Update subtitle, bio, profilePictureUrl
 * - PATCH  /profile/slug         - Change showcase slug (with collision check)
 * - POST   /profile/picture      - Generate GCS signed upload URL
 * - DELETE /profile/entries/:id  - Remove entry from profile, recompute stats
 * - POST   /profile/entries/:id  - Add entry to profile, recompute stats
 * - GET    /preferences          - Get showcase preferences
 * - PATCH  /preferences          - Update showcase preferences
 * - POST   /preferences/apply-to-existing - Add showcase to selected pipelines
 */

const profileStore = new ShowcaseProfileStore(db);
const showcaseStore = new ShowcaseStore(db);
const userStore = new UserStore(db);

const PROFILE_PICTURE_BUCKET = `${process.env.GOOGLE_CLOUD_PROJECT || 'fitglue'}-showcase-assets`;
const PROFILE_PICTURE_MAX_SIZE = 5 * 1024 * 1024; // 5MB



function slugify(text: string): string {
    return text.toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '')
        .substring(0, 64);
}

/**
 * Recompute aggregate stats from entries array.
 */
function recomputeStats(entries: Array<Record<string, unknown>>): Record<string, unknown> {
    let totalDistanceMeters = 0;
    let totalDurationSeconds = 0;
    let totalSets = 0;
    let totalReps = 0;
    let totalWeightKg = 0;
    let latestActivityAt: Date | null = null;

    for (const entry of entries) {
        totalDistanceMeters += (entry.distance_meters as number) || 0;
        totalDurationSeconds += (entry.duration_seconds as number) || 0;
        totalSets += (entry.total_sets as number) || 0;
        totalReps += (entry.total_reps as number) || 0;
        totalWeightKg += (entry.total_weight_kg as number) || 0;

        const startTime = entry.start_time as Date | undefined;
        if (startTime && (!latestActivityAt || startTime > latestActivityAt)) {
            latestActivityAt = startTime;
        }
    }

    const stats: Record<string, unknown> = {
        total_activities: entries.length,
        total_distance_meters: totalDistanceMeters,
        total_duration_seconds: totalDurationSeconds,
        total_sets: totalSets,
        total_reps: totalReps,
        total_weight_kg: totalWeightKg,
    };
    if (latestActivityAt) {
        stats.latest_activity_at = latestActivityAt;
    }
    return stats;
}

// --- Route: GET /profile ---
async function handleGetProfile(userId: string): Promise<Record<string, unknown>> {
    const profile = await profileStore.getByUserId(userId);
    if (!profile) {
        return { profile: null, activities: [] };
    }

    const activities = await showcaseStore.listByUserId(userId);

    return {
        profile,
        activities: activities.map(a => ({
            showcaseId: a.showcaseId,
            title: a.title,
            activityType: a.activityType,
            source: a.source,
            startTime: a.startTime,
            createdAt: a.createdAt,
            // Flag if this activity is currently in the profile entries
            inProfile: profile.entries.some(e => e.showcaseId === a.showcaseId),
        })),
    };
}

// --- Route: PATCH /profile ---
async function handleUpdateProfile(
    userId: string,
    body: Record<string, unknown>
): Promise<Record<string, unknown>> {
    const profile = await profileStore.getByUserId(userId);
    if (!profile) {
        throw new HttpError(404, 'No showcase profile found');
    }

    const updates: Record<string, unknown> = {
        updated_at: FirestoreTimestamp.now(),
    };

    if (typeof body.subtitle === 'string') {
        updates.subtitle = body.subtitle.substring(0, 100);
    }
    if (typeof body.bio === 'string') {
        updates.bio = body.bio.substring(0, 500);
    }
    if (typeof body.profilePictureUrl === 'string') {
        updates.profile_picture_url = body.profilePictureUrl;
    }
    if (typeof body.visible === 'boolean') {
        updates.visible = body.visible;
    }

    await profileStore.update(profile.slug, updates);
    return { success: true };
}

// --- Route: PATCH /profile/slug ---
async function handleUpdateSlug(
    userId: string,
    body: Record<string, unknown>
): Promise<Record<string, unknown>> {
    const newSlugRaw = body.slug as string;
    if (!newSlugRaw || typeof newSlugRaw !== 'string') {
        throw new HttpError(400, 'Missing slug');
    }

    const newSlug = slugify(newSlugRaw);
    if (!newSlug || newSlug.length < 2) {
        throw new HttpError(400, 'Slug must be at least 2 characters');
    }

    const profile = await profileStore.getByUserId(userId);
    if (!profile) {
        throw new HttpError(404, 'No showcase profile found');
    }

    // Same slug — no-op
    if (profile.slug === newSlug) {
        return { success: true, slug: newSlug };
    }

    // Check for collision
    const exists = await profileStore.exists(newSlug);
    if (exists) {
        throw new HttpError(409, 'Someone is already using that slug');
    }

    // Migrate: read old profile data, write to new doc, delete old
    const oldDoc = await db.collection('showcase_profiles').doc(profile.slug).get();
    const oldData = oldDoc.data() as Record<string, unknown>;
    oldData.slug = newSlug;
    oldData.updated_at = FirestoreTimestamp.now();

    await profileStore.set(newSlug, oldData);
    await profileStore.delete(profile.slug);

    return { success: true, slug: newSlug };
}

// --- Route: POST /profile/picture ---
const ALLOWED_IMAGE_TYPES = ['image/webp', 'image/jpeg', 'image/png', 'image/gif'];
const EXT_MAP: Record<string, string> = {
    'image/webp': 'webp',
    'image/jpeg': 'jpg',
    'image/png': 'png',
    'image/gif': 'gif',
};

async function handlePictureUpload(
    userId: string,
    body: Record<string, unknown>
): Promise<Record<string, unknown>> {
    const contentType = (typeof body.contentType === 'string' && ALLOWED_IMAGE_TYPES.includes(body.contentType))
        ? body.contentType
        : 'image/webp';
    const ext = EXT_MAP[contentType] || 'webp';

    const storage = getStorage();
    const bucket = storage.bucket(PROFILE_PICTURE_BUCKET);
    const filePath = `showcase-profiles/${userId}/avatar.${ext}`;
    const file = bucket.file(filePath);

    const [uploadUrl] = await file.getSignedUrl({
        version: 'v4',
        action: 'write',
        expires: Date.now() + 15 * 60 * 1000, // 15 minutes
        contentType,
        extensionHeaders: {
            'x-goog-content-length-range': `0,${PROFILE_PICTURE_MAX_SIZE}`,
        },
    });

    const assetsBaseUrl = process.env.ASSETS_BASE_URL || `https://storage.googleapis.com/${PROFILE_PICTURE_BUCKET}`;
    const publicUrl = `${assetsBaseUrl}/${filePath}`;

    return {
        uploadUrl,
        publicUrl,
        contentType,
        maxSizeBytes: PROFILE_PICTURE_MAX_SIZE,
    };
}

// --- Route: DELETE /profile/entries/:showcaseId ---
async function handleRemoveEntry(
    userId: string,
    showcaseId: string
): Promise<Record<string, unknown>> {
    const profile = await profileStore.getByUserId(userId);
    if (!profile) {
        throw new HttpError(404, 'No showcase profile found');
    }

    // Get raw doc for array manipulation
    const doc = await db.collection('showcase_profiles').doc(profile.slug).get();
    const data = doc.data() as Record<string, unknown>;
    const entries = (data.entries as Array<Record<string, unknown>>) || [];

    const filtered = entries.filter(e => e.showcase_id !== showcaseId);
    if (filtered.length === entries.length) {
        throw new HttpError(404, 'Entry not found in profile');
    }

    const stats = recomputeStats(filtered);
    await profileStore.update(profile.slug, {
        entries: filtered,
        ...stats,
        updated_at: FirestoreTimestamp.now(),
    });

    return { success: true, totalEntries: filtered.length };
}

// --- Route: POST /profile/entries/:showcaseId ---
async function handleAddEntry(
    userId: string,
    showcaseId: string
): Promise<Record<string, unknown>> {
    const profile = await profileStore.getByUserId(userId);
    if (!profile) {
        throw new HttpError(404, 'No showcase profile found');
    }

    // Ensure the activity exists and belongs to user
    const activity = await showcaseStore.get(showcaseId);
    if (!activity || activity.userId !== userId) {
        throw new HttpError(404, 'Showcased activity not found');
    }

    // Check not already in profile
    if (profile.entries.some(e => e.showcaseId === showcaseId)) {
        return { success: true, message: 'Entry already in profile' };
    }

    // Fetch activity data from GCS to compute pill stats
    let distanceMeters = 0;
    let durationSeconds = 0;
    let totalSets = 0;
    let totalReps = 0;
    let totalWeightKg = 0;
    let routeThumbnailUrl = '';

    if (activity.activityDataUri) {
        try {
            const gcsMatch = activity.activityDataUri.match(/^gs:\/\/([^/]+)\/(.+)$/);
            if (gcsMatch) {
                const [, bucketName, filePath] = gcsMatch;
                const bucket = getStorage().bucket(bucketName);
                const [contents] = await bucket.file(filePath).download();
                const parsed = JSON.parse(contents.toString());

                // Handle both EnrichedActivityEvent wrapper and direct StandardizedActivity
                const activityData = parsed.activity_data || parsed.activityData || (parsed.sessions ? parsed : null);

                if (activityData?.sessions) {
                    for (const session of activityData.sessions) {
                        distanceMeters += session.totalDistance ?? session.total_distance ?? 0;
                        durationSeconds += session.totalElapsedTime ?? session.total_elapsed_time ?? 0;
                        const strengthSets = session.strengthSets || session.strength_sets || [];
                        for (const s of strengthSets) {
                            totalSets++;
                            totalReps += s.reps || 0;
                            const weight = s.weightKg ?? s.weight_kg ?? 0;
                            totalWeightKg += weight * (s.reps || 0);
                        }
                    }
                }
            }
        } catch {
            // Non-fatal: stats will be zero if GCS fetch fails
        }
    }

    // Check enrichment metadata for route thumbnail
    if (activity.enrichmentMetadata?.route_thumbnail_url) {
        routeThumbnailUrl = activity.enrichmentMetadata.route_thumbnail_url;
    }

    // Build entry from showcased activity (including pill data)
    const newEntry: Record<string, unknown> = {
        showcase_id: activity.showcaseId,
        title: activity.title,
        activity_type: activity.activityType,
        source: activity.source,
        distance_meters: distanceMeters,
        duration_seconds: durationSeconds,
        total_sets: totalSets,
        total_reps: totalReps,
        total_weight_kg: totalWeightKg,
    };
    if (activity.startTime) {
        newEntry.start_time = activity.startTime;
    }
    if (routeThumbnailUrl) {
        newEntry.route_thumbnail_url = routeThumbnailUrl;
    }

    // Get raw doc for array manipulation
    const doc = await db.collection('showcase_profiles').doc(profile.slug).get();
    const data = doc.data() as Record<string, unknown>;
    const entries = (data.entries as Array<Record<string, unknown>>) || [];
    entries.push(newEntry);

    const stats = recomputeStats(entries);
    await profileStore.update(profile.slug, {
        entries,
        ...stats,
        updated_at: FirestoreTimestamp.now(),
    });

    return { success: true, totalEntries: entries.length };
}

// --- Route: GET /preferences ---
async function handleGetPreferences(userId: string): Promise<Record<string, unknown>> {
    const userDoc = await db.collection('users').doc(userId).get();
    const userData = userDoc.data() as Record<string, unknown>;
    const prefs = (userData?.showcase_preferences as Record<string, unknown>) || {};

    return {
        defaultDestination: prefs.default_destination === true,
    };
}

// --- Route: PATCH /preferences ---
async function handleUpdatePreferences(
    userId: string,
    body: Record<string, unknown>
): Promise<Record<string, unknown>> {
    const updates: Record<string, unknown> = {};

    if (typeof body.defaultDestination === 'boolean') {
        updates['showcase_preferences.default_destination'] = body.defaultDestination;
    }

    if (Object.keys(updates).length > 0) {
        await db.collection('users').doc(userId).update(updates);
    }

    return { success: true };
}

// --- Route: POST /preferences/apply-to-existing ---
async function handleApplyToExisting(
    userId: string,
    body: Record<string, unknown>
): Promise<Record<string, unknown>> {
    const pipelineIds = body.pipelineIds as string[];
    if (!Array.isArray(pipelineIds) || pipelineIds.length === 0) {
        throw new HttpError(400, 'Missing pipelineIds array');
    }

    let updated = 0;

    for (const pipelineId of pipelineIds) {
        const pipelineRef = db.collection('users').doc(userId)
            .collection('pipelines').doc(pipelineId);
        const pipelineDoc = await pipelineRef.get();

        if (!pipelineDoc.exists) continue;

        const pipelineData = pipelineDoc.data() as Record<string, unknown>;
        const destinations = (pipelineData.destinations as number[]) || [];

        if (!destinations.includes(Destination.DESTINATION_SHOWCASE)) {
            destinations.push(Destination.DESTINATION_SHOWCASE);
            await pipelineRef.update({ destinations });
            updated++;
        }
    }

    return { success: true, updated };
}

// --- Main handler ---
export const handler: FrameworkHandler = async (req, ctx) => {
    if (!ctx.userId) {
        throw new HttpError(401, 'Unauthorized');
    }

    const userId = ctx.userId;

    // Tier check for all routes
    await requireAthleteTier(userStore, userId);

    return await routeRequest(req as RoutableRequest, ctx, [
        {
            method: 'GET',
            pattern: '*/profile',
            handler: async () => handleGetProfile(userId),
        },
        {
            method: 'PATCH',
            pattern: '*/profile/slug',
            handler: async () => handleUpdateSlug(userId, req.body as Record<string, unknown>),
        },
        {
            method: 'PATCH',
            pattern: '*/profile',
            handler: async () => handleUpdateProfile(userId, req.body as Record<string, unknown>),
        },
        {
            method: 'POST',
            pattern: '*/profile/picture',
            handler: async () => handlePictureUpload(userId, (req.body as Record<string, unknown>) || {}),
        },
        {
            method: 'DELETE',
            pattern: '*/profile/entries/:showcaseId',
            handler: async (match: RouteMatch) => handleRemoveEntry(userId, match.params.showcaseId),
        },
        {
            method: 'POST',
            pattern: '*/profile/entries/:showcaseId',
            handler: async (match: RouteMatch) => handleAddEntry(userId, match.params.showcaseId),
        },
        {
            method: 'GET',
            pattern: '*/preferences',
            handler: async () => handleGetPreferences(userId),
        },
        {
            method: 'PATCH',
            pattern: '*/preferences',
            handler: async () => handleUpdatePreferences(userId, req.body as Record<string, unknown>),
        },
        {
            method: 'POST',
            pattern: '*/preferences/apply-to-existing',
            handler: async () => handleApplyToExisting(userId, req.body as Record<string, unknown>),
        },
    ]);
};

// Export the wrapped function
export const showcaseManagementHandler = createCloudFunction(handler, {
    auth: {
        strategies: [new FirebaseAuthStrategy()]
    },
    skipExecutionLogging: true
});
