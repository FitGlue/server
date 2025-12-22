import { initializeApp } from 'firebase-admin/app';
import { getFirestore } from 'firebase-admin/firestore';
// Import directly from source for ts-node (experimental, requires ts-node to handle it or configured paths).
// If this fails, we will point to build.
import { UserService } from '../src/typescript/shared/src/index';

// Initialize Firebase
const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-local';
initializeApp({
    projectId
});

const db = getFirestore();
const userService = new UserService(db);

// Minimal CLI logic
async function main() {
    const args = process.argv.slice(2);
    const command = args[0];

    try {
        if (command === 'create-user') {
            const userId = args[1];
            if (!userId) throw new Error('Usage: create-user <userId>');

            await userService.createUser(userId);
            console.log(`User ${userId} created (or already exists).`);

        } else if (command === 'create-key') {
            const userId = args[1];
            const label = args[2] || 'Default Key';
            const scopeStr = args[3] || '';
            const scopes = scopeStr ? scopeStr.split(',') : [];

            if (!userId) throw new Error('Usage: create-key <userId> [label] [scope1,scope2]');

            const apiKey = await userService.createIngressApiKey(userId, label, scopes);

            console.log(`\n=== API KEY CREATED ===`);
            console.log(`User: ${userId}`);
            console.log(`Label: ${label}`);
            console.log(`Scopes: ${scopes.join(', ')}`);
            console.log(`KEY: ${apiKey}`);
            console.log(`(Store this key securely! We only store the hash.)\n`);

        } else if (command === 'set-hevy-key') {
            const userId = args[1];
            const key = args[2];
            const hevyUserId = args[3];

            if (!userId || !key) throw new Error('Usage: set-hevy-key <userId> <hevyApiKey> [hevyUserId]');

            await userService.setHevyIntegration(userId, key, hevyUserId);
            console.log(`Hevy configuration updated for user ${userId}`);

        } else {
            console.log('Available commands:');
            console.log('  create-user <userId>');
            console.log('  create-key <userId> [label] [scopes]');
            console.log('  set-hevy-key <userId> <hevyApiKey> [hevyUserId]');
        }
    } catch (e: any) {
        console.error('Error:', e.message);
        process.exit(1);
    }
}

if (require.main === module) {
    main();
}
