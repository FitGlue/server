module.exports = {
  parser: '@typescript-eslint/parser',
  ignorePatterns: ['**/dist/**', '**/build/**', '**/node_modules/**'],
  env: {
    node: true,
  },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
  ],
  parserOptions: {
    ecmaVersion: 2020,
    sourceType: 'module',
  },
  rules: {
    // ============================================================================
    // Code Quality & Best Practices
    // ============================================================================
    '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
    '@typescript-eslint/no-explicit-any': 'error', // NO 'any' usage allowed
    '@typescript-eslint/explicit-function-return-type': 'off',
    '@typescript-eslint/no-non-null-assertion': 'warn',
    'no-console': ['error', { allow: ['warn', 'error'] }], // Use logger instead
    'prefer-const': 'error',
    'no-var': 'error',

    // ============================================================================
    // Import Organization
    // ============================================================================
    '@typescript-eslint/no-var-requires': 'error',
    'no-restricted-syntax': [
      'error',
      {
        selector: 'CallExpression[callee.type="Import"]',
        message: 'Avoid dynamic import() for types/constants that can be imported at the top of the file.'
      }
    ],

    // ============================================================================
    // Naming Conventions
    // ============================================================================
    '@typescript-eslint/naming-convention': [
      'error',
      {
        selector: 'variable',
        format: ['camelCase', 'UPPER_CASE', 'PascalCase'],
        leadingUnderscore: 'allow',
      },
      {
        selector: 'function',
        format: ['camelCase', 'PascalCase'],
      },
      {
        selector: 'typeLike',
        format: ['PascalCase'],
      },
      {
        selector: 'enumMember',
        format: ['UPPER_CASE'],
      },
    ],

    // ============================================================================
    // Code Complexity & Readability
    // ============================================================================
    'complexity': ['warn', 15], // Warn on complex functions
    'max-depth': ['warn', 4], // Max nesting depth
    'max-lines-per-function': ['warn', { max: 150, skipBlankLines: true, skipComments: true }],
    'max-params': ['warn', 5], // Too many params = refactor needed

    // ============================================================================
    // Equality & Comparison
    // ============================================================================
    'eqeqeq': ['error', 'always', { null: 'ignore' }], // Use === instead of ==

    // ============================================================================
    // Formatting & Style (works well with Prettier)
    // ============================================================================
    'quotes': ['error', 'single', { avoidEscape: true }],
    'semi': ['error', 'always'],
    'comma-dangle': ['error', {
      arrays: 'only-multiline',
      objects: 'only-multiline',
      imports: 'only-multiline',
      exports: 'only-multiline',
      functions: 'never',
    }],
  },
  overrides: [
    {
      files: ['**/*.test.ts', '**/*.spec.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-var-requires': 'off',
        '@typescript-eslint/no-require-imports': 'off',
        'max-lines-per-function': 'off',
        'complexity': 'off',
      },
    },
    {
      // CLI tools need console output and use dynamic typing
      files: ['admin-cli/**/*.ts'],
      rules: {
        'no-console': 'off',
        '@typescript-eslint/no-explicit-any': 'off',
      },
    },
    {
      // Generated OpenAPI schema files - disable rules that conflict with generated output
      files: ['shared/src/integrations/*/schema.ts'],
      rules: {
        '@typescript-eslint/naming-convention': 'off',
        'quotes': 'off',
        'semi': 'off',
        'comma-dangle': 'off',
      },
    },
    {
      // Build output files - exclude from strict linting
      files: ['**/build/**', '**/dist/**'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/naming-convention': 'off',
      },
    },
    {
      // Cloud Functions use console for logging
      files: ['auth-hooks/**/*.ts'],
      rules: {
        'no-console': 'off',
      },
    },
    {
      // MCP server uses dynamic types for tool arguments
      files: ['mcp-server/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
      },
    },
    {
      // Billing handler uses dynamic types for Stripe webhook payloads
      files: ['billing-handler/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
      },
    },
  ],
};
