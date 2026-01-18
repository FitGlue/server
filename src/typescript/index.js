
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

exports.activitiesHandler = (req, res) => {
  const activities = require('./activities-handler/build/index');
  return activities.activitiesHandler(req, res);
};

exports.mockSourceHandler = (req, res) => {
  const mockSource = require('./mock-source-handler/build/index');
  return mockSource.mockSourceHandler(req, res);
};

exports.userProfileHandler = (req, res) => {
  const userProfile = require('./user-profile-handler/build/index');
  return userProfile.userProfileHandler(req, res);
};

exports.userIntegrationsHandler = (req, res) => {
  const userIntegrations = require('./user-integrations-handler/build/index');
  return userIntegrations.userIntegrationsHandler(req, res);
};

exports.userPipelinesHandler = (req, res) => {
  const userPipelines = require('./user-pipelines-handler/build/index');
  return userPipelines.userPipelinesHandler(req, res);
};

exports.registryHandler = (req, res) => {
  const registry = require('./registry-handler/build/index');
  return registry.registryHandler(req, res);
};

exports.integrationRequestHandler = (req, res) => {
  const integrationRequest = require('./integration-request-handler/dist/index');
  return integrationRequest.integrationRequestHandler(req, res);
};

exports.mobileSyncHandler = (req, res) => {
  const mobileSync = require('./mobile-sync-handler/dist/index');
  return mobileSync.mobileSyncHandler(req, res);
};

exports.billingHandler = (req, res) => {
  const billing = require('./billing-handler/dist/index');
  return billing.billingHandler(req, res);
};

exports.showcaseHandler = (req, res) => {
  const showcase = require('./showcase-handler/build/index');
  return showcase.showcaseHandler(req, res);
};

exports.fileUploadHandler = (req, res) => {
  const fileUpload = require('./file-upload-handler/build/index');
  return fileUpload.fileUploadHandler(req, res);
};
