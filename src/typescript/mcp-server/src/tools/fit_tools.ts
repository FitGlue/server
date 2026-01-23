
import * as path from 'path';
import * as fs from 'fs';
import { execFile } from 'child_process';
import { promisify } from 'util';

const execFileAsync = promisify(execFile);

// Resolve path to bin directory relative to compiled dist/tools/fit_tools.js
// structure: server/src/typescript/mcp-server/dist/tools/fit_tools.js
const BIN_DIR = path.resolve(__dirname, '../../../../../bin');
const FIT_INSPECT_BIN = path.join(BIN_DIR, 'fit-inspect');
const FIT_GEN_BIN = path.join(BIN_DIR, 'fit-gen');

export function registerFitTools(registerTool: (tool: any, handler: (args: any) => Promise<any>) => void) {

  // --- inspect_fit_file ---
  registerTool(
    {
      name: 'inspect_fit_file',
      description: "Inspect a FIT file using the project's fit-inspect binary.",
      inputSchema: {
        type: 'object',
        properties: {
          filePath: { type: 'string', description: 'Absolute path to the FIT file' },
        },
        required: ['filePath'],
      },
    },
    async ({ filePath }: { filePath: string }) => {
      if (!fs.existsSync(filePath)) {
        throw new Error(`File not found: ${filePath}`);
      }

      try {
        const { stdout, stderr } = await execFileAsync(FIT_INSPECT_BIN, [filePath]);
        if (stderr) {
          console.warn('fit-inspect stderr:', stderr);
        }
        return { output: stdout };
      } catch (error: any) {
        throw new Error(`Failed to inspect FIT file: ${error.message}\nStderr: ${error.stderr}`);
      }
    }
  );

  // --- generate_fit_file ---
  registerTool(
    {
      name: 'generate_fit_file',
      description: 'Generate a FIT file from StandardizedActivity JSON using fit-gen.',
      inputSchema: {
        type: 'object',
        properties: {
          inputJsonPath: { type: 'string', description: 'Path to input JSON file' },
          outputFitPath: { type: 'string', description: 'Path where FIT file should be written' },
        },
        required: ['inputJsonPath', 'outputFitPath'],
      },
    },
    async ({ inputJsonPath, outputFitPath }: { inputJsonPath: string, outputFitPath: string }) => {
      if (!fs.existsSync(inputJsonPath)) {
        throw new Error(`Input file not found: ${inputJsonPath}`);
      }

      try {
        const { stdout, stderr } = await execFileAsync(FIT_GEN_BIN, [
          '-input', inputJsonPath,
          '-output', outputFitPath
        ]);

        return {
          message: 'FIT file generated successfully',
          outputPath: outputFitPath,
          stdout: stdout,
          stderr: stderr
        };
      } catch (error: any) {
        throw new Error(`Failed to generate FIT file: ${error.message}\nStderr: ${error.stderr}`);
      }
    }
  );
}
