import axios from 'axios';
import { createHmac } from 'crypto';

// Testing hypothesis: The function uses the PATH string as the secret key because TF injected it as a plain env var.
const HEVY_SECRET = process.env.HEVY_SECRET || 'verifier-secret-value';
const TARGET_URL = process.env.TARGET_URL || 'https://hevy-handler-56cqxmt5jq-uc.a.run.app';

async function main() {
    console.log(`Verifying Cloud Deployment: ${TARGET_URL}`);

    const payload = {
        user_id: "verify_user_cloud",
        workout: {
            title: "Cloud Verification Workout",
            exercises: [
                { title: "Pushups", sets: [{ weight: 0, reps: 10 }] }
            ]
        }
    };

    const payloadString = JSON.stringify(payload);
    const hmac = createHmac('sha256', HEVY_SECRET);
    hmac.update(payloadString);
    const signature = hmac.digest('hex');

    console.log('Sending payload...');
    try {
        const res = await axios.post(TARGET_URL, payload, {
            headers: {
                'X-Hevy-Signature': signature,
                'Content-Type': 'application/json'
            }
        });
        console.log(`Response: ${res.status} ${res.statusText}`);
        console.log('Body:', res.data);
    } catch (e: any) {
        console.error('Error:', e.message);
        if (e.response) {
            console.error('Data:', e.response.data);
        }
    }
}

main();
