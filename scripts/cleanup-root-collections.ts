#!/usr/bin/env npx ts-node

/**
 * cleanup-root-collections.ts
 *
 * Removes old data from root-level collections after migration to user sub-collections.
 * ONLY run this after verifying the migration was successful!
 *
 * Usage:
 *   npx ts-node scripts/cleanup-root-collections.ts [--dry-run] [--batch-size=50]
 *
 * Options:
 *   --dry-run      Preview what would be deleted without making changes
 *   --batch-size   Number of documents to process per batch (default: 50)
 */

import * as admin from 'firebase-admin';

// Initialize Firebase Admin
if (!admin.apps.length) {
    admin.initializeApp();
}

const db = admin.firestore();

interface CleanupStats {
    executions: { total: number; deleted: number; skipped: number };
    pendingInputs: { total: number; deleted: number; skipped: number };
}

async function cleanupCollection(
    collectionName: string,
    subCollectionName: string,
    dryRun: boolean,
    batchSize: number
): Promise<{ total: number; deleted: number; skipped: number }> {
    const stats = { total: 0, deleted: 0, skipped: 0 };

    console.log(`\n🗑️ Cleaning up ${collectionName}...`);

    let lastDoc: admin.firestore.QueryDocumentSnapshot | undefined;

    while (true) {
        let query = db.collection(collectionName)
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
                console.warn(`  ⚠️ ${doc.id} has no user_id, skipping`);
                stats.skipped++;
                continue;
            }

            // Verify it exists in user sub-collection before deleting
            const targetRef = db.collection('users').doc(userId).collection(subCollectionName).doc(doc.id);
            const existingDoc = await targetRef.get();

            if (!existingDoc.exists) {
                console.warn(`  ⚠️ ${doc.id} NOT found in user sub-collection, skipping delete`);
                stats.skipped++;
                continue;
            }

            if (!dryRun) {
                batch.delete(doc.ref);
                batchOperations++;
            }

            console.log(`  ${dryRun ? '[DRY RUN] ' : ''}🗑️ Deleted ${collectionName}/${doc.id}`);
            stats.deleted++;
        }

        if (!dryRun && batchOperations > 0) {
            await batch.commit();
            console.log(`  📝 Committed batch of ${batchOperations} deletes`);
        }

        console.log(`  Progress: ${stats.total} processed, ${stats.deleted} deleted, ${stats.skipped} skipped`);
    }

    return stats;
}

async function main() {
    const args = process.argv.slice(2);
    const dryRun = args.includes('--dry-run');
    const batchSizeArg = args.find(a => a.startsWith('--batch-size='));
    const batchSize = batchSizeArg ? parseInt(batchSizeArg.split('=')[1], 10) : 50;

    console.log('🧹 Root Collection Cleanup Script');
    console.log('==================================');
    console.log(`Mode: ${dryRun ? 'DRY RUN (no changes will be made)' : 'LIVE'}`);
    console.log(`Batch Size: ${batchSize}`);

    if (!dryRun) {
        console.log('\n⚠️  WARNING: This will PERMANENTLY DELETE data from root collections!');
        console.log('    Make sure migration was verified before proceeding.');
        console.log('    Press Ctrl+C within 5 seconds to cancel...\n');
        await new Promise(resolve => setTimeout(resolve, 5000));
    }

    const stats: CleanupStats = {
        executions: await cleanupCollection('executions', 'executions', dryRun, batchSize),
        pendingInputs: await cleanupCollection('pending_inputs', 'pending_inputs', dryRun, batchSize),
    };

    console.log('\n📊 Cleanup Summary');
    console.log('==================');
    console.log('Executions:');
    console.log(`  Total: ${stats.executions.total}`);
    console.log(`  Deleted: ${stats.executions.deleted}`);
    console.log(`  Skipped: ${stats.executions.skipped}`);
    console.log('Pending Inputs:');
    console.log(`  Total: ${stats.pendingInputs.total}`);
    console.log(`  Deleted: ${stats.pendingInputs.deleted}`);
    console.log(`  Skipped: ${stats.pendingInputs.skipped}`);

    if (dryRun) {
        console.log('\n⚠️ This was a dry run. Run without --dry-run to apply changes.');
    } else {
        console.log('\n✅ Cleanup complete!');
    }
}

main().catch(err => {
    console.error('Cleanup failed:', err);
    process.exit(1);
});
