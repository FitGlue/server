
// Main entrypoint for Monorepo deployment.
// Function Framework will look for exports here if this is the main package.

// Lazy load handlers to prevent one build failure from crashing the entire entrypoint
exports.hevyWebhookHandler = (req, res) => {
  const hevy = require('./hevy-handler/build/index');
  return hevy.hevyWebhookHandler(req, res);
};

exports.stravaOAuthHandler = (req, res) => {
  const strava = require('./strava-oauth-handler/build/index');
  return strava.stravaOAuthHandler(req, res);
};

exports.fitbitOAuthHandler = (req, res) => {
  const fitbit = require('./fitbit-oauth-handler/build/index');
  return fitbit.fitbitOAuthHandler(req, res);
};

exports.fitbitWebhookHandler = (req, res) => {
  const fitbit = require('./fitbit-handler/build/index');
  return fitbit.fitbitWebhookHandler(req, res);
};

exports.authOnCreate = (event) => {
  const auth = require('./auth-hooks/build/index');
  return auth.authOnCreate(event);
};

exports.waitlistHandler = (req, res) => {
  const waitlist = require('./waitlist-handler/build/index');
  return waitlist.waitlistHandler(req, res);
};

exports.inputsHandler = (req, res) => {
  const inputs = require('./inputs-handler/build/index');
  return inputs.inputsHandler(req, res);
};
