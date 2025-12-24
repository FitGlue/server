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

import { randomUUID } from 'crypto';

program.command('users:create')
    .argument('[userId]', 'User ID to create (optional, will generate UUID if omitted)')
    .description('Create a new user and generating an Ingress API Key')
    .action(async (userId) => {
        try {
            if (!userId) {
                userId = randomUUID();
                console.log(`Generated User ID: ${userId}`);
            }

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
                    choices: ['read:activity'],
                    default: ['read:activity'],
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

            const stravaAnswers = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'configureStrava',
                    message: 'Configure Strava Integration?',
                    default: false
                },
                {
                    type: 'input',
                    name: 'accessToken',
                    message: 'Access Token:',
                    when: (answers) => answers.configureStrava
                },
                {
                    type: 'input',
                    name: 'refreshToken',
                    message: 'Refresh Token:',
                    when: (answers) => answers.configureStrava
                },
                {
                    type: 'input',
                    name: 'expiresAt',
                    message: 'Expires At (Unix Timestamp Seconds):',
                    when: (answers) => answers.configureStrava,
                    validate: (input) => !isNaN(parseInt(input)) || 'Must be a number'
                },
                {
                    type: 'input',
                    name: 'athleteId',
                    message: 'Athlete ID:',
                    when: (answers) => answers.configureStrava,
                    validate: (input) => !isNaN(parseInt(input)) || 'Must be a number'
                }
            ]);

            if (stravaAnswers.configureStrava) {
                await userService.setStravaIntegration(
                    userId,
                    stravaAnswers.accessToken,
                    stravaAnswers.refreshToken,
                    parseInt(stravaAnswers.expiresAt, 10),
                    parseInt(stravaAnswers.athleteId, 10)
                );
                console.log('Strava integration configured.');
            }

            const fitbitAnswers = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'configureFitbit',
                    message: 'Configure Fitbit Integration?',
                    default: false
                },
                {
                    type: 'input',
                    name: 'accessToken',
                    message: 'Access Token:',
                    when: (answers) => answers.configureFitbit
                },
                {
                    type: 'input',
                    name: 'refreshToken',
                    message: 'Refresh Token:',
                    when: (answers) => answers.configureFitbit
                },
                {
                    type: 'input',
                    name: 'expiresAt',
                    message: 'Expires At (Unix Timestamp Seconds):',
                    when: (answers) => answers.configureFitbit,
                    validate: (input) => !isNaN(parseInt(input)) || 'Must be a number'
                },
                {
                    type: 'input',
                    name: 'fitbitUserId',
                    message: 'Fitbit User ID:',
                    when: (answers) => answers.configureFitbit
                }
            ]);

            if (fitbitAnswers.configureFitbit) {
                await userService.setFitbitIntegration(
                    userId,
                    fitbitAnswers.accessToken,
                    fitbitAnswers.refreshToken,
                    parseInt(fitbitAnswers.expiresAt, 10),
                    fitbitAnswers.fitbitUserId
                );
                console.log('Fitbit integration configured.');
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

            const stravaAnswers = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'updateStrava',
                    message: 'Update Strava Integration?',
                    default: false
                },
                {
                    type: 'input',
                    name: 'accessToken',
                    message: 'Access Token:',
                    when: (answers) => answers.updateStrava
                },
                {
                    type: 'input',
                    name: 'refreshToken',
                    message: 'Refresh Token:',
                    when: (answers) => answers.updateStrava
                },
                {
                    type: 'input',
                    name: 'expiresAt',
                    message: 'Expires At (Unix Timestamp Seconds):',
                    when: (answers) => answers.updateStrava,
                    validate: (input) => !isNaN(parseInt(input)) || 'Must be a number'
                },
                {
                    type: 'input',
                    name: 'athleteId',
                    message: 'Athlete ID:',
                    when: (answers) => answers.updateStrava,
                    validate: (input) => !isNaN(parseInt(input)) || 'Must be a number'
                }
            ]);

            if (stravaAnswers.updateStrava) {
                await userService.setStravaIntegration(
                    userId,
                    stravaAnswers.accessToken,
                    stravaAnswers.refreshToken,
                    parseInt(stravaAnswers.expiresAt, 10),
                    parseInt(stravaAnswers.athleteId, 10)
                );
                console.log('Strava integration updated.');
            }

            const fitbitAnswers = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'updateFitbit',
                    message: 'Update Fitbit Integration?',
                    default: false
                },
                {
                    type: 'input',
                    name: 'accessToken',
                    message: 'Access Token:',
                    when: (answers) => answers.updateFitbit
                },
                {
                    type: 'input',
                    name: 'refreshToken',
                    message: 'Refresh Token:',
                    when: (answers) => answers.updateFitbit
                },
                {
                    type: 'input',
                    name: 'expiresAt',
                    message: 'Expires At (Unix Timestamp Seconds):',
                    when: (answers) => answers.updateFitbit,
                    validate: (input) => !isNaN(parseInt(input)) || 'Must be a number'
                },
                {
                    type: 'input',
                    name: 'fitbitUserId',
                    message: 'Fitbit User ID:',
                    when: (answers) => answers.updateFitbit
                }
            ]);

            if (fitbitAnswers.updateFitbit) {
                await userService.setFitbitIntegration(
                    userId,
                    fitbitAnswers.accessToken,
                    fitbitAnswers.refreshToken,
                    parseInt(fitbitAnswers.expiresAt, 10),
                    fitbitAnswers.fitbitUserId
                );
                console.log('Fitbit integration updated.');
            }
        } catch (error) {
            console.error('Error updating user:', error);
            process.exit(1);
        }
    });

