import { Command } from 'commander';
import { db } from '@fitglue/shared/dist/framework';
import { ActivityStore, ExecutionStore } from '@fitglue/shared/dist/storage/firestore';
import { ExecutionService } from '@fitglue/shared/dist/domain/services';

const activityStore = new ActivityStore(db);
const executionService = new ExecutionService(new ExecutionStore(db));

export const addSynchronizedCommands = (program: Command) => {
  program.command('synchronized:list')
    .argument('<userId>', 'User ID')
    .option('-l, --limit <number>', 'Limit results', '20')
    .description('List synchronized activities for a user')
    .action(async (userId, options) => {
      try {
        const limit = parseInt(options.limit, 10);
        console.log(`Fetching synchronized activities for user ${userId} (limit: ${limit})...`);

        const activities = await activityStore.listSynchronized(userId, limit);

        if (activities.length === 0) {
          console.log('No synchronized activities found.');
          return;
        }

        console.log(`\nFound ${activities.length} synchronized activities:`);
        console.log('--------------------------------------------------');
        activities.forEach(activity => {
          const dests = activity.destinations ? Object.keys(activity.destinations).join(', ') : 'None';
          console.log(`[${activity.activityId}] ${activity.title}`);
          console.log(`  Type: ${activity.type}, Source: ${activity.source}`);
          console.log(`  Synced: ${activity.syncedAt?.toISOString() || 'Unknown'}`);
          console.log(`  Destinations: ${dests}`);
          console.log('--------------------------------------------------');
        });

      } catch (error: unknown) {
        if (error instanceof Error) {
          console.error(`❌ Error listing synchronized activities: ${error.message}`);
        } else {
          console.error('❌ An unknown error occurred');
        }
        process.exit(1);
      }
    });

  program.command('synchronized:get')
    .argument('<userId>', 'User ID')
    .argument('<activityId>', 'Activity ID')
    .option('-v, --verbose', 'Show full execution trace details')
    .description('Get details of a specific synchronized activity')
    .action(async (userId, activityId, options) => {
      try {
        const activity = await activityStore.getSynchronized(userId, activityId);

        if (!activity) {
          console.error('Synchronized activity not found');
          process.exit(1);
        }

        console.log('\nSynchronized Activity Details:');
        console.log('--------------------------------------------------');
        console.log(`Activity ID: ${activity.activityId}`);
        console.log(`Title: ${activity.title}`);
        console.log(`Description: ${activity.description || '(none)'}`);
        console.log(`Type: ${activity.type}`);
        console.log(`Source: ${activity.source}`);
        console.log(`Start Time: ${activity.startTime?.toISOString() || 'Unknown'}`);
        console.log(`Synced At: ${activity.syncedAt?.toISOString() || 'Unknown'}`);
        console.log(`Pipeline ID: ${activity.pipelineId}`);
        console.log(`Pipeline Execution ID: ${activity.pipelineExecutionId || '(not stored)'}`);

        if (activity.destinations && Object.keys(activity.destinations).length > 0) {
          console.log('Destinations:');
          for (const [dest, extId] of Object.entries(activity.destinations)) {
            console.log(`  ${dest}: ${extId}`);
          }
        }
        console.log('--------------------------------------------------');

        // Fetch execution trace if pipelineExecutionId is present
        if (activity.pipelineExecutionId) {
          console.log('\nPipeline Execution Trace:');
          console.log('--------------------------------------------------');
          try {
            const executions = await executionService.listByPipeline(activity.pipelineExecutionId);
            if (executions.length === 0) {
              console.log('No execution records found for this pipeline.');
            } else {
              executions.forEach(exec => {
                const status = exec.data.status !== undefined ? `STATUS_${exec.data.status}` : 'UNKNOWN';
                const duration = exec.data.startTime && exec.data.endTime
                  ? `${((exec.data.endTime as Date).getTime() - (exec.data.startTime as Date).getTime())}ms`
                  : 'N/A';
                console.log(`[${exec.data.service}] ${status} (${duration})`);
                console.log(`  Execution ID: ${exec.id}`);
                console.log(`  Time: ${exec.data.timestamp?.toISOString() || 'Unknown'}`);
                if (exec.data.errorMessage) {
                  console.log(`  Error: ${exec.data.errorMessage}`);
                }
                if (options.verbose) {
                  if (exec.data.inputsJson) {
                    console.log(`  Inputs: ${exec.data.inputsJson}`);
                  }
                  if (exec.data.outputsJson) {
                    console.log(`  Outputs: ${exec.data.outputsJson}`);
                  }
                }
                console.log('--------------------------------------------------');
              });
            }
          } catch (err) {
            console.error('Failed to fetch execution trace:', err);
          }
        } else {
          console.log('\n(No pipeline execution ID - execution trace unavailable)');
        }

      } catch (error: unknown) {
        if (error instanceof Error) {
          console.error(`❌ Error getting synchronized activity: ${error.message}`);
        } else {
          console.error('❌ An unknown error occurred');
        }
        process.exit(1);
      }
    });
};
