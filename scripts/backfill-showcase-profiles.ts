#!/usr/bin/env npx ts-node

/**
 * backfill-showcase-profiles.ts
 *
 * One-off script to generate showcase_profiles documents from existing
 * showcased_activities. Groups activities by user, computes aggregate stats,
 * and writes profile documents.
 *
 * Usage:
 *   npx ts-node scripts/backfill-showcase-profiles.ts --env=dev [--dry-run]
 *   npx ts-node scripts/backfill-showcase-profiles.ts --env=prod --dry-run
 *
 * Options:
 *   --env=dev|test|prod   (required) Target environment
 *   --dry-run             Preview changes without writing
 */

import * as admin from 'firebase-admin';
import { Storage } from '@google-cloud/storage';

const ENVIRONMENTS: Record<string, string> = {
    dev: 'fitglue-server-dev',
    test: 'fitglue-server-test',
    prod: 'fitglue-server-prod',
};

function resolveEnv(args: string[]): string {
    const envArg = args.find(a => a.startsWith('--env='));
    if (!envArg) {
        console.error('❌ Missing required --env flag. Usage: --env=dev|test|prod');
        process.exit(1);
    }
    const env = envArg.split('=')[1];
    const projectId = ENVIRONMENTS[env];
    if (!projectId) {
        console.error(`❌ Unknown environment "${env}". Must be one of: ${Object.keys(ENVIRONMENTS).join(', ')}`);
        process.exit(1);
    }
    return projectId;
}

const scriptArgs = process.argv.slice(2);
const projectId = resolveEnv(scriptArgs);

if (!admin.apps.length) {
    admin.initializeApp({ projectId });
}

const db = admin.firestore();
const storage = new Storage({ projectId });

interface RawShowcase {
    showcase_id: string;
    user_id: string;
    title: string;
    activity_type: number;
    source: number;
    start_time: admin.firestore.Timestamp | null;
    enrichment_metadata: Record<string, string>;
    activity_data_uri: string;
    owner_display_name: string;
    expires_at: admin.firestore.Timestamp | null;
}

function slugify(name: string): string {
    return name
        .toLowerCase()
        .trim()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '');
}

async function loadActivityData(uri: string): Promise<any | null> {
    if (!uri) {
        console.log('      (no activity_data_uri)');
        return null;
    }
    try {
        // URI format: gs://bucket/path
        const match = uri.match(/^gs:\/\/([^/]+)\/(.+)$/);
        if (!match) return null;
        const [, bucket, path] = match;
        const [content] = await storage.bucket(bucket).file(path).download();
        const parsed = JSON.parse(content.toString('utf8'));

        // New format (EnrichedActivityEvent): extract activity_data from wrapper
        if (parsed.activity_data || parsed.activityData) {
            return parsed.activity_data || parsed.activityData;
        }
        // Old format: the file IS the StandardizedActivity directly
        if (parsed.sessions) {
            return parsed;
        }

        console.warn(`      ⚠️ Unknown GCS data format (keys: ${Object.keys(parsed).join(', ')})`);
        return null;
    } catch (err) {
        console.warn(`  ⚠️ Could not load activity data from ${uri}: ${err}`);
        return null;
    }
}

function computeStrengthStats(activityData: any): { sets: number; reps: number; weightKg: number } {
    let sets = 0;
    let reps = 0;
    let weightKg = 0;
    if (!activityData?.sessions) return { sets, reps, weightKg };

    for (const session of activityData.sessions) {
        // Proto JSON uses camelCase: strengthSets, weightKg
        const strengthSets = session.strengthSets || session.strength_sets || [];
        for (const s of strengthSets) {
            sets++;
            reps += s.reps || 0;
            const weight = s.weightKg ?? s.weight_kg ?? 0;
            weightKg += weight * (s.reps || 0);
        }
    }
    return { sets, reps, weightKg };
}

function computeDistanceDuration(activityData: any): { distance: number; duration: number } {
    let distance = 0;
    let duration = 0;
    if (!activityData?.sessions) return { distance, duration };

    for (const session of activityData.sessions) {
        // Proto JSON uses camelCase: totalDistance, totalElapsedTime
        distance += session.totalDistance ?? session.total_distance ?? 0;
        duration += session.totalElapsedTime ?? session.total_elapsed_time ?? 0;
    }
    return { distance, duration };
}

