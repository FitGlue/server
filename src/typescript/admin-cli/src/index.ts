import { Command } from 'commander';
import inquirer from 'inquirer';
import {
    UserService,
    ApiKeyService,
    ExecutionService,
    UserStore,
    ActivityStore,
    ApiKeyStore,
    ExecutionStore,
    EnricherProviderType,
    db,
    createFitbitClient,
    UserRecord,
    ExecutionStatus,
    ActivityType,
    EnricherConfig,
    PipelineConfig,
    ExecutionRecord,
    Destination,
    INTEGRATIONS,
    UserIntegrations,
} from '@fitglue/shared';


import * as admin from 'firebase-admin';



// Initialize Firebase if not already initialized
if (admin.apps.length === 0) {
    admin.initializeApp({
        credential: admin.credential.applicationDefault()
    });
}

const userStore = new UserStore(db);
const activityStore = new ActivityStore(db);
const apiKeyStore = new ApiKeyStore(db);
const executionStore = new ExecutionStore(db);

const userService = new UserService(userStore, activityStore);
const apiKeyService = new ApiKeyService(apiKeyStore);
const executionService = new ExecutionService(executionStore);

const program = new Command();

program
    .name('fitglue-admin')
    .description('CLI for FitGlue administration')
    .version('1.0.0')
    .configureHelp({
        sortSubcommands: true
    })
    .showHelpAfterError()
    .showSuggestionAfterError();

import { addActivitiesCommands } from './commands/activities';
addActivitiesCommands(program, userService);

import { addInputsCommands } from './commands/inputs';
addInputsCommands(program);

import { addTerraformCommands } from './commands/terraform';
addTerraformCommands(program);

import { randomUUID } from 'crypto';
import * as crypto from 'crypto';

program.command('users:create-auth')
    .argument('<userId>', 'User ID to create Auth user for')
    .description('Create a Firebase Auth user for an existing Firestore User ID')
    .action(async (userId) => {
        try {
            // Verify user exists
            const user = await userService.getUser(userId);
            if (!user) {
                console.error(`User ${userId} not found in Firestore`);
                process.exit(1);
            }

            console.log(`Creating Auth user for ${userId}...`);

            const answers = await inquirer.prompt([
                {
                    type: 'input',
                    name: 'email',
                    message: 'Email:',
                    validate: (input) => input.includes('@') || 'Invalid email'
                },
                {
                    type: 'password',
                    name: 'password',
                    message: 'Password (min 6 chars):',
                    validate: (input) => input.length >= 6 || 'Password too short'
                }
            ]);

            try {
                const userRecord = await admin.auth().createUser({
                    uid: userId, // FORCE the UID to match Firestore
                    email: answers.email,
                    password: answers.password,
                });
                console.log(`✅ Auth user created successfully: ${userRecord.uid}`);
            } catch (err: unknown) {
                const e = err as { code?: string };
                if (e.code === 'auth/uid-already-exists') {
                    console.error('❌ Auth user already exists for this UID.');
                } else if (e.code === 'auth/email-already-exists') {
                    console.error('❌ Email already in use.');
                } else {
                    console.error('❌ Error creating auth user:', err);
                }
            }

        } catch (error: unknown) {
            console.error('Error:', error);
            process.exit(1);
        }
    });

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
                // Generate key
                const token = `fg_sk_${crypto.randomBytes(32).toString('hex')}`;
                const hash = crypto.createHash('sha256').update(token).digest('hex');

                await apiKeyService.create(hash, {
                    label: answers.label,
                    scopes: answers.scopes,
                    userId,
                    createdAt: new Date()
                });

                console.log('\n==========================================');
                console.log(`INGRESS API KEY (${answers.label}):`);
                console.log(token);
                console.log('==========================================\n');
            }

            console.log('User creation complete. Use "users:configure-hevy" or "users:connect" to set up integrations.');

        } catch (error: unknown) {
            console.error('Error creating user:', error);
            process.exit(1);
        }
    });

