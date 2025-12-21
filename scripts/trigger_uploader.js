const http = require('http');

console.log('Triggering Strava Uploader (EnrichedActivityEvent)...');

// Mock EnrichedActivityEvent (Same as Router output)
const enrichedEvent = {
  user_id: "user_upload_123",
  activity_id: "act_789",
  gcs_uri: "gs://fitglue-artifacts/activities/user_upload_123/456.fit",
  description: "Test upload"
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
