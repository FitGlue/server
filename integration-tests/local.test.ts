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
        const payload = {
            user_id: "test_user",
            workout: {
                title: "Integration Test Workout",
                exercises: []
            }
        };
        try {
            const payloadString = JSON.stringify(payload);
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
            if (e.response) {
                 expect(e.response.status).toBe(200);
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
        } catch (e: any) {
            // Expected: function returns error on failure
            expect(e.response).toBeDefined();
            console.log("Uploader Safe Failure:", e.response.data);
        }
    });
});