program.command('users:configure-integration')
    .argument('<provider>', 'Provider key (hevy, mock, etc)')
    .argument('<userId>', 'User ID to configure')
    .description('Configure an integration for a user')
    .action(async (provider, userId) => {
        try {
            // Validate provider
            const definition = INTEGRATIONS[provider as keyof UserIntegrations];
            if (!definition) {
                console.error(`Unknown provider: ${provider}`);
                console.log('Available providers:', Object.keys(INTEGRATIONS).join(', '));
                process.exit(1);
            }

            // Verify user exists
            const user = await userService.getUser(userId);
            if (!user) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }

            console.log(`Configuring ${definition.displayName} for user ${userId}...`);

            const answers: Record<string, unknown> = {};

            // Dynamic prompts
            if (definition.configurableFields.length > 0) {
                const prompts = definition.configurableFields.map(field => ({
                    type: field.type === 'password' ? 'password' : (field.type === 'boolean' ? 'confirm' : 'input'),
                    name: field.field,
                    message: `${field.name}:`,
                    validate: (input: unknown) => {
                        if (field.required && (input === '' || input === undefined)) return `${field.name} is required`;
                        return true;
                    }
                }));

                const results = await inquirer.prompt(prompts);
                Object.assign(answers, results);
            } else {
                console.log('No configurable fields for this integration.');
            }

            const payload = {
                ...answers,
                createdAt: new Date(),
                lastUsedAt: new Date(),
                enabled: true
            };

            if (!('enabled' in payload)) {
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                (payload as any).enabled = true;
            }

            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            await userService.setIntegration(userId, provider as keyof UserIntegrations, payload as any);
            console.log(`✅ ${definition.displayName} integration configured.`);

        } catch (error: unknown) {
            console.error('Error configuring integration:', error);
            process.exit(1);
        }
    });

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

        } catch (error: unknown) {
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

            await userService.deleteUser(userId);
            // Note: In a real app we might want to recursively delete subcollections or related data
            console.log(`User ${userId} deleted.`);
        } catch (error: unknown) {
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
        [EnricherProviderType.ENRICHER_PROVIDER_PARKRUN]: 'Parkrun',
        [EnricherProviderType.ENRICHER_PROVIDER_CONDITION_MATCHER]: 'Condition Matcher',
        [EnricherProviderType.ENRICHER_PROVIDER_AUTO_INCREMENT]: 'Auto Increment',
        [EnricherProviderType.ENRICHER_PROVIDER_USER_INPUT]: 'User Input',
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
        { name: 'Parkrun', value: EnricherProviderType.ENRICHER_PROVIDER_PARKRUN },
        { name: 'Condition Matcher', value: EnricherProviderType.ENRICHER_PROVIDER_CONDITION_MATCHER },
        { name: 'Auto Increment', value: EnricherProviderType.ENRICHER_PROVIDER_AUTO_INCREMENT },
        { name: 'User Input', value: EnricherProviderType.ENRICHER_PROVIDER_USER_INPUT },
        { name: 'Mock', value: EnricherProviderType.ENRICHER_PROVIDER_MOCK }
    ];

    return allChoices.filter(choice => !selectedProviderTypes.includes(choice.value));
};

// Helper to format ActivityType enum string to human-readable format
const formatActivityType = (type: string | number | undefined): string => {
    if (type === undefined || type === null) return 'N/A';

    // If number, try to resolve to enum string key
    if (typeof type === 'number') {
        const resolved = ActivityType[type];
        if (resolved) {
            type = resolved;
        }
    }

    const typeStr = String(type);
    if (typeStr.startsWith('ACTIVITY_TYPE_')) {
        return typeStr.replace('ACTIVITY_TYPE_', '').split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase()).join(' ');
    }
    return typeStr;
};

const getDestinationName = (dest: number | string): string => {
    if (typeof dest === 'string') return dest;
    const name = Destination[dest];
    if (name) {
        return name.replace('DESTINATION_', '').split('_').map((w: string) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase()).join(' ');
    }
    return `Unknown(${dest})`;
};

