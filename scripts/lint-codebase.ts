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
