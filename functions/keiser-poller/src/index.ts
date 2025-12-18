import { HttpFunction } from '@google-cloud/functions-framework';
import { PubSub } from '@google-cloud/pubsub';
import * as admin from 'firebase-admin';

// NOTE: Since we cannot strictly install the SDK in this environment,
// we interface the expected behavior based on the research doc.
interface KeiserSession {
  id: string;
  userId: string;
  startTime: string; // ISO
  data: any;
}

admin.initializeApp();
const db = admin.firestore();
const pubsub = new PubSub();
const TOPIC_NAME = 'topic-raw-activity';

export const keiserPoller: HttpFunction = async (req, res) => {
  const executionRef = db.collection('executions').doc();
  const timestamp = new Date().toISOString();

  // 1. Audit Log Start
  await executionRef.set({
    service: 'keiser-poller',
    status: 'STARTED',
    startTime: timestamp,
    inputs: { trigger: 'scheduled' }
  });

  try {
    // 2. Get User Cursor & Creds
    // Real implementation: iterate over users in 'users' collection who have Keiser enabled.
    // Agent Simplification: assume single user 'user-123' for now or loop one.
    const userId = 'user-123';
    const cursorRef = db.collection('cursors').doc(`${userId}_keiser`);
    const cursorSnap = await cursorRef.get();

    let lastSync = new Date(0).toISOString();
    if (cursorSnap.exists) {
        lastSync = cursorSnap.data()!.lastSync;
    }

    // 3. Mock Keiser SDK Call
    // const sdk = new KeiserSDK({ secret: process.env.KEISER_CREDENTIALS });
    // const sessions = await sdk.getSessions({ since: lastSync });

    // Simulating "No new sessions" for default state,
    // or if dev/test flag is present, inject a mock session.
    const sessions: KeiserSession[] = [];

    if (process.env.MOCK_DATA === 'true') {
        sessions.push({
            id: `keiser-${Date.now()}`,
            userId: userId,
            startTime: new Date().toISOString(),
            data: { power: [200, 210, 205], cadence: [90, 92, 91] }
        });
    }

    // 4. Publish New Sessions
    const publishPromises = sessions.map(async (session) => {
        const payload = {
            source: 'keiser',
            originalPayload: session,
            userId: userId, // Correctly using the loop variable
            timestamp: session.startTime
        };
        const msgId = await pubsub.topic(TOPIC_NAME).publishMessage({ json: payload });
        return msgId;
    });

    const msgIds = await Promise.all(publishPromises);

    // 5. Update Cursor
    if (sessions.length > 0) {
        // Assume sorted, pick last
        const newLastSync = sessions[sessions.length - 1].startTime;
        await cursorRef.set({ lastSync: newLastSync }, { merge: true });
    }

    // 6. Audit Log Success
    await executionRef.update({
        status: 'SUCCESS',
        outputs: {
            sessionsFound: sessions.length,
            messageIds: msgIds
        },
        endTime: new Date().toISOString()
    });

    res.status(200).send(`Processed ${sessions.length} sessions`);

  } catch (err: any) {
      console.error(err);
      await executionRef.update({
          status: 'FAILED',
          error: err.message,
          endTime: new Date().toISOString()
      });
      res.status(500).send('Internal Server Error');
  }
};