// Helper to format user output
const formatUserOutput = (user: UserRecord) => {
    // Adapter for legacy format where doc was passed
    const data = user

    if (!data) return;

    const integrations = [];
    if (data.integrations?.hevy?.apiKey) integrations.push('Hevy');
    if (data.integrations?.strava?.enabled) integrations.push('Strava');
    if (data.integrations?.fitbit?.enabled) integrations.push('Fitbit');
    if (data.integrations?.mock?.enabled) integrations.push('Mock');

    console.log(`ID: ${data.userId}`);
    // Handle created_at (snake) or createdAt (legacy)
    const createdAt = data.createdAt;
    console.log(`   Created: ${createdAt || 'Unknown'}`);
    console.log(`   Integrations: ${integrations.join(', ') || 'None'}`);

    if (data.pipelines && Array.isArray(data.pipelines) && data.pipelines.length > 0) {
        console.log(`   Pipelines:`);
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        data.pipelines.forEach((p: any, index: number) => {
            console.log(`     #${index + 1} [${p.id}]`);
            console.log(`       Source: ${p.source}`);
            if (p.enrichers && p.enrichers.length > 0) {
                console.log(`       Enrichers:`);
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                p.enrichers.forEach((e: any, eIdx: number) => {
                    const providerName = getEnricherProviderName(e.provider_type || e.providerType);
                    console.log(`         ${eIdx + 1}. ${providerName}`);
                    if (e.inputs && Object.keys(e.inputs).length > 0) {
                        Object.entries(e.inputs).forEach(([key, val]) => {
                            let printed = false;
                            if (typeof val === 'string' && (val.trim().startsWith('{') || val.trim().startsWith('['))) {
                                try {
                                    const parsed = JSON.parse(val);
                                    const formatted = JSON.stringify(parsed, null, 2).split('\n');
                                    console.log(`              ${key}:`);
                                    formatted.forEach(line => console.log(`                ${line}`));
                                    printed = true;
                                } catch (err) {
                                    // Not valid JSON, fall through
                                }
                            }

                            if (!printed) {
                                console.log(`              ${key}: ${val}`);
                            }
                        });
                    }
                });
            } else {
                console.log(`       Enrichers: (None)`);
            }
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const destinationNames = p.destinations?.map((d: any) => getDestinationName(d)).join(', ') || 'None';
            console.log(`       Destinations: ${destinationNames}`);
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
            const users = await userService.listUsers();
            if (users.length === 0) {
                console.log('No users found.');
                return;
            }

            console.log('\nFound ' + users.length + ' users:');
            console.log('--------------------------------------------------');
            users.forEach(user => formatUserOutput(user));
            console.log('');
        } catch (error: unknown) {
            console.error('Error listing users:', error);
            process.exit(1);
        }
    });

program.command('users:get')
    .argument('<userId>', 'User ID to get')
    .description('Get details of a specific user')
    .action(async (userId) => {
        try {
            const user = await userService.getUser(userId);
            if (!user) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }

            console.log('\nUser Details:');
            console.log('--------------------------------------------------');
            formatUserOutput(user);
            console.log('');
        } catch (error: unknown) {
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
            const user = await userService.getUser(userId);
            if (!user) {
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
                    `scope=${encodeURIComponent('activity heartrate profile location')}&` +
                    `state=${state}`;
            }

            console.log('\n==========================================');
            console.log(`OAuth Authorization URL for ${provider}:`);
            console.log('==========================================');
            console.log(authUrl);
            console.log('==========================================\n');
            console.log(`User should visit this URL to authorize ${provider} access.`);
            console.log('After authorization, tokens will be automatically stored in Firestore.\n');

        } catch (error: unknown) {
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

            // Delegate deletion to service which handles batching
            const deletedCount = await userService.deleteAllUsers();

            if (deletedCount === 0) {
                console.log('No users to delete.');
                return;
            }

            console.log(`Deleted ${deletedCount} users.`);
            console.log('All users deleted.');

            // await batch.commit(); // Batch not used here anymore
            console.log('All users deleted.');
            console.log('All users deleted.');

        } catch (error: unknown) {
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
            const user = await userService.getUser(userId);
            if (!user) {
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
                const selectedProviderTypes = enrichers.map((e: EnricherConfig) => e.providerType);
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
                    choices: ['strava', 'mock'],
                    validate: (input) => input.length > 0 || 'Must select at least one destination'
                }
            ]);

            console.log('\nAdding pipeline...');
            const id = await userService.addPipeline(userId, sourceAnswers.source, enrichers, destAnswers.destinations);
            console.log(`Pipeline added successfully! ID: ${id}`);

        } catch (error: unknown) {
            console.error('Error adding pipeline:', error);
            process.exit(1);
        }
    });

