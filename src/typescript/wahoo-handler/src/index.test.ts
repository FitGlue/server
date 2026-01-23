describe('wahoo-handler', () => {
  it('should export wahooWebhookHandler', () => {
    // Dynamic import to avoid module loading issues during test setup
    return import('./index').then((module) => {
      expect(module.wahooWebhookHandler).toBeDefined();
      expect(typeof module.wahooWebhookHandler).toBe('function');
    });
  });
});
