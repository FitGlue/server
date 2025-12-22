
// Main entrypoint for Monorepo deployment.
// Function Framework will look for exports here if this is the main package.

// Lazy load handlers to prevent one build failure from crashing the entire entrypoint
exports.hevyWebhookHandler = (req, res) => {
  const hevy = require('./hevy-handler/build/index');
  return hevy.hevyWebhookHandler(req, res);
};

exports.keiserPoller = (req, res) => {
  const keiser = require('./keiser-poller/build/index');
  return keiser.keiserPoller(req, res);
};
