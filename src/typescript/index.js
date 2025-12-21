
// Main entrypoint for Monorepo deployment.
// Function Framework will look for exports here if this is the main package.

const hevy = require('./hevy-handler/build/index');
const keiser = require('./keiser-poller/build/index');

exports.hevyWebhookHandler = hevy.hevyWebhookHandler;
exports.keiserPoller = keiser.keiserPoller;
