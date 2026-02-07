import { Command } from 'commander';
import { db } from '@fitglue/shared/framework';
import { ExecutionStore } from '@fitglue/shared/storage';
import { ExecutionService } from '@fitglue/shared/domain/services';
import { PipelineRun } from '@fitglue/shared/types';

const executionService = new ExecutionService(new ExecutionStore(db));

/**
 * Helper to get pipeline_runs collection for a user.
 * This replaces the old synchronized_activities pattern.
 */
function pipelineRunsCollection(userId: string) {
  return db.collection('users').doc(userId).collection('pipeline_runs');
}

export const addSynchronizedCommands = (program: Command) => {
  program.command('synchronized:list')
    .argument('<userId>', 'User ID')
    .option('-l, --limit <number>', 'Limit results', '20')
    .description('List pipeline runs (synchronized activities) for a user')
    .action(async (userId, options) => {
      try {
        const limit = parseInt(options.limit, 10);
        console.log(`Fetching pipeline runs for user ${userId} (limit: ${limit})...`);

        const snapshot = await pipelineRunsCollection(userId)
          .orderBy('createdAt', 'desc')
          .limit(limit)
          .get();

        if (snapshot.empty) {
          console.log('No pipeline runs found.');
          return;
        }

        console.log(`\nFound ${snapshot.size} pipeline runs:`);
        console.log('--------------------------------------------------');
        snapshot.docs.forEach(doc => {
          const run = doc.data() as PipelineRun;
          const dests = run.destinations?.map(d => `${d.destination}(${d.status})`).join(', ') || 'None';
          console.log(`[${run.activityId}] ${run.title}`);
          console.log(`  Type: ${run.type}, Source: ${run.source}`);
          console.log(`  Status: ${run.status}`);
          console.log(`  Created: ${run.createdAt instanceof Date ? run.createdAt.toISOString() : run.createdAt || 'Unknown'}`);
          console.log(`  Destinations: ${dests}`);
          console.log('--------------------------------------------------');
        });

      } catch (error: unknown) {
        if (error instanceof Error) {
          console.error(`❌ Error listing pipeline runs: ${error.message}`);
        } else {
          console.error('❌ An unknown error occurred');
        }
        process.exit(1);
      }
    });

  program.command('synchronized:get')
    .argument('<userId>', 'User ID')
    .argument('<runId>', 'Pipeline Run ID')
    .option('-v, --verbose', 'Show full execution trace details')
    .description('Get details of a specific pipeline run')
    .action(async (userId, runId, options) => {
      try {
        const doc = await pipelineRunsCollection(userId).doc(runId).get();

        if (!doc.exists) {
          console.error('Pipeline run not found');
          process.exit(1);
        }

        const run = doc.data() as PipelineRun;

        console.log('\nPipeline Run Details:');
        console.log('--------------------------------------------------');
        console.log(`Run ID: ${run.id}`);
        console.log(`Activity ID: ${run.activityId}`);
        console.log(`Title: ${run.title}`);
        console.log(`Description: ${run.description || '(none)'}`);
        console.log(`Type: ${run.type}`);
        console.log(`Source: ${run.source}`);
        console.log(`Start Time: ${run.startTime instanceof Date ? run.startTime.toISOString() : run.startTime || 'Unknown'}`);
        console.log(`Created At: ${run.createdAt instanceof Date ? run.createdAt.toISOString() : run.createdAt || 'Unknown'}`);
        console.log(`Status: ${run.status}`);
        console.log(`Pipeline ID: ${run.pipelineId}`);

        if (run.destinations && run.destinations.length > 0) {
          console.log('Destinations:');
          for (const dest of run.destinations) {
            console.log(`  ${dest.destination}: ${dest.status}${dest.externalId ? ` (${dest.externalId})` : ''}`);
          }
        }

        if (run.boosters && run.boosters.length > 0) {
          console.log('Boosters:');
          for (const booster of run.boosters) {
            console.log(`  ${booster.providerName}: ${booster.status} (${booster.durationMs}ms)`);
            if (booster.error) {
              console.log(`    Error: ${booster.error}`);
            }
          }
        }
        console.log('--------------------------------------------------');

        // Fetch execution trace using the run ID as pipelineExecutionId
        console.log('\nPipeline Execution Trace:');
        console.log('--------------------------------------------------');
        try {
          const executions = await executionService.listByPipeline(run.id);
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

      } catch (error: unknown) {
        if (error instanceof Error) {
          console.error(`❌ Error getting pipeline run: ${error.message}`);
        } else {
          console.error('❌ An unknown error occurred');
        }
        process.exit(1);
      }
    });
};
