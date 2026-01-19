#!/usr/bin/env npx ts-node
/**
 * generate-enum-formatters.ts
 *
 * Generates formatter functions for proto enums.
 * Output: TypeScript formatters for server and web, Go formatters for server.
 *
 * Usage: npx ts-node scripts/generate-enum-formatters.ts
 */

import * as fs from 'fs';
import * as path from 'path';

const TS_PB_DIR = path.join(__dirname, '..', 'src', 'typescript', 'shared', 'src', 'types', 'pb');
const GO_FORMATTERS_DIR = path.join(__dirname, '..', 'src', 'go', 'pkg', 'types', 'formatters');
const WEB_DIR = path.join(__dirname, '..', '..', 'web');
const WEB_PB_DIR = path.join(WEB_DIR, 'src', 'types', 'pb');

// Enums to generate formatters for, with their source file and display name overrides
interface EnumConfig {
  file: string;
  enumName: string;
  prefix: string;
  displayNameOverrides?: Record<string, string>;
  defaultValue?: string;
}

const ENUM_CONFIGS: EnumConfig[] = [
  {
    file: 'standardized_activity.ts',
    enumName: 'ActivityType',
    prefix: 'ACTIVITY_TYPE_',
    displayNameOverrides: {
      'UNSPECIFIED': 'Workout',
      'EBIKE_RIDE': 'E-Bike Ride',
      'EMOUNTAIN_BIKE_RIDE': 'E-Mountain Bike Ride',
      'HIGH_INTENSITY_INTERVAL_TRAINING': 'HIIT',
    },
    defaultValue: 'Workout',
  },
  {
    file: 'standardized_activity.ts',
    enumName: 'MuscleGroup',
    prefix: 'MUSCLE_GROUP_',
    displayNameOverrides: {
      'UNSPECIFIED': 'Unknown',
    },
    defaultValue: 'Unknown',
  },
  {
    file: 'events.ts',
    enumName: 'Destination',
    prefix: 'DESTINATION_',
    displayNameOverrides: {
      'UNSPECIFIED': 'Unknown',
    },
    defaultValue: 'Unknown',
  },
  {
    file: 'events.ts',
    enumName: 'CloudEventType',
    prefix: 'CLOUD_EVENT_TYPE_',
    defaultValue: 'Unknown',
  },
  {
    file: 'events.ts',
    enumName: 'CloudEventSource',
    prefix: 'CLOUD_EVENT_SOURCE_',
    defaultValue: 'Unknown',
  },
  {
    file: 'activity.ts',
    enumName: 'ActivitySource',
    prefix: 'SOURCE_',
    displayNameOverrides: {
      'UNKNOWN': 'Unknown',
      'FILE_UPLOAD': 'File Upload',
      'PARKRUN_RESULTS': 'Parkrun Results',
    },
    defaultValue: 'Unknown',
  },
];

// Parse enum values from generated TypeScript file
function parseEnumFromFile(filePath: string, enumName: string): Array<{ name: string; value: number }> {
  const content = fs.readFileSync(filePath, 'utf-8');
  const enumPattern = new RegExp(`export enum ${enumName}\\s*\\{([^}]+)\\}`, 's');
  const match = content.match(enumPattern);

  if (!match) {
    console.error(`Could not find enum ${enumName} in ${filePath}`);
    return [];
  }

  const entries: Array<{ name: string; value: number }> = [];
  const valuePattern = /(\w+)\s*=\s*(-?\d+)/g;
  let valueMatch;

  while ((valueMatch = valuePattern.exec(match[1])) !== null) {
    entries.push({
      name: valueMatch[1],
      value: parseInt(valueMatch[2], 10),
    });
  }

  return entries;
}

// Convert SCREAMING_SNAKE_CASE to Title Case
function toTitleCase(str: string): string {
  return str
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ');
}

