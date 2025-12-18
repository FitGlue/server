const http = require('http');

console.log('Triggering Strava Uploader (EnrichedActivityEvent)...');

// Mock EnrichedActivityEvent (Same as Router output)
const enrichedEvent = {
  userId: "user_123",
  gcsUri: "gs://fitglue-artifacts/activities/user_123/123.fit",
  description: "Enriched Activity #Power"
};

const dataBuffer = Buffer.from(JSON.stringify(enrichedEvent));
const pubSubMessage = {
  message: {
    data: dataBuffer.toString('base64'),
    messageId: "99999",
    publishTime: new Date().toISOString()
  }
};

const options = {
  hostname: 'localhost',
  port: 8083,
  path: '/',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Ce-Id': '9999',
    'Ce-Specversion': '1.0',
    'Ce-Type': 'google.cloud.pubsub.topic.v1.messagePublished',
    'Ce-Source': '//pubsub.googleapis.com/projects/fitglue-local/topics/topic-job-upload-strava',
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
