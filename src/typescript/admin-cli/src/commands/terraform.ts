import { Command } from 'commander';
import { spawnSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import inquirer from 'inquirer';

export function addTerraformCommands(program: Command) {
  program.command('terraform:unlock')
    .argument('<environment>', 'Environment to unlock (dev, test, prod)')
    .description('Attempt to find and clear a Terraform state lock for a specific environment')
    // eslint-disable-next-line complexity
    .action(async (environment: string) => {
      try {
        if (!['dev', 'test', 'prod'].includes(environment)) {
          console.error('❌ Invalid environment. Must be one of: dev, test, prod');
          process.exit(1);
        }
        // Determine terraform directory. The CLI is expected to be run from the server root.
        const terraformDir = path.join(process.cwd(), 'terraform');

        if (!fs.existsSync(terraformDir)) {
          console.error('Could not find terraform directory at:', terraformDir);
          console.error('Make sure you are running the CLI from the server root.');
          process.exit(1);
        }

        console.log(`Checking for locks in ${terraformDir} for environment: ${environment}...`);

        // 1. Re-initialize with the correct backend config
        const backendFile = path.join(terraformDir, 'envs', `${environment}.backend.tfvars`);

        if (!fs.existsSync(backendFile)) {
          console.error(`❌ Backend config file not found: ${backendFile}`);
          process.exit(1);
        }

        console.log(`Re-initializing Terraform for ${environment}...`);
        const initResult = spawnSync('terraform', ['init', `-backend-config=envs/${environment}.backend.tfvars`, '-reconfigure', '-no-color'], {
          cwd: terraformDir,
          encoding: 'utf-8'
        });

        if (initResult.status !== 0) {
          console.error('❌ Terraform init failed:');
          console.error(initResult.stderr || initResult.stdout);
          process.exit(1);
        }

        // 2. Run terraform plan with a short lock timeout to detect locks
        // terraform state list is a read operation that doesn't acquire locks,
        // so it won't detect if another process holds a write lock.
        // Using plan with -lock-timeout=1s will fail fast if locked.
        console.log('Checking for active locks...');
        const planResult = spawnSync('terraform', [
          'plan',
          '-lock-timeout=1s',
          `-var-file=envs/${environment}.tfvars`,
          '-input=false',
          '-no-color'
        ], {
          cwd: terraformDir,
          encoding: 'utf-8'
        });

        // Check if it failed due to a lock
        const output = (planResult.stderr || planResult.stdout).toString();
        const lockError = output.includes('Error acquiring the state lock') || output.includes('Error locking state');

        if (planResult.status === 0 || (!lockError && planResult.status !== 0)) {
          // Plan succeeded or failed for a non-lock reason
          if (planResult.status === 0) {
            console.log('✅ No state lock detected. Terraform plan succeeded.');
          } else {
            console.log('✅ No state lock detected (plan failed for other reasons, but no lock issue).');
          }
          return;
        }
        const lockIdMatch = output.match(/ID:\s+([0-9]+)/);

        if (lockIdMatch) {
          const lockId = lockIdMatch[1];
          console.log(`⚠️  Detected state lock with ID: ${lockId}`);

          const { confirm } = await inquirer.prompt([{
            type: 'confirm',
            name: 'confirm',
            message: `Do you want to force-unlock ID ${lockId}?`,
            default: false
          }]);

          if (confirm) {
            console.log(`Unlocking ${lockId}...`);
            const unlockResult = spawnSync('terraform', ['force-unlock', '-force', lockId], {
              cwd: terraformDir,
              stdio: 'inherit'
            });

            if (unlockResult.status === 0) {
              console.log('✅ Successfully unlocked.');
            } else {
              console.error('❌ Failed to unlock.');
            }
          } else {
            console.log('Aborted.');
          }
        } else if (output.includes('Error acquiring the state lock')) {
          console.error('❌ Error acquiring state lock, but could not parse Lock ID.');
          console.error(output);
        } else {
          console.error('❌ Terraform plan failed with an unexpected error:');
          console.error(output);
        }

      } catch (error: unknown) {
        console.error('Error:', error);
        process.exit(1);
      }
    });
}
