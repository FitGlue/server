#!/usr/bin/env npx ts-node

/**
 * backfill-plugin-defaults.ts
 *
 * One-off script to populate user-level plugin defaults from existing
 * pipeline configurations. For each user's pipelines, extracts source
 * and destination configs and writes them as plugin defaults using
 * setIfNotExists semantics (first config wins).
 *
 * Usage:
 *   npx ts-node scripts/backfill-plugin-defaults.ts --env=dev [--dry-run]
 *   npx ts-node scripts/backfill-plugin-defaults.ts --env=prod --dry-run
 *
 * Options:
 *   --env=dev|test|prod   (required) Target environment
 *   --dry-run             Preview changes without writing
 */

import * as admin from 'firebase-admin';

const ENVIRONMENTS: Record<string, string> = {
    dev: 'fitglue-server-dev',
    test: 'fitglue-server-test',
    prod: 'fitglue-server-prod',
};

/**
 * Map proto source enum names/values to registry IDs.
 * Must stay in sync with the registry.
 */
const SOURCE_TO_REGISTRY: Record<string, string> = {
    'SOURCE_STRAVA': 'strava',
    'SOURCE_FITBIT': 'fitbit',
    'SOURCE_HEVY': 'hevy',
    'SOURCE_INTERVALS': 'intervals',
    'SOURCE_TRAININGPEAKS': 'trainingpeaks',
    'SOURCE_FIT_FILE': 'fit-file',
    '1': 'strava',
    '2': 'fitbit',
    '3': 'hevy',
    '4': 'intervals',
    '5': 'trainingpeaks',
    '6': 'fit-file',
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

function getSourceRegistryId(source: string | number): string | null {
    const key = String(source).toUpperCase();
    return SOURCE_TO_REGISTRY[key] || SOURCE_TO_REGISTRY[String(source)] || null;
}

const scriptArgs = process.argv.slice(2);
const projectId = resolveEnv(scriptArgs);

if (!admin.apps.length) {
    admin.initializeApp({ projectId });
}

const db = admin.firestore();

async function main() {
    const dryRun = scriptArgs.includes('--dry-run');

    console.log('📋 Plugin Defaults Backfill Script');
    console.log('===================================');
    console.log(`Project: ${projectId}`);
    console.log(`Mode: ${dryRun ? 'DRY RUN' : 'LIVE'}`);

    // Step 1: Read all users
    console.log('\n📥 Reading users...');
    const usersSnapshot = await db.collection('users').get();
    console.log(`  Found ${usersSnapshot.size} users`);

    let usersProcessed = 0;
    let defaultsCreated = 0;
    let defaultsSkipped = 0;
    let usersWithNoPipelines = 0;

    for (const userDoc of usersSnapshot.docs) {
        const userId = userDoc.id;

        // Step 2: Read user's pipelines
        const pipelinesSnapshot = await db
            .collection('users')
            .doc(userId)
            .collection('pipelines')
            .get();

        if (pipelinesSnapshot.empty) {
            usersWithNoPipelines++;
            continue;
        }

        usersProcessed++;
        const displayName = userDoc.data()?.display_name || userId;
        console.log(`\n👤 ${displayName} — ${pipelinesSnapshot.size} pipeline(s)`);

        // Track which plugins we've already set defaults for (first wins)
        const seen = new Set<string>();

        for (const pipelineDoc of pipelinesSnapshot.docs) {
            const pipeline = pipelineDoc.data();

            // Extract source config
            const sourceConfig = pipeline.source_config || pipeline.sourceConfig || {};
            if (Object.keys(sourceConfig).length > 0) {
                const sourceId = getSourceRegistryId(pipeline.source);
                if (sourceId && !seen.has(sourceId)) {
                    seen.add(sourceId);
                    if (!dryRun) {
                        // Check if default already exists
                        const existingDoc = await db
                            .collection('users').doc(userId)
                            .collection('plugin_defaults').doc(sourceId)
                            .get();

                        if (!existingDoc.exists) {
                            await db
                                .collection('users').doc(userId)
                                .collection('plugin_defaults').doc(sourceId)
                                .set({
                                    plugin_id: sourceId,
                                    config: sourceConfig,
                                    created_at: admin.firestore.FieldValue.serverTimestamp(),
                                    updated_at: admin.firestore.FieldValue.serverTimestamp(),
                                });
                            console.log(`    ✅ Source default: ${sourceId}`);
                            defaultsCreated++;
                        } else {
                            console.log(`    ⏭️  Source default already exists: ${sourceId}`);
                            defaultsSkipped++;
                        }
                    } else {
                        console.log(`    [DRY RUN] Would create source default: ${sourceId} → ${JSON.stringify(sourceConfig)}`);
                        defaultsCreated++;
                    }
                }
            }

            // Extract destination configs
            const destConfigs = pipeline.destination_configs || pipeline.destinationConfigs || {};
            for (const [destId, destCfg] of Object.entries(destConfigs)) {
                const config = (destCfg as any)?.config || {};
                if (Object.keys(config).length === 0) continue;
                if (seen.has(destId)) continue;
                seen.add(destId);

                if (!dryRun) {
                    const existingDoc = await db
                        .collection('users').doc(userId)
                        .collection('plugin_defaults').doc(destId)
                        .get();

                    if (!existingDoc.exists) {
                        await db
                            .collection('users').doc(userId)
                            .collection('plugin_defaults').doc(destId)
                            .set({
                                plugin_id: destId,
                                config,
                                created_at: admin.firestore.FieldValue.serverTimestamp(),
                                updated_at: admin.firestore.FieldValue.serverTimestamp(),
                            });
                        console.log(`    ✅ Destination default: ${destId}`);
                        defaultsCreated++;
                    } else {
                        console.log(`    ⏭️  Destination default already exists: ${destId}`);
                        defaultsSkipped++;
                    }
                } else {
                    console.log(`    [DRY RUN] Would create destination default: ${destId} → ${JSON.stringify(config)}`);
                    defaultsCreated++;
                }
            }
        }
    }

    console.log('\n📊 Backfill Summary');
    console.log('====================');
    console.log(`Users processed: ${usersProcessed}`);
    console.log(`Users with no pipelines: ${usersWithNoPipelines}`);
    console.log(`Defaults created: ${defaultsCreated}`);
    console.log(`Defaults skipped (already exist): ${defaultsSkipped}`);

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
