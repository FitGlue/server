
// Main entrypoint for Monorepo deployment.
// Function Framework will look for exports here if this is the main package.

// Lazy load handlers to prevent one build failure from crashing the entire entrypoint
exports.hevyHandler = (req, res) => {
  const hevy = require('./hevy-handler/dist/index');
  return hevy.hevyHandler(req, res);
};

exports.stravaOAuthHandler = (req, res) => {
  const strava = require('./strava-oauth-handler/dist/index');
  return strava.stravaOAuthHandler(req, res);
};

exports.spotifyOAuthHandler = (req, res) => {
  const spotify = require('./spotify-oauth-handler/dist/index');
  return spotify.spotifyOAuthHandler(req, res);
};

exports.fitbitOAuthHandler = (req, res) => {
  const fitbit = require('./fitbit-oauth-handler/dist/index');
  return fitbit.fitbitOAuthHandler(req, res);
};

exports.googleOAuthHandler = (req, res) => {
  const google = require('./google-oauth-handler/dist/index');
  return google.googleOAuthHandler(req, res);
};


exports.fitbitWebhookHandler = (req, res) => {
  const fitbit = require('./fitbit-handler/dist/index');
  return fitbit.fitbitWebhookHandler(req, res);
};

exports.authOnCreate = (event) => {
  const auth = require('./auth-hooks/dist/index');
  return auth.authOnCreate(event);
};

exports.inputsHandler = (req, res) => {
  const inputs = require('./inputs-handler/dist/index');
  return inputs.inputsHandler(req, res);
};

exports.activitiesHandler = (req, res) => {
  const activities = require('./activities-handler/dist/index');
  return activities.activitiesHandler(req, res);
};

exports.mockSourceHandler = (req, res) => {
  const mockSource = require('./mock-source-handler/dist/index');
  return mockSource.mockSourceHandler(req, res);
};

exports.userProfileHandler = (req, res) => {
  const userProfile = require('./user-profile-handler/dist/index');
  return userProfile.userProfileHandler(req, res);
};

exports.userIntegrationsHandler = (req, res) => {
  const userIntegrations = require('./user-integrations-handler/dist/index');
  return userIntegrations.userIntegrationsHandler(req, res);
};

exports.userPipelinesHandler = (req, res) => {
  const userPipelines = require('./user-pipelines-handler/dist/index');
  return userPipelines.userPipelinesHandler(req, res);
};

exports.registryHandler = (req, res) => {
  const registry = require('./registry-handler/dist/index');
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
  const showcase = require('./showcase-handler/dist/index');
  return showcase.showcaseHandler(req, res);
};

exports.repostHandler = (req, res) => {
  const repost = require('./repost-handler/dist/index');
  return repost.repostHandler(req, res);
};

exports.adminHandler = (req, res) => {
  const admin = require('./admin-handler/dist/index');
  return admin.adminHandler(req, res);
};

exports.stravaWebhookHandler = (req, res) => {
  const strava = require('./strava-handler/dist/index');
  return strava.stravaWebhookHandler(req, res);
};

exports.trainingPeaksOAuthHandler = (req, res) => {
  const tp = require('./trainingpeaks-oauth-handler/dist/index');
  return tp.trainingPeaksOAuthHandler(req, res);
};

exports.ouraOAuthHandler = (req, res) => {
  const oura = require('./oura-oauth-handler/dist/index');
  return oura.ouraOAuthHandler(req, res);
};

exports.wahooOAuthHandler = (req, res) => {
  const wahoo = require('./wahoo-oauth-handler/dist/index');
  return wahoo.wahooOAuthHandler(req, res);
};

exports.wahooWebhookHandler = (req, res) => {
  const wahoo = require('./wahoo-handler/dist/index');
  return wahoo.wahooWebhookHandler(req, res);
};

exports.ouraWebhookHandler = (req, res) => {
  const oura = require('./oura-handler/dist/index');
  return oura.ouraWebhookHandler(req, res);
};

exports.polarOAuthHandler = (req, res) => {
  const polar = require('./polar-oauth-handler/dist/index');
  return polar.polarOAuthHandler(req, res);
};

exports.polarWebhookHandler = (req, res) => {
  const polar = require('./polar-handler/dist/index');
  return polar.polarWebhookHandler(req, res);
};

exports.userDataHandler = (req, res) => {
  const userData = require('./user-data-handler/dist/index');
  return userData.userDataHandler(req, res);
};

exports.registrationSummaryHandler = (event) => {
  const regSummary = require('./registration-summary-handler/dist/index');
  return regSummary.registrationSummaryHandler(event);
};

exports.connectionActionsHandler = (req, res) => {
  const connectionActions = require('./connection-actions-handler/dist/index');
  return connectionActions.connectionActionsHandler(req, res);
};

exports.githubHandler = (req, res) => {
  const github = require('./github-handler/dist/index');
  return github.githubHandler(req, res);
};

exports.githubOAuthHandler = (req, res) => {
  const github = require('./github-oauth-handler/dist/index');
  return github.githubOAuthHandler(req, res);
};
