describe('auth-email-handler', () => {
    it('should export authEmailHandler', () => {
        // Dynamic import to avoid module loading issues during test setup
        return import('./index').then((module) => {
            expect(module.authEmailHandler).toBeDefined();
            expect(typeof module.authEmailHandler).toBe('function');
        });
    });
});