program.command('users:remove-pipeline')
    .argument('<userId>', 'User ID')
    .description('Remove a pipeline from a user')
    .action(async (userId) => {
        try {
            const user = await userService.getUser(userId);
            if (!user) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }
            const data = user;
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
                    choices: pipelines.map((p: PipelineConfig) => ({
                        // eslint-disable-next-line @typescript-eslint/no-explicit-any
                        name: `${p.source} -> ${p.destinations.map((d: any) => getDestinationName(d)).join(', ')} [${p.id}]`,
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

        } catch (error: unknown) {
            console.error('Error removing pipeline:', error);
            process.exit(1);
        }
    });

program.command('users:replace-pipeline')
    .argument('<userId>', 'User ID')
    .description('Replace/Reconfigure an existing pipeline')
    .action(async (userId) => {
        try {
            const user = await userService.getUser(userId);
            if (!user) {
                console.error(`User ${userId} not found`);
                process.exit(1);
            }
            const data = user;
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
                    choices: pipelines.map((p: PipelineConfig) => ({
                        // eslint-disable-next-line @typescript-eslint/no-explicit-any
                        name: `${p.source} -> ${p.destinations.map((d: any) => getDestinationName(d)).join(', ')} [${p.id}]`,
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
            console.log('\n--- Configure Enrichers (Order Matters) ---');
            // eslint-disable-next-line no-constant-condition
            while (true) {
                const selectedProviderTypes = enrichers.map((e: EnricherConfig) => e.providerType);
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
                    choices: ['strava', 'mock'],
                    validate: (input) => input.length > 0 || 'Must select at least one destination'
                }
            ]);

            await userService.replacePipeline(userId, pipelineId, sourceAnswers.source, enrichers, destAnswers.destinations);
            console.log(`Pipeline ${pipelineId} replaced successfully.`);

        } catch (error: unknown) {
            console.error('Error replacing pipeline:', error);
            process.exit(1);
        }
    });

// --- Execution Inspection Commands ---

// Helper to convert ExecutionStatus enum to readable string
// Uses TypeScript's built-in enum reverse mapping (e.g., ExecutionStatus[5] === "STATUS_WAITING")
function executionStatusToString(status: number | undefined): string {
    if (status === undefined || status === null) return 'UNKNOWN';
    const name = ExecutionStatus[status];
    // Remove "STATUS_" prefix for cleaner display
    return name ? name.replace(/^STATUS_/, '') : `UNKNOWN(${status})`;
}

// Helper to print execution table uniformly
function printExecutionTable(executions: { id: string, data: ExecutionRecord }[]) {
    console.log('-------------------------------------------------------------------------------------------------------------------------------------');
    console.log('Timestamp               | ID                                   | Service                   | Status  | Trigger');
    console.log('-------------------------------------------------------------------------------------------------------------------------------------');
    executions.forEach(item => {
        const data = item.data;
        const time = data.timestamp instanceof Date ? data.timestamp.toISOString() :
            (data.timestamp as unknown as { toDate: () => Date })?.toDate ? (data.timestamp as unknown as { toDate: () => Date }).toDate().toISOString() : 'Unknown';
        const status = executionStatusToString(data.status);
        const service = (data.service || 'unknown').padEnd(25);
        const trigger = (data.triggerType || 'N/A').padEnd(7);

        // Simple padding and truncated output for table-like look
        const id = item.id.padEnd(36);
        const statusStr = status.padEnd(7);

        console.log(`${time.slice(0, 23)} | ${id} | ${service} | ${statusStr} | ${trigger}`);
    });
    console.log('-------------------------------------------------------------------------------------------------------------------------------------\n');
}

program
    .command('executions:list')
    .description('List recent executions')
    .option('-s, --service <service>', 'Filter by service name')
    .option('-st, --status <status>', 'Filter by status (STATUS_STARTED, STATUS_SUCCESS, STATUS_FAILED)')
    .option('-u, --user <userId>', 'Filter by user ID')
    .option('-l, --limit <number>', 'Number of records to show', '20')
    .action(async (options) => {
        try {
            const limit = parseInt(options.limit, 10);

            console.log(`Fetching up to ${limit} executions...`);
            const executions = await executionService.listExecutions({
                service: options.service,
                status: options.status, // Pass string status directly
                userId: options.user,
                limit
            });

            if (executions.length === 0) {
                console.log('No executions found matching criteria.');
                return;
            }

            console.log('\nFound ' + executions.length + ' executions:');
            printExecutionTable(executions);

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error listing executions: ${error.message}`);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('executions:list-watch')
    .description('Watch recent executions with real-time updates')
    .option('-s, --service <service>', 'Filter by service name')
    .option('-st, --status <status>', 'Filter by status (STATUS_STARTED, STATUS_SUCCESS, STATUS_FAILED)')
    .option('-u, --user <userId>', 'Filter by user ID')
    .option('-l, --limit <number>', 'Number of records to show', '20')
    .action(async (options) => {
        try {
            const limit = parseInt(options.limit, 10);

            console.log(`Watching up to ${limit} executions...`);
            console.log('Press Ctrl+C to stop.\n');

            const unsubscribe = executionService.watchExecutions({
                service: options.service,
                status: options.status, // Pass string status directly
                userId: options.user,
                limit
            }, (executions) => {
                // Clear screen and move cursor to top-left
                process.stdout.write('\x1b[2J\x1b[0;0H');

                console.log(`Watching Executions (Limit: ${limit}) | Service: ${options.service || 'All'} | Status: ${options.status || 'All'}`);
                console.log(`Last updated: ${new Date().toLocaleTimeString()}`);
                console.log('Press Ctrl+C to stop.\n');

                if (executions.length === 0) {
                    console.log('No executions found matching criteria.');
                    return;
                }

                printExecutionTable(executions);
            }, (error) => {
                console.error('Watch error:', error.message);
                process.exit(1);
            });

            // Keep the process alive. Inquirer or other tools might interfere, but simple action will wait for signals.
            process.on('SIGINT', () => {
                unsubscribe();
                console.log('\nStopped watching.');
                process.exit(0);
            });

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error starting watch: ${error.message}`);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('executions:latest')
    .description('Get the latest execution details')
    .option('-s, --service <service>', 'Filter by service name')
    .option('--status <status>', 'Filter by status (e.g. FAILED, SUCCESS)')
    .action(async (options) => {
        try {
            const executions = await executionService.listExecutions({
                service: options.service,
                status: options.status, // Pass raw string
                limit: 1
            });

            if (executions.length === 0) {
                console.log('No executions found matching criteria.');
                return;
            }

            const latest = executions[0];
            printExecutionDetails(latest.id, latest.data, false);

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error fetching execution: ${error.message}`);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('executions:latest-watch')
    .description('Watch for the latest execution and print full details')
    .option('-s, --service <service>', 'Filter by service name')
    .option('--status <status>', 'Filter by status (e.g. FAILED, SUCCESS, STARTED)')
    .action(async (options) => {
        try {
            console.log('Watching for executions...');
            if (options.service) console.log(`Filter: Service=${options.service}`);
            if (options.status) console.log(`Filter: Status=${options.status}`);
            console.log('Press Ctrl+C to stop.\n');

            const unsubscribe = executionService.watchExecutions({
                service: options.service,
                status: options.status, // Can be undefined, specific status, or raw string
                limit: 1
            }, (executions) => {
                if (executions.length > 0) {
                    const latest = executions[0];
                    // Always redraw if we have data, even if ID is same (to show status updates)

                    // Clear screen and move cursor to top-left
                    process.stdout.write('\x1b[2J\x1b[0;0H');

                    console.log(`Watching Latest Execution | Service: ${options.service || 'All'} | Status: ${options.status || 'All'}`);
                    console.log(`Last updated: ${new Date().toLocaleTimeString()}`);
                    console.log('Press Ctrl+C to stop.\n');

                    printExecutionDetails(latest.id, latest.data, false);
                }
            }, (error) => {
                console.error('Watch error:', error.message);
                process.exit(1);
            });

            // Keep the process alive.
            process.on('SIGINT', () => {
                unsubscribe();
                console.log('\nStopped watching.');
                process.exit(0);
            });

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error starting watch: ${error.message}`);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

// Helper to find user by pipeline ID
const findUserByPipelineId = async (pipelineId: string): Promise<UserRecord | null> => {
    // Inefficient scan, but acceptable for admin CLI/dev tools
    const users = await userService.listUsers();
    for (const user of users) {
        if (user.pipelines && user.pipelines.some(p => p.id === pipelineId)) {
            return user;
        }
    }
    return null;
};



// --- Test Automation Commands ---

const createTestIngressKey = async (userId: string): Promise<string> => {
    // Generate a new key specifically for testing
    // We won't check for existing ones because we can't retrieve their secrets.
    const token = `fg_sk_test_${userId}`;
    const hash = crypto.createHash('sha256').update(token).digest('hex');

    const existingKey = await apiKeyService.getByHash(hash);

    if (existingKey) {
        console.log(`✅ Test key already exists for user ${userId}.`);
    } else {
        console.log(`Creating test key for user ${userId}...`);
        await apiKeyService.create(hash, {
            label: `Auto Test Key (${new Date().toISOString()})`,
            scopes: ['read:activity'],
            userId,
            createdAt: new Date()
        });
    }

    return token;
};

const configureTestPipeline = async (pipelineId: string, behavior: 'success' | 'fail' | 'lag') => {
    console.log(`Searching for pipeline ${pipelineId}...`);
    const user = await findUserByPipelineId(pipelineId);

    if (!user) {
        console.error(`❌ Pipeline ${pipelineId} not found in any user.`);
        process.exit(1);
    }

    console.log(`Found pipeline in user ${user.userId}.`);

    // 1. Reconfigure Pipeline
    console.log(`Reconfiguring for test scenario: ${behavior}...`);
    const enrichers: EnricherConfig[] = [{
        providerType: EnricherProviderType.ENRICHER_PROVIDER_MOCK,
        inputs: {
            behavior: behavior,
            name: `Test Activity (${behavior})`,
            description: `Automated test with behavior: ${behavior}`
        }
    }];

    const destinations = ['mock'];

    await userService.replacePipeline(user.userId, pipelineId, 'SOURCE_TEST', enrichers, destinations);
    console.log(`✅ Pipeline ${pipelineId} reconfigured successfully.`);

    // 2. Create/Get Ingress Key
    console.log('Generating temporary Ingress Key for trigger...');
    const ingressKey = await createTestIngressKey(user.userId);

    // 3. Get URL
    const url = 'https://dev.fitglue.tech/hooks/test'; // Hardcoded as per user request (mapped in firebase.json)
    console.log(`Target URL: ${url}`);

    // 4. Trigger
    console.log('🚀 Triggering pipeline...');
    const payload = {
        id: `auto_${behavior}_${Date.now()}`
    };

    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${ingressKey}`,
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        if (response.ok) {
            console.log(`✅ Trigger successful! Status: ${response.status}`);
            const data = await response.text();
            console.log('Response:', data);
        } else {
            console.error(`❌ Trigger failed! Status: ${response.status}`);
            const text = await response.text();
            console.error('Response:', text);
        }
    } catch (e: unknown) {
        const errorMessage = e instanceof Error ? e.message : String(e);
        console.error(`❌ Error triggering pipeline: ${errorMessage}`);
    }
};

program
    .command('test:success')
    .argument('<pipelineId>', 'Pipeline ID to configure')
    .description('Configure pipeline for successful mock execution')
    .action(async (pipelineId) => {
        await configureTestPipeline(pipelineId, 'success');
    });

program
    .command('test:fail')
    .argument('<pipelineId>', 'Pipeline ID to configure')
    .description('Configure pipeline for failed mock execution')
    .action(async (pipelineId) => {
        await configureTestPipeline(pipelineId, 'fail');
    });

program
    .command('test:lag')
    .argument('<pipelineId>', 'Pipeline ID to configure')
    .description('Configure pipeline for lagged mock execution and trigger it')
    .action(async (pipelineId) => {
        await configureTestPipeline(pipelineId, 'lag');
    });

// Recursively truncate large arrays/objects for display
function truncateData(obj: unknown, verbose: boolean = false, depth: number = 0): unknown {
    if (verbose) return obj;
    if (depth > 10) return '[Max Depth]';
    if (!obj) return obj;

    if (Array.isArray(obj)) {
        if (obj.length > 3) {
            // Keep first 3, indicate truncation
            const truncated = obj.slice(0, 3).map(item => truncateData(item, verbose, depth + 1));
            truncated.push(`... ${obj.length - 3} more items hidden (use --verbose to see all) ...`);
            return truncated;
        }
        return obj.map(item => truncateData(item, verbose, depth + 1));
    }

    if (typeof obj === 'object' && obj !== null) {
        const newObj: Record<string, unknown> = {};
        for (const [key, value] of Object.entries(obj as Record<string, unknown>)) {
            // Special handling for large arrays commonly found in FIT data
            if (['sessions', 'laps', 'records', 'points'].includes(key) && Array.isArray(value)) {
                newObj[key] = truncateData(value, verbose, depth + 1);
            } else {
                newObj[key] = truncateData(value, verbose, depth + 1);
            }
        }
        return newObj;
    }

    return obj;
}

// Helper to print full execution details
function printExecutionDetails(executionId: string, data: ExecutionRecord, verbose: boolean) {
    console.log('\n==========================================');
    console.log('EXECUTION DETAILS');
    console.log('==========================================');
    console.log(`ID:            ${executionId}`);
    console.log(`Service:       ${data.service || 'N/A'}`);
    console.log(`Status:        ${executionStatusToString(data.status)}`);
    console.log(`Trigger:       ${data.triggerType || 'N/A'}`);
    console.log(`User ID:       ${data.userId || 'N/A'}`);
    console.log(`Test Run ID:   ${data.testRunId || 'N/A'}`);
    console.log('------------------------------------------');
    console.log(`Timestamp:     ${data.timestamp instanceof Date ? data.timestamp.toISOString() : 'N/A'}`);
    console.log(`Start Time:    ${data.startTime instanceof Date ? data.startTime.toISOString() : 'N/A'}`);
    console.log(`End Time:      ${data.endTime instanceof Date ? data.endTime.toISOString() : 'N/A'}`);

    if (data.errorMessage) {
        console.log('------------------------------------------');
        console.log(`ERROR:         ${data.errorMessage}`);
    }

    console.log('------------------------------------------');

    if (data.inputsJson) {
        console.log('\n[INPUTS]');
        try {
            const parsed = JSON.parse(data.inputsJson);

            // Format Activity Type in Inputs
            if (parsed.activity_type) {
                parsed.activity_type = formatActivityType(parsed.activity_type);
            }
            if (parsed.result?.activity_type) {
                parsed.result.activity_type = formatActivityType(parsed.result.activity_type);
            }

            console.dir(truncateData(parsed, verbose), { depth: null, colors: true });
        } catch {
            console.log(data.inputsJson);
        }
    } else {
        console.log('\n[INPUTS] (None)');
    }

    if (data.outputsJson) {
        console.log('\n[OUTPUTS]');
        try {
            const parsed = JSON.parse(data.outputsJson);

            // Format Activity Type in Outputs
            if (parsed.activity_type) {
                parsed.activity_type = formatActivityType(parsed.activity_type);
            }
            if (parsed.result?.activity_type) {
                parsed.result.activity_type = formatActivityType(parsed.result.activity_type);
            }

            console.dir(truncateData(parsed, verbose), { depth: null, colors: true });
        } catch {
            console.log(data.outputsJson);
        }
    }

    // Raw metadata dump if needed
    if (Object.keys(data).some(k => !['executionId', 'service', 'status', 'timestamp', 'userId', 'triggerType', 'startTime', 'endTime', 'errorMessage', 'inputsJson', 'outputsJson', 'testRunId'].includes(k))) {
        console.log('\n[OTHER METADATA]');
        const other: Record<string, unknown> = { ...data } as unknown as Record<string, unknown>;
        delete other.executionId;
        delete other.service;
        delete other.status;
        delete other.timestamp;
        delete other.userId;
        delete other.triggerType;
        delete other.startTime;
        delete other.endTime;
        delete other.errorMessage;
        delete other.inputsJson;
        delete other.outputsJson;
        delete other.testRunId;
        console.dir(other, { depth: null, colors: true });
    }

    console.log('==========================================\n');
}

program
    .command('executions:get <executionId>')
    .description('Get full details of a specific execution')
    .option('-v, --verbose', 'Show full execution details')
    .action(async (executionId, options) => {
        try {
            const execution = await executionService.get(executionId);
            if (!execution) {
                console.log(`Execution ${executionId} not found.`);
                process.exit(1);
            }

            printExecutionDetails(executionId, execution, options.verbose || false);

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error getting execution: ${error.message}`);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('executions:get-by-pipeline <pipelineExecutionId>')
    .description('Get all executions for a specific pipeline run')
    .action(async (pipelineExecutionId) => {
        try {
            console.log(`Fetching pipeline execution tree for ${pipelineExecutionId}...`);
            const executions = await executionService.listByPipeline(pipelineExecutionId);

            if (executions.length === 0) {
                console.log('No executions found.');
                return;
            }

            console.log(`\nFound ${executions.length} executions in pipeline run:`);
            printExecutionTable(executions);

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error getting pipeline executions: ${error.message}`);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('executions:create <executionId>')
    .description('Create a test execution record')
    .option('-s, --service <service>', 'Service name', 'test-service')
    .option('-t, --trigger <trigger>', 'Trigger type', 'http')
    .option('-u, --user <userId>', 'User ID')
    .action(async (executionId, options) => {
        try {
            console.log(`Creating execution ${executionId}...`);

            // Match logExecutionPending exactly
            await executionService.create(executionId, {
                executionId,
                service: options.service,
                triggerType: options.trigger,
                timestamp: new Date(),
                status: 4 // STATUS_PENDING
            });

            console.log('Execution created successfully.');
        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error creating execution: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('executions:update <executionId>')
    .description('Update an execution record')
    .option('--status <status>', 'Status (0-4)', '2')
    .option('--error <message>', 'Error message')
    .option('--inputs <json>', 'Inputs JSON')
    .option('--outputs <json>', 'Outputs JSON')
    .action(async (executionId, options) => {
        try {
            console.log(`Updating execution ${executionId}...`);

            const updateData: Record<string, unknown> = {
                status: parseInt(options.status, 10)
            };

            if (options.error) {
                updateData.errorMessage = options.error;
                updateData.endTime = new Date();
            }

            if (options.inputs) {
                updateData.inputsJson = options.inputs;
            }

            if (options.outputs) {
                updateData.outputsJson = options.outputs;
            }

            await executionService.update(executionId, updateData);

            console.log('Execution updated successfully.');
        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error updating execution: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('executions:clean')
    .description('Delete ALL execution logs from the database')
    .option('-f, --force', 'Force deletion without prompt')
    .action(async (options) => {
        try {
            if (!options.force) {
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
            }

            if (!options.force) {
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
            }

            console.log('Deleting all executions...');
            const deletedCount = await executionService.deleteAllExecutions();
            console.log(`Successfully deleted ${deletedCount} executions.`);

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error cleaning executions: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

// --- Bucket Commands ---

const formatBucket = (bucket: { name: string; metadata: BucketMetadata; }): void => {
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

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error listing buckets: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
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

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error getting bucket: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('buckets:from-execution <executionId>')
    .description('Get details of the bucket used in a specific execution')
    .action(async (executionId) => {
        try {
            console.log(`Fetching execution ${executionId}...`);
            const data = await executionService.get(executionId);
            if (!data) {
                console.error(`Execution ${executionId} not found.`);
                process.exit(1);
            }
            let fitFileUri = null

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
                if (!fitFileUri && data.inputsJson) {
                    try {
                        const inputs = JSON.parse(data.inputsJson);
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

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error getting bucket from execution: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
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

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error downloading file: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

program
    .command('files:download-execution <executionId> [localPath]')
    .description('Download a file used in a specific execution')
    .action(async (executionId, localPath) => {
        try {
            console.log(`Fetching execution ${executionId}...`);
            const data = await executionService.get(executionId);
            if (!data) {
                console.error(`Execution ${executionId} not found.`);
                process.exit(1);
            }
            const uris = new Set<string>();

            // Recursive function to find gs:// URIs in objects/strings
            const findUris = (obj: unknown) => {
                if (typeof obj === 'string') {
                    if (obj.startsWith('gs://')) {
                        uris.add(obj);
                    }
                } else if (typeof obj === 'object' && obj !== null) {
                    Object.values(obj as Record<string, unknown>).forEach(findUris);
                }
            };

            // Scan top level
            findUris(data);

            // Scan inputs/outputs specifically if they are JSON strings
            if (data.inputsJson) {
                try {
                    findUris(JSON.parse(data.inputsJson));
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

        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error(`❌ Error downloading from execution: ${error.message}`);
                console.error('Stack:', error.stack);
            } else {
                console.error(`❌ An unknown error occurred`);
            }
            process.exit(1);
        }
    });

// Register replay commands
import { registerReplayCommands } from './commands/replay';
import { BucketMetadata } from '@google-cloud/storage';
registerReplayCommands(program);

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

        // Build choice list from ActivityType enum, filtering out non-strings or internal keys
        const activityTypeChoices = Object.keys(ActivityType)
            .filter(k => isNaN(Number(k))) // Filter out reverse mapping (numbers)
            .map(k => ({
                name: formatActivityType(k), // Use our helper for nice names
                value: ActivityType[k as keyof typeof ActivityType] // Enum value (number) or string if needed
            }))
            .sort((a, b) => a.name.localeCompare(b.name));

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
                    choices: activityTypeChoices
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
                // We need to store the *string* representation (e.g. "ACTIVITY_TYPE_RUN")
                // because type_mapper.go expects that or friendly name.
                // ActivityType[number] returns the string key.
                target_type: typeof ruleAnswers.targetType === 'number' ? ActivityType[ruleAnswers.targetType] : ruleAnswers.targetType
            });
            addRule = ruleAnswers.addAnother;
        }
        inputs = {
            rules: JSON.stringify(rules)
        };
    } else if (providerType === EnricherProviderType.ENRICHER_PROVIDER_CONDITION_MATCHER) {
        // Reuse choice list logic
        const activityTypeChoices = Object.keys(ActivityType)
            .filter(k => isNaN(Number(k)))
            .map(k => ({
                name: formatActivityType(k),
                value: ActivityType[k as keyof typeof ActivityType]
            }))
            .sort((a, b) => a.name.localeCompare(b.name));

        const cmConfig = await inquirer.prompt([
            {
                type: 'list',
                name: 'activityType',
                message: 'Activity Type Condition:',
                choices: [{ name: '(Any)', value: '' }, ...activityTypeChoices],
                default: ''
            },
            {
                type: 'checkbox',
                name: 'days',
                message: 'Days of Week:',
                choices: [
                    { name: 'Monday', value: '1' },
                    { name: 'Tuesday', value: '2' },
                    { name: 'Wednesday', value: '3' },
                    { name: 'Thursday', value: '4' },
                    { name: 'Friday', value: '5' },
                    { name: 'Saturday', value: '6' },
                    { name: 'Sunday', value: '0' }
                ]
            },
            {
                type: 'input',
                name: 'startTime',
                message: 'Start Time (HH:MM):',
                validate: (val) => !val || /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/.test(val) || 'Invalid time format'
            },
            {
                type: 'input',
                name: 'endTime',
                message: 'End Time (HH:MM):',
                validate: (val) => !val || /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/.test(val) || 'Invalid time format'
            },
            {
                type: 'input',
                name: 'locationCoords',
                message: 'Location (optional, "Lat,Long"):',
                validate: (input) => !input || /^-?\d+(\.\d+)?,-?\d+(\.\d+)?$/.test(input) || 'Invalid format'
            },
            {
                type: 'input',
                name: 'radius',
                message: 'Radius (m) (default 200):',
                when: (answers) => !!answers.locationCoords
            },
            {
                type: 'input',
                name: 'titleTemplate',
                message: 'Title Template (optional):',
            },
            {
                type: 'input',
                name: 'descTemplate',
                message: 'Description Template (optional):',
            }
        ]);

        inputs = {};
        if (cmConfig.activityType) inputs.activity_type = typeof cmConfig.activityType === 'number' ? ActivityType[cmConfig.activityType] : cmConfig.activityType;
        if (cmConfig.days.length > 0) inputs.days_of_week = cmConfig.days.join(',');
        if (cmConfig.startTime) inputs.start_time = cmConfig.startTime;
        if (cmConfig.endTime) inputs.end_time = cmConfig.endTime;
        if (cmConfig.locationCoords) {
            const parts = cmConfig.locationCoords.split(',');
            inputs.location_lat = parts[0].trim();
            inputs.location_lng = parts[1].trim();
            inputs.location_radius = cmConfig.radius || '200';
        }
        if (cmConfig.titleTemplate) inputs.title_template = cmConfig.titleTemplate;
        if (cmConfig.descTemplate) inputs.description_template = cmConfig.descTemplate;
    } else if (providerType === EnricherProviderType.ENRICHER_PROVIDER_PARKRUN) {
        const parkrunConfig = await inquirer.prompt([
            {
                type: 'confirm',
                name: 'enableTitling',
                message: 'Enable Activity Titling (e.g. "Bushy Park Parkrun")?',
                default: true
            },
            {
                type: 'input',
                name: 'tags',
                message: 'Tags to add (comma-separated):',
                default: 'Race'
            }
        ]);
        inputs = {
            enable_titling: parkrunConfig.enableTitling.toString(),
            tags: parkrunConfig.tags
        };

    } else if (providerType === EnricherProviderType.ENRICHER_PROVIDER_AUTO_INCREMENT) {
        const incrementConfig = await inquirer.prompt([
            {
                type: 'input',
                name: 'key',
                message: 'Counter Key (required, unique ID e.g. "parkrun_bushy"):',
                validate: (input) => input.length > 0 || 'Required'
            },
            {
                type: 'input',
                name: 'filter',
                message: 'Title Filter (required, substring match):',
                validate: (input) => input.length > 0 || 'Required'
            },
            {
                type: 'input',
                name: 'initialValue',
                message: 'Starting Number (optional, default 1):',
                validate: (input) => !input || !isNaN(parseInt(input)) || 'Must be a number'
            }
        ]);
        inputs = {
            counter_key: incrementConfig.key,
            title_contains: incrementConfig.filter,
            initial_value: incrementConfig.initialValue
        };
    } else if (providerType === EnricherProviderType.ENRICHER_PROVIDER_USER_INPUT) {
        const userConfig = await inquirer.prompt([
            {
                type: 'checkbox',
                name: 'fields',
                message: 'Fields to ask user for:',
                choices: ['title', 'description'],
                default: ['title']
            }
        ]);
        inputs = {
            fields: userConfig.fields.join(',')
        };
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
    // Clean up empty inputs
    Object.keys(inputs).forEach(key => {
        if (inputs[key] === '' || inputs[key] === undefined) {
            delete inputs[key];
        }
    });

    return inputs;
}
