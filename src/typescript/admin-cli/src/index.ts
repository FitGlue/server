import * as admin from 'firebase-admin';
import { Command } from 'commander';
import inquirer from 'inquirer';
import { UserService } from '@fitglue/shared/dist/domain/services/user';
import { EnricherProviderType } from '@fitglue/shared/dist/types/pb/user';

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

            if (answers.createIngressKey) {
                const key = await userService.createIngressApiKey(userId, answers.label, answers.scopes);
                console.log('\n==========================================');
                console.log(`INGRESS API KEY (${answers.label}):`);
                console.log(key);
                console.log('==========================================\n');
            }

            console.log('User creation complete. Use "users:configure-hevy" or "users:connect" to set up integrations.');

        } catch (error) {
            console.error('Error creating user:', error);
            process.exit(1);
        }
    });

program.command('users:configure-hevy')
    .argument('<userId>', 'User ID to configure')
    .description('Configure Hevy integration for a user')
    .action(async (userId) => {
        try {
            // Verify user exists
            const userDoc = await db.collection('users').doc(userId).get();
            if (!userDoc.exists) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }

            const answers = await inquirer.prompt([
                {
                    type: 'password',
                    name: 'apiKey',
                    message: 'Hevy API Key:',
                    validate: (input) => input.length > 0 || 'API Key is required'
                }
            ]);

            await userService.setHevyIntegration(userId, answers.apiKey);
            console.log('Hevy integration configured.');

        } catch (error) {
            console.error('Error configuring Hevy:', error);
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

import { createFitbitClient } from '@fitglue/shared/dist/integrations/fitbit/client';

program.command('fitbit:subscribe')
    .argument('[userId]', 'FitGlue User ID')
    .description('Create a Fitbit subscription for the user to receive activity updates')
    .action(async (userId) => {
        try {
            if (!userId) {
                const answers = await inquirer.prompt([
                    {
                        type: 'input',
                        name: 'userId',
                        message: 'Enter FitGlue User ID:',
                        validate: (input) => input.length > 0 || 'Required'
                    }
                ]);
                userId = answers.userId;
            }

            // Ensure env var is set for shared library calls (secrets) so it fetches from the correct GSM project
            if (!process.env.GOOGLE_CLOUD_PROJECT) {
                process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-dev';
            }

            const user = await userService.getUser(userId);
            if (!user) {
                console.error('User not found');
                process.exit(1);
            }

            if (!user.integrations?.fitbit?.accessToken) {
                console.error('User does not have Fitbit integration configured');
                process.exit(1);
            }

            console.log(`Creating Fitbit subscription for user: ${userId} (Fitbit ID: ${user.integrations.fitbit.fitbitUserId})...`);

            const client = createFitbitClient(userService, userId);

            // POST /1/user/-/{collection-path}/apiSubscriptions/{subscription-id}.json
            // collection-path: activities
            // subscription-id: fitglue-activities
            const { data, error, response } = await client.POST("/1/user/-/{collection-path}/apiSubscriptions/{subscription-id}.json", {
                params: {
                    path: {
                        'collection-path': 'activities',
                        'subscription-id': 'fitglue-activities'
                    },
                    header: {
                        'X-Fitbit-Subscriber-Id': '1'
                    }
                }
            });

            if (error) {
                if (response.status === 409) {
                    console.log('✅ Subscription already exists (409 Conflict). This is expected.');
                    return;
                }
                console.error('❌ Failed to create subscription:', error);
                console.error(`Status: ${response.status} ${response.statusText}`);
                process.exit(1);
            }

            console.log('✅ Subscription created successfully!');
            console.log(JSON.stringify(data, null, 2));

        } catch (error) {
            console.error('Error creating subscription:', error);
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

// Helper to map EnricherProviderType to human-readable names
const getEnricherProviderName = (providerType: EnricherProviderType): string => {
    const mapping: Record<EnricherProviderType, string> = {
        [EnricherProviderType.ENRICHER_PROVIDER_UNSPECIFIED]: 'Unspecified',
        [EnricherProviderType.ENRICHER_PROVIDER_FITBIT_HEART_RATE]: 'Fitbit Heart Rate',
        [EnricherProviderType.ENRICHER_PROVIDER_WORKOUT_SUMMARY]: 'Workout Summary',
        [EnricherProviderType.ENRICHER_PROVIDER_MUSCLE_HEATMAP]: 'Muscle Heatmap',
        [EnricherProviderType.ENRICHER_PROVIDER_VIRTUAL_GPS]: 'Virtual GPS',
        [EnricherProviderType.ENRICHER_PROVIDER_SOURCE_LINK]: 'Source Link',
        [EnricherProviderType.ENRICHER_PROVIDER_METADATA_PASSTHROUGH]: 'Metadata Passthrough',
        [EnricherProviderType.ENRICHER_PROVIDER_TYPE_MAPPER]: 'Type Mapper',
        [EnricherProviderType.ENRICHER_PROVIDER_MOCK]: 'Mock',
        [EnricherProviderType.UNRECOGNIZED]: 'Unrecognized',
    };
    return mapping[providerType] || `Unknown (${providerType})`;
};

// Helper to get available enricher choices, excluding already-selected ones
const getAvailableEnricherChoices = (selectedProviderTypes: EnricherProviderType[]) => {
    const allChoices = [
        { name: 'Fitbit Heart Rate', value: EnricherProviderType.ENRICHER_PROVIDER_FITBIT_HEART_RATE },
        { name: 'Workout Summary', value: EnricherProviderType.ENRICHER_PROVIDER_WORKOUT_SUMMARY },
        { name: 'Muscle Heatmap', value: EnricherProviderType.ENRICHER_PROVIDER_MUSCLE_HEATMAP },
        { name: 'Virtual GPS', value: EnricherProviderType.ENRICHER_PROVIDER_VIRTUAL_GPS },
        { name: 'Source Link', value: EnricherProviderType.ENRICHER_PROVIDER_SOURCE_LINK },
        { name: 'Type Mapper', value: EnricherProviderType.ENRICHER_PROVIDER_TYPE_MAPPER },
        { name: 'Metadata Passthrough', value: EnricherProviderType.ENRICHER_PROVIDER_METADATA_PASSTHROUGH },
        { name: 'Mock', value: EnricherProviderType.ENRICHER_PROVIDER_MOCK }
    ];

    return allChoices.filter(choice => !selectedProviderTypes.includes(choice.value));
};

// Helper to format user output
const formatUserOutput = (doc: admin.firestore.DocumentSnapshot) => {
    const data = doc.data();
    if (!data) return;

    const integrations = [];
    if (data.integrations?.hevy?.api_key || data.integrations?.hevy?.apiKey) integrations.push('Hevy');
    if (data.integrations?.strava?.enabled) integrations.push('Strava');
    if (data.integrations?.fitbit?.enabled) integrations.push('Fitbit');

    console.log(`ID: ${doc.id}`);
    // Handle created_at (snake) or createdAt (legacy)
    const createdAt = data.created_at || data.createdAt;
    console.log(`   Created: ${createdAt?.toDate?.()?.toISOString() || 'Unknown'}`);
    console.log(`   Integrations: ${integrations.join(', ') || 'None'}`);

    if (data.pipelines && Array.isArray(data.pipelines) && data.pipelines.length > 0) {
        console.log(`   Pipelines:`);
        data.pipelines.forEach((p: any, index: number) => {
            console.log(`     #${index + 1} [${p.id}]`);
            console.log(`       Source: ${p.source}`);
            if (p.enrichers && p.enrichers.length > 0) {
                // Enrichers use provider_type in DB now
                const enricherDesc = p.enrichers.map((e: any) => getEnricherProviderName(e.provider_type || e.providerType)).join(' -> ');
                console.log(`       Enrichers: ${enricherDesc}`);
            } else {
                console.log(`       Enrichers: (None)`);
            }
            console.log(`       Destinations: ${p.destinations?.join(', ') || 'None'}`);
        });
    } else {
        console.log(`   Pipelines: None`);
    }
    console.log('--------------------------------------------------');
};

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
            snapshot.forEach(doc => formatUserOutput(doc));
            console.log('');
        } catch (error) {
            console.error('Error listing users:', error);
            process.exit(1);
        }
    });

program.command('users:get')
    .argument('<userId>', 'User ID to get')
    .description('Get details of a specific user')
    .action(async (userId) => {
        try {
            const doc = await db.collection('users').doc(userId).get();
            if (!doc.exists) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }

            console.log('\nUser Details:');
            console.log('--------------------------------------------------');
            formatUserOutput(doc);
            console.log('');
        } catch (error) {
            console.error('Error getting user:', error);
            process.exit(1);
        }
    });

import { generateOAuthState } from '@fitglue/shared';

program.command('users:connect')
    .argument('<userId>', 'User ID to connect')
    .argument('<provider>', 'Provider to connect (strava or fitbit)')
    .description('Generate OAuth authorization URL for a user')
    .action(async (userId, provider) => {
        try {
            if (!['strava', 'fitbit'].includes(provider)) {
                console.error('Provider must be "strava" or "fitbit"');
                process.exit(1);
            }

            // Verify user exists
            const userDoc = await db.collection('users').doc(userId).get();
            if (!userDoc.exists) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }

            // Prompt for Client ID
            const { clientId } = await inquirer.prompt([
                {
                    type: 'input',
                    name: 'clientId',
                    message: `Enter ${provider === 'strava' ? 'Strava' : 'Fitbit'} Client ID:`,
                    validate: (input) => input.length > 0 || 'Client ID is required'
                }
            ]);

            // Get environment from GOOGLE_CLOUD_PROJECT or default to dev
            // Ensure env var is set for shared library calls (secrets) so it fetches from the correct GSM project
            if (!process.env.GOOGLE_CLOUD_PROJECT) {
                process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-dev';
            }
            const project = process.env.GOOGLE_CLOUD_PROJECT;
            const env = project.includes('-prod') ? 'prod' : project.includes('-test') ? 'test' : 'dev';
            const baseUrl = env === 'prod' ? 'https://fitglue.tech' : `https://${env}.fitglue.tech`;

            // Generate state token
            const state = await generateOAuthState(userId);

            let authUrl: string;
            if (provider === 'strava') {
                authUrl = `https://www.strava.com/oauth/authorize?` +
                    `client_id=${clientId}&` +
                    `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/strava/callback`)}&` +
                    `response_type=code&` +
                    `scope=read,activity:read_all,activity:write&` +
                    `state=${state}`;
            } else {
                authUrl = `https://www.fitbit.com/oauth2/authorize?` +
                    `client_id=${clientId}&` +
                    `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/fitbit/callback`)}&` +
                    `response_type=code&` +
                    `scope=${encodeURIComponent('activity heartrate profile')}&` +
                    `state=${state}`;
            }

            console.log('\n==========================================');
            console.log(`OAuth Authorization URL for ${provider}:`);
            console.log('==========================================');
            console.log(authUrl);
            console.log('==========================================\n');
            console.log(`User should visit this URL to authorize ${provider} access.`);
            console.log('After authorization, tokens will be automatically stored in Firestore.\n');

        } catch (error) {
            console.error('Error generating OAuth URL:', error);
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

program.command('users:add-pipeline')
    .argument('<userId>', 'User ID to add pipeline to')
    .description('Add a processing pipeline to a user')
    .action(async (userId) => {
        try {
            // Check user exists
            const userDoc = await db.collection('users').doc(userId).get();
            if (!userDoc.exists) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }

            const sourceAnswers = await inquirer.prompt([
                {
                    type: 'list',
                    name: 'source',
                    message: 'Select Source:',
                    choices: ['SOURCE_HEVY', 'SOURCE_FITBIT', 'SOURCE_TEST']
                }
            ]);

            const enrichers = [];
            const addMore = true;

            console.log('\n--- Configure Enrichers (Order Matters) ---');
            while (addMore) {
                const selectedProviderTypes = enrichers.map((e: any) => e.providerType);
                const availableChoices = getAvailableEnricherChoices(selectedProviderTypes);

                if (availableChoices.length === 0) {
                    console.log('All available enrichers have been added.');
                    break;
                }

                const enricherAnswer = await inquirer.prompt([
                    {
                        type: 'confirm',
                        name: 'add',
                        message: 'Add an enricher?',
                        default: false
                    }
                ]);

                if (!enricherAnswer.add) {
                    break;
                }

                const config = await inquirer.prompt([
                    {
                        type: 'list',
                        name: 'providerType',
                        message: 'Enricher Provider:',
                        choices: availableChoices
                    }
                ]);


                // Provider-specific configuration prompts
                const inputs = await promptForEnricherConfig(config.providerType);


                enrichers.push({
                    providerType: config.providerType,
                    inputs
                });
            }

            const destAnswers = await inquirer.prompt([
                {
                    type: 'checkbox',
                    name: 'destinations',
                    message: 'Select Destinations:',
                    choices: ['strava'],
                    validate: (input) => input.length > 0 || 'Must select at least one destination'
                }
            ]);

            console.log('\nAdding pipeline...');
            const id = await userService.addPipeline(userId, sourceAnswers.source, enrichers, destAnswers.destinations);
            console.log(`Pipeline added successfully! ID: ${id}`);

        } catch (error) {
            console.error('Error adding pipeline:', error);
            process.exit(1);
        }
    });

program.command('users:remove-pipeline')
    .argument('<userId>', 'User ID')
    .description('Remove a pipeline from a user')
    .action(async (userId) => {
        try {
            const userDoc = await db.collection('users').doc(userId).get();
            if (!userDoc.exists) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }
            const data = userDoc.data();
            const pipelines = data?.pipelines || [];

            if (pipelines.length === 0) {
                console.log('No pipelines found for this user.');
                return;
            }

            const { pipelineId } = await inquirer.prompt([
                {
                    type: 'list',
                    name: 'pipelineId',
                    message: 'Select pipeline to remove:',
                    choices: pipelines.map((p: any) => ({
                        name: `${p.source} -> ${p.destinations.join(', ')} [${p.id}]`,
                        value: p.id
                    }))
                }
            ]);

            const { confirm } = await inquirer.prompt([{
                type: 'confirm',
                name: 'confirm',
                message: `Are you sure you want to remove pipeline ${pipelineId}?`,
                default: false
            }]);

            if (confirm) {
                await userService.removePipeline(userId, pipelineId);
                console.log(`Pipeline ${pipelineId} removed.`);
            } else {
                console.log('Cancelled.');
            }

        } catch (error) {
            console.error('Error removing pipeline:', error);
            process.exit(1);
        }
    });

program.command('users:replace-pipeline')
    .argument('<userId>', 'User ID')
    .description('Replace/Reconfigure an existing pipeline')
    .action(async (userId) => {
        try {
            const userDoc = await db.collection('users').doc(userId).get();
            if (!userDoc.exists) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }
            const data = userDoc.data();
            const pipelines = data?.pipelines || [];

            if (pipelines.length === 0) {
                console.log('No pipelines found for this user.');
                return;
            }

            const { pipelineId } = await inquirer.prompt([
                {
                    type: 'list',
                    name: 'pipelineId',
                    message: 'Select pipeline to replace:',
                    choices: pipelines.map((p: any) => ({
                        name: `${p.source} -> ${p.destinations.join(', ')} [${p.id}]`,
                        value: p.id
                    }))
                }
            ]);

            console.log(`\nReconfiguring Pipeline ${pipelineId}...`);

            // --- Re-use configuration prompts (Duplicate logic from add-pipeline for now for simplicity) ---
            const sourceAnswers = await inquirer.prompt([
                {
                    type: 'list',
                    name: 'source',
                    message: 'Select Source:',
                    choices: ['SOURCE_HEVY', 'SOURCE_FITBIT', 'SOURCE_TEST']
                }
            ]);

            const enrichers = [];
            const addMore = true;
            console.log('\n--- Configure Enrichers (Order Matters) ---');
            // eslint-disable-next-line no-constant-condition
            while (true) {
                const selectedProviderTypes = enrichers.map((e: any) => e.providerType);
                const availableChoices = getAvailableEnricherChoices(selectedProviderTypes);

                if (availableChoices.length === 0) {
                    console.log('All available enrichers have been added.');
                    break;
                }

                const enricherAnswer = await inquirer.prompt([
                    {
                        type: 'confirm',
                        name: 'add',
                        message: 'Add an enricher?',
                        default: false
                    }
                ]);

                if (!enricherAnswer.add) break;

                const config = await inquirer.prompt([
                    {
                        type: 'list',
                        name: 'providerType',
                        message: 'Enricher Provider:',
                        choices: availableChoices
                    }
                ]);


                // Provider-specific configuration prompts
                const inputs = await promptForEnricherConfig(config.providerType);


                enrichers.push({
                    providerType: config.providerType,
                    inputs
                });
            }

            const destAnswers = await inquirer.prompt([
                {
                    type: 'checkbox',
                    name: 'destinations',
                    message: 'Select Destinations:',
                    choices: ['strava'],
                    validate: (input) => input.length > 0 || 'Must select at least one destination'
                }
            ]);

            await userService.replacePipeline(userId, pipelineId, sourceAnswers.source, enrichers, destAnswers.destinations);
            console.log(`Pipeline ${pipelineId} replaced successfully.`);

        } catch (error) {
            console.error('Error replacing pipeline:', error);
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

            // eslint-disable-next-line no-constant-condition
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

// --- Bucket Commands ---

const formatBucket = (bucket: any) => {
    console.log(`Name: ${bucket.name}`);
    console.log(`Location: ${bucket.metadata.location}`);
    console.log(`Storage Class: ${bucket.metadata.storageClass}`);
    console.log(`Created: ${bucket.metadata.timeCreated}`);
    console.log(`Updated: ${bucket.metadata.updated}`);
    console.log(`Link: ${bucket.metadata.selfLink}`);
    console.log('--------------------------------------------------');
};

program
    .command('buckets:list')
    .description('List GCS buckets')
    .action(async () => {
        try {
            console.log('Fetching buckets...');
            // We need to provide a bucket name to get the storage client if no default bucket is set in options
            // The name doesn't matter for accessing the .storage property
            const [buckets] = await admin.storage().bucket('fitglue-placeholder').storage.getBuckets();

            if (buckets.length === 0) {
                console.log('No buckets found.');
                return;
            }

            console.log(`\nFound ${buckets.length} buckets:`);
            console.log('--------------------------------------------------');
            buckets.forEach(bucket => {
                console.log(`- ${bucket.name} (${bucket.metadata.location})`);
            });
            console.log('');

        } catch (error: any) {
            console.error('Error listing buckets:', error.message);
            process.exit(1);
        }
    });

program
    .command('buckets:get <bucketName>')
    .description('Get details of a specific GCS bucket')
    .action(async (bucketName) => {
        try {
            console.log(`Fetching bucket ${bucketName}...`);
            const bucket = admin.storage().bucket(bucketName);
            const [exists] = await bucket.exists();

            if (!exists) {
                console.error(`Bucket ${bucketName} not found.`);
                process.exit(1);
            }

            const [metadata] = await bucket.getMetadata();
            // Create a pseudo-bucket object with the metadata structure expected by formatBucket
            // or just adapt formatBucket to work with what we have.
            // The bucket object from getBuckets() already has metadata populated.
            // When getting a single bucket, we get the metadata separately.

            console.log('\nBucket Details:');
            console.log('--------------------------------------------------');
            formatBucket({ name: bucketName, metadata });
            console.log('');

        } catch (error: any) {
            console.error('Error getting bucket:', error.message);
            process.exit(1);
        }
    });

program
    .command('buckets:from-execution <executionId>')
    .description('Get details of the bucket used in a specific execution')
    .action(async (executionId) => {
        try {
            console.log(`Fetching execution ${executionId}...`);
            const doc = await db.collection('executions').doc(executionId).get();
            if (!doc.exists) {
                console.error(`Execution ${executionId} not found.`);
                process.exit(1);
            }

            const data = doc.data()!;
            let fitFileUri = data.fit_file_uri;

            // If not found at top level, check within inputs or outputs
            if (!fitFileUri) {
                // Check outputs first
                if (data.outputsJson) {
                    try {
                        const outputs = JSON.parse(data.outputsJson);
                        fitFileUri = outputs.fit_file_uri || outputs.uri;
                    } catch (e) {
                        // ignore
                    }
                }

                // Check inputs if still not found
                if (!fitFileUri && data.inputs) {
                    try {
                        const inputs = JSON.parse(data.inputs);
                        fitFileUri = inputs.fit_file_uri || inputs.uri || inputs.fileUri;
                    } catch (e) {
                        // ignore
                    }
                }
            }

            if (!fitFileUri) {
                console.error('Could not find a fit_file_uri (or similar) in the execution data.');
                process.exit(1);
            }

            console.log(`Found URI: ${fitFileUri}`);

            // Parse bucket name from gs://<bucket>/...
            if (!fitFileUri.startsWith('gs://')) {
                console.error('URI does not start with gs://');
                process.exit(1);
            }

            const parts = fitFileUri.split('/');
            // ["gs:", "", "bucket-name", "path", "to", "file"]
            if (parts.length < 3) {
                console.error('Invalid GCS URI format.');
                process.exit(1);
            }

            const bucketName = parts[2];
            console.log(`Identified Bucket: ${bucketName}`);

            // Reuse the get bucket logic
            const bucket = admin.storage().bucket(bucketName);
            const [exists] = await bucket.exists();

            if (!exists) {
                console.error(`Bucket ${bucketName} not found.`);
                process.exit(1);
            }

            const [metadata] = await bucket.getMetadata();

            console.log('\nBucket Details:');
            console.log('--------------------------------------------------');
            formatBucket({ name: bucketName, metadata });
            console.log('');

        } catch (error: any) {
            console.error('Error getting bucket from execution:', error.message);
            process.exit(1);
        }
    });



// --- File Commands ---

import * as fs from 'fs';
import * as path from 'path';

// Define default download directory relative to the project root (assuming we run from server/)
const DEFAULT_DOWNLOAD_DIR = 'downloads';

const getDestPath = (localPath: string | undefined, filePath: string): string => {
    if (localPath) {
        return localPath;
    }
    const basename = path.basename(filePath);
    return path.join(DEFAULT_DOWNLOAD_DIR, basename);
};
program
    .command('files:download <bucketOrUri> [remotePath] [localPath]')
    .description('Download a file from GCS')
    .action(async (bucketOrUri, remotePath, localPath) => {
        try {
            let bucketName: string;
            let filePath: string;
            let destPath: string;

            if (bucketOrUri.startsWith('gs://')) {
                // Parse gs:// URI
                const parts = bucketOrUri.split('/');
                if (parts.length < 4) {
                    console.error('Invalid GCS URI. Must be gs://<bucket>/<path/to/file>');
                    process.exit(1);
                }
                bucketName = parts[2];
                filePath = parts.slice(3).join('/');

                // If remotePath argument is provided, treat it as localPath
                destPath = getDestPath(remotePath || localPath, filePath);
            } else {
                // Classic arguments
                if (!remotePath) {
                    console.error('Remote path is required when not using a gs:// URI.');
                    process.exit(1);
                }
                bucketName = bucketOrUri;
                filePath = remotePath;
                destPath = getDestPath(localPath, filePath);
            }

            console.log(`Downloading gs://${bucketName}/${filePath} to ${destPath}...`);

            const bucket = admin.storage().bucket(bucketName);
            const file = bucket.file(filePath);

            const [exists] = await file.exists();
            if (!exists) {
                console.error(`File gs://${bucketName}/${filePath} does not exist.`);
                process.exit(1);
            }

            // Create directory if it doesn't exist
            const dir = path.dirname(destPath);
            if (!fs.existsSync(dir)) {
                fs.mkdirSync(dir, { recursive: true });
            }

            await file.download({ destination: destPath });
            console.log('Download complete.');

        } catch (error: any) {
            console.error('Error downloading file:', error.message);
            process.exit(1);
        }
    });

