/**
 * lint-codebase.ts
 *
 * Static analysis tool for FitGlue codebase consistency.
 * Ensures configuration alignment across TypeScript, Go, Terraform, and registries.
 *
 * Usage: npx ts-node scripts/lint-codebase.ts [--verbose]
 */

import * as fs from 'fs';
import * as path from 'path';
import { Project, SyntaxKind, CallExpression, Node } from 'ts-morph';

// ============================================================================
// Configuration
// ============================================================================

const SERVER_ROOT = path.resolve(__dirname, '..');
const TS_SRC_DIR = path.join(SERVER_ROOT, 'src/typescript');
const GO_SRC_DIR = path.join(SERVER_ROOT, 'src/go');
const TERRAFORM_DIR = path.join(SERVER_ROOT, 'terraform');
const PROTO_DIR = path.join(SERVER_ROOT, 'src/proto');

// Packages that are NOT cloud functions (excluded from function checks)
const NON_FUNCTION_PACKAGES = ['shared', 'admin-cli', 'mcp-server', 'node_modules'];

// Handler directories that don't need Terraform (internal tools)
const NO_TERRAFORM_REQUIRED = ['admin-cli', 'mcp-server'];

// ============================================================================
// Result Types
// ============================================================================

interface CheckResult {
  name: string;
  passed: boolean;
  errors: string[];
  warnings: string[];
}

// ============================================================================
// Utility Functions
// ============================================================================

function getDirectories(dirPath: string): string[] {
  if (!fs.existsSync(dirPath)) return [];
  return fs.readdirSync(dirPath, { withFileTypes: true })
    .filter(dirent => dirent.isDirectory())
    .map(dirent => dirent.name);
}

function getFiles(dirPath: string, extension?: string): string[] {
  if (!fs.existsSync(dirPath)) return [];
  return fs.readdirSync(dirPath, { withFileTypes: true })
    .filter(dirent => dirent.isFile())
    .filter(dirent => !extension || dirent.name.endsWith(extension))
    .map(dirent => dirent.name);
}

// ============================================================================
// Check 1: Terraform Coverage
// ============================================================================

function checkTerraformCoverage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // Special case mappings: directory name -> expected Terraform resource name
  const specialCaseMappings: Record<string, string> = {
    'auth-hooks': 'auth_on_create', // Uses Gen1 function with different naming
  };

  // Get all TypeScript handler directories
  const tsDirs = getDirectories(TS_SRC_DIR)
    .filter(d => d.endsWith('-handler') || d === 'auth-hooks')
    .filter(d => !NO_TERRAFORM_REQUIRED.includes(d));

  // Get all Go function directories
  const goDirs = getDirectories(path.join(GO_SRC_DIR, 'functions'));

  // Read Terraform files
  const tfFiles = ['functions.tf', 'oauth_functions.tf']
    .map(f => path.join(TERRAFORM_DIR, f))
    .filter(f => fs.existsSync(f));

  const tfContent = tfFiles.map(f => fs.readFileSync(f, 'utf-8')).join('\n');

  // Extract function names from Terraform
  // Pattern: resource "google_cloudfunctions2_function" "name" or resource "google_cloudfunctions_function" "name"
  const tfFunctionPattern = /resource\s+"google_cloudfunctions2?_function"\s+"([^"]+)"/g;
  const tfFunctions = new Set<string>();
  let match;
  while ((match = tfFunctionPattern.exec(tfContent)) !== null) {
    tfFunctions.add(match[1]);
  }

  // Helper to normalize names for comparison
  const normalize = (name: string): string => name.replace(/-/g, '_').toLowerCase();

  // Check TypeScript handlers
  for (const dir of tsDirs) {
    // Check special case mappings first
    if (specialCaseMappings[dir]) {
      const expectedTfName = specialCaseMappings[dir];
      if (!tfFunctions.has(expectedTfName)) {
        errors.push(`TypeScript function '${dir}' not found in Terraform (expected '${expectedTfName}')`);
      }
      continue;
    }

    // Convert handler name: 'hevy-handler' -> 'hevy_handler'
    const normalizedDir = normalize(dir);

    // Check if any Terraform resource matches
    const found = [...tfFunctions].some(tf => normalize(tf) === normalizedDir);

    if (!found) {
      errors.push(`TypeScript function '${dir}' not found in Terraform`);
    }
  }

  // Check Go functions
  for (const dir of goDirs) {
    const normalizedDir = normalize(dir);

    // Go functions: 'enricher' -> 'enricher', 'strava-uploader' -> 'strava_uploader'
    const found = [...tfFunctions].some(tf => normalize(tf) === normalizedDir);

    if (!found) {
      errors.push(`Go function '${dir}' not found in Terraform`);
    }
  }

  // Check Go functions are in build_function_zips.py
  const buildScriptPath = path.join(SERVER_ROOT, 'scripts/build_function_zips.py');
  if (fs.existsSync(buildScriptPath)) {
    const buildScriptContent = fs.readFileSync(buildScriptPath, 'utf-8');

    for (const dir of goDirs) {
      // Check if function is mentioned in the build script
      if (!buildScriptContent.includes(`"${dir}"`)) {
        errors.push(`Go function '${dir}' not in build_function_zips.py (deployment will fail)`);
      }
    }
  }

  return {
    name: 'Terraform Coverage',
    passed: errors.length === 0,
    errors,
    warnings
  };
}



// ============================================================================
// Check 2: Root index.js Exports
// ============================================================================

