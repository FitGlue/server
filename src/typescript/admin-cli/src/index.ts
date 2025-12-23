import * as admin from 'firebase-admin';
import { Command } from 'commander';
import inquirer from 'inquirer';
import { UserService } from '@fitglue/shared/dist/services/user_service';

// Initialize Firebase
if (admin.apps.length === 0) {
    admin.initializeApp({
        credential: admin.credential.applicationDefault()
    });
}
const db = admin.firestore();
const userService = new UserService(db);

const program = new Command();

program
    .name('fitglue-admin')
    .description('CLI for FitGlue administration')
    .version('1.0.0');

program.command('users:create')
    .argument('<userId>', 'User ID to create')
    .description('Create a new user and generating an Ingress API Key')
    .action(async (userId) => {
        try {
            console.log(`Creating user ${userId}...`);
            await userService.createUser(userId);
            console.log(`User ${userId} created/ensured.`);

            const answers = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'createIngressKey',
                    message: 'Generate an Ingress API Key?',
                    default: true
                },
                {
                    type: 'input',
                    name: 'label',
                    message: 'Key Label:',
                    default: 'Default Key',
                    when: (answers) => answers.createIngressKey
                },
                {
                    type: 'checkbox',
                    name: 'scopes',
                    message: 'Select Scopes:',
                    choices: ['write:activity', 'read:activity', 'test:mock_fetch'],
                    default: ['write:activity'],
                    when: (answers) => answers.createIngressKey
                }
            ]);

            if (answers.createIngressKey) {
                const key = await userService.createIngressApiKey(userId, answers.label, answers.scopes);
                console.log('\n==========================================');
                console.log(`INGRESS API KEY (${answers.label}):`);
                console.log(key);
                console.log('==========================================\n');
            }

            const hevyAnswers = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'configureHevy',
                    message: 'Configure Hevy Integration?',
                    default: false
                },
                {
                    type: 'password',
                    name: 'apiKey',
                    message: 'Hevy API Key:',
                    when: (answers) => answers.configureHevy
                }
            ]);

            if (hevyAnswers.configureHevy) {
                await userService.setHevyIntegration(userId, hevyAnswers.apiKey);
                console.log('Hevy integration configured.');
            }

        } catch (error) {
            console.error('Error creating user:', error);
            process.exit(1);
        }
    });

program.command('users:update')
    .argument('<userId>', 'User ID to update')
    .description('Update an existing user configuration')
    .action(async (userId) => {
        try {
            const hevyAnswers = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'updateHevy',
                    message: 'Update Hevy Integration?',
                    default: true
                },
                {
                    type: 'password',
                    name: 'apiKey',
                    message: 'New Hevy API Key:',
                    when: (answers) => answers.updateHevy
                }
            ]);

            if (hevyAnswers.updateHevy) {
                await userService.setHevyIntegration(userId, hevyAnswers.apiKey);
                console.log('Hevy integration updated.');
            }
        } catch (error) {
            console.error('Error updating user:', error);
            process.exit(1);
        }
    });

program.parse();
