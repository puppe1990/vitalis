// Protocol-level tests: drive the real McpServer over an in-memory transport.
//
// The unit tests cover each tool in isolation; these prove the server actually
// advertises the tools, that a tool result reaches the client as text, and that
// an API failure arrives as isError instead of killing the session.

import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";

import { buildServer, readConfig } from "../src/index.js";
import { diary } from "./factories.js";

/** Wire a client to a server over the in-memory pair and connect both. */
async function connected(client, server) {
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  await Promise.all([server.connect(serverTransport), client.connect(clientTransport)]);
  return client;
}

function fakeVitalis(overrides = {}) {
  const base = {
    getDiary: async () => diary(),
    searchFoods: async () => ({ foods: [], count: 0 }),
    getFood: async () => ({ food: {}, portion: {} }),
    logEntry: async () => ({ entry: {} }),
    deleteEntry: async () => null,
    addWater: async () => ({ drunk_ml: 250, target_ml: 2240, percent: 11.2 }),
    getGoal: async () => ({ goal: {} }),
    saveGoal: async () => ({ goal: {} }),
    getProgress: async () => ({ days: [] }),
    listWeight: async () => ({ weights: [], count: 0 }),
    saveWeight: async () => ({ kg: 68.4, date: "2026-11-16" }),
    createFood: async () => ({ food: { id: 1, name: "X", kcal_100g: 100 } }),
  };
  return { ...base, ...overrides };
}

describe("MCP server", () => {
  it("advertises every tool over the protocol", async () => {
    const server = buildServer({ client: fakeVitalis() });
    const client = await connected(
      new Client({ name: "test", version: "1.0.0" }, { capabilities: {} }),
      server
    );

    const { tools } = await client.listTools();
    const names = tools.map((t) => t.name).sort();

    assert.deepEqual(names, [
      "add_water",
      "create_food",
      "delete_entry",
      "get_day",
      "get_food",
      "get_goal",
      "get_progress",
      "list_weight",
      "log_meal",
      "save_goal",
      "save_weight",
      "search_foods",
    ]);
    assert.ok(
      tools.every((t) => t.inputSchema),
      "every tool needs an input schema"
    );
  });

  it("returns the diary prose as text content", async () => {
    const server = buildServer({ client: fakeVitalis() });
    const client = await connected(
      new Client({ name: "test", version: "1.0.0" }, { capabilities: {} }),
      server
    );

    const result = await client.callTool({ name: "get_day", arguments: { date: "2026-11-16" } });

    assert.equal(result.isError, undefined);
    assert.equal(result.content[0].type, "text");
    assert.match(result.content[0].text, /2026-11-16/);
  });

  it("turns an API failure into isError, not a crash", async () => {
    const server = buildServer({
      client: fakeVitalis({
        logEntry: async () => {
          const err = new Error("Vitalis API 422: food_id does not exist");
          err.status = 422;
          throw err;
        },
      }),
    });
    const client = await connected(
      new Client({ name: "test", version: "1.0.0" }, { capabilities: {} }),
      server
    );

    const result = await client.callTool({
      name: "log_meal",
      arguments: { food_id: 99999, grams: 100, meal: "almoco" },
    });

    assert.equal(result.isError, true);
    assert.match(result.content[0].text, /food_id does not exist/);
  });

  it("turns a schema violation into isError", async () => {
    const server = buildServer({ client: fakeVitalis() });
    const client = await connected(
      new Client({ name: "test", version: "1.0.0" }, { capabilities: {} }),
      server
    );

    const result = await client.callTool({
      name: "log_meal",
      arguments: { food_id: 1, grams: 99999, meal: "almoco" },
    });

    assert.equal(result.isError, true);
    assert.match(result.content[0].text, /grams|5000/);
  });
});

describe("readConfig", () => {
  it("defaults the URL and requires a token", () => {
    assert.throws(() => readConfig({}), /VITALIS_TOKEN/);
  });

  it("reads both values from the environment", () => {
    const cfg = readConfig({ VITALIS_URL: "http://example.test:9000", VITALIS_TOKEN: "vit_x" });
    assert.deepEqual(cfg, { baseUrl: "http://example.test:9000", token: "vit_x" });
  });

  it("falls back to the local default URL", () => {
    const cfg = readConfig({ VITALIS_TOKEN: "vit_x" });
    assert.equal(cfg.baseUrl, "http://127.0.0.1:4258");
  });
});
