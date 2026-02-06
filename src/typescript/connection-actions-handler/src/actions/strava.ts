/**
 * Strava Action: Import Cardio PRs
 *
 * Fetches athlete activities from Strava and extracts best_efforts to populate
 * FitGlue personal records (5K, 10K, Half Marathon times).
 */

import { db } from '@fitglue/shared/framework';
import { Timestamp } from 'firebase-admin/firestore';

interface ActionResult {
    recordsImported: number;
    recordsSkipped: number;
    details: string[];
}

interface Logger {
    info: (msg: string, data?: Record<string, unknown>) => void;
}

interface StravaActivity {
    id: number;
    name: string;
    type: string;
    start_date: string;
    distance: number;
    moving_time: number;
    best_efforts?: StravaBestEffort[];
}

interface StravaBestEffort {
    id: number;
    name: string;
    elapsed_time: number;
    moving_time: number;
    start_date: string;
    pr_rank: number | null;
    distance: number;
}

const BEST_EFFORT_MAP: Record<string, string> = {
    '5k': 'fastest_5k',
    '10k': 'fastest_10k',
    'Half-Marathon': 'fastest_half_marathon',
};

export async function importStravaCardioPRs(userId: string, logger: Logger): Promise<ActionResult> {
    const result: ActionResult = {
        recordsImported: 0,
        recordsSkipped: 0,
        details: [],
    };

    const accessToken = await getStravaAccessToken(userId);
    const activities = await fetchStravaActivities(accessToken, logger);
    logger.info('Fetched Strava activities', { count: activities.length });

    const newRecords = await extractBestEfforts(accessToken, activities);
    await saveRecordsToFirestore(userId, newRecords, result);

    logger.info('Strava PR import complete', {
        imported: result.recordsImported,
        skipped: result.recordsSkipped,
    });

    return result;
}

async function getStravaAccessToken(userId: string): Promise<string> {
    const userDoc = await db.collection('users').doc(userId).get();
    const userData = userDoc.data();
    const stravaIntegration = userData?.integrations?.strava;

    if (!stravaIntegration?.access_token) {
        throw new Error('Strava not connected. Please connect your Strava account first.');
    }

    const expiresAt = stravaIntegration.expires_at?.toDate?.() ?? new Date(stravaIntegration.expires_at);

    if (expiresAt < new Date()) {
        return await refreshStravaToken(userId, stravaIntegration.refresh_token);
    }

    return stravaIntegration.access_token;
}

async function refreshStravaToken(userId: string, refreshToken: string): Promise<string> {
    const clientId = process.env.STRAVA_CLIENT_ID;
    const clientSecret = process.env.STRAVA_CLIENT_SECRET;

    if (!clientId || !clientSecret) {
        throw new Error('Missing Strava OAuth credentials');
    }

    const response = await fetch('https://www.strava.com/api/v3/oauth/token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            client_id: clientId,
            client_secret: clientSecret,
            refresh_token: refreshToken,
            grant_type: 'refresh_token',
        }),
    });

    if (!response.ok) {
        throw new Error(`Failed to refresh Strava token: ${response.status}`);
    }

    const data = await response.json() as {
        access_token: string;
        refresh_token: string;
        expires_at: number;
    };

    await db.collection('users').doc(userId).update({
        'integrations.strava.access_token': data.access_token,
        'integrations.strava.refresh_token': data.refresh_token,
        'integrations.strava.expires_at': Timestamp.fromDate(new Date(data.expires_at * 1000)),
    });

    return data.access_token;
}

async function fetchStravaActivities(accessToken: string, logger: Logger): Promise<StravaActivity[]> {
    const activities: StravaActivity[] = [];
    const perPage = 100;
    const maxPages = 5;

    for (let page = 1; page <= maxPages; page++) {
        const response = await fetch(
            `https://www.strava.com/api/v3/athlete/activities?page=${page}&per_page=${perPage}`,
            { headers: { Authorization: `Bearer ${accessToken}` } }
        );

        if (!response.ok) {
            if (response.status === 429) {
                logger.info('Rate limited by Strava API, stopping pagination', { page });
                break;
            }
            throw new Error(`Strava API error: ${response.status}`);
        }

        const pageActivities = await response.json() as StravaActivity[];
        if (pageActivities.length === 0) break;

        activities.push(...pageActivities);
        if (pageActivities.length < perPage) break;
    }

    return activities;
}

async function extractBestEfforts(
    accessToken: string,
    activities: StravaActivity[]
): Promise<Map<string, { time: number; activityId: number; date: string }>> {
    const newRecords = new Map<string, { time: number; activityId: number; date: string }>();

    for (const activity of activities) {
        if (!['Run', 'VirtualRun'].includes(activity.type)) continue;

        const detailedActivity = await fetchActivityWithEfforts(accessToken, activity.id);
        if (!detailedActivity.best_efforts) continue;

        for (const effort of detailedActivity.best_efforts) {
            const recordType = BEST_EFFORT_MAP[effort.name];
            if (!recordType) continue;

            const existingBest = newRecords.get(recordType);
            if (!existingBest || effort.moving_time < existingBest.time) {
                newRecords.set(recordType, {
                    time: effort.moving_time,
                    activityId: activity.id,
                    date: effort.start_date,
                });
            }
        }
    }

    return newRecords;
}

async function fetchActivityWithEfforts(accessToken: string, activityId: number): Promise<StravaActivity> {
    const response = await fetch(
        `https://www.strava.com/api/v3/activities/${activityId}?include_all_efforts=true`,
        { headers: { Authorization: `Bearer ${accessToken}` } }
    );

    if (!response.ok) {
        throw new Error(`Failed to fetch activity ${activityId}: ${response.status}`);
    }

    return await response.json() as StravaActivity;
}

async function saveRecordsToFirestore(
    userId: string,
    newRecords: Map<string, { time: number; activityId: number; date: string }>,
    result: ActionResult
): Promise<void> {
    const recordsCollection = db.collection('users').doc(userId).collection('personal_records');

    for (const [recordType, newRecord] of newRecords.entries()) {
        const existingDoc = await recordsCollection.doc(recordType).get();
        const existing = existingDoc.data();

        if (existing && existing.value <= newRecord.time) {
            result.recordsSkipped++;
            result.details.push(`${formatRecordType(recordType)}: existing ${formatDuration(existing.value)} is faster`);
            continue;
        }

        const improvement = existing
            ? ((existing.value - newRecord.time) / existing.value) * 100
            : undefined;

        await recordsCollection.doc(recordType).set({
            record_type: recordType,
            value: newRecord.time,
            unit: 'seconds',
            activity_id: newRecord.activityId.toString(),
            achieved_at: Timestamp.fromDate(new Date(newRecord.date)),
            previous_value: existing?.value,
            improvement,
            source: 'strava_import',
        });

        result.recordsImported++;
        const improvementStr = improvement ? ` (${improvement.toFixed(1)}% faster)` : ' (first record)';
        result.details.push(`${formatRecordType(recordType)}: ${formatDuration(newRecord.time)}${improvementStr}`);
    }
}

function formatRecordType(recordType: string): string {
    const map: Record<string, string> = {
        fastest_5k: 'Fastest 5K',
        fastest_10k: 'Fastest 10K',
        fastest_half_marathon: 'Fastest Half Marathon',
    };
    return map[recordType] ?? recordType;
}

function formatDuration(seconds: number): string {
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;

    if (hours > 0) {
        return `${hours}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}
