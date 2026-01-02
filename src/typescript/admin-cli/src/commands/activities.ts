import { Command } from 'commander';
import { adminDb } from '../firebase';

export function addActivitiesCommands(program: Command) {
  program.command('activities:list-processed <userId>')
    .description('List processed activities for a user')
    .action(async (userId) => {
      try {
        console.log(`Fetching processed activities for user: ${userId} `);
        const { storage } = await import('@fitglue/shared');
        const snapshot = await storage.getRawActivitiesCollection(userId)
          .orderBy('processedAt', 'desc') // Note: We use CamelCase property name because of Converter!
          // Converter maps 'processed_at' (DB) -> 'processedAt' (Model).
          // SDK `orderBy` on typed collection usually expects Model keys?
          // NO. `firebase-admin` `orderBy` acts on DB FIELDS even with converter, UNLESS using `Query<T>` specific wrappers which admin SDK does partially.
          // In standard JS SDK, if you use a converter, you often have to use the DB field name for queries.
          // BUT wait, `firestore-data-converter` interface:
          // `toFirestore` maps Model -> DB.
          // Queries filter on DB fields.
          // So we MUST use `processed_at`.
          .orderBy('processed_at', 'desc')
          .limit(20)
          .get();

        if (snapshot.empty) {
          console.log('No processed activities found.');
          return;
        }

        console.log('\nFound ' + snapshot.size + ' activities:');
        console.log('--------------------------------------------------');
        snapshot.forEach(doc => {
          const data = doc.data();
          const date = data.processedAt?.toISOString() || 'Unknown'; // UserRecord types uses Date object
          console.log(`[${data.source}] ${data.externalId} (Processed: ${date})`);
        });
        console.log('--------------------------------------------------\n');

      } catch (error) {
        console.error('Failed to list activities:', error);
        process.exit(1);
      }
    });

  program.command('activities:delete-processed <userId> <source> <activityId>')
    .description('Delete a processed activity record to allow re-ingestion')
    .action(async (userId, source, activityId) => {
      try {
        const { storage } = await import('@fitglue/shared');
        const ref = storage.getRawActivitiesCollection(userId).doc(activityId);
        // Note: activityId CLI arg is short ID? Or the constructed ID?
        // CLI arg says <activityId>.
        // Original code: const id = `${source}_${activityId}`;
        // Ref was ...doc(id).
        // My `getRawActivitiesCollection` wrapper returns collection reference.
        // I need to use the constructed ID.
        const id = `${source}_${activityId}`;
        const refStored = storage.getRawActivitiesCollection(userId).doc(id);

        const doc = await refStored.get();
        if (!doc.exists) {
          console.log(`Processed activity record ${id} not found for user ${userId}`);
          return;
        }

        await refStored.delete();
        console.log(`✅ Deleted processed activity record: ${id} `);
      } catch (error) {
        console.error('Failed to delete processed activity:', error);
        process.exit(1);
      }
    });
}