// Generate TypeScript formatter function
function generateTsFormatter(config: EnumConfig, entries: Array<{ name: string; value: number }>): string {
  const { enumName, prefix, displayNameOverrides = {}, defaultValue = 'Unknown' } = config;
  const funcName = `format${enumName}`;

  const cases = entries
    .filter(e => e.name !== 'UNRECOGNIZED')
    .map(entry => {
      const nameWithoutPrefix = entry.name.replace(prefix, '');
      const displayName = displayNameOverrides[nameWithoutPrefix] || toTitleCase(nameWithoutPrefix);
      return `    case ${enumName}.${entry.name}: return '${displayName}';`;
    })
    .join('\n');

  return `
export function ${funcName}(value: ${enumName} | number | string | undefined | null): string {
  if (value === undefined || value === null) return '${defaultValue}';

  // Handle string numeric values
  if (typeof value === 'string') {
    const parsed = parseInt(value, 10);
    if (!isNaN(parsed)) {
      value = parsed;
    } else {
      // Already a formatted string, return cleaned up
      return value.replace(/([A-Z])/g, ' $1').trim();
    }
  }

  switch (value) {
${cases}
    default: return '${defaultValue}';
  }
}
`;
}

// Generate Go formatter function
function generateGoFormatter(config: EnumConfig, entries: Array<{ name: string; value: number }>): string {
  const { enumName, prefix, displayNameOverrides = {}, defaultValue = 'Unknown' } = config;
  const funcName = `Format${enumName}`;

  const cases = entries
    .filter(e => e.name !== 'UNRECOGNIZED')
    .map(entry => {
      const nameWithoutPrefix = entry.name.replace(prefix, '');
      const displayName = displayNameOverrides[nameWithoutPrefix] || toTitleCase(nameWithoutPrefix);
      return `\tcase pb.${enumName}_${entry.name}:\n\t\treturn "${displayName}"`;
    })
    .join('\n');

  return `
func ${funcName}(value pb.${enumName}) string {
\tswitch value {
${cases}
\tdefault:
\t\treturn "${defaultValue}"
\t}
}
`;
}

// Main generator
function main(): void {
  console.log('🔧 Generating enum formatters...\n');

  let tsContent = `// Code generated by generate-enum-formatters.ts. DO NOT EDIT.
/* eslint-disable */

import { ActivityType, MuscleGroup } from './standardized_activity';
import { Destination, CloudEventType, CloudEventSource } from './events';
import { ActivitySource } from './activity';
`;

  let goContent = `// Code generated by generate-enum-formatters.ts. DO NOT EDIT.
package formatters

import pb "github.com/fitglue/server/src/go/pkg/types/pb"
`;

  for (const config of ENUM_CONFIGS) {
    const filePath = path.join(TS_PB_DIR, config.file);
    if (!fs.existsSync(filePath)) {
      console.warn(`⚠️  Skipping ${config.enumName}: file not found (${filePath})`);
      continue;
    }

    const entries = parseEnumFromFile(filePath, config.enumName);
    if (entries.length === 0) {
      console.warn(`⚠️  No entries found for ${config.enumName}`);
      continue;
    }

    console.log(`✅ ${config.enumName}: ${entries.length} values`);

    tsContent += generateTsFormatter(config, entries);
    goContent += generateGoFormatter(config, entries);
  }

  // Write TypeScript formatters (server)
  const tsOutputPath = path.join(TS_PB_DIR, 'enum-formatters.ts');
  fs.writeFileSync(tsOutputPath, tsContent.trim() + '\n');
  console.log(`\n📁 TypeScript (server): ${tsOutputPath}`);

  // Write Go formatters
  if (!fs.existsSync(GO_FORMATTERS_DIR)) {
    fs.mkdirSync(GO_FORMATTERS_DIR, { recursive: true });
  }
  const goOutputPath = path.join(GO_FORMATTERS_DIR, 'formatters.go');
  fs.writeFileSync(goOutputPath, goContent.trim() + '\n');
  console.log(`📁 Go: ${goOutputPath}`);

  // Copy to web if exists
  if (fs.existsSync(WEB_DIR)) {
    if (!fs.existsSync(WEB_PB_DIR)) {
      fs.mkdirSync(WEB_PB_DIR, { recursive: true });
    }
    const webOutputPath = path.join(WEB_PB_DIR, 'enum-formatters.ts');
    fs.writeFileSync(webOutputPath, tsContent.trim() + '\n');
    console.log(`📁 TypeScript (web): ${webOutputPath}`);
  } else {
    console.log('⏭️  Web directory not found, skipping web formatters');
  }

  console.log('\n✨ Done!');
}

main();
