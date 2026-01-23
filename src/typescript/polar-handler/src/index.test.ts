describe('polar-handler', () => {
  it('should export polarWebhookHandler', () => {
    // Dynamic import to avoid module loading issues during test setup
    return import('./index').then((module) => {
      expect(module.polarWebhookHandler).toBeDefined();
      expect(typeof module.polarWebhookHandler).toBe('function');
    });
  });
});
