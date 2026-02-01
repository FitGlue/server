#!/usr/bin/env npx ts-node

/**
 * migrate-to-user-subcollections.ts
 *
 * Migrates executions and pending_inputs from root-level collections
 * to user sub-collections:
 *   - executions -> users/{userId}/executions
 *   - pending_inputs -> users/{userId}/pending_inputs
 *
 * Usage:
 *   npx ts-node scripts/migrate-to-user-subcollections.ts [--dry-run] [--batch-size=50]
 *
 * Options:
 *   --dry-run      Preview what would be migrated without making changes
 *   --batch-size   Number of documents to process per batch (default: 50)
 */

import * as admin from 'firebase-admin';

// Initialize Firebase Admin
if (!admin.apps.length) {
    admin.initializeApp();
}

const db = admin.firestore();

interface MigrationStats {
    executions: { total: number; migrated: number; failed: number; skipped: number };
    pendingInputs: { total: number; migrated: number; failed: number; skipped: number };
}

async function migrateExecutions(dryRun: boolean, batchSize: number): Promise<MigrationStats['executions']> {
    const stats = { total: 0, migrated: 0, failed: 0, skipped: 0 };

    console.log('\n📦 Migrating executions...');

    let lastDoc: admin.firestore.QueryDocumentSnapshot | undefined;

    while (true) {
        let query = db.collection('executions')
            .orderBy('__name__')
            .limit(batchSize);

        if (lastDoc) {
            query = query.startAfter(lastDoc);
        }

        const snapshot = await query.get();

        if (snapshot.empty) {
            break;
        }

        stats.total += snapshot.size;
        lastDoc = snapshot.docs[snapshot.docs.length - 1];

        const batch = db.batch();
        let batchOperations = 0;

        for (const doc of snapshot.docs) {
            const data = doc.data();
            const userId = data.user_id;

            if (!userId) {
                console.warn(`  ⚠️ Execution ${doc.id} has no user_id, skipping`);
                stats.skipped++;
                continue;
            }

            const targetRef = db.collection('users').doc(userId).collection('executions').doc(doc.id);

            // Check if already exists in target
            const existingDoc = await targetRef.get();
            if (existingDoc.exists) {
                console.log(`  ⏭️ Execution ${doc.id} already exists in user sub-collection`);
                stats.skipped++;
                continue;
            }

            if (!dryRun) {
                batch.set(targetRef, data);
                batchOperations++;
            }

            console.log(`  ${dryRun ? '[DRY RUN] ' : ''}✓ ${doc.id} → users/${userId}/executions/${doc.id}`);
            stats.migrated++;
        }

        if (!dryRun && batchOperations > 0) {
            await batch.commit();
            console.log(`  📝 Committed batch of ${batchOperations} documents`);
        }

        console.log(`  Progress: ${stats.total} processed, ${stats.migrated} migrated, ${stats.skipped} skipped`);
    }

    return stats;
}

async function migratePendingInputs(dryRun: boolean, batchSize: number): Promise<MigrationStats['pendingInputs']> {
    const stats = { total: 0, migrated: 0, failed: 0, skipped: 0 };

    console.log('\n📦 Migrating pending_inputs...');

    let lastDoc: admin.firestore.QueryDocumentSnapshot | undefined;

    while (true) {
        let query = db.collection('pending_inputs')
            .orderBy('__name__')
            .limit(batchSize);

        if (lastDoc) {
            query = query.startAfter(lastDoc);
        }

        const snapshot = await query.get();

        if (snapshot.empty) {
            break;
        }

        stats.total += snapshot.size;
        lastDoc = snapshot.docs[snapshot.docs.length - 1];

        const batch = db.batch();
        let batchOperations = 0;

        for (const doc of snapshot.docs) {
            const data = doc.data();
            const userId = data.user_id;

            if (!userId) {
                console.warn(`  ⚠️ PendingInput ${doc.id} has no user_id, skipping`);
                stats.skipped++;
                continue;
            }

            const targetRef = db.collection('users').doc(userId).collection('pending_inputs').doc(doc.id);

            // Check if already exists in target
            const existingDoc = await targetRef.get();
            if (existingDoc.exists) {
                console.log(`  ⏭️ PendingInput ${doc.id} already exists in user sub-collection`);
                stats.skipped++;
                continue;
            }

            if (!dryRun) {
                batch.set(targetRef, data);
                batchOperations++;
            }

            console.log(`  ${dryRun ? '[DRY RUN] ' : ''}✓ ${doc.id} → users/${userId}/pending_inputs/${doc.id}`);
            stats.migrated++;
        }

        if (!dryRun && batchOperations > 0) {
            await batch.commit();
            console.log(`  📝 Committed batch of ${batchOperations} documents`);
        }

        console.log(`  Progress: ${stats.total} processed, ${stats.migrated} migrated, ${stats.skipped} skipped`);
    }

    return stats;
}

async function main() {
    const args = process.argv.slice(2);
    const dryRun = args.includes('--dry-run');
    const batchSizeArg = args.find(a => a.startsWith('--batch-size='));
    const batchSize = batchSizeArg ? parseInt(batchSizeArg.split('=')[1], 10) : 50;

    console.log('🚀 User Sub-Collection Migration Script');
    console.log('=======================================');
    console.log(`Mode: ${dryRun ? 'DRY RUN (no changes will be made)' : 'LIVE'}`);
    console.log(`Batch Size: ${batchSize}`);

    const stats: MigrationStats = {
        executions: await migrateExecutions(dryRun, batchSize),
        pendingInputs: await migratePendingInputs(dryRun, batchSize),
    };

    console.log('\n📊 Migration Summary');
    console.log('====================');
    console.log('Executions:');
    console.log(`  Total: ${stats.executions.total}`);
    console.log(`  Migrated: ${stats.executions.migrated}`);
    console.log(`  Skipped: ${stats.executions.skipped}`);
    console.log(`  Failed: ${stats.executions.failed}`);
    console.log('Pending Inputs:');
    console.log(`  Total: ${stats.pendingInputs.total}`);
    console.log(`  Migrated: ${stats.pendingInputs.migrated}`);
    console.log(`  Skipped: ${stats.pendingInputs.skipped}`);
    console.log(`  Failed: ${stats.pendingInputs.failed}`);

    if (dryRun) {
        console.log('\n⚠️ This was a dry run. Run without --dry-run to apply changes.');
    } else {
        console.log('\n✅ Migration complete!');
        console.log('\n📝 Next steps:');
        console.log('1. Verify data in user sub-collections using Firebase Console');
        console.log('2. Once verified, run cleanup script to remove root collection data');
    }
}

main().catch(err => {
    console.error('Migration failed:', err);
    process.exit(1);
});
