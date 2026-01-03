import { Command } from 'commander';
import { ExecutionStore, db } from '@fitglue/shared';
import { execSync } from 'child_process';
import axios from 'axios';
import * as readline from 'readline';
import inquirer from 'inquirer';

const executionStore = new ExecutionStore(db);

// Helper for interactive confirmation
async function confirm(message: string): Promise<boolean> {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  return new Promise((resolve) => {
    rl.question(`${message} (y/n): `, (answer) => {
      rl.close();
      resolve(answer.toLowerCase() === 'y' || answer.toLowerCase() === 'yes');
    });
  });
}

// Map services to webhook URLs by environment
const SERVICE_TO_WEBHOOK: Record<string, Record<string, string>> = {
  'hevy-webhook-handler': {
    'dev': 'https://dev.fitglue.tech/hooks/hevy',
    'test': 'https://test.fitglue.tech/hooks/hevy',
    'prod': 'https://fitglue.tech/hooks/hevy',
  },
  'fitbit-handler': {
    'dev': 'https://dev.fitglue.tech/hooks/fitbit',
    'test': 'https://test.fitglue.tech/hooks/fitbit',
    'prod': 'https://fitglue.tech/hooks/fitbit',
  },
};

// Auto-discover subscription name for a service
function getSubscriptionName(service: string): string {
  try {
    const output = execSync(`gcloud pubsub subscriptions list --format="value(name)" --filter="name~${service}"`, {
      encoding: 'utf-8',
    });
    const subscriptions = output.trim().split('\n').filter(Boolean);
    if (subscriptions.length > 0) {
      // Return just the subscription name (not the full path)
      const fullPath = subscriptions[0];
      return fullPath.split('/').pop() || fullPath;
    }
  } catch (error) {
    // Fall through
  }

  throw new Error(`Could not find subscription for service: ${service}`);
}

export function registerReplayCommands(program: Command) {
  // Pub/Sub replay
  program
    .command('replay:pubsub <execution-id>')
    .description('Replay a Pub/Sub execution by seeking to its timestamp')
    .option('--yes', 'Skip confirmation prompt')
    .action(async (executionId: string, options: { yes?: boolean }) => {
      const execution = await executionStore.get(executionId);
      if (!execution) {
        console.error(`❌ Execution ${executionId} not found`);
        process.exit(1);
      }

      // Get subscription name
      let subscription: string;
      try {
        subscription = getSubscriptionName(execution.service);
      } catch (error: any) {
        console.error(`❌ ${error.message}`);
        process.exit(1);
      }

      // Seek to 60 seconds before execution to account for cold starts/queueing
      const seekTime = new Date(execution.timestamp!);
      seekTime.setSeconds(seekTime.getSeconds() - 60);

      console.log(`\n📋 Replay Details:`);
      console.log(`   Execution ID: ${executionId}`);
      console.log(`   Service: ${execution.service}`);
      console.log(`   Subscription: ${subscription}`);
      console.log(`   Original time: ${execution.timestamp}`);
      console.log(`   Seek time: ${seekTime.toISOString()}`);
      console.log(`   Status: ${execution.status}`);

      if (!options.yes) {
        const proceed = await confirm('\n🔄 Proceed with replay?');
        if (!proceed) {
          console.log('Cancelled.');
          return;
        }
      }

      try {
        const cmd = `gcloud pubsub subscriptions seek ${subscription} --time=${seekTime.toISOString()}`;
        console.log(`\nExecuting: ${cmd}`);
        execSync(cmd, { stdio: 'inherit' });
        console.log('\n✅ Replay initiated. Check logs for new execution.');
      } catch (error: any) {
        console.error('❌ Failed to replay:', error.message);
        process.exit(1);
      }
    });

  // HTTP webhook replay
  program
    .command('replay:webhook <execution-id>')
    .description('Replay an HTTP webhook execution')
    .option('--env <env>', 'Target environment (dev/test/prod)', 'dev')
    .option('--yes', 'Skip confirmation prompt')
    .action(async (executionId: string, options: { env: string; yes?: boolean }) => {
      const execution = await executionStore.get(executionId);
      if (!execution) {
        console.error(`❌ Execution ${executionId} not found`);
        process.exit(1);
      }

      const webhookUrls = SERVICE_TO_WEBHOOK[execution.service];
      if (!webhookUrls) {
        console.error(`❌ No webhook URL for service: ${execution.service}`);
        console.log('Available services:', Object.keys(SERVICE_TO_WEBHOOK));
        process.exit(1);
      }

      const url = webhookUrls[options.env];
      if (!url) {
        console.error(`❌ No URL for env: ${options.env}`);
        console.log('Available envs:', Object.keys(webhookUrls));
        process.exit(1);
      }

      const payload = execution.inputsJson ? JSON.parse(execution.inputsJson) : {};

      console.log(`\n📋 Replay Details:`);
      console.log(`   Execution ID: ${executionId}`);
      console.log(`   Service: ${execution.service}`);
      console.log(`   Environment: ${options.env}`);
      console.log(`   URL: ${url}`);
      console.log(`   Status: ${execution.status}`);
      console.log(`   Payload preview: ${JSON.stringify(payload).substring(0, 100)}...`);

      const headers: Record<string, string> = { 'Content-Type': 'application/json' };

      if (!options.yes) {
        console.log('\n--- Custom Headers ---');
        let addingHeaders = true;
        while (addingHeaders) {
          const { add } = await inquirer.prompt([{
            type: 'confirm',
            name: 'add',
            message: 'Add a custom header?',
            default: false
          }]);

          if (!add) {
            addingHeaders = false;
            break;
          }

          const input = await inquirer.prompt([
            { type: 'input', name: 'key', message: 'Header Name:' },
            { type: 'input', name: 'value', message: 'Header Value:' }
          ]);

          if (input.key && input.value) {
            headers[input.key] = input.value;
            console.log(`Added header: ${input.key} = ${input.value}`);
          }
        }

        const proceed = await confirm('\n🔄 Proceed with replay?');
        if (!proceed) {
          console.log('Cancelled.');
          return;
        }
      }

      try {
        console.log('\nSending request...');
        const response = await axios.post(url, payload, {
          headers: headers,
          validateStatus: () => true,
        });

        if (response.status >= 200 && response.status < 300) {
          console.log(`✅ Success: ${response.status} ${response.statusText}`);
        } else {
          console.log(`⚠️  Response: ${response.status} ${response.statusText}`);
          console.log('Response data:', response.data);
        }
      } catch (error: any) {
        console.error('❌ Failed:', error.message);
        process.exit(1);
      }
    });
}
