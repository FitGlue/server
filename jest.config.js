module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  testMatch: ['**/integration-tests/**/*.test.ts'],
  setupFilesAfterEnv: [],
  testTimeout: 30000, // 30s timeout for integration tests
  transformIgnorePatterns: [
    "node_modules/(?!(uuid)/)"
  ],
};
