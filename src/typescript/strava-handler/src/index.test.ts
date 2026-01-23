describe('strava-handler', () => {
  it('should export stravaWebhookHandler', () => {
    // Dynamic import to avoid module loading issues during test setup
    return import('./index').then((module) => {
      expect(module.stravaWebhookHandler).toBeDefined();
      expect(typeof module.stravaWebhookHandler).toBe('function');
    });
  });
});
