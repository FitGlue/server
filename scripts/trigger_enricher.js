const http = require('http');

console.log('Triggering Enricher (activity.proto Payload)...');

// Mock ActivityPayload (pb.ActivityPayload)
const activityPayload = {
  source: 2, // HEVY
  user_id: "user_123",
  timestamp: new Date().toISOString(),
  original_payload_json: "{}",
  metadata: {}
};

const dataBuffer = Buffer.from(JSON.stringify(activityPayload));
const pubSubMessage = {
  message: {
    data: dataBuffer.toString('base64'),
    messageId: "12345",
    publishTime: new Date().toISOString()
  }
};

const options = {
  hostname: 'localhost',
  port: 8081,
  path: '/',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Ce-Id': '1234',
    'Ce-Specversion': '1.0',
    'Ce-Type': 'google.cloud.pubsub.topic.v1.messagePublished',
    'Ce-Source': '//pubsub.googleapis.com/projects/fitglue-local/topics/topic-raw-activity',
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