program
    .command('files:download-execution <executionId> [localPath]')
    .description('Download a file used in a specific execution')
    .action(async (executionId, localPath) => {
        try {
            console.log(`Fetching execution ${executionId}...`);
            const doc = await db.collection('executions').doc(executionId).get();
            if (!doc.exists) {
                console.error(`Execution ${executionId} not found.`);
                process.exit(1);
            }

            const data = doc.data()!;
            const uris = new Set<string>();

            // Recursive function to find gs:// URIs in objects/strings
            const findUris = (obj: any) => {
                if (typeof obj === 'string') {
                    if (obj.startsWith('gs://')) {
                        uris.add(obj);
                    }
                } else if (typeof obj === 'object' && obj !== null) {
                    Object.values(obj).forEach(findUris);
                }
            };

            // Scan top level
            findUris(data);

            // Scan inputs/outputs specifically if they are JSON strings
            if (data.inputs) {
                try {
                    findUris(JSON.parse(data.inputs));
                } catch { /* ignore */ }
            }
            if (data.outputsJson) { // consistent with previous code usage
                try {
                    findUris(JSON.parse(data.outputsJson));
                } catch { /* ignore */ }
            }

            if (uris.size === 0) {
                console.error('No gs:// URIs found in this execution.');
                process.exit(1);
            }

            let selectedUri: string;
            if (uris.size === 1) {
                selectedUri = uris.values().next().value!;
                console.log(`Found only one URI: ${selectedUri}`);
            } else {
                const answers = await inquirer.prompt([
                    {
                        type: 'list',
                        name: 'uri',
                        message: 'Found multiple URIs. Which one do you want to download?',
                        choices: Array.from(uris)
                    }
                ]);
                selectedUri = answers.uri;
            }

            // Reuse download logic
            const parts = selectedUri.split('/');
            if (parts.length < 4) {
                console.error('Invalid GCS URI format found.');
                process.exit(1);
            }

            const bucketName = parts[2];
            const filePath = parts.slice(3).join('/');
            const destPath = getDestPath(localPath, filePath);

            console.log(`Downloading ${selectedUri} to ${destPath}...`);

            const bucket = admin.storage().bucket(bucketName);
            const file = bucket.file(filePath);

            const [exists] = await file.exists();
            if (!exists) {
                console.error(`File ${selectedUri} does not exist.`);
                process.exit(1);
            }

            // Create directory if it doesn't exist
            const dir = path.dirname(destPath);
            if (!fs.existsSync(dir)) {
                fs.mkdirSync(dir, { recursive: true });
            }

            await file.download({ destination: destPath });
            console.log('Download complete.');

        } catch (error: any) {
            console.error('Error downloading from execution:', error.message);
            process.exit(1);
        }
    });