program.command('users:delete')
    .argument('<userId>', 'User ID to delete')
    .description('Delete a user and their associated data')
    .action(async (userId) => {
        try {
            const confirm = await inquirer.prompt([{
                type: 'confirm',
                name: 'delete',
                message: `Are you sure you want to PERMANENTLY delete user ${userId}?`,
                default: false
            }]);

            if (!confirm.delete) {
                console.log('Aborted.');
                return;
            }

            await db.collection('users').doc(userId).delete();
            // Note: In a real app we might want to recursively delete subcollections or related data
            console.log(`User ${userId} deleted.`);
        } catch (error) {
            console.error('Error deleting user:', error);
            process.exit(1);
        }
    });

program.command('users:list')
    .description('List all users in the system')
    .action(async () => {
        try {
            console.log('Fetching users...');
            const snapshot = await db.collection('users').get();
            if (snapshot.empty) {
                console.log('No users found.');
                return;
            }

            console.log('\nFound ' + snapshot.size + ' users:');
            console.log('--------------------------------------------------');
            snapshot.forEach(doc => {
                const data = doc.data();
                const integrations = [];
                if (data.integrations?.hevy?.apiKey) integrations.push('Hevy');
                if (data.integrations?.keiser?.enabled) integrations.push('Keiser');

                console.log(`ID: ${doc.id}`);
                console.log(`   Created: ${data.createdAt?.toDate?.()?.toISOString() || 'Unknown'}`);
                console.log(`   Integrations: ${integrations.join(', ') || 'None'}`);
                console.log('--------------------------------------------------');
            });
            console.log('');
        } catch (error) {
            console.error('Error listing users:', error);
            process.exit(1);
        }
    });

program.command('users:clean')
    .description('Delete ALL users from the system')
    .action(async () => {
        try {
            const confirm = await inquirer.prompt([{
                type: 'confirm',
                name: 'delete',
                message: 'Are you sure you want to PERMANENTLY DELETE ALL USERS? This cannot be undone.',
                default: false
            }]);

            if (!confirm.delete) {
                console.log('Aborted.');
                return;
            }

            // Double confirmation for safety
            const doubleConfirm = await inquirer.prompt([{
                type: 'input',
                name: 'confirmation',
                message: 'Type "DELETE ALL" to confirm:',
            }]);

            if (doubleConfirm.confirmation !== 'DELETE ALL') {
                console.log('Confirmation failed. Aborted.');
                return;
            }

            console.log('Fetching users to delete...');
            const snapshot = await db.collection('users').get();

            if (snapshot.empty) {
                console.log('No users to delete.');
                return;
            }

            console.log(`Deleting ${snapshot.size} users...`);
            const batch = db.batch();
            snapshot.docs.forEach(doc => {
                batch.delete(doc.ref);
            });

            await batch.commit();
            console.log('All users deleted.');

        } catch (error) {
            console.error('Error cleaning users:', error);
            process.exit(1);
        }
    });

// --- Execution Inspection Commands ---

