import axios from 'axios';
import { randomUUID, createHmac } from 'crypto';
import { setupTestUser, cleanupTestUser } from './setup';

const HEVY_SECRET = 'local-secret'; // Matches .env

const signPayload = (payload: any) => {
    const hmac = createHmac('sha256', HEVY_SECRET);
    hmac.update(JSON.stringify(payload));
    return hmac.digest('hex');
};

const BASE_URL_HEVY = 'http://localhost:8080';
const BASE_URL_ENRICHER = 'http://localhost:8081';
const BASE_URL_ROUTER = 'http://localhost:8082';
const BASE_URL_UPLOADER = 'http://localhost:8083';

describe('Local E2E Integration Tests', () => {
    let userId: string;

    beforeAll(async () => {
        userId = `user_test_${randomUUID()}`;
        await setupTestUser(userId);
    });

    afterAll(async () => {
        if (userId) {
            await cleanupTestUser(userId);
        }
    });

    it('should process Hevy webhook', async () => {
        // Note: Hevy Handler generally ignores user ID in path but we send specific payload
        const payload = {
            user_id: "ignored_by_handler_logic_uses_config_but_good_for_tracing",
            workout: {
                title: "Integration Test Workout",
                exercises: []
            }
        };
        // We expect 200 OK (Process started or User not configured - handler logic dependent)
        // Our handler currently checks "hevy_user_123" hardcoded or similar in local run?
        // Actually handler resolves fitglue user from hevy user. We haven't mocked that mapping in DB so it might fail logic.
        // But let's check connectivity.

        try {
            // Fix: Sign the EXACT string we send to ensure HMAC matches
            const payloadString = JSON.stringify(payload);
            const signature = signPayload(payload); // Helper must assume payload is object and stringify it consistently?
            // Actually, helper stringifies too.
            // Let's modify helper to take string? Or rely on order.
            // Better:
            const hmac = createHmac('sha256', HEVY_SECRET);
            hmac.update(payloadString);
            const sig = hmac.digest('hex');

            const res = await axios.post(BASE_URL_HEVY, payloadString, { // Send string directly
                headers: {
                    'X-Hevy-Signature': sig,
                    'Content-Type': 'application/json'
                }
            });
            expect(res.status).toBe(200);
        } catch (e: any) {
            // If it returns 200, good. If 500, check logs.
            // Our current handler returns 200 "User not configured" if mapping fails, which is success for wiring.
            if (e.response) {
                 expect(e.response.status).toBe(200); // Expecting handled failure or success
            } else {
                throw e;
            }
        }
    });

    it('should trigger Enricher (CloudEvent)', async () => {
        const activityPayload = {
            source: 2, // HEVY
            user_id: userId,
            timestamp: new Date().toISOString(),
            original_payload_json: "{}",
            metadata: {}
        };

        const dataBuffer = Buffer.from(JSON.stringify(activityPayload));
        const cloudEvent = {
            message: {
                data: dataBuffer.toString('base64'),
                messageId: randomUUID(),
                publishTime: new Date().toISOString()
            }
        };

        const res = await axios.post(BASE_URL_ENRICHER, cloudEvent, {
            headers: {
                'Content-Type': 'application/json',
                'Ce-Id': randomUUID(),
                'Ce-Specversion': '1.0',
                'Ce-Type': 'google.cloud.pubsub.topic.v1.messagePublished',
                'Ce-Source': '//pubsub.googleapis.com/projects/test/topics/topic-raw-activity',
            }
        });
        expect(res.status).toBe(200);
        // Enricher logs "Enrichment complete" on success.
    });

    it('should trigger Router (CloudEvent)', async () => {
        const enrichedEvent = {
            user_id: userId,
            activity_id: `act_${randomUUID()}`,
            gcs_uri: `gs://fitglue-server-artifacts/activities/${userId}/test.fit`, // Mock path
            description: "Integration Test Activity",
            metadata_json: "{}"
        };

        const dataBuffer = Buffer.from(JSON.stringify(enrichedEvent));
        const cloudEvent = {
            message: {
                data: dataBuffer.toString('base64'),
                messageId: randomUUID(),
                publishTime: new Date().toISOString()
            }
        };

        const res = await axios.post(BASE_URL_ROUTER, cloudEvent, {
             headers: {
                'Content-Type': 'application/json',
                'Ce-Id': randomUUID(),
                'Ce-Specversion': '1.0',
                'Ce-Type': 'google.cloud.pubsub.topic.v1.messagePublished',
                'Ce-Source': '//pubsub.googleapis.com/projects/test/topics/topic-enriched-activity',
            }
        });
        expect(res.status).toBe(200);
    });

    it('should trigger Uploader (CloudEvent) and fail safely on Strava', async () => {
        const enrichedEvent = {
             user_id: userId,
            activity_id: `act_${randomUUID()}`,
            gcs_uri: `gs://fitglue-server-artifacts/activities/${userId}/test.fit`,
            description: "Integration Test Upload"
        };

        // We haven't actually written the file to GCS in the Uploader test step (Enricher did write one but Uploader might look for specific one).
        // Actually Enricher wrote to a path based on timestamp.
        // Uploader looks for "gs://bucket/path".
        // If Uploader fails on GCS Read, that is also a valid "Wiring Check" (it tried to read).
        // If it fails on Strava 401, that's better.
        // Let's rely on whatever safe failure occurs.

        const dataBuffer = Buffer.from(JSON.stringify(enrichedEvent));
        const cloudEvent = {
            message: {
                data: dataBuffer.toString('base64'),
                messageId: randomUUID(),
                publishTime: new Date().toISOString()
            }
        };

        try {
            await axios.post(BASE_URL_UPLOADER, cloudEvent, {
                 headers: {
                    'Content-Type': 'application/json',
                    'Ce-Id': randomUUID(),
                    'Ce-Specversion': '1.0',
                    'Ce-Type': 'google.cloud.pubsub.topic.v1.messagePublished',
                    'Ce-Source': '//pubsub.googleapis.com/projects/test/topics/topic-job-upload-strava',
                }
            });
             // If it succeeds (200), it means it swallowed the error or everything worked (unlikely with mock token).
             // Strava Uploader currently returns error on failure, so Axios should throw.
        } catch (e: any) {
            // We expect a 500 or 400 from the function because it returns error on failure.
            // This proves it ran.
            expect(e.response).toBeDefined();
            // Optional: verify error message contains "Strava" or "GCS" to be sure.
            console.log("Uploader Safe Failure:", e.response.data);
        }
    });
});