async function main() {
    const dryRun = scriptArgs.includes('--dry-run');

    console.log('📋 Showcase Profile Backfill Script');
    console.log('====================================');
    console.log(`Project: ${projectId}`);
    console.log(`Mode: ${dryRun ? 'DRY RUN' : 'LIVE'}`);

    // Step 1: Read all showcased activities
    console.log('\n📥 Reading showcased_activities...');
    const snapshot = await db.collection('showcased_activities').get();
    console.log(`  Found ${snapshot.size} showcased activities`);

    // Step 2: Group by user
    const userShowcases = new Map<string, RawShowcase[]>();
    for (const doc of snapshot.docs) {
        const data = doc.data() as RawShowcase;
        if (!data.user_id) {
            console.warn(`  ⚠️ ${doc.id} has no user_id, skipping`);
            continue;
        }
        // Skip expired showcases
        if (data.expires_at && data.expires_at.toDate() < new Date()) {
            continue;
        }
        const existing = userShowcases.get(data.user_id) || [];
        existing.push({ ...data, showcase_id: doc.id });
        userShowcases.set(data.user_id, existing);
    }

    console.log(`  Grouped into ${userShowcases.size} users`);

    // Step 3: For each user, look up their tier and build the profile
    let profilesCreated = 0;
    let profilesSkipped = 0;

    for (const [userId, showcases] of userShowcases.entries()) {
        // Look up user for display name fallback
        const userDoc = await db.collection('users').doc(userId).get();
        const userData = userDoc.exists ? userDoc.data()! : {};

        // Determine display name and slug
        const displayName = showcases[0]?.owner_display_name || userData.display_name || 'Unknown';
        const slug = slugify(displayName);
        if (!slug) {
            console.warn(`  ⚠️ User ${userId} has no valid display name for slug`);
            profilesSkipped++;
            continue;
        }

        console.log(`\n👤 ${displayName} (${slug}) — ${showcases.length} showcases`);

        // Build entries with stats from activity data
        const entries = [];
        let totalDistance = 0;
        let totalDuration = 0;
        let totalSets = 0;
        let totalReps = 0;
        let totalWeightKg = 0;
        let latestTime: Date | null = null;

        for (const showcase of showcases) {
            // Try to load activity data from GCS for stats
            const activityData = await loadActivityData(showcase.activity_data_uri);
            const { distance, duration } = computeDistanceDuration(activityData);
            const strength = computeStrengthStats(activityData);

            const startTime = showcase.start_time?.toDate() || null;
            if (startTime && (!latestTime || startTime > latestTime)) {
                latestTime = startTime;
            }

            entries.push({
                showcase_id: showcase.showcase_id,
                title: showcase.title,
                activity_type: showcase.activity_type,
                source: showcase.source,
                start_time: showcase.start_time || null,
                route_thumbnail_url: showcase.enrichment_metadata?.route_thumbnail_url || '',
                distance_meters: distance,
                duration_seconds: duration,
                total_sets: strength.sets,
                total_reps: strength.reps,
                total_weight_kg: strength.weightKg,
            });

            totalDistance += distance;
            totalDuration += duration;
            totalSets += strength.sets;
            totalReps += strength.reps;
            totalWeightKg += strength.weightKg;

            console.log(`    📝 ${showcase.title} — ${distance > 0 ? (distance / 1000).toFixed(1) + 'km' : ''} ${strength.sets > 0 ? strength.sets + ' sets' : ''}`);
        }

        const profile = {
            slug,
            user_id: userId,
            display_name: displayName,
            entries,
            total_activities: entries.length,
            total_distance_meters: totalDistance,
            total_duration_seconds: totalDuration,
            total_sets: totalSets,
            total_reps: totalReps,
            total_weight_kg: totalWeightKg,
            latest_activity_at: latestTime ? admin.firestore.Timestamp.fromDate(latestTime) : null,
            created_at: admin.firestore.FieldValue.serverTimestamp(),
            updated_at: admin.firestore.FieldValue.serverTimestamp(),
        };

        if (!dryRun) {
            await db.collection('showcase_profiles').doc(slug).set(profile);
            console.log(`  ✅ Created profile: showcase_profiles/${slug}`);
        } else {
            console.log(`  [DRY RUN] Would create profile: showcase_profiles/${slug}`);
            console.log(`    Stats: ${entries.length} activities, ${(totalDistance / 1000).toFixed(1)}km, ${totalSets} sets, ${totalReps} reps, ${(totalWeightKg).toFixed(0)}kg`);
        }

        profilesCreated++;
    }

    console.log('\n📊 Backfill Summary');
    console.log('====================');
    console.log(`Profiles created: ${profilesCreated}`);
    console.log(`Profiles skipped: ${profilesSkipped}`);

    if (dryRun) {
        console.log('\n⚠️ This was a dry run. Run without --dry-run to apply changes.');
    } else {
        console.log('\n✅ Backfill complete!');
    }
}

main().catch(err => {
    console.error('Backfill failed:', err);
    process.exit(1);
});
