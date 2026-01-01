import { Command } from 'commander';
import { adminDb } from '../firebase';

export function addActivitiesCommands(program: Command) {
  program.command('activities:list-processed <userId>')
    .description('List processed activities for a user')
    .action(async (userId) => {
      try {
        console.log(`Fetching processed activities for user: ${userId} `);
        const snapshot = await adminDb.collection('users').doc(userId).collection('raw_activities')
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
          const date = data.processed_at?.toDate().toISOString() || 'Unknown';
          console.log(`[${data.source}] ${data.external_id} (Processed: ${date})`);
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
        const id = `${source}_${activityId} `;
        const ref = adminDb.collection('users').doc(userId).collection('raw_activities').doc(id);

        const doc = await ref.get();
        if (!doc.exists) {
          console.log(`Processed activity record ${id} not found for user ${userId}`);
          return;
        }

        await ref.delete();
        console.log(`✅ Deleted processed activity record: ${id} `);
      } catch (error) {
        console.error('Failed to delete processed activity:', error);
        process.exit(1);
      }
    });
}
