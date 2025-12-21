const http = require('http');

console.log('Triggering Router (EnrichedActivityEvent)...');

// Mock EnrichedActivityEvent (pb.EnrichedActivityEvent)
const enrichedEvent = {
  user_id: "user_router_123",
  activity_id: "act_456",
  gcs_uri: "gs://fitglue-artifacts/activities/user_router_123/123.fit",
  description: "Enriched Activity #Power"
};

const dataBuffer = Buffer.from(JSON.stringify(enrichedEvent));
const pubSubMessage = {
  message: {
    data: dataBuffer.toString('base64'),
    messageId: "67890",
    publishTime: new Date().toISOString()
  }
};

const options = {
  hostname: 'localhost',
  port: 8082,
  path: '/',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Ce-Id': '5678',
    'Ce-Specversion': '1.0',
    'Ce-Type': 'google.cloud.pubsub.topic.v1.messagePublished',
    'Ce-Source': '//pubsub.googleapis.com/projects/fitglue-local/topics/topic-enriched-activity',
  }
};

const req = http.request(options, (res) => {
  console.log(`STATUS: ${res.statusCode}`);
  res.setEncoding('utf8');
  res.on('data', (chunk) => console.log(`BODY: ${chunk}`));
});

req.on('error', (e) => {
  console.error(`problem with request: ${e.message}`);
});

req.write(JSON.stringify(pubSubMessage));
req.end();
