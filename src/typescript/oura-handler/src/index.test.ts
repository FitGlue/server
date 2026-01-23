describe('oura-handler', () => {
  it('should export ouraWebhookHandler', () => {
    // Dynamic import to avoid module loading issues during test setup
    return import('./index').then((module) => {
      expect(module.ouraWebhookHandler).toBeDefined();
      expect(typeof module.ouraWebhookHandler).toBe('function');
    });
  });
});