function checkIndexJsExports(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const indexPath = path.join(TS_SRC_DIR, 'index.js');
  if (!fs.existsSync(indexPath)) {
    errors.push('Root index.js not found');
    return { name: 'Root index.js Exports', passed: false, errors, warnings };
  }

  const indexContent = fs.readFileSync(indexPath, 'utf-8');

  // Get all handler directories (handlers + auth-hooks)
  const handlerDirs = getDirectories(TS_SRC_DIR)
    .filter(d => d.endsWith('-handler') || d === 'auth-hooks')
    .filter(d => !NON_FUNCTION_PACKAGES.includes(d));

  // Extract referenced packages from index.js
  // Pattern: require('./xxx-handler/build/index') or require('./xxx-handler/dist/index')
  const requirePattern = /require\(['"]\.\/([^/]+)\/(build|dist)\/index['"]\)/g;
  const exportedPackages = new Set<string>();
  let match;
  while ((match = requirePattern.exec(indexContent)) !== null) {
    exportedPackages.add(match[1]);
  }

  // Check each handler is exported
  for (const dir of handlerDirs) {
    if (!exportedPackages.has(dir)) {
      errors.push(`Handler '${dir}' not exported in index.js`);
    }
  }

  // Check for exports that don't have matching directories
  for (const pkg of exportedPackages) {
    if (!handlerDirs.includes(pkg)) {
      warnings.push(`index.js exports '${pkg}' but directory doesn't match expected patterns`);
    }
  }

  return {
    name: 'Root index.js Exports',
    passed: errors.length === 0,
    errors,
    warnings
  };
}


// ============================================================================
// Check 3: Connector Pattern (Source Handlers)
// ============================================================================

function checkConnectorPattern(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // Known source handlers that should use the Connector pattern
  const sourceHandlers = ['hevy-handler', 'fitbit-handler', 'mock-source-handler', 'mobile-sync-handler'];

  const project = new Project({
    tsConfigFilePath: path.join(TS_SRC_DIR, 'shared/tsconfig.json'),
    skipAddingFilesFromTsConfig: true
  });

  for (const handler of sourceHandlers) {
    const handlerDir = path.join(TS_SRC_DIR, handler);
    if (!fs.existsSync(handlerDir)) continue;

    const srcDir = path.join(handlerDir, 'src');
    if (!fs.existsSync(srcDir)) continue;

    // Check for connector.ts
    const connectorPath = path.join(srcDir, 'connector.ts');
    const hasConnector = fs.existsSync(connectorPath);

    // Check index.ts for createWebhookProcessor usage
    const indexPath = path.join(srcDir, 'index.ts');
    if (!fs.existsSync(indexPath)) continue;

    const indexContent = fs.readFileSync(indexPath, 'utf-8');
    const usesWebhookProcessor = indexContent.includes('createWebhookProcessor');

    if (!hasConnector && !usesWebhookProcessor) {
      warnings.push(`Source handler '${handler}' does not use Connector pattern`);
    } else if (hasConnector && !usesWebhookProcessor) {
      warnings.push(`Source handler '${handler}' has connector.ts but doesn't use createWebhookProcessor`);
    }
  }

  return {
    name: 'Connector Pattern',
    passed: true, // Warnings only, not blocking
    errors,
    warnings
  };
}

// ============================================================================
// Check 4: Registry Coverage
// ============================================================================

function checkRegistryCoverage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const registryPath = path.join(TS_SRC_DIR, 'shared/src/plugin/registry.ts');
  if (!fs.existsSync(registryPath)) {
    errors.push('Plugin registry not found');
    return { name: 'Registry Coverage', passed: false, errors, warnings };
  }

  const project = new Project({
    skipAddingFilesFromTsConfig: true
  });

  const sourceFile = project.addSourceFileAtPath(registryPath);

  // Find all registerEnricher calls and extract the EnricherProviderType
  const registeredEnrichers = new Set<string>();
  const registerEnricherCalls = sourceFile.getDescendantsOfKind(SyntaxKind.CallExpression)
    .filter((call: CallExpression) => call.getExpression().getText() === 'registerEnricher');

  for (const call of registerEnricherCalls) {
    const args = call.getArguments();
    if (args.length >= 1) {
      // First arg is EnricherProviderType.XXX
      const enumText = args[0].getText();
      registeredEnrichers.add(enumText);
    }
  }

  // Read the user.proto generated types to find all EnricherProviderType values
  const userTypesPath = path.join(TS_SRC_DIR, 'shared/src/types/pb/user.ts');
  if (fs.existsSync(userTypesPath)) {
    const userContent = fs.readFileSync(userTypesPath, 'utf-8');

    // Extract enum values from the generated file
    const enumPattern = /ENRICHER_PROVIDER_[A-Z_]+\s*=\s*\d+/g;
    const allEnumValues: string[] = [];
    let match;
    while ((match = enumPattern.exec(userContent)) !== null) {
      const enumName = match[0].split('=')[0].trim();
      if (enumName !== 'ENRICHER_PROVIDER_UNSPECIFIED') {
        allEnumValues.push(`EnricherProviderType.${enumName}`);
      }
    }

    // Check each enum value is registered
    for (const enumValue of allEnumValues) {
      if (!registeredEnrichers.has(enumValue)) {
        errors.push(`${enumValue} is not registered in registry.ts`);
      }
    }
  }

  // Check source registrations
  const registerSourceCalls = sourceFile.getDescendantsOfKind(SyntaxKind.CallExpression)
    .filter((call: CallExpression) => call.getExpression().getText() === 'registerSource');

  const registeredSources = new Set<string>();
  for (const call of registerSourceCalls) {
    const args = call.getArguments();
    if (args.length >= 1) {
      // Extract id from the manifest object
      const manifestText = args[0].getText();
      const idMatch = /id:\s*['"]([^'"]+)['"]/.exec(manifestText);
      if (idMatch) {
        registeredSources.add(idMatch[1]);
      }
    }
  }

  // Known sources that should be registered
  const expectedSources = ['hevy', 'fitbit', 'mock', 'apple-health', 'health-connect'];
  for (const source of expectedSources) {
    if (!registeredSources.has(source)) {
      warnings.push(`Source '${source}' may not be registered`);
    }
  }

  return {
    name: 'Registry Coverage',
    passed: errors.length === 0,
    errors,
    warnings
  };
}
// ============================================================================
// Check 5: Plugin Registration (Proto-Driven)
// ============================================================================

/**
 * Comprehensive check that ensures all proto-defined plugin types that are
 * used in the codebase are properly registered in the TypeScript registry.
 *
 * Checks:
 * - EnricherProviderType (user.proto) -> registerEnricher()
 * - ActivitySource (activity.proto) -> registerSource()
 * - Destination (events.proto) -> registerDestination()
 * - UserIntegrations fields (user.proto) -> registerIntegration()
 */
function checkPluginRegistration(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const registryPath = path.join(TS_SRC_DIR, 'shared/src/plugin/registry.ts');
  if (!fs.existsSync(registryPath)) {
    errors.push('Plugin registry not found');
    return { name: 'Plugin Registration', passed: false, errors, warnings };
  }

  const registryContent = fs.readFileSync(registryPath, 'utf-8');

  // Helper: Extract enum values from proto file
  const extractProtoEnum = (protoFile: string, enumName: string): string[] => {
    const protoPath = path.join(PROTO_DIR, protoFile);
    if (!fs.existsSync(protoPath)) return [];

    const content = fs.readFileSync(protoPath, 'utf-8');
    const enumPattern = new RegExp(`enum\\s+${enumName}\\s*\\{([^}]+)\\}`, 's');
    const enumMatch = content.match(enumPattern);
    if (!enumMatch) return [];

    const values: string[] = [];
    const valuePattern = /^\s*(\w+)\s*=/gm;
    let match;
    while ((match = valuePattern.exec(enumMatch[1])) !== null) {
      const value = match[1];
      // Skip UNSPECIFIED, UNKNOWN, TEST, and MOCK values (development only)
      if (!value.includes('UNSPECIFIED') && !value.includes('UNKNOWN') &&
        !value.includes('_TEST') && !value.includes('_MOCK')) {
        values.push(value);
      }
    }
    return values;
  };

  // Helper: Extract integration field names from UserIntegrations message
  const extractIntegrations = (): string[] => {
    const protoPath = path.join(PROTO_DIR, 'user.proto');
    if (!fs.existsSync(protoPath)) return [];

    const content = fs.readFileSync(protoPath, 'utf-8');
    const msgPattern = /message\s+UserIntegrations\s*\{([^}]+)\}/s;
    const msgMatch = content.match(msgPattern);
    if (!msgMatch) return [];

    const integrations: string[] = [];
    // Match field names like: HevyIntegration hevy = 1;
    const fieldPattern = /(\w+)Integration\s+(\w+)\s*=/g;
    let match;
    while ((match = fieldPattern.exec(msgMatch[1])) !== null) {
      integrations.push(match[2]); // e.g., "hevy", "fitbit", "strava"
    }
    return integrations;
  };

  // Helper: Check if enum value is used in codebase
  const isUsedInCode = (enumValue: string): boolean => {
    // Search in Go files
    const goFiles = findFilesRecursive(GO_SRC_DIR, '.go');
    for (const file of goFiles) {
      if (file.includes('_test.go')) continue;
      const content = fs.readFileSync(file, 'utf-8');
      if (content.includes(enumValue)) return true;
    }

    // Search in TypeScript files
    const tsFiles = findFilesRecursive(TS_SRC_DIR, '.ts');
    for (const file of tsFiles) {
      if (file.includes('.test.ts') || file.includes('.spec.ts')) continue;
      const content = fs.readFileSync(file, 'utf-8');
      if (content.includes(enumValue)) return true;
    }

    return false;
  };

  // Helper: Find files recursively
  const findFilesRecursive = (dir: string, extension: string): string[] => {
    if (!fs.existsSync(dir)) return [];
    const files: string[] = [];
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
      const fullPath = path.join(dir, item.name);
      if (item.isDirectory() && !item.name.includes('node_modules')) {
        files.push(...findFilesRecursive(fullPath, extension));
      } else if (item.isFile() && item.name.endsWith(extension)) {
        files.push(fullPath);
      }
    }
    return files;
  };

  // -------------------------------------------------------------------------
  // 1. Check Enrichers (EnricherProviderType -> registerEnricher)
  // -------------------------------------------------------------------------
  const enricherEnums = extractProtoEnum('user.proto', 'EnricherProviderType');
  const registeredEnrichersPattern = /registerEnricher\s*\(\s*EnricherProviderType\.(\w+)/g;
  const registeredEnrichers = new Set<string>();
  let match;
  while ((match = registeredEnrichersPattern.exec(registryContent)) !== null) {
    registeredEnrichers.add(match[1]);
  }

  for (const enumValue of enricherEnums) {
    if (isUsedInCode(enumValue) && !registeredEnrichers.has(enumValue)) {
      errors.push(`Enricher '${enumValue}' used in code but not registered in registry.ts`);
    }
  }

  // -------------------------------------------------------------------------
  // 2. Check Sources (ActivitySource -> registerSource)
  // -------------------------------------------------------------------------
  const sourceEnums = extractProtoEnum('activity.proto', 'ActivitySource');
  // Sources use string ids like 'hevy', 'fitbit' rather than enum names
  const registeredSourcesPattern = /registerSource\s*\(\s*\{[^}]*id:\s*['"]([^'"]+)['"]/g;
  const registeredSources = new Set<string>();
  while ((match = registeredSourcesPattern.exec(registryContent)) !== null) {
    registeredSources.add(match[1].toLowerCase());
  }

  for (const enumValue of sourceEnums) {
    // Convert SOURCE_HEVY -> hevy
    const sourceId = enumValue.replace(/^SOURCE_/, '').toLowerCase();
    if (isUsedInCode(enumValue) && !registeredSources.has(sourceId)) {
      errors.push(`Source '${enumValue}' used in code but not registered in registry.ts (expected id: '${sourceId}')`);
    }
  }

  // -------------------------------------------------------------------------
  // 3. Check Destinations (Destination -> registerDestination)
  // -------------------------------------------------------------------------
  const destEnums = extractProtoEnum('events.proto', 'Destination');
  const registeredDestsPattern = /registerDestination\s*\(\s*\{[^}]*id:\s*['"]([^'"]+)['"]/g;
  const registeredDests = new Set<string>();
  while ((match = registeredDestsPattern.exec(registryContent)) !== null) {
    registeredDests.add(match[1].toLowerCase());
  }

  for (const enumValue of destEnums) {
    // Convert DESTINATION_STRAVA -> strava
    const destId = enumValue.replace(/^DESTINATION_/, '').replace(/_UPDATE$/, '-update').toLowerCase();
    if (isUsedInCode(enumValue) && !registeredDests.has(destId)) {
      errors.push(`Destination '${enumValue}' used in code but not registered in registry.ts (expected id: '${destId}')`);
    }
  }

  // -------------------------------------------------------------------------
  // 4. Check Integrations (UserIntegrations fields -> registerIntegration)
  // -------------------------------------------------------------------------
  const integrationFields = extractIntegrations();
  const registeredIntegrationsPattern = /registerIntegration\s*\(\s*\{[^}]*id:\s*['"]([^'"]+)['"]/g;
  const registeredIntegrations = new Set<string>();
  while ((match = registeredIntegrationsPattern.exec(registryContent)) !== null) {
    registeredIntegrations.add(match[1].toLowerCase());
  }

  for (const integration of integrationFields) {
    // Skip mock integration (development only)
    if (integration.toLowerCase() === 'mock') continue;

    // Check if the integration is enabled/used (has XxxIntegration message)
    const protoContent = fs.readFileSync(path.join(PROTO_DIR, 'user.proto'), 'utf-8');
    const hasMessage = protoContent.includes(`message ${integration.charAt(0).toUpperCase() + integration.slice(1)}Integration`);

    if (hasMessage && !registeredIntegrations.has(integration.toLowerCase())) {
      errors.push(`Integration '${integration}' defined in UserIntegrations but not registered in registry.ts`);
    }
  }

  return {
    name: 'Plugin Registration',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 5: Workspace Membership
// ============================================================================

function checkWorkspaceMembership(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const packageJsonPath = path.join(TS_SRC_DIR, 'package.json');
  if (!fs.existsSync(packageJsonPath)) {
    errors.push('Root TypeScript package.json not found');
    return { name: 'Workspace Membership', passed: false, errors, warnings };
  }

  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
  const workspaces: string[] = packageJson.workspaces || [];

  // Get all directories that have a package.json
  const allDirs = getDirectories(TS_SRC_DIR)
    .filter(d => !NON_FUNCTION_PACKAGES.includes(d) || d === 'shared')
    .filter(d => d !== 'node_modules')
    .filter(d => fs.existsSync(path.join(TS_SRC_DIR, d, 'package.json')));

  // Check each directory is in workspaces
  for (const dir of allDirs) {
    if (!workspaces.includes(dir)) {
      errors.push(`Package '${dir}' not in workspaces list`);
    }
  }

  // Check for workspaces entries that don't exist
  for (const ws of workspaces) {
    const wsPath = path.join(TS_SRC_DIR, ws);
    if (!fs.existsSync(wsPath)) {
      warnings.push(`Workspace '${ws}' directory doesn't exist`);
    }
  }

  return {
    name: 'Workspace Membership',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 6: Protobuf Type Alignment
// ============================================================================

function checkProtobufAlignment(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const pbGeneratedDir = path.join(TS_SRC_DIR, 'shared/src/types/pb');

  // Get all .proto files
  const protoFiles = getFiles(PROTO_DIR, '.proto');

  // Get all generated .ts files (excluding google/ subdirectory)
  const generatedFiles = getFiles(pbGeneratedDir, '.ts');

  // Check each proto has a corresponding generated file
  for (const proto of protoFiles) {
    const baseName = proto.replace('.proto', '.ts');
    if (!generatedFiles.includes(baseName)) {
      errors.push(`Proto '${proto}' has no generated TypeScript (run 'make generate')`);
    }
  }

  // Optional: Check for generated files without corresponding proto
  for (const ts of generatedFiles) {
    const baseName = ts.replace('.ts', '.proto');
    if (!protoFiles.includes(baseName)) {
      warnings.push(`Generated file '${ts}' has no corresponding .proto file`);
    }
  }

  return {
    name: 'Protobuf Alignment',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 7: Firebase Routing
// ============================================================================

function checkFirebaseRouting(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // Handlers that are NOT public-facing (internal or background triggers)
  const NON_PUBLIC_HANDLERS = [
    'auth-hooks',         // Firebase Auth trigger, not HTTP
    'admin-cli',          // CLI tool, not a function
    'mcp-server',         // MCP server, not a Cloud Function
    'shared',             // Library package
  ];

  const firebaseJsonPath = path.join(SERVER_ROOT, '..', 'web', 'firebase.json');
  if (!fs.existsSync(firebaseJsonPath)) {
    warnings.push('web/firebase.json not found (skipping check)');
    return { name: 'Firebase Routing', passed: true, errors, warnings };
  }

  const firebaseConfig = JSON.parse(fs.readFileSync(firebaseJsonPath, 'utf-8'));
  const rewrites = firebaseConfig.hosting?.rewrites || [];

  // Extract all serviceIds from rewrites that start with /api, /auth, or /hooks
  const routedServiceIds = new Set<string>();
  for (const rewrite of rewrites) {
    const source = rewrite.source || '';
    const serviceId = rewrite.run?.serviceId;

    if (serviceId && (source.startsWith('/api') || source.startsWith('/auth') || source.startsWith('/hooks'))) {
      routedServiceIds.add(serviceId);
    }
  }

  // Get all TypeScript handler directories (public-facing)
  const handlerDirs = getDirectories(TS_SRC_DIR)
    .filter(d => d.endsWith('-handler') || d.endsWith('-oauth-handler'))
    .filter(d => !NON_PUBLIC_HANDLERS.includes(d));

  // Check each handler is routed
  for (const dir of handlerDirs) {
    // Convert directory name to expected serviceId (same format, kebab-case)
    // e.g., 'hevy-handler' -> could be 'hevy-handler' or 'hevy-webhook-handler'
    const possibleServiceIds = [
      dir,
      dir.replace('-handler', '-webhook-handler'),
    ];

    const found = possibleServiceIds.some(id => routedServiceIds.has(id));

    if (!found) {
      errors.push(`Handler '${dir}' not routed in web/firebase.json`);
    }
  }

  return {
    name: 'Firebase Routing',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 8: Destination Topic Mapping Sync
// ============================================================================

/**
 * Validates that the DestinationTopics mapping in events-helper.ts stays in
 * sync with the dest_topic extensions defined in events.proto.
 */
function checkDestinationTopicSync(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // 1. Extract dest_topic extensions from events.proto
  const protoPath = path.join(PROTO_DIR, 'events.proto');
  if (!fs.existsSync(protoPath)) {
    errors.push('events.proto not found');
    return { name: 'Destination Topic Mapping Sync', passed: false, errors, warnings };
  }

  const protoContent = fs.readFileSync(protoPath, 'utf-8');

  // Pattern: DESTINATION_STRAVA = 1 [(fitglue.events.dest_topic) = "topic-job-upload-strava"];
  const destTopicRegex = /(DESTINATION_\w+)\s*=\s*\d+\s*\[.*dest_topic\)\s*=\s*"([^"]+)"/g;
  const protoMappings: Map<string, string> = new Map();

  let match;
  while ((match = destTopicRegex.exec(protoContent)) !== null) {
    protoMappings.set(match[1], match[2]);
  }

  if (protoMappings.size === 0) {
    warnings.push('No dest_topic extensions found in events.proto (expected for destinations)');
    return { name: 'Destination Topic Mapping Sync', passed: true, errors, warnings };
  }

  // 2. Extract mapping from events-helper.ts
  const helperPath = path.join(TS_SRC_DIR, 'shared/src/types/events-helper.ts');
  if (!fs.existsSync(helperPath)) {
    errors.push('events-helper.ts not found - destination topic mapping required');
    return { name: 'Destination Topic Mapping Sync', passed: false, errors, warnings };
  }

  const helperContent = fs.readFileSync(helperPath, 'utf-8');

  // Pattern: [Destination.DESTINATION_STRAVA]: "topic-job-upload-strava"
  const tsMappingRegex = /\[Destination\.(DESTINATION_\w+)\]:\s*['"]([^'"]+)['"]/g;
  const tsMappings: Map<string, string> = new Map();

  while ((match = tsMappingRegex.exec(helperContent)) !== null) {
    tsMappings.set(match[1], match[2]);
  }

  // 3. Compare mappings
  for (const [dest, topic] of protoMappings) {
    if (!tsMappings.has(dest)) {
      errors.push(`Missing destination in events-helper.ts: ${dest} -> ${topic}`);
    } else if (tsMappings.get(dest) !== topic) {
      errors.push(
        `Topic mismatch for ${dest}: proto has "${topic}", TS has "${tsMappings.get(dest)}"`
      );
    }
  }

  // Check for extra TS mappings not in proto
  for (const [dest] of tsMappings) {
    if (!protoMappings.has(dest) && dest !== 'DESTINATION_UNSPECIFIED') {
      warnings.push(`Extra destination in events-helper.ts not in proto: ${dest}`);
    }
  }

  return {
    name: 'Destination Topic Mapping Sync',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 9: Events Helper Completeness
// ============================================================================

/**
 * Validates that events-helper.ts exports all required helper functions
 * for event types, sources, and destinations.
 */
function checkEventsHelperCompleteness(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const helperPath = path.join(TS_SRC_DIR, 'shared/src/types/events-helper.ts');
  if (!fs.existsSync(helperPath)) {
    errors.push('events-helper.ts not found');
    return { name: 'Events Helper Completeness', passed: false, errors, warnings };
  }

  const helperContent = fs.readFileSync(helperPath, 'utf-8');

  // Required exports for full functionality
  const requiredExports = [
    'CloudEventTypeURN',
    'CloudEventSourceURN',
    'DestinationTopics',
    'getCloudEventType',
    'getCloudEventSource',
    'getDestinationTopic',
    'getDestinationName',
    'parseDestination'
  ];

  for (const exportName of requiredExports) {
    const exportPattern = new RegExp(`export\\s+(const|function)\\s+${exportName}\\b`);
    if (!exportPattern.test(helperContent)) {
      errors.push(`Missing required export in events-helper.ts: ${exportName}`);
    }
  }

  return {
    name: 'Events Helper Completeness',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 10: Destination Uploader Pattern
// ============================================================================

/**
 * Validates that Go destination uploaders follow required patterns:
 * - Loop prevention (isLoopOrigin or similar)
 * - UPDATE support (handleXxxUpdate function)
 * - SynchronizedActivity persistence
 * - Billing (IncrementSyncCount)
 */
function checkDestinationUploaderPattern(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // Uploaders that are excluded from certain checks
  const EXEMPT_FROM_LOOP_PREVENTION = ['mock-uploader', 'showcase-uploader']; // No external webhooks
  const EXEMPT_FROM_UPDATE = ['mock-uploader', 'showcase-uploader']; // One-shot destinations

  // Get all Go uploader function directories
  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  const uploaderDirs = getDirectories(goFunctionsDir)
    .filter(d => d.endsWith('-uploader'))
    .filter(d => d !== 'mock-uploader'); // Exclude mock from all checks

  for (const dir of uploaderDirs) {
    const functionPath = path.join(goFunctionsDir, dir, 'function.go');
    if (!fs.existsSync(functionPath)) {
      warnings.push(`Uploader '${dir}' missing function.go`);
      continue;
    }

    const content = fs.readFileSync(functionPath, 'utf-8');
    const uploaderName = dir.replace('-uploader', '');

    // Check for loop prevention (exempt showcase - no external webhooks)
    if (!EXEMPT_FROM_LOOP_PREVENTION.includes(dir)) {
      const hasLoopPrevention =
        content.includes('isLoopOrigin') ||
        content.includes('loop_prevention') ||
        content.includes('origin_destination') ||
        content.includes('OriginDestination') ||
        content.includes('SOURCE_STRAVA'); // Strava has implicit loop prevention

      if (!hasLoopPrevention) {
        // Grandfather existing uploaders with warnings, new ones get errors
        const isGrandfathered = ['strava-uploader'].includes(dir);
        if (isGrandfathered) {
          warnings.push(`Uploader '${dir}' should add explicit loop prevention`);
        } else {
          errors.push(`Uploader '${dir}' missing loop prevention check`);
        }
      }
    }

    // Check for UPDATE support (exempt mock and showcase)
    if (!EXEMPT_FROM_UPDATE.includes(dir)) {
      const hasUpdateHandler =
        content.includes('handleUpdate') ||
        content.includes('Handle' + capitalize(uploaderName) + 'Update') ||
        content.includes('handle' + capitalize(uploaderName) + 'Update');

      if (!hasUpdateHandler) {
        errors.push(`Uploader '${dir}' missing UPDATE handler`);
      }
    }

    // Check for SynchronizedActivity persistence
    const hasSyncPersistence =
      content.includes('SetSynchronizedActivity') ||
      content.includes('SynchronizedActivity');

    if (!hasSyncPersistence) {
      warnings.push(`Uploader '${dir}' may not persist SynchronizedActivity`);
    }

    // Check for billing increment
    const hasBilling = content.includes('IncrementSyncCount');
    if (!hasBilling) {
      warnings.push(`Uploader '${dir}' may not increment sync count for billing`);
    }
  }

  return {
    name: 'Destination Uploader Pattern',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// Helper to capitalize first letter
function capitalize(str: string): string {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

// ============================================================================
// Check 11: Destination Enum Coverage
// ============================================================================

/**
 * Validates that every DESTINATION_* enum in events.proto has:
 * - A corresponding *-uploader Go function
 * - A Pub/Sub topic in pubsub.tf
 * - A registry entry in registry.ts
 */
function checkDestinationEnumCoverage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // 1. Extract all destination enum values from events.proto
  const protoPath = path.join(PROTO_DIR, 'events.proto');
  if (!fs.existsSync(protoPath)) {
    errors.push('events.proto not found');
    return { name: 'Destination Enum Coverage', passed: false, errors, warnings };
  }

  const protoContent = fs.readFileSync(protoPath, 'utf-8');
  const destEnumPattern = /DESTINATION_(\w+)\s*=\s*\d+/g;
  const destinations: string[] = [];

  let match;
  while ((match = destEnumPattern.exec(protoContent)) !== null) {
    const dest = match[1];
    // Skip UNSPECIFIED and MOCK
    if (dest !== 'UNSPECIFIED' && dest !== 'MOCK') {
      destinations.push(dest.toLowerCase());
    }
  }

  // 2. Check for Go uploader
  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  const uploaderDirs = getDirectories(goFunctionsDir)
    .filter(d => d.endsWith('-uploader'))
    .map(d => d.replace('-uploader', ''));

  for (const dest of destinations) {
    if (!uploaderDirs.includes(dest)) {
      errors.push(`DESTINATION_${dest.toUpperCase()} missing Go uploader (expected '${dest}-uploader')`);
    }
  }

  // 3. Check for Pub/Sub topic
  const pubsubPath = path.join(TERRAFORM_DIR, 'pubsub.tf');
  if (fs.existsSync(pubsubPath)) {
    const pubsubContent = fs.readFileSync(pubsubPath, 'utf-8');

    for (const dest of destinations) {
      const topicName = `topic-job-upload-${dest}`;
      if (!pubsubContent.includes(topicName)) {
        errors.push(`DESTINATION_${dest.toUpperCase()} missing Pub/Sub topic '${topicName}'`);
      }
    }
  }

  // 4. Check for registry entry
  const registryPath = path.join(TS_SRC_DIR, 'shared/src/plugin/registry.ts');
  if (fs.existsSync(registryPath)) {
    const registryContent = fs.readFileSync(registryPath, 'utf-8');

    for (const dest of destinations) {
      // Look for registerDestination with id: 'strava' etc
      const idPattern = new RegExp(`registerDestination\\s*\\(\\s*\\{[^}]*id:\\s*['"]${dest}['"]`, 's');
      if (!idPattern.test(registryContent)) {
        errors.push(`DESTINATION_${dest.toUpperCase()} not registered in registry.ts`);
      }
    }
  }

  return {
    name: 'Destination Enum Coverage',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 12: Loop Prevention in Destinations
// ============================================================================

/**
 * Validates that destination uploaders properly check for loop scenarios:
 * - Check if activity source matches the destination (self-loop)
 * - Check origin_destination metadata
 */
function checkLoopPrevention(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  const uploaderDirs = getDirectories(goFunctionsDir)
    .filter(d => d.endsWith('-uploader') && !d.includes('mock'));

  for (const dir of uploaderDirs) {
    const functionPath = path.join(goFunctionsDir, dir, 'function.go');
    if (!fs.existsSync(functionPath)) continue;

    const content = fs.readFileSync(functionPath, 'utf-8');
    const uploaderName = dir.replace('-uploader', '');

    // Check for source-based loop prevention (e.g., SOURCE_HEVY check)
    const sourceEnumName = `SOURCE_${uploaderName.toUpperCase()}`;
    const hasSourceCheck = content.includes(sourceEnumName) ||
      content.includes(`ActivitySource_${sourceEnumName}`);

    // Check for origin_destination check
    const hasOriginCheck =
      content.includes('origin_destination') ||
      content.includes('OriginDestination') ||
      content.includes('EnrichmentMetadata');

    // At least one form of loop prevention should exist
    if (!hasSourceCheck && !hasOriginCheck) {
      errors.push(`Uploader '${dir}' lacks loop prevention (no ${sourceEnumName} or origin_destination check)`);
    } else if (!hasSourceCheck) {
      warnings.push(`Uploader '${dir}' may not check source for self-loop prevention`);
    } else if (!hasOriginCheck) {
      warnings.push(`Uploader '${dir}' may not check origin_destination metadata`);
    }
  }

  return {
    name: 'Loop Prevention in Destinations',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 14: Environment Variable Access (T8)
// ============================================================================

/**
 * Validates that environment variables are accessed through config.ts,
 * not directly via process.env.
 */
function checkEnvVarAccess(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const configPath = path.join(TS_SRC_DIR, 'shared/src/config.ts');
  const allowedFiles = [configPath, 'config.ts', 'config.test.ts', 'framework/index.ts'];

  // Find all TypeScript files
  const findTsFiles = (dir: string): string[] => {
    if (!fs.existsSync(dir)) return [];
    const files: string[] = [];
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
      const fullPath = path.join(dir, item.name);
      if (item.isDirectory() && !item.name.includes('node_modules') && !item.name.includes('dist')) {
        files.push(...findTsFiles(fullPath));
      } else if (item.isFile() && item.name.endsWith('.ts') && !item.name.endsWith('.test.ts')) {
        files.push(fullPath);
      }
    }
    return files;
  };

  const tsFiles = findTsFiles(TS_SRC_DIR);

  for (const file of tsFiles) {
    const isConfigFile = allowedFiles.some(allowed => file.includes(allowed));
    if (isConfigFile) continue;

    const content = fs.readFileSync(file, 'utf-8');
    const envVarPattern = /process\.env\.(\w+)/g;
    let match;

    while ((match = envVarPattern.exec(content)) !== null) {
      const varName = match[1];
      // Warning instead of error to allow migration time
      warnings.push(`${path.relative(SERVER_ROOT, file)}: Move \`process.env.${varName}\` to \`shared/src/config.ts\``);
    }
  }

  return {
    name: 'Environment Variable Access (T8)',
    passed: true, // Warnings only - allow time for migration
    errors,
    warnings
  };
}

// ============================================================================
// Check 15: Protobuf Generation Freshness (E1)
// ============================================================================

/**
 * Validates that generated proto files are newer than source .proto files.
 */
function checkProtoFreshness(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const protoFiles = getFiles(PROTO_DIR, '.proto');
  const tsPbDir = path.join(TS_SRC_DIR, 'shared/src/types/pb');
  const goPbDir = path.join(GO_SRC_DIR, 'pkg/types/pb');

  for (const proto of protoFiles) {
    const protoPath = path.join(PROTO_DIR, proto);
    const protoStat = fs.statSync(protoPath);
    const baseName = proto.replace('.proto', '');

    // Check TypeScript generated file
    const tsGenPath = path.join(tsPbDir, `${baseName}.ts`);
    if (fs.existsSync(tsGenPath)) {
      const tsStat = fs.statSync(tsGenPath);
      if (protoStat.mtime > tsStat.mtime) {
        errors.push(`${proto}: TypeScript types are stale (run \`make generate\`)`);
      }
    }

    // Check Go generated file
    const goGenPath = path.join(goPbDir, `${baseName}.pb.go`);
    if (fs.existsSync(goGenPath)) {
      const goStat = fs.statSync(goGenPath);
      if (protoStat.mtime > goStat.mtime) {
        errors.push(`${proto}: Go types are stale (run \`make generate\`)`);
      }
    }
  }

  return {
    name: 'Protobuf Generation Freshness (E1)',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 16: No Manual Enum Re-definitions (E2)
// ============================================================================

/**
 * Detects manual enum definitions that should use generated proto types.
 */
function checkNoManualEnums(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const PROTO_ENUMS = ['ActivityType', 'Destination', 'ActivitySource', 'MuscleGroup', 'CloudEventType', 'CloudEventSource'];
  const ALLOWED_PATHS = ['types/pb/', 'enum-formatters.ts'];

  const findTsFiles = (dir: string): string[] => {
    if (!fs.existsSync(dir)) return [];
    const files: string[] = [];
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
      const fullPath = path.join(dir, item.name);
      if (item.isDirectory() && !item.name.includes('node_modules') && !item.name.includes('dist')) {
        files.push(...findTsFiles(fullPath));
      } else if (item.isFile() && (item.name.endsWith('.ts') || item.name.endsWith('.tsx'))) {
        files.push(fullPath);
      }
    }
    return files;
  };

  // Check server
  const serverFiles = findTsFiles(TS_SRC_DIR);
  for (const file of serverFiles) {
    const isAllowed = ALLOWED_PATHS.some(p => file.includes(p));
    if (isAllowed) continue;

    const content = fs.readFileSync(file, 'utf-8');
    for (const enumName of PROTO_ENUMS) {
      const enumDefPattern = new RegExp(`enum\\s+${enumName}\\s*\\{`, 'g');
      if (enumDefPattern.test(content)) {
        errors.push(`${path.relative(SERVER_ROOT, file)}: Manual \`enum ${enumName}\` definition - use import from \`types/pb/\``);
      }
    }
  }

  // Check web if exists
  const webSrcDir = path.join(SERVER_ROOT, '..', 'web', 'src');
  if (fs.existsSync(webSrcDir)) {
    const webFiles = findTsFiles(webSrcDir);
    for (const file of webFiles) {
      const isAllowed = ALLOWED_PATHS.some(p => file.includes(p));
      if (isAllowed) continue;

      const content = fs.readFileSync(file, 'utf-8');
      for (const enumName of PROTO_ENUMS) {
        const enumDefPattern = new RegExp(`enum\\s+${enumName}\\s*\\{`, 'g');
        if (enumDefPattern.test(content)) {
          errors.push(`web/${path.relative(webSrcDir, file)}: Manual \`enum ${enumName}\` definition - use import from \`types/pb/\``);
        }
      }
    }
  }

  return {
    name: 'No Manual Enum Re-definitions (E2)',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 17: Formatter Coverage (E7)
// ============================================================================

/**
 * Ensures all proto enums have corresponding formatters in enum-formatters.ts.
 */
function checkFormatterCoverage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const formatterPath = path.join(TS_SRC_DIR, 'shared/src/types/pb/enum-formatters.ts');
  const generatorPath = path.join(SERVER_ROOT, 'scripts/generate-enum-formatters.ts');

  if (!fs.existsSync(formatterPath)) {
    errors.push('enum-formatters.ts not found (run `make generate`)');
    return { name: 'Formatter Coverage (E7)', passed: false, errors, warnings };
  }

  // Extract enums from proto-generated files
  const tsPbDir = path.join(TS_SRC_DIR, 'shared/src/types/pb');
  const protoEnums: string[] = [];

  const pbFiles = getFiles(tsPbDir, '.ts').filter(f => !f.includes('enum-formatters'));
  for (const file of pbFiles) {
    const content = fs.readFileSync(path.join(tsPbDir, file), 'utf-8');
    const enumPattern = /export enum (\w+)/g;
    let match;
    while ((match = enumPattern.exec(content)) !== null) {
      protoEnums.push(match[1]);
    }
  }

  // Check which have formatters
  const formatterContent = fs.readFileSync(formatterPath, 'utf-8');
  const formatterPattern = /export function format(\w+)\(/g;
  const formatters = new Set<string>();
  let fmatch;
  while ((fmatch = formatterPattern.exec(formatterContent)) !== null) {
    formatters.add(fmatch[1]);
  }

  // Check which are configured in generator
  if (fs.existsSync(generatorPath)) {
    const genContent = fs.readFileSync(generatorPath, 'utf-8');
    for (const enumName of protoEnums) {
      if (!formatters.has(enumName) && !genContent.includes(`enumName: '${enumName}'`)) {
        warnings.push(`Proto enum '${enumName}' has no formatter - add to ENUM_CONFIGS in generate-enum-formatters.ts`);
      }
    }
  }

  return {
    name: 'Formatter Coverage (E7)',
    passed: true, // Warnings only for now
    errors,
    warnings
  };
}

// ============================================================================
// Check 18: Jest Config Inheritance (T3)
// ============================================================================

/**
 * Validates all handler jest.config.js files extend shared/jest.config.base.js.
 */
function checkJestConfigInheritance(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const handlersDir = TS_SRC_DIR;
  const handlerDirs = getDirectories(handlersDir)
    .filter(d => d.endsWith('-handler'))
    .filter(d => !NON_FUNCTION_PACKAGES.includes(d));

  for (const dir of handlerDirs) {
    const jestConfigPath = path.join(handlersDir, dir, 'jest.config.js');
    if (!fs.existsSync(jestConfigPath)) {
      warnings.push(`Handler '${dir}' missing jest.config.js`);
      continue;
    }

    const content = fs.readFileSync(jestConfigPath, 'utf-8');
    if (!content.includes('jest.config.base') && !content.includes('../shared/')) {
      errors.push(`Handler '${dir}' jest.config.js should extend '../shared/jest.config.base.js'`);
    }
  }

  return {
    name: 'Jest Config Inheritance (T3)',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 19: Handler Package Scripts (T4)
// ============================================================================

/**
 * Validates all handlers have required npm scripts.
 */
function checkHandlerPackageScripts(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const REQUIRED_SCRIPTS = ['build', 'test'];
  const handlersDir = TS_SRC_DIR;
  const handlerDirs = getDirectories(handlersDir)
    .filter(d => d.endsWith('-handler'))
    .filter(d => !NON_FUNCTION_PACKAGES.includes(d));

  for (const dir of handlerDirs) {
    const pkgPath = path.join(handlersDir, dir, 'package.json');
    if (!fs.existsSync(pkgPath)) {
      errors.push(`Handler '${dir}' missing package.json`);
      continue;
    }

    const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf-8'));
    const scripts = pkg.scripts || {};

    for (const script of REQUIRED_SCRIPTS) {
      if (!scripts[script]) {
        warnings.push(`Handler '${dir}' missing '${script}' script in package.json`);
      }
    }
  }

  return {
    name: 'Handler Package Scripts (T4)',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 20: Web Types Alignment (W7/W13)
// ============================================================================

/**
 * Validates web uses generated types from types/pb/, not local definitions.
 * Only runs if ../web exists.
 */
function checkWebTypesAlignment(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'Web Types Alignment (W7/W13)', passed: true, errors, warnings: ['../web not found, skipping'] };
  }

  const webPbDir = path.join(webDir, 'src', 'types', 'pb');
  const serverPbDir = path.join(TS_SRC_DIR, 'shared/src/types/pb');

  if (!fs.existsSync(webPbDir)) {
    errors.push('web/src/types/pb/ not found - run `make generate` from server');
    return { name: 'Web Types Alignment (W7/W13)', passed: false, errors, warnings };
  }

  // Check that web has all the server generated types
  const serverFiles = getFiles(serverPbDir, '.ts');
  const webFiles = new Set(getFiles(webPbDir, '.ts'));

  for (const file of serverFiles) {
    if (!webFiles.has(file)) {
      errors.push(`web/src/types/pb/ missing '${file}' - run \`make generate\` from server`);
    }
  }

  // Compare timestamps to ensure web types are fresh
  for (const file of serverFiles) {
    const serverPath = path.join(serverPbDir, file);
    const webPath = path.join(webPbDir, file);
    if (fs.existsSync(webPath)) {
      const serverStat = fs.statSync(serverPath);
      const webStat = fs.statSync(webPath);
      if (serverStat.mtime > webStat.mtime) {
        warnings.push(`web/src/types/pb/${file} is stale - run \`make generate\` from server`);
      }
    }
  }

  return {
    name: 'Web Types Alignment (W7/W13)',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 21: Firestore Converter Completeness (T1)
// ============================================================================

/**
 * Validates Firestore converters handle both snake_case and camelCase field names.
 */
function checkConverterCompleteness(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const converterPaths = [
    path.join(TS_SRC_DIR, 'shared/src/firestore/converters.ts'),
  ];

  for (const converterPath of converterPaths) {
    if (!fs.existsSync(converterPath)) continue;

    const content = fs.readFileSync(converterPath, 'utf-8');

    // Check for common snake_case fields that should have camelCase fallback
    const fieldsToCheck = [
      { snake: 'activity_type', camel: 'activityType' },
      { snake: 'user_id', camel: 'userId' },
      { snake: 'external_id', camel: 'externalId' },
      { snake: 'created_at', camel: 'createdAt' },
      { snake: 'updated_at', camel: 'updatedAt' },
      { snake: 'start_time', camel: 'startTime' },
      { snake: 'pipeline_id', camel: 'pipelineId' },
    ];

    for (const field of fieldsToCheck) {
      // Check if snake_case is used without camelCase fallback
      const snakePattern = new RegExp(`data\\.${field.snake}(?![\\w])`, 'g');
      const camelPattern = new RegExp(`data\\.${field.camel}(?![\\w])`, 'g');
      const fallbackPattern = new RegExp(`data\\.${field.snake}\\s*\\|\\|\\s*data\\.${field.camel}|data\\.${field.camel}\\s*\\|\\|\\s*data\\.${field.snake}`, 'g');

      const hasSnake = snakePattern.test(content);
      const hasCamel = camelPattern.test(content);
      const hasFallback = fallbackPattern.test(content);

      if (hasSnake && !hasFallback && !hasCamel) {
        warnings.push(`${path.basename(converterPath)}: Field '${field.snake}' should include fallback for '${field.camel}'`);
      }
      if (hasCamel && !hasFallback && !hasSnake) {
        warnings.push(`${path.basename(converterPath)}: Field '${field.camel}' should include fallback for '${field.snake}'`);
      }
    }
  }

  return {
    name: 'Firestore Converter Completeness (T1)',
    passed: true, // Warnings only
    errors,
    warnings
  };
}

// ============================================================================
// Check 22: Proto Import Path (T5)
// ============================================================================

/**
 * Validates TypeScript handlers import proto types from shared/src/types/pb/.
 */
function checkProtoImportPath(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const findTsFiles = (dir: string): string[] => {
    if (!fs.existsSync(dir)) return [];
    const files: string[] = [];
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
      const fullPath = path.join(dir, item.name);
      if (item.isDirectory() && !item.name.includes('node_modules') && !item.name.includes('dist') && !item.name.includes('types')) {
        files.push(...findTsFiles(fullPath));
      } else if (item.isFile() && item.name.endsWith('.ts') && !item.name.endsWith('.test.ts')) {
        files.push(fullPath);
      }
    }
    return files;
  };

  const handlerDirs = getDirectories(TS_SRC_DIR).filter(d => d.endsWith('-handler'));

  for (const dir of handlerDirs) {
    const files = findTsFiles(path.join(TS_SRC_DIR, dir));
    for (const file of files) {
      const content = fs.readFileSync(file, 'utf-8');

      // Check for proto type imports not from shared
      const badImportPatterns = [
        /from\s+['"]\.\.?\/.*\/pb['"]/g, // Relative pb imports
        /from\s+['"]@fitglue\/shared\/types\/pb/g, // Should be from shared/src/types/pb
      ];

      // Good pattern: from '@fitglue/shared' or from '../shared/src/types/pb'
      const protoTypeNames = ['ActivityType', 'Destination', 'ActivitySource', 'StandardizedActivity'];

      for (const typeName of protoTypeNames) {
        const usesType = content.includes(typeName);
        if (!usesType) continue;

        const hasGoodImport = content.includes(`from '@fitglue/shared'`) ||
          content.includes(`from '../shared'`) ||
          content.includes(`types/pb`);

        if (usesType && !hasGoodImport) {
          warnings.push(`${path.relative(SERVER_ROOT, file)}: Uses '${typeName}' but may not import from shared/types/pb`);
        }
      }
    }
  }

  return {
    name: 'Proto Import Path (T5)',
    passed: true, // Warnings only
    errors,
    warnings
  };
}

// ============================================================================
// Check 23: useApi over useAuth (W1) - Web Only
// ============================================================================

/**
 * Validates web components use useApi() for API calls, not direct fetch with useAuth().
 */
function checkUseApiPattern(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'useApi over useAuth (W1)', passed: true, errors, warnings: ['../web not found, skipping'] };
  }

  const webAppDir = path.join(webDir, 'src', 'app');
  if (!fs.existsSync(webAppDir)) {
    return { name: 'useApi over useAuth (W1)', passed: true, errors, warnings };
  }

  const findTsxFiles = (dir: string): string[] => {
    const files: string[] = [];
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
      const fullPath = path.join(dir, item.name);
      if (item.isDirectory() && !item.name.includes('node_modules')) {
        files.push(...findTsxFiles(fullPath));
      } else if (item.isFile() && (item.name.endsWith('.tsx') || item.name.endsWith('.ts')) && !item.name.endsWith('.test.tsx')) {
        files.push(fullPath);
      }
    }
    return files;
  };

  const webFiles = findTsxFiles(webAppDir);

  for (const file of webFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(webDir, file);

    // Skip the useApi hook itself and services
    if (file.includes('hooks/useApi') || file.includes('services/')) continue;

    // Check for direct fetch calls in components
    const hasFetch = /[^a-zA-Z]fetch\s*\(/g.test(content);
    const hasUseAuth = /useAuth\s*\(\s*\)/g.test(content);

    if (hasFetch && !file.includes('useApi')) {
      warnings.push(`${fileName}: Uses direct fetch() - consider using useApi() hook`);
    }

    // Check for useAuth followed by API-like patterns (token usage for fetch)
    if (hasUseAuth && content.includes('token') && hasFetch) {
      errors.push(`${fileName}: Uses useAuth() + fetch() - use useApi() instead for authenticated requests`);
    }
  }

  return {
    name: 'useApi over useAuth (W1)',
    passed: errors.length === 0,
    errors,
    warnings
  };
}

// ============================================================================
// Check 24: Protobuf JSON Serialization (G1)
// ============================================================================

/**
 * Validates Go code uses protojson instead of encoding/json for proto types.
 * This wraps the existing lint-proto-json.sh script output.
 */
function checkProtoJsonSerialization(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'Protobuf JSON Serialization (G1)', passed: true, errors, warnings };
  }

  const findGoFiles = (dir: string): string[] => {
    const files: string[] = [];
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
      const fullPath = path.join(dir, item.name);
      if (item.isDirectory()) {
        files.push(...findGoFiles(fullPath));
      } else if (item.isFile() && item.name.endsWith('.go') && !item.name.endsWith('_test.go')) {
        files.push(fullPath);
      }
    }
    return files;
  };

  const goFiles = findGoFiles(goFunctionsDir);

  for (const file of goFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(SERVER_ROOT, file);

    // Check if file imports pb package
    const importsPb = /import\s+.*".*\/pb"/.test(content) || /pb\./.test(content);

    // Check if file uses encoding/json
    const usesEncodingJson = /import\s+.*"encoding\/json"/.test(content);

    // Check for json.Marshal or json.Unmarshal
    const usesJsonMarshal = /json\.(Marshal|Unmarshal)/.test(content);

    if (importsPb && usesEncodingJson && usesJsonMarshal) {
      warnings.push(`${fileName}: Uses encoding/json with proto types - verify using protojson for pb.* types`);
    }
  }

  return {
    name: 'Protobuf JSON Serialization (G1)',
    passed: true, // Warnings only - manual verification needed
    errors,
    warnings
  };
}

// ============================================================================
// Check 25: Go Context Propagation (G2)
// ============================================================================

function checkGoContextPropagation(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'Go Context Propagation (G2)', passed: true, errors, warnings };
  }

  const findGoFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory()) {
          files.push(...findGoFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.go') && !item.name.endsWith('_test.go')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const goFiles = findGoFiles(goFunctionsDir);

  for (const file of goFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(SERVER_ROOT, file);

    // Check for context.Background() or context.TODO() which should be ctx from caller
    const backgroundCtx = /context\.(Background|TODO)\(\)/.test(content);
    if (backgroundCtx && !file.includes('main.go') && !file.includes('_test.go')) {
      warnings.push(`${fileName}: Uses context.Background()/TODO() - prefer passing ctx from caller`);
    }
  }

  return {
    name: 'Go Context Propagation (G2)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 26: Go Error Wrapping (G3)
// ============================================================================

function checkGoErrorWrapping(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'Go Error Wrapping (G3)', passed: true, errors, warnings };
  }

  const findGoFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory()) {
          files.push(...findGoFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.go') && !item.name.endsWith('_test.go')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const goFiles = findGoFiles(goFunctionsDir);

  for (const file of goFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(SERVER_ROOT, file);

    // Check for fmt.Errorf without %w (should wrap errors)
    const errorfWithoutWrap = /fmt\.Errorf\([^)]+\)/.test(content) && !content.includes('%w');
    const hasErrorReturn = /return.*err/.test(content);

    if (errorfWithoutWrap && hasErrorReturn) {
      warnings.push(`${fileName}: Uses fmt.Errorf without %w - consider wrapping errors`);
    }
  }

  return {
    name: 'Go Error Wrapping (G3)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 27: Go Logger Usage (G4)
// ============================================================================

function checkGoLoggerUsage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'Go Logger Usage (G4)', passed: true, errors, warnings };
  }

  const findGoFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory()) {
          files.push(...findGoFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.go') && !item.name.endsWith('_test.go')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const goFiles = findGoFiles(goFunctionsDir);

  for (const file of goFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(SERVER_ROOT, file);

    // Check for log.Print/Printf/Println instead of structured logging
    const usesStdLog = /\blog\.(Print|Printf|Println|Fatal|Fatalf)\b/.test(content);
    if (usesStdLog) {
      warnings.push(`${fileName}: Uses log.Print* - consider using structured logging (ctx.Logger)`);
    }
  }

  return {
    name: 'Go Logger Usage (G4)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 28: Go Test File Coverage (G8)
// ============================================================================

function checkGoTestCoverage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'Go Test File Coverage (G8)', passed: true, errors, warnings };
  }

  const uploaderDirs = getDirectories(goFunctionsDir).filter(d => d.includes('uploader'));

  for (const dir of uploaderDirs) {
    const fullDir = path.join(goFunctionsDir, dir);
    const files = fs.readdirSync(fullDir);
    const hasTestFile = files.some(f => f.endsWith('_test.go'));

    if (!hasTestFile) {
      warnings.push(`Uploader '${dir}' has no test file (*_test.go)`);
    }
  }

  return {
    name: 'Go Test File Coverage (G8)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 29: Shared Exports (T2)
// ============================================================================

function checkSharedExports(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const sharedIndexPath = path.join(TS_SRC_DIR, 'shared/src/index.ts');
  if (!fs.existsSync(sharedIndexPath)) {
    return { name: 'Shared Exports (T2)', passed: true, errors, warnings: ['shared/src/index.ts not found'] };
  }

  const indexContent = fs.readFileSync(sharedIndexPath, 'utf-8');

  // Key exports that should be in index.ts
  const requiredExports = [
    'converters',
    'types/pb',
    'framework',
    'config',
  ];

  for (const exp of requiredExports) {
    if (!indexContent.includes(exp)) {
      warnings.push(`shared/src/index.ts should export '${exp}'`);
    }
  }

  return {
    name: 'Shared Exports (T2)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 30: Date Handling (T6)
// ============================================================================

function checkDateHandling(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const converterPath = path.join(TS_SRC_DIR, 'shared/src/firestore/converters.ts');
  if (!fs.existsSync(converterPath)) {
    return { name: 'Date Handling (T6)', passed: true, errors, warnings };
  }

  const content = fs.readFileSync(converterPath, 'utf-8');

  // Check for Date parsing patterns that don't use toDate helper
  const manualDateParsing = /new Date\(data\./.test(content);
  const hasToDateHelper = content.includes('toDate') || content.includes('Timestamp');

  if (manualDateParsing && !hasToDateHelper) {
    warnings.push('converters.ts: Manual Date parsing detected - use toDate() helper for Firestore Timestamps');
  }

  return {
    name: 'Date Handling (T6)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 31: Error Response Format (T7)
// ============================================================================

function checkErrorResponseFormat(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const handlerDirs = getDirectories(TS_SRC_DIR).filter(d => d.endsWith('-handler'));

  for (const dir of handlerDirs) {
    const indexPath = path.join(TS_SRC_DIR, dir, 'src/index.ts');
    if (!fs.existsSync(indexPath)) continue;

    const content = fs.readFileSync(indexPath, 'utf-8');

    // Check for non-standard error responses
    const hasStatusSend = /res\.status\(\d+\)\.send\(/.test(content);
    const hasJsonError = /\.json\(\s*\{\s*error:/.test(content);
    const hasStringError = /\.send\(\s*['"`]/.test(content);

    if (hasStringError && !hasJsonError) {
      warnings.push(`${dir}: Uses string error responses - prefer { error: string } format`);
    }
  }

  return {
    name: 'Error Response Format (T7)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 32: Integration Field Parity (X3)
// ============================================================================

function checkIntegrationFieldParity(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const tsConverterPath = path.join(TS_SRC_DIR, 'shared/src/firestore/converters.ts');
  const goConverterPath = path.join(GO_SRC_DIR, 'pkg/firestore/user_converter.go');

  if (!fs.existsSync(tsConverterPath) || !fs.existsSync(goConverterPath)) {
    return { name: 'Integration Field Parity (X3)', passed: true, errors, warnings };
  }

  const tsContent = fs.readFileSync(tsConverterPath, 'utf-8');
  const goContent = fs.readFileSync(goConverterPath, 'utf-8');

  // Check for integration fields that exist in one but not the other
  const integrationFields = ['strava', 'fitbit', 'hevy', 'parkrun'];

  for (const field of integrationFields) {
    const inTs = tsContent.toLowerCase().includes(field);
    const inGo = goContent.toLowerCase().includes(field);

    if (inTs && !inGo) {
      warnings.push(`Integration '${field}' in TS converter but not Go converter`);
    }
    if (inGo && !inTs) {
      warnings.push(`Integration '${field}' in Go converter but not TS converter`);
    }
  }

  return {
    name: 'Integration Field Parity (X3)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 33: ActivitySource Handler Coverage (X4)
// ============================================================================

function checkSourceHandlerCoverage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // Get ActivitySource enum values from generated types
  const activitySourcePath = path.join(TS_SRC_DIR, 'shared/src/types/pb/activity.ts');
  if (!fs.existsSync(activitySourcePath)) {
    return { name: 'ActivitySource Handler Coverage (X4)', passed: true, errors, warnings };
  }

  const content = fs.readFileSync(activitySourcePath, 'utf-8');
  const enumPattern = /SOURCE_(\w+)\s*=/g;
  const sources: string[] = [];
  let match;
  while ((match = enumPattern.exec(content)) !== null) {
    const sourceName = match[1];
    if (!['UNKNOWN', 'UNSPECIFIED', 'TEST'].includes(sourceName)) {
      sources.push(sourceName);
    }
  }

  // Check for corresponding handlers
  const handlerDirs = getDirectories(TS_SRC_DIR).filter(d => d.endsWith('-handler'));
  const handlerNames = handlerDirs.map(d => d.toLowerCase());

  // Exemptions (sources that don't need dedicated handlers)
  const exemptions = ['PARKRUN_RESULTS']; // Uses inputs-handler

  for (const source of sources) {
    if (exemptions.includes(source)) continue;

    const expectedHandler = source.toLowerCase().replace(/_/g, '-');
    const hasHandler = handlerNames.some(h => h.includes(expectedHandler) || h.includes(source.toLowerCase()));

    if (!hasHandler) {
      warnings.push(`Source '${source}' may not have a dedicated handler`);
    }
  }

  return {
    name: 'ActivitySource Handler Coverage (X4)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 34: Numeric Enum Usage (E3)
// ============================================================================

function checkNumericEnumUsage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const findTsFiles = (dir: string): string[] => {
    if (!fs.existsSync(dir)) return [];
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules') && !item.name.includes('dist') && !item.name.includes('types')) {
          files.push(...findTsFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.ts') && !item.name.endsWith('.test.ts')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  // Check converters for numeric comparisons with enum-like context
  const converterPath = path.join(TS_SRC_DIR, 'shared/src/firestore/converters.ts');
  if (fs.existsSync(converterPath)) {
    const content = fs.readFileSync(converterPath, 'utf-8');

    // Check for patterns like "=== 27" or "!== 0" that might be enum comparisons
    const numericPattern = /(?:type|source|activityType|destination)\s*[=!]==\s*\d+/gi;
    const matches = content.match(numericPattern);
    if (matches) {
      for (const m of matches) {
        warnings.push(`converters.ts: Numeric enum comparison '${m}' - use enum constant`);
      }
    }
  }

  return {
    name: 'Numeric Enum Usage (E3)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 35: Mandatory Formatter Usage (E6)
// ============================================================================

function checkMandatoryFormatterUsage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const findTsFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules') && !item.name.includes('dist')) {
          files.push(...findTsFiles(fullPath));
        } else if (item.isFile() && (item.name.endsWith('.tsx') || item.name.endsWith('.ts'))) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  // Check server TypeScript
  const serverFiles = findTsFiles(TS_SRC_DIR);
  for (const file of serverFiles) {
    // Skip the generated formatter file itself
    if (file.includes('enum-formatters')) continue;

    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(SERVER_ROOT, file);

    // Check for case statements with enum types (manual mapping)
    const casePatterns = [
      /case\s+ActivityType\./g,
      /case\s+Destination\./g,
      /case\s+ActivitySource\./g,
      /case\s+MuscleGroup\./g,
    ];

    for (const pattern of casePatterns) {
      if (pattern.test(content)) {
        const enumName = pattern.source.match(/case\s+(\w+)/)?.[1];
        warnings.push(`${fileName}: Manual enum-to-string mapping for ${enumName} - use format${enumName}()`);
      }
    }

    // Check for manual formatter function definitions (duplicating generated formatters)
    const manualFormatterPatterns = [
      { pattern: /const\s+formatActivityType\s*=|function\s+formatActivityType\s*\(/g, name: 'formatActivityType' },
      { pattern: /const\s+getDestinationName\s*=|function\s+getDestinationName\s*\(/g, name: 'formatDestination' },
      { pattern: /const\s+formatDestination\s*=|function\s+formatDestination\s*\(/g, name: 'formatDestination' },
      { pattern: /const\s+formatActivitySource\s*=|function\s+formatActivitySource\s*\(/g, name: 'formatActivitySource' },
    ];

    for (const { pattern, name } of manualFormatterPatterns) {
      if (pattern.test(content)) {
        warnings.push(`${fileName}: Manual ${name} function - import from 'enum-formatters' instead`);
      }
    }

    // Check for EnumType[value] pattern (reverse enum lookup for display)
    const reverseLookupPatterns = [
      { pattern: /ActivityType\[\w+\]/g, name: 'ActivityType' },
      { pattern: /Destination\[\w+\]/g, name: 'Destination' },
    ];

    for (const { pattern, name } of reverseLookupPatterns) {
      if (pattern.test(content)) {
        warnings.push(`${fileName}: ${name}[value] reverse lookup - use format${name}() instead`);
      }
    }
  }

  // Check web if exists
  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (fs.existsSync(webDir)) {
    const webAppDir = path.join(webDir, 'src', 'app');
    if (fs.existsSync(webAppDir)) {
      const webFiles = findTsFiles(webAppDir);
      for (const file of webFiles) {
        if (file.includes('enum-formatters')) continue;

        const content = fs.readFileSync(file, 'utf-8');
        const fileName = path.relative(webDir, file);

        const casePatterns = [
          /case\s+ActivityType\./g,
          /case\s+Destination\./g,
          /case\s+ActivitySource\./g,
        ];

        for (const pattern of casePatterns) {
          if (pattern.test(content)) {
            const enumName = pattern.source.match(/case\s+(\w+)/)?.[1];
            warnings.push(`web/${fileName}: Manual enum-to-string mapping for ${enumName} - use format${enumName}()`);
          }
        }
      }
    }
  }

  return {
    name: 'Mandatory Formatter Usage (E6)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 36: API Endpoint Alignment (W12)
// ============================================================================

function checkApiEndpointAlignment(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  const firebaseJsonPath = path.join(webDir, 'firebase.json');

  if (!fs.existsSync(firebaseJsonPath)) {
    return { name: 'API Endpoint Alignment (W12)', passed: true, errors, warnings };
  }

  const firebaseConfig = JSON.parse(fs.readFileSync(firebaseJsonPath, 'utf-8'));
  const rewrites = firebaseConfig.hosting?.rewrites || [];
  const apiRewrites = rewrites
    .filter((r: { source?: string }) => r.source?.startsWith('/api/'))
    .map((r: { source: string }) => r.source.replace('/api/', '').replace('/**', ''));

  // Check web services for API calls
  const servicesDir = path.join(webDir, 'src', 'app', 'services');
  if (!fs.existsSync(servicesDir)) {
    return { name: 'API Endpoint Alignment (W12)', passed: true, errors, warnings };
  }

  const serviceFiles = fs.readdirSync(servicesDir).filter(f => f.endsWith('.ts'));

  for (const file of serviceFiles) {
    const content = fs.readFileSync(path.join(servicesDir, file), 'utf-8');

    // Find API calls
    const apiPattern = /api\.(?:get|post|put|delete|patch)\(['"`]\/([^'"`]+)/g;
    let match;
    while ((match = apiPattern.exec(content)) !== null) {
      const endpoint = match[1].split('/')[0];
      if (!apiRewrites.some((r: string) => endpoint.includes(r) || r.includes(endpoint))) {
        warnings.push(`${file}: API endpoint '${endpoint}' may not be in firebase.json rewrites`);
      }
    }
  }

  return {
    name: 'API Endpoint Alignment (W12)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 40: Uploader External ID Tracking (G5)
// ============================================================================

function checkUploaderExternalIdTracking(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'Uploader External ID Tracking (G5)', passed: true, errors, warnings };
  }

  const uploaderDirs = getDirectories(goFunctionsDir).filter(d => d.includes('uploader'));

  for (const dir of uploaderDirs) {
    const functionPath = path.join(goFunctionsDir, dir, 'function.go');
    if (!fs.existsSync(functionPath)) continue;

    const content = fs.readFileSync(functionPath, 'utf-8');

    // Check for external ID handling in uploaders
    const hasExternalIdField = /ExternalId|externalId|external_id/.test(content);
    const storesExternalId = /SetExternalId|setExternalId|\.ExternalId\s*=/.test(content);

    if (!hasExternalIdField || !storesExternalId) {
      warnings.push(`Uploader '${dir}' may not track external IDs for updates`);
    }
  }

  return {
    name: 'Uploader External ID Tracking (G5)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 41: OAuth Token Refresh Pattern (G6)
// ============================================================================

function checkOAuthTokenRefresh(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'OAuth Token Refresh Pattern (G6)', passed: true, errors, warnings };
  }

  const oauthHandlers = getDirectories(goFunctionsDir).filter(d =>
    d.includes('oauth') || d.includes('strava') || d.includes('fitbit')
  );

  for (const dir of oauthHandlers) {
    const functionPath = path.join(goFunctionsDir, dir, 'function.go');
    if (!fs.existsSync(functionPath)) continue;

    const content = fs.readFileSync(functionPath, 'utf-8');

    // Check for token refresh pattern
    const hasRefreshToken = /refreshToken|refresh_token/.test(content);
    const hasTokenExpiry = /expiresAt|expires_at|ExpiresIn/.test(content);
    const hasRefreshLogic = /TokenRefresh|refreshAccessToken|oauth.*refresh/.test(content);

    if (hasRefreshToken && !hasTokenExpiry) {
      warnings.push(`Handler '${dir}' has refresh token but may not track expiry`);
    }
  }

  return {
    name: 'OAuth Token Refresh Pattern (G6)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 42: Go Struct Field Naming (G7)
// ============================================================================

function checkGoStructFieldNaming(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const goFunctionsDir = path.join(GO_SRC_DIR, 'functions');
  if (!fs.existsSync(goFunctionsDir)) {
    return { name: 'Go Struct Field Naming (G7)', passed: true, errors, warnings };
  }

  const findGoFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory()) {
          files.push(...findGoFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.go') && !item.name.endsWith('_test.go') && !item.name.endsWith('.pb.go')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const goFiles = findGoFiles(goFunctionsDir);

  for (const file of goFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(SERVER_ROOT, file);

    // Check for snake_case in struct field names (should use CamelCase in Go)
    const structPattern = /type\s+\w+\s+struct\s*\{([^}]+)\}/g;
    let match;
    while ((match = structPattern.exec(content)) !== null) {
      const structBody = match[1];
      // Check for snake_case field names (not json tags)
      const fieldPattern = /^\s*([a-z][a-z_]*[a-z])\s+\w+/gm;
      let fieldMatch;
      while ((fieldMatch = fieldPattern.exec(structBody)) !== null) {
        if (fieldMatch[1].includes('_')) {
          warnings.push(`${fileName}: Struct field '${fieldMatch[1]}' uses snake_case - use CamelCase`);
        }
      }
    }
  }

  return {
    name: 'Go Struct Field Naming (G7)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 43: String-to-Enum Mapping Completeness (E4)
// ============================================================================

function checkStringToEnumMapping(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // Check for string-to-enum mapping functions that may be incomplete
  const findTsFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules') && !item.name.includes('dist')) {
          files.push(...findTsFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.ts') && !item.name.endsWith('.test.ts')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const serverFiles = findTsFiles(TS_SRC_DIR);

  for (const file of serverFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(SERVER_ROOT, file);

    // Check for parseActivityType or similar functions
    const parsePatterns = [
      /function\s+parse\w*ActivityType|const\s+parse\w*ActivityType\s*=/g,
      /function\s+stringTo\w*ActivityType|const\s+stringTo\w*ActivityType\s*=/g,
    ];

    for (const pattern of parsePatterns) {
      if (pattern.test(content)) {
        // Check if it has a default case or handles unknowns
        const hasDefaultCase = content.includes('default:') || content.includes('UNSPECIFIED');
        if (!hasDefaultCase) {
          warnings.push(`${fileName}: String-to-enum mapping may not handle unknown values`);
        }
      }
    }
  }

  return {
    name: 'String-to-Enum Mapping Completeness (E4)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 44: Registry Display Name Coverage (E5)
// ============================================================================

function checkRegistryDisplayNameCoverage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const registryPath = path.join(TS_SRC_DIR, 'shared/src/registry.ts');
  if (!fs.existsSync(registryPath)) {
    return { name: 'Registry Display Name Coverage (E5)', passed: true, errors, warnings };
  }

  const content = fs.readFileSync(registryPath, 'utf-8');

  // Check that all registered items have displayName
  const registrationPattern = /{\s*(?:id|type|providerType)\s*:/g;
  const displayNamePattern = /displayName\s*:/g;

  const registrations = (content.match(registrationPattern) || []).length;
  const displayNames = (content.match(displayNamePattern) || []).length;

  if (registrations > displayNames) {
    warnings.push(`registry.ts: ${registrations - displayNames} registration(s) may be missing displayName`);
  }

  return {
    name: 'Registry Display Name Coverage (E5)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 37: Auth Guard on Protected Routes (W3)
// ============================================================================

function checkAuthGuard(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'Auth Guard on Protected Routes (W3)', passed: true, errors, warnings };
  }

  const pagesDir = path.join(webDir, 'src', 'app', 'pages');
  if (!fs.existsSync(pagesDir)) {
    return { name: 'Auth Guard on Protected Routes (W3)', passed: true, errors, warnings };
  }

  const pageFiles = fs.readdirSync(pagesDir).filter(f => f.endsWith('.tsx'));
  const publicPages = ['LandingPage', 'LoginPage', 'SignupPage', 'NotFoundPage', 'PublicShowcase'];

  for (const file of pageFiles) {
    const baseName = file.replace('.tsx', '');
    const isPublic = publicPages.some(p => baseName.includes(p));
    if (isPublic) continue;

    const content = fs.readFileSync(path.join(pagesDir, file), 'utf-8');

    // Check for auth check pattern
    const hasAuthCheck = content.includes('useAuth') ||
      content.includes('AuthGuard') ||
      content.includes('isAuthenticated') ||
      content.includes('ProtectedRoute');

    if (!hasAuthCheck) {
      warnings.push(`${file}: Protected page may not have auth guard`);
    }
  }

  return {
    name: 'Auth Guard on Protected Routes (W3)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 38: Enum Display Mapping (W8)
// ============================================================================

function checkEnumDisplayMapping(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'Enum Display Mapping (W8)', passed: true, errors, warnings };
  }

  const formatterPath = path.join(webDir, 'src', 'types', 'pb', 'enum-formatters.ts');
  if (!fs.existsSync(formatterPath)) {
    return { name: 'Enum Display Mapping (W8)', passed: true, errors, warnings: ['enum-formatters.ts not found in web'] };
  }

  const content = fs.readFileSync(formatterPath, 'utf-8');

  // Check for UNSPECIFIED handling
  const hasUnspecifiedHandling = content.includes('UNSPECIFIED') && content.includes('return');
  if (!hasUnspecifiedHandling) {
    warnings.push('enum-formatters.ts should handle UNSPECIFIED enum values');
  }

  return {
    name: 'Enum Display Mapping (W8)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 39: Null Safety (W9)
// ============================================================================

function checkNullSafety(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'Null Safety (W9)', passed: true, errors, warnings };
  }

  const servicesDir = path.join(webDir, 'src', 'app', 'services');
  if (!fs.existsSync(servicesDir)) {
    return { name: 'Null Safety (W9)', passed: true, errors, warnings };
  }

  const serviceFiles = fs.readdirSync(servicesDir).filter(f => f.endsWith('.ts'));

  for (const file of serviceFiles) {
    const content = fs.readFileSync(path.join(servicesDir, file), 'utf-8');

    // Check for API responses that might need null checks
    const hasApiCall = /api\.(get|post|put|delete)/.test(content);
    const hasNullCheck = content.includes('|| []') ||
      content.includes('|| {}') ||
      content.includes('?? ') ||
      content.includes('?.') ||
      content.includes('if (!');

    if (hasApiCall && !hasNullCheck) {
      warnings.push(`${file}: API response may need null safety checks`);
    }
  }

  return {
    name: 'Null Safety (W9)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 45: useState with Complex Objects (W4)
// ============================================================================

function checkUseStateComplexObjects(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'useState with Complex Objects (W4)', passed: true, errors, warnings };
  }

  const findTsxFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules')) {
          files.push(...findTsxFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.tsx')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const webAppDir = path.join(webDir, 'src', 'app');
  if (!fs.existsSync(webAppDir)) {
    return { name: 'useState with Complex Objects (W4)', passed: true, errors, warnings };
  }

  const webFiles = findTsxFiles(webAppDir);

  for (const file of webFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(webDir, file);

    // Check for useState with object/array initializers that might benefit from useReducer
    const complexStatePattern = /useState<[^>]*\[\]>|useState<{[^}]+}>|useState\(\{[^}]+\}\)/g;
    const setStateSpread = /set\w+\(\s*\(?\s*prev\s*=>/g;

    const hasComplexState = complexStatePattern.test(content);
    const usesSpreadUpdate = setStateSpread.test(content);

    // Multiple complex states with spread updates might benefit from useReducer
    const stateCount = (content.match(/useState</g) || []).length;
    if (stateCount > 5 && hasComplexState) {
      warnings.push(`${fileName}: ${stateCount} useState hooks - consider useReducer for complex state`);
    }
  }

  return {
    name: 'useState with Complex Objects (W4)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 46: useEffect Dependency Array (W5)
// ============================================================================

function checkUseEffectDependencies(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'useEffect Dependency Array (W5)', passed: true, errors, warnings };
  }

  const findTsxFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules')) {
          files.push(...findTsxFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.tsx')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const webAppDir = path.join(webDir, 'src', 'app');
  if (!fs.existsSync(webAppDir)) {
    return { name: 'useEffect Dependency Array (W5)', passed: true, errors, warnings };
  }

  const webFiles = findTsxFiles(webAppDir);

  for (const file of webFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(webDir, file);

    // Check for useEffect without dependency array (runs on every render)
    const useEffectNoDeps = /useEffect\(\s*\(\)\s*=>\s*{[^}]+}\s*\)/g;
    const effectCount = (content.match(useEffectNoDeps) || []).length;

    if (effectCount > 0) {
      warnings.push(`${fileName}: ${effectCount} useEffect without dependency array`);
    }
  }

  return {
    name: 'useEffect Dependency Array (W5)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 47: Context vs Props Drilling (W6)
// ============================================================================

function checkContextUsage(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'Context vs Props Drilling (W6)', passed: true, errors, warnings };
  }

  const findTsxFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules')) {
          files.push(...findTsxFiles(fullPath));
        } else if (item.isFile() && item.name.endsWith('.tsx')) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const webAppDir = path.join(webDir, 'src', 'app');
  if (!fs.existsSync(webAppDir)) {
    return { name: 'Context vs Props Drilling (W6)', passed: true, errors, warnings };
  }

  const webFiles = findTsxFiles(webAppDir);

  for (const file of webFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(webDir, file);

    // Check for components passing many props that might be candidates for context
    const propsDrillPattern = /\w+={(\w+)}\s+\w+={(\w+)}\s+\w+={(\w+)}\s+\w+={(\w+)}\s+\w+={(\w+)}/g;
    if (propsDrillPattern.test(content)) {
      warnings.push(`${fileName}: Deep props drilling detected - consider using Context`);
    }
  }

  return {
    name: 'Context vs Props Drilling (W6)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 48: CSS Custom Properties Usage (W10)
// ============================================================================

function checkCssCustomProperties(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'CSS Custom Properties Usage (W10)', passed: true, errors, warnings };
  }

  const cssDir = path.join(webDir, 'src');
  const findCssFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules')) {
          files.push(...findCssFiles(fullPath));
        } else if (item.isFile() && (item.name.endsWith('.css') || item.name.endsWith('.scss'))) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const cssFiles = findCssFiles(cssDir);

  for (const file of cssFiles) {
    const content = fs.readFileSync(file, 'utf-8');
    const fileName = path.relative(webDir, file);

    // Check for hardcoded colors instead of CSS custom properties
    const hardcodedColors = content.match(/#[0-9a-fA-F]{3,8}|rgb\([^)]+\)|rgba\([^)]+\)/g) || [];
    const varUsage = content.match(/var\(--/g) || [];

    if (hardcodedColors.length > 10 && varUsage.length < hardcodedColors.length) {
      warnings.push(`${fileName}: ${hardcodedColors.length} hardcoded colors - consider using CSS custom properties`);
    }
  }

  return {
    name: 'CSS Custom Properties Usage (W10)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Check 49: Responsive Media Queries (W11)
// ============================================================================

function checkResponsiveMediaQueries(): CheckResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  const webDir = path.join(SERVER_ROOT, '..', 'web');
  if (!fs.existsSync(webDir)) {
    return { name: 'Responsive Media Queries (W11)', passed: true, errors, warnings };
  }

  const cssDir = path.join(webDir, 'src');
  const findCssFiles = (dir: string): string[] => {
    const files: string[] = [];
    try {
      const items = fs.readdirSync(dir, { withFileTypes: true });
      for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory() && !item.name.includes('node_modules')) {
          files.push(...findCssFiles(fullPath));
        } else if (item.isFile() && (item.name.endsWith('.css') || item.name.endsWith('.scss'))) {
          files.push(fullPath);
        }
      }
    } catch { /* ignore */ }
    return files;
  };

  const cssFiles = findCssFiles(cssDir);
  let totalMediaQueries = 0;
  let filesWithoutResponsive = 0;

  for (const file of cssFiles) {
    const content = fs.readFileSync(file, 'utf-8');

    const mediaQueries = (content.match(/@media/g) || []).length;
    totalMediaQueries += mediaQueries;

    // Skip small files
    if (content.length > 500 && mediaQueries === 0) {
      filesWithoutResponsive++;
    }
  }

  if (filesWithoutResponsive > 5) {
    warnings.push(`${filesWithoutResponsive} CSS files over 500 chars without @media queries - consider responsive design`);
  }

  return {
    name: 'Responsive Media Queries (W11)',
    passed: true,
    errors,
    warnings
  };
}

// ============================================================================
// Main Runner
// ============================================================================

function printResult(result: CheckResult, verbose: boolean): void {
  const status = result.passed ? '✅' : '❌';
  console.log(`\n${status} ${result.name}`);

  for (const error of result.errors) {
    console.log(`   ❌ ${error}`);
  }

  if (verbose || result.warnings.length <= 3) {
    for (const warning of result.warnings) {
      console.log(`   ⚠️  ${warning}`);
    }
  } else if (result.warnings.length > 0) {
    console.log(`   ⚠️  ${result.warnings.length} warnings (use --verbose to see all)`);
  }
}

function main(): void {
  const verbose = process.argv.includes('--verbose') || process.argv.includes('-v');

  console.log('🔍 FitGlue Codebase Lint');
  console.log('========================');

  const checks = [
    checkTerraformCoverage,
    checkIndexJsExports,
    checkConnectorPattern,
    checkRegistryCoverage,
    checkPluginRegistration,
    checkWorkspaceMembership,
    checkProtobufAlignment,
    checkFirebaseRouting,
    checkDestinationTopicSync,
    checkEventsHelperCompleteness,
    checkDestinationUploaderPattern,
    checkDestinationEnumCoverage,
    checkLoopPrevention,
    // Phase 2-6 New Checks
    checkEnvVarAccess,
    checkProtoFreshness,
    checkNoManualEnums,
    checkFormatterCoverage,
    checkJestConfigInheritance,
    checkHandlerPackageScripts,
    checkWebTypesAlignment,
    checkConverterCompleteness,
    checkProtoImportPath,
    checkUseApiPattern,
    checkProtoJsonSerialization,
    // Additional checks
    checkGoContextPropagation,
    checkGoErrorWrapping,
    checkGoLoggerUsage,
    checkGoTestCoverage,
    checkSharedExports,
    checkDateHandling,
    checkErrorResponseFormat,
    checkIntegrationFieldParity,
    checkSourceHandlerCoverage,
    checkNumericEnumUsage,
    checkMandatoryFormatterUsage,
    checkApiEndpointAlignment,
    checkAuthGuard,
    checkEnumDisplayMapping,
    checkNullSafety,
    checkUploaderExternalIdTracking,
    checkOAuthTokenRefresh,
    checkGoStructFieldNaming,
    checkStringToEnumMapping,
    checkRegistryDisplayNameCoverage,
    checkUseStateComplexObjects,
    checkUseEffectDependencies,
    checkContextUsage,
    checkCssCustomProperties,
    checkResponsiveMediaQueries,
  ];

  const results: CheckResult[] = [];

  for (const check of checks) {
    try {
      const result = check();
      results.push(result);
      printResult(result, verbose);
    } catch (error) {
      console.error(`\n❌ Check failed with error: ${error}`);
      results.push({
        name: check.name,
        passed: false,
        errors: [`Check threw error: ${error}`],
        warnings: []
      });
    }
  }

  // Summary
  console.log('\n========================');
  const passed = results.filter(r => r.passed).length;
  const total = results.length;
  const allPassed = passed === total;

  if (allPassed) {
    console.log(`✅ All ${total} checks passed!`);
  } else {
    console.log(`❌ ${passed}/${total} checks passed`);
    console.log('\nFailed checks:');
    for (const result of results.filter(r => !r.passed)) {
      console.log(`  - ${result.name}`);
    }
  }

  // Exit with error code if any check failed
  process.exit(allPassed ? 0 : 1);
}

main();
