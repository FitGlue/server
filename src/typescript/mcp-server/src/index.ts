
import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  Tool,
} from '@modelcontextprotocol/sdk/types.js';

// Tools
import { registerUserTools } from './tools/users';
import { registerFitTools } from './tools/fit_tools';

async function main() {
  const server = new Server(
    {
      name: 'fitglue-mcp-server',
      version: '1.0.0',
    },
    {
      capabilities: {
        tools: {},
      },
    }
  );

  // Registry for tools
  const tools: Tool[] = [];
  const handlers: Record<string, (args: any) => Promise<any>> = {};

  const registerTool = (tool: Tool, handler: (args: any) => Promise<any>) => {
    tools.push(tool);
    handlers[tool.name] = handler;
  };

  // Register all tool sets
  registerUserTools(registerTool);
  registerFitTools(registerTool);

  server.setRequestHandler(ListToolsRequestSchema, async () => {
    return {
      tools: tools,
    };
  });

  server.setRequestHandler(CallToolRequestSchema, async (request: any) => {
    const handler = handlers[request.params.name];
    if (!handler) {
      throw new Error(`Tool not found: ${request.params.name}`);
    }

    try {
      const result = await handler(request.params.arguments);

      // If result is already formatted as MCP content, return it
      if (result && typeof result === 'object' && 'content' in result) {
        return result;
      }

      // Default stringification
      return {
        content: [
          {
            type: 'text',
            text: JSON.stringify(result, null, 2),
          },
        ],
      };
    } catch (error: any) {
      return {
        content: [
          {
            type: 'text',
            text: `Error: ${error.message}`,
          },
        ],
        isError: true,
      };
    }
  });

  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error('FitGlue MCP Server running on stdio');
}

main().catch((error) => {
  console.error('Fatal error in MCP server:', error);
  process.exit(1);
});