program.parse();

async function promptForEnricherConfig(providerType: EnricherProviderType): Promise<{ [key: string]: string }> {
    let inputs: { [key: string]: string } = {};

    if (providerType === EnricherProviderType.ENRICHER_PROVIDER_VIRTUAL_GPS) {
        const gpsConfig = await inquirer.prompt([
            {
                type: 'list',
                name: 'route',
                message: 'Select Route:',
                choices: [
                    { name: 'London Hyde Park (~4km)', value: 'london' },
                    { name: 'NYC Central Park (~10km)', value: 'nyc' }
                ],
                default: 'london'
            },
            {
                type: 'confirm',
                name: 'force',
                message: 'Force overwrite existing GPS data?',
                default: false
            }
        ]);
        inputs = {
            route: gpsConfig.route,
            ...(gpsConfig.force && { force: 'true' })
        };
    } else if (providerType === EnricherProviderType.ENRICHER_PROVIDER_WORKOUT_SUMMARY) {
        const summaryConfig = await inquirer.prompt([
            {
                type: 'list',
                name: 'format',
                message: 'Format Style:',
                choices: [
                    { name: 'Compact (4×8@100kg)', value: 'compact' },
                    { name: 'Detailed (4 sets × 8 reps @ 100.0kg)', value: 'detailed' },
                    { name: 'Verbose (4 sets of 8 reps at 100.0 kilograms)', value: 'verbose' }
                ],
                default: 'detailed'
            },
            {
                type: 'confirm',
                name: 'showStats',
                message: 'Show Headline Stats (sets, volume, etc)?',
                default: true
            }
        ]);
        inputs = {
            format: summaryConfig.format,
            show_stats: summaryConfig.showStats.toString()
        };
    } else if (providerType === EnricherProviderType.ENRICHER_PROVIDER_MUSCLE_HEATMAP) {
        const heatmapConfig = await inquirer.prompt([
            {
                type: 'list',
                name: 'style',
                message: 'Visualization Style:',
                choices: [
                    { name: 'Emoji Bars (🟪🟪🟪⬜⬜)', value: 'emoji' },
                    { name: 'Percentage (Chest: 80%)', value: 'percentage' },
                    { name: 'Text Only (High: Chest)', value: 'text' }
                ],
                default: 'emoji'
            },
            {
                type: 'number',
                name: 'barLength',
                message: 'Bar Length (3-10):',
                default: 5,
                validate: (input) => (input >= 3 && input <= 10) || 'Must be between 3 and 10'
            },
            {
                type: 'list',
                name: 'preset',
                message: 'Coefficient Preset:',
                choices: [
                    { name: 'Standard (Balanced)', value: 'standard' },
                    { name: 'Powerlifting (Emphasize compounds)', value: 'powerlifting' },
                    { name: 'Bodybuilding (Emphasize isolation)', value: 'bodybuilding' }
                ],
                default: 'standard'
            }
        ]);
        inputs = {
            style: heatmapConfig.style,
            bar_length: heatmapConfig.barLength.toString(),
            preset: heatmapConfig.preset
        };
    } else if (providerType === EnricherProviderType.ENRICHER_PROVIDER_TYPE_MAPPER) {
        const rules = [];
        let addRule = true;
        console.log('\n--- Configure Type Mapper Rules ---');
        while (addRule) {
            const ruleAnswers = await inquirer.prompt([
                {
                    type: 'input',
                    name: 'substring',
                    message: 'Substring to match (case-insensitive):',
                    validate: (input) => input.length > 0 || 'Required'
                },
                {
                    type: 'list',
                    name: 'targetType',
                    message: 'Target Activity Type:',
                    choices: [
                        'AlpineSki', 'BackcountrySki', 'Badminton', 'Canoeing', 'Crossfit', 'EBikeRide',
                        'Elliptical', 'EMountainBikeRide', 'Golf', 'GravelRide', 'Handcycle',
                        'HighIntensityIntervalTraining', 'Hike', 'IceSkate', 'InlineSkate', 'Kayaking',
                        'Kitesurf', 'MountainBikeRide', 'NordicSki', 'Pickleball', 'Pilates', 'Racquetball',
                        'Ride', 'RockClimbing', 'RollerSki', 'Rowing', 'Run', 'Sail', 'Skateboard',
                        'Snowboard', 'Snowshoe', 'Soccer', 'Squash', 'StairStepper', 'StandUpPaddling',
                        'Surfing', 'Swim', 'TableTennis', 'Tennis', 'TrailRun', 'Velomobile', 'VirtualRide',
                        'VirtualRow', 'VirtualRun', 'Walk', 'WeightTraining', 'Wheelchair', 'Windsurf',
                        'Workout', 'Yoga'
                    ]
                },
                {
                    type: 'confirm',
                    name: 'addAnother',
                    message: 'Add another rule?',
                    default: false
                }
            ]);

            rules.push({
                substring: ruleAnswers.substring,
                target_type: ruleAnswers.targetType
            });

            addRule = ruleAnswers.addAnother;
        }
        inputs = { rules: JSON.stringify(rules) };
    } else {
        // Only prompt for JSON if the provider might need config
        // Skip for providers with no config options
        const noConfigProviders = [
            EnricherProviderType.ENRICHER_PROVIDER_METADATA_PASSTHROUGH,
            EnricherProviderType.ENRICHER_PROVIDER_SOURCE_LINK,
            EnricherProviderType.ENRICHER_PROVIDER_FITBIT_HEART_RATE
        ];

        if (!noConfigProviders.includes(providerType)) {
            const jsonInput = await inquirer.prompt([{
                type: 'input',
                name: 'inputsJson',
                message: 'Inputs (JSON string, optional):',
                validate: (input) => {
                    if (!input) return true;
                    try { JSON.parse(input); return true; } catch (e) { return 'Invalid JSON'; }
                }
            }]);
            inputs = jsonInput.inputsJson ? JSON.parse(jsonInput.inputsJson) : {};
        }
    }
    return inputs;
}