program
    .command('executions:list')
    .description('List recent executions')
    .option('-s, --service <service>', 'Filter by service name')
    .option('-st, --status <status>', 'Filter by status (STATUS_STARTED, STATUS_SUCCESS, STATUS_FAILED)')
    .option('-u, --user <userId>', 'Filter by user ID')
    .option('-l, --limit <number>', 'Number of records to show', '20')
    .action(async (options) => {
        try {
            let query: admin.firestore.Query = db.collection('executions').orderBy('timestamp', 'desc');

            if (options.service) {
                query = query.where('service', '==', options.service);
            }
            if (options.status) {
                query = query.where('status', '==', options.status);
            }
            if (options.user) {
                query = query.where('user_id', '==', options.user);
            }

            const limit = parseInt(options.limit, 10);
            query = query.limit(limit);

            console.log(`Fetching up to ${limit} executions...`);
            const snapshot = await query.get();

            if (snapshot.empty) {
                console.log('No executions found matching criteria.');
                return;
            }

            console.log('\nFound ' + snapshot.size + ' executions:');
            console.log('--------------------------------------------------');
            snapshot.forEach(doc => {
                const data = doc.data();
                const time = data.timestamp ? (data.timestamp as admin.firestore.Timestamp).toDate().toISOString() : 'N/A';
                const status = data.status || 'UNKNOWN';
                const service = data.service || 'unknown';
                const trigger = data.trigger_type || 'N/A';

                console.log(`${time} | ${doc.id} | ${service} | ${status} | ${trigger}`);
            });
            console.log('--------------------------------------------------\n');

        } catch (error: any) {
            console.error('Error listing executions:', error.message);
            process.exit(1);
        }
    });

program
    .command('executions:get <executionId>')
    .description('Get full details of a specific execution')
    .action(async (executionId) => {
        try {
            const doc = await db.collection('executions').doc(executionId).get();
            if (!doc.exists) {
                console.error(`Execution ${executionId} not found.`);
                process.exit(1);
            }

            const data = doc.data()!;
            console.log('Execution Details:');
            console.log(`ID: ${doc.id}`);
            console.log(`Service: ${data.service}`);
            console.log(`Status: ${data.status}`);
            console.log(`Timestamp: ${data.timestamp ? (data.timestamp as admin.firestore.Timestamp).toDate().toISOString() : 'N/A'}`);
            console.log(`User ID: ${data.user_id || 'N/A'}`);
            console.log(`Trigger Type: ${data.trigger_type || 'N/A'}`);

            if (data.errorMessage) {
                console.log(`Error: ${data.errorMessage}`);
            }

            if (data.inputs) {
                console.log('Inputs:');
                try {
                    // Try to pretty print JSON string
                    console.log(JSON.stringify(JSON.parse(data.inputs), null, 2));
                } catch {
                    console.log(data.inputs);
                }
            }

            if (data.outputsJson) {
                console.log('Outputs:');
                try {
                    // Try to pretty print JSON string
                    console.log(JSON.stringify(JSON.parse(data.outputsJson), null, 2));
                } catch {
                    console.log(data.outputsJson);
                }
            }

        } catch (error: any) {
            console.error('Error getting execution:', error.message);
            process.exit(1);
        }
    });

program
    .command('executions:clean')
    .description('Delete ALL execution logs from the database')
    .action(async () => {
        try {
            const { confirm1 } = await inquirer.prompt([
                {
                    type: 'confirm',
                    name: 'confirm1',
                    message: 'WARNING: This will delete ALL execution logs. Are you sure?',
                    default: false
                }
            ]);

            if (!confirm1) {
                console.log('Operation cancelled.');
                return;
            }

            const { confirm2 } = await inquirer.prompt([
                {
                    type: 'input',
                    name: 'confirm2',
                    message: 'Type "DELETE ALL" to confirm:'
                }
            ]);

            if (confirm2 !== 'DELETE ALL') {
                console.log('Confirmation failed. Operation cancelled.');
                return;
            }

            console.log('Deleting all executions...');

            // Delete in batches using a query
            // Firestore deletion of collections requires recursive delete or batching
            // For simplicity/safety here without firebase-tools, we'll fetch and delete in batches of 500

            let deletedCount = 0;
            const batchSize = 500;

            while (true) {
                const snapshot = await db.collection('executions').limit(batchSize).get();
                if (snapshot.empty) {
                    break;
                }

                const batch = db.batch();
                snapshot.docs.forEach(doc => {
                    batch.delete(doc.ref);
                });

                await batch.commit();
                deletedCount += snapshot.size;
                console.log(`Deleted ${deletedCount} records...`);
            }

            console.log(`Successfully deleted ${deletedCount} executions.`);

        } catch (error: any) {
            console.error('Error cleaning executions:', error.message);
            process.exit(1);
        }
    });

program.parse();
