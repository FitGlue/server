const http = require('http');

console.log('Triggering Keiser Poller (HTTP)...');

const options = {
  hostname: 'localhost',
  port: 8084,
  path: '/',
  method: 'POST', // Cloud Scheduler sends POST
  headers: {
    'Content-Type': 'application/json',
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

req.end();
