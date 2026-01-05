import { Command } from 'commander';
import { db, InputStore, InputService } from '@fitglue/shared';
import { PubSub } from '@google-cloud/pubsub';
import inquirer from 'inquirer';

const pubsub = new PubSub();
const TOPIC = process.env.PUBSUB_TOPIC || 'activity-updates';

const store = new InputStore(db);
const service = new InputService(store);

export const addInputsCommands = (program: Command) => {
  program.command('inputs:list')
    .argument('<userId>', 'User ID to list pending inputs for')
    .description('List pending user inputs')
    .action(async (userId) => {
      try {
        console.log(`Fetching pending inputs for user ${userId}...`);
        const inputs = await service.listPendingInputs(userId);

        if (inputs.length === 0) {
          console.log('No pending inputs found.');
          return;
        }

        console.log('\nFound ' + inputs.length + ' pending inputs:');
        console.log('--------------------------------------------------');
        inputs.forEach(input => {
          console.log(`Activity ID: ${input.activityId}`);
          console.log(`  Required Fields: ${input.requiredFields?.join(', ') || 'None'}`);
          console.log(`  Created At: ${input.createdAt?.toISOString() || 'Unknown'}`);
          console.log('--------------------------------------------------');
        });

      } catch (error: unknown) {
        if (error instanceof Error) {
          console.error(`❌ Error listing inputs: ${error.message}`);
          console.error('Stack:', error.stack);
        } else {
          console.error(`❌ An unknown error occurred`);
        }
        process.exit(1);
      }
    });

  program.command('inputs:get')
    .argument('<activityId>', 'Activity ID (Pending Input ID)')
    .description('Get details of a specific pending input')
    .action(async (activityId) => {
      try {
        const input = await service.getPendingInput(activityId);
        if (!input) {
          console.error('Pending input not found');
          process.exit(1);
        }

        console.log('\nPending Input Details:');
        console.log('--------------------------------------------------');
        console.log(`Activity ID: ${input.activityId}`);
        console.log(`User ID: ${input.userId}`);
        console.log(`Status: ${input.status} (1=WAITING, 2=COMPLETED)`);
        console.log(`Required Fields: ${input.requiredFields?.join(', ') || 'None'}`);
        console.log(`Created At: ${input.createdAt?.toISOString() || 'Unknown'}`);
        if (input.inputData) {
          console.log(`Input Data: ${JSON.stringify(input.inputData, null, 2)}`);
        }
        console.log(`Original Payload: ${input.originalPayload ? '(Present)' : '(Missing)'}`);
        console.log('--------------------------------------------------\n');

      } catch (error: unknown) {
        if (error instanceof Error) {
          console.error(`❌ Error getting input: ${error.message}`);
          console.error('Stack:', error.stack);
        } else {
          console.error(`❌ An unknown error occurred`);
        }
        process.exit(1);
      }
    });

  program.command('inputs:resolve')
    .argument('<activityId>', 'Activity ID')
    .option('--data <json>', 'JSON string of input data (e.g. \'{"title":"Run"}\')')
    .description('Resolve a pending input and resume the pipeline')
    .action(async (activityId, options) => {
      try {
        let inputData: Record<string, string>;
        const input = await service.getPendingInput(activityId);

        if (!input) {
          console.error('Pending input not found');
          process.exit(1);
        }

        if (options.data) {
          try {
            inputData = JSON.parse(options.data);
          } catch (e) {
            console.error('Invalid JSON data provided');
            process.exit(1);
          }
        } else {
          const fields = input.requiredFields || ['description'];
          const questions = fields.map((f: string) => ({
            type: 'input',
            name: f,
            message: `Enter value for '${f}':`
          }));

          inputData = await inquirer.prompt(questions);
        }

        console.log(`Resolving input for ${activityId}...`);

        // Update via Service (validates status)
        // Pass userId from input because CLI is admin (impersonating user)
        await service.resolveInput(activityId, input.userId, inputData);

        // Publish to PubSub (This logic stays in CLI/Handler, service just updates DB state?)
        // The implementation plan says "API Handler ... re-publishes".
        // Ideally the Service handles re-publishing if it has PubSub access?
        // But InputService currently only has InputStore.
        // Let's keep pubsub usage here for now, but retrieve payload from input object.

        // Re-fetch to be sure? We already fetched 'input'.
        if (!input.originalPayload) {
          console.error('Original payload missing, cannot resume');
          process.exit(1);
        }

        // originalPayload type in TS is generic/duck-typed in converter
        // In proto it is bytes or object. Converter says "Pass through, might be buffer/bytes"
        const payloadStr = typeof input.originalPayload === 'string' ? input.originalPayload : JSON.stringify(input.originalPayload);
        const dataBuffer = Buffer.from(payloadStr);
        await pubsub.topic(TOPIC).publishMessage({ data: dataBuffer });

        console.log('✅ Input resolved and activity re-published successfully.');

      } catch (error: unknown) {
        if (error instanceof Error) {
          console.error(`❌ Error resolving input: ${error.message}`);
          console.error('Stack:', error.stack);
        } else {
          console.error(`❌ An unknown error occurred`);
        }
        process.exit(1);
      }
    });
};
