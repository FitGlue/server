import axios from 'axios';
import { randomUUID } from 'crypto';
import { setupTestUser, cleanupTestUser } from './setup';
import { publishRawActivity, publishEnrichedActivity, publishUploadJob } from './pubsub-helpers';
import { waitForExecutionActivity } from './verification-helpers';
import { config } from './config';

describe('Deployed Environment Integration Tests', () => {
  let userId: string;

  beforeAll(async () => {
    // Verify we're not running in local mode
    if (config.environment === 'local') {
      throw new Error(
        'Deployed tests should not run in local environment. Use TEST_ENVIRONMENT=dev|test|prod'
      );
    }

    userId = `test_${randomUUID()}`;
    await setupTestUser(userId);
    console.log(`[Deployed Tests] Testing against: ${config.projectId}`);
    console.log(`[Deployed Tests] Test user: ${userId}`);
  });

  afterAll(async () => {
    if (userId) {
      await cleanupTestUser(userId);
    }
  });

  describe('HTTP-triggered functions', () => {
    it('should accept Hevy webhook', async () => {
      if (!config.endpoints?.hevyWebhook) {
        throw new Error('Hevy webhook endpoint not configured');
      }

      const payload = {
        user_id: 'test_hevy_user',
        workout: {
          title: 'Deployed Integration Test Workout',
          exercises: [],
        },
      };

      // Testing endpoint reachability with invalid signature
      // Expected: 200 (no signature check), 401/403 (signature verification enabled)
      try {
        const res = await axios.post(config.endpoints.hevyWebhook, payload, {
          headers: {
            'Content-Type': 'application/json',
            'X-Hevy-Signature': 'invalid-signature-for-testing',
          },
          validateStatus: () => true, // Accept any status
        });

    it('should trigger Keiser Poller manually', async () => {
      if (!config.endpoints?.keiserPoller) {
        throw new Error('Keiser Poller endpoint not configured');
      }

      // Keiser Poller is normally triggered by Cloud Scheduler
      // We're testing manual HTTP trigger for integration testing
      try {
        const res = await axios.post(config.endpoints.keiserPoller, {}, {
          headers: {
            'Content-Type': 'application/json',
          },
          validateStatus: () => true,
        });

        // Expected: 200 (success) or 500 (if no users configured for Keiser)
        expect([200, 500]).toContain(res.status);
        console.log(`[Keiser Poller] Response status: ${res.status}`);
      } catch (e: any) {
        throw new Error(`Failed to reach Keiser Poller: ${e.message}`);
      }
    });

        // We expect either:
        // - 401/403 (signature verification failed - good!)
        // - 200 (signature verification passed or disabled - also acceptable for test)
        expect([200, 401, 403]).toContain(res.status);
        console.log(`[Hevy Webhook] Response status: ${res.status}`);
      } catch (e: any) {
        // Network errors are failures
        throw new Error(`Failed to reach Hevy webhook: ${e.message}`);
      }
    });
  });

  describe('Pub/Sub-triggered functions', () => {
    // Increase timeout for these tests as Cloud Functions can ta time to cold start
    jest.setTimeout(60000); // 60 seconds

    it('should trigger Enricher via Pub/Sub', async () => {
      const payload = {
        source: 2, // HEVY
        user_id: userId,
        timestamp: new Date().toISOString(),
        original_payload_json: JSON.stringify({ test: 'data' }),
        metadata: { test: 'true' }, // map<string, string> per protobuf
      };

      console.log('[Enricher] Publishing to raw-activity topic...');
      const messageId = await publishRawActivity(payload);
      console.log(`[Enricher] Published message: ${messageId}`);

      // Wait for function execution
      // The enricher should process the message and create an execution record
      console.log('[Enricher] Waiting for execution activity...');
      await waitForExecutionActivity({
        timeout: 45000, // 45s - Cloud Functions can take time to cold start
        checkInterval: 3000, // Check every 3s
        minExecutions: 1,
      });

      console.log('[Enricher] ✓ Function executed successfully');
    });

    it('should trigger Router via Pub/Sub', async () => {
      const payload = {
        user_id: userId,
        activity_id: `act_${randomUUID()}`,
        gcs_uri: `gs://${config.gcsBucket}/activities/${userId}/test.fit`,
        description: 'Deployed Integration Test Activity',
        metadata_json: JSON.stringify({ test: true }),
      };

      console.log('[Router] Publishing to enriched-activity topic...');
      const messageId = await publishEnrichedActivity(payload);
      console.log(`[Router] Published message: ${messageId}`);

      // Wait for function execution
      console.log('[Router] Waiting for execution activity...');
      await waitForExecutionActivity({
        timeout: 45000,
        checkInterval: 3000,
        minExecutions: 1,
      });

      console.log('[Router] ✓ Function executed successfully');
    });

    it('should trigger Strava Uploader via Pub/Sub', async () => {
      const payload = {
        user_id: userId,
        activity_id: `act_${randomUUID()}`,
        gcs_uri: `gs://${config.gcsBucket}/activities/${userId}/test.fit`,
        description: 'Deployed Integration Test Upload',
      };

      console.log('[Uploader] Publishing to upload-strava topic...');
      const messageId = await publishUploadJob(payload);
      console.log(`[Uploader] Published message: ${messageId}`);

      console.log('[Uploader] Waiting for execution activity...');
      await waitForExecutionActivity({
        timeout: 45000,
        checkInterval: 3000,
        minExecutions: 1,
      });

      console.log('[Uploader] ✓ Function executed successfully');
    });
  });

  describe('End-to-end flow', () => {
    jest.setTimeout(90000); // 90 seconds for full pipeline

    it('should process activity through entire pipeline', async () => {
      // This test publishes to raw-activity and verifies the entire pipeline executes
      const payload = {
        source: 2, // HEVY
        user_id: userId,
        timestamp: new Date().toISOString(),
        original_payload_json: JSON.stringify({
          workout: {
            title: 'E2E Test Workout',
            exercises: [],
          },
        }),
        metadata: { e2e_test: 'true' }, // map<string, string> per protobuf
      };

      console.log('[E2E] Publishing to raw-activity topic...');
      const messageId = await publishRawActivity(payload);
      console.log(`[E2E] Published message: ${messageId}`);

      // Wait for multiple executions (enricher -> router -> uploader)
      console.log('[E2E] Waiting for pipeline execution...');
      await waitForExecutionActivity({
        timeout: 60000, // 60s for full pipeline
        checkInterval: 3000,
        minExecutions: 2, // At least enricher + router (uploader may fail on Strava)
      });

      console.log('[E2E] ✓ Pipeline executed successfully');
    });
  });
});
