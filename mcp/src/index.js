#!/usr/bin/env node
// MCP server for Vitalis.
//
// Speaks stdio, so it drops into any MCP client. Configuration is two env vars:
//
//   VITALIS_URL    base URL of the app (default http://127.0.0.1:4258)
//   VITALIS_TOKEN  bearer token from Settings → API access (MCP)
//
// The server holds no state: every tool call is one HTTP request to the app, so
// the diary the model reads is always the diary the browser shows.

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";

import { VitalisClient, VitalisError } from "./client.js";
import { createTools } from "./tools.js";

export const DEFAULT_URL = "http://127.0.0.1:4258";

/** Read the configuration, failing with a message that says how to fix it. */
export function readConfig(env = process.env) {
  const baseUrl = env.VITALIS_URL?.trim() || DEFAULT_URL;
  const token = env.VITALIS_TOKEN?.trim();
  if (!token) {
    throw new Error(
      "VITALIS_TOKEN is not set. Open Settings → API access (MCP) in Vitalis, " +
        "create a token and export it as VITALIS_TOKEN."
    );
  }
  return { baseUrl, token };
}

/**
 * Build the MCP server with every tool registered.
 *
 * Exported so a test can drive the real protocol surface (list + call) without
 * spawning a child process.
 */
export function buildServer({ baseUrl, token, client } = {}) {
  const vitalis = client ?? new VitalisClient({ baseUrl, token });

  const server = new McpServer(
    { name: "vitalis", version: "1.0.0" },
    {
      instructions:
        "Vitalis é um diário de calorias. Use get_day antes de sugerir uma refeição, " +
        "search_foods para achar o id do alimento e log_meal para registrar a porção. " +
        "log_photo salva a foto do prato (caminho local) no Backblaze B2.",
    }
  );

  for (const tool of createTools(vitalis)) {
    server.registerTool(
      tool.name,
      { description: tool.description, inputSchema: tool.schema.shape },
      async (args) => {
        try {
          const { text } = await tool.run(args);
          return { content: [{ type: "text", text }] };
        } catch (err) {
          // A tool failure is a normal answer to the model, not a crash: it must
          // come back as isError so the agent can retry with better arguments
          // instead of the session dying.
          const message =
            err instanceof VitalisError
              ? `${err.message}${err.status === 401 ? " — verifique VITALIS_TOKEN." : ""}`
              : err.message || String(err);
          return { isError: true, content: [{ type: "text", text: message }] };
        }
      }
    );
  }

  return server;
}

async function main() {
  const config = readConfig();
  const server = buildServer(config);
  await server.connect(new StdioServerTransport());
}

// Only auto-start when executed directly: importing this module in a test must
// not open a stdio transport or demand VITALIS_TOKEN.
if (process.argv[1] && import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    console.error(`vitalis-mcp: ${err.message}`);
    process.exit(1);
  });
}
