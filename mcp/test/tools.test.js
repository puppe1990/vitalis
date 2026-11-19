// Tests for the MCP tool handlers.
//
// Each tool is a thin adapter over the client, so these tests pin the two
// things an adapter can silently get wrong: the argument it forwards and the
// prose it hands back to the model. The fake client records calls, so a tool
// that forwards `grams` as a string or drops the meal fails here.

import assert from "node:assert/strict";
import { beforeEach, describe, it } from "node:test";

import { createTools } from "../src/tools.js";
import { diary, entry, food, goal, progress } from "./factories.js";

/** A fake client that records every call and returns a fixed payload. */
function fakeClient(overrides = {}) {
  const calls = [];
  const record = (name) => async (args) => {
    calls.push({ name, args });
    return overrides[name] ? overrides[name](args) : {};
  };
  return {
    calls,
    getDiary: record("getDiary"),
    searchFoods: record("searchFoods"),
    getFood: record("getFood"),
    createFood: record("createFood"),
    logEntry: record("logEntry"),
    logPhoto: record("logPhoto"),
    deleteEntry: record("deleteEntry"),
    deletePhoto: record("deletePhoto"),
    addWater: record("addWater"),
    getGoal: record("getGoal"),
    saveGoal: record("saveGoal"),
    setTarget: record("setTarget"),
    getProgress: record("getProgress"),
    listWeight: record("listWeight"),
    saveWeight: record("saveWeight"),
  };
}

/** Run a tool by name and return its text content plus the raw JSON payload. */
function call(tools, name, args) {
  const tool = tools.find((t) => t.name === name);
  assert.ok(tool, `tool ${name} not registered`);
  return tool.run(args);
}

describe("tool registry", () => {
  it("registers exactly the documented tools", () => {
    const tools = createTools(fakeClient());
    const names = tools.map((t) => t.name).sort();
    assert.deepEqual(names, [
      "add_water",
      "create_food",
      "delete_entry",
      "delete_photo",
      "get_day",
      "get_food",
      "get_goal",
      "get_progress",
      "list_weight",
      "log_meal",
      "log_photo",
      "save_goal",
      "save_weight",
      "search_foods",
      "set_target",
    ]);
  });

  it("every tool has a non-empty description for the model", () => {
    const tools = createTools(fakeClient());
    for (const tool of tools) {
      assert.ok(
        tool.description && tool.description.length > 20,
        `${tool.name} needs a description`
      );
    }
  });

  it("every tool declares a schema", () => {
    const tools = createTools(fakeClient());
    for (const tool of tools) {
      assert.ok(tool.schema, `${tool.name} needs a zod schema`);
    }
  });
});

describe("search_foods", () => {
  it("forwards the query and limit", async () => {
    const client = fakeClient({ searchFoods: () => ({ foods: [food()], count: 1 }) });
    const tools = createTools(client);

    const result = await call(tools, "search_foods", { q: "frango", limit: 5 });

    assert.equal(client.calls[0].name, "searchFoods");
    assert.equal(client.calls[0].args.q, "frango");
    assert.equal(client.calls[0].args.limit, 5);
    assert.match(result.text, /Peito de frango|Arroz|Banana/);
  });

  it("says so when nothing matches", async () => {
    const tools = createTools(fakeClient({ searchFoods: () => ({ foods: [], count: 0 }) }));

    const result = await call(tools, "search_foods", { q: "xyz" });

    assert.match(result.text, /nenhum|no food|nothing/i);
  });

  it("includes the food id so the model can log it next turn", async () => {
    const tools = createTools(
      fakeClient({
        searchFoods: () => ({ foods: [food({ id: 4711, name: "Banana prata" })], count: 1 }),
      })
    );

    const result = await call(tools, "search_foods", { q: "banana" });

    assert.match(result.text, /4711/);
  });
});

describe("log_meal", () => {
  it("forwards foodId, grams, meal and date as numbers/strings", async () => {
    const client = fakeClient({ logEntry: () => ({ entry: entry({ food_id: 7, grams: 150 }) }) });
    const tools = createTools(client);

    await call(tools, "log_meal", { food_id: 7, grams: 150, meal: "almoco", date: "2026-11-16" });

    assert.deepEqual(client.calls[0].args, {
      foodId: 7,
      grams: 150,
      meal: "almoco",
      date: "2026-11-16",
    });
  });

  it("reports the calories it logged", async () => {
    const tools = createTools(
      fakeClient({ logEntry: () => ({ entry: entry({ food_name: "Arroz", kcal: 156 }) }) })
    );

    const result = await call(tools, "log_meal", { food_id: 4, grams: 120, meal: "almoco" });

    assert.match(result.text, /156/);
    assert.match(result.text, /Arroz/);
  });

  it("rejects a portion outside the app's bounds before calling the API", async () => {
    const client = fakeClient();
    const tools = createTools(client);

    await assert.rejects(
      () => call(tools, "log_meal", { food_id: 1, grams: 9000, meal: "almoco" }),
      /5000|grams/i
    );
    assert.equal(client.calls.length, 0);
  });

  it("rejects an unknown meal", async () => {
    const client = fakeClient();
    const tools = createTools(client);

    await assert.rejects(
      () => call(tools, "log_meal", { food_id: 1, grams: 100, meal: "brunch" }),
      /meal/i
    );
    assert.equal(client.calls.length, 0);
  });
});

describe("log_photo", () => {
  it("forwards the local path, meal and date", async () => {
    const client = fakeClient({
      logPhoto: () => ({ photo: { id: 9, meal: "almoco", content_type: "image/jpeg" } }),
    });
    const tools = createTools(client);

    const result = await call(tools, "log_photo", {
      path: "/tmp/prato.jpg",
      meal: "almoço",
      date: "2026-11-16",
    });

    assert.deepEqual(client.calls[0].args, {
      path: "/tmp/prato.jpg",
      meal: "almoco",
      date: "2026-11-16",
    });
    assert.match(result.text, /9/);
    assert.match(result.text, /Almoço/);
  });

  it("forwards several local paths in one call", async () => {
    const client = fakeClient({
      logPhoto: () => ({
        photos: [
          { id: 9, meal: "almoco" },
          { id: 10, meal: "almoco" },
        ],
        photo: { id: 9, meal: "almoco" },
      }),
    });
    const tools = createTools(client);

    const result = await call(tools, "log_photo", {
      path: ["/tmp/prato1.jpg", "/tmp/prato2.jpg"],
      meal: "almoço",
      date: "2026-11-16",
    });

    assert.deepEqual(client.calls[0].args, {
      path: ["/tmp/prato1.jpg", "/tmp/prato2.jpg"],
      meal: "almoco",
      date: "2026-11-16",
    });
    assert.match(result.text, /9/);
    assert.match(result.text, /10/);
  });
});

describe("delete_photo", () => {
  it("forwards the photo id", async () => {
    const client = fakeClient();
    const tools = createTools(client);

    const result = await call(tools, "delete_photo", { id: 9 });

    assert.deepEqual(client.calls[0].args, { id: 9 });
    assert.match(result.text, /9/);
  });
});

describe("get_day", () => {
  it("summarises the balance, macros, water and meals", async () => {
    const tools = createTools(fakeClient({ getDiary: () => diary() }));

    const result = await call(tools, "get_day", { date: "2026-11-16" });

    assert.match(result.text, /2026-11-16/);
    assert.match(result.text, /1622|1\.622/);
    assert.match(result.text, /kcal/i);
  });

  it("uses the display status the dashboard shows", async () => {
    // The API sends status_text alongside the raw band; the tool must prefer it
    // so an MCP client never calls a day in progress a "steep deficit".
    const payload = diary();
    payload.balance.status = "deficit_high";
    payload.balance.status_text = "in_progress";
    const tools = createTools(fakeClient({ getDiary: () => payload }));

    const result = await call(tools, "get_day", {});

    assert.match(result.text, /andamento|progress/i);
    assert.doesNotMatch(result.text, /acentuado|steep/i);
  });

  it("mentions the meal breakdown so the agent can answer 'what did I eat'", async () => {
    const tools = createTools(fakeClient({ getDiary: () => diary() }));

    const result = await call(tools, "get_day", {});

    assert.match(result.text, /[Aa]lmo|Lunch|lunch/);
  });
});

describe("add_water", () => {
  it("defaults to one glass and forwards the date", async () => {
    const client = fakeClient({
      addWater: () => ({ drunk_ml: 750, target_ml: 2240, percent: 33.5 }),
    });
    const tools = createTools(client);

    const result = await call(tools, "add_water", { ml: 250, date: "2026-11-16" });

    assert.equal(client.calls[0].args.ml, 250);
    assert.match(result.text, /750/);
  });

  it("reports the resulting percentage", async () => {
    const tools = createTools(
      fakeClient({ addWater: () => ({ drunk_ml: 1120, target_ml: 2240, percent: 50 }) })
    );

    const result = await call(tools, "add_water", { ml: 250 });

    assert.match(result.text, /50/);
  });
});

describe("save_goal", () => {
  it("forwards every profile field", async () => {
    const client = fakeClient({ saveGoal: () => goal() });
    const tools = createTools(client);

    await call(tools, "save_goal", {
      sex: "female",
      age_years: 32,
      height_cm: 168,
      weight_kg: 64,
      activity: "moderate",
      direction: "lose",
    });

    assert.deepEqual(client.calls[0].args, {
      sex: "female",
      age_years: 32,
      height_cm: 168,
      weight_kg: 64,
      activity: "moderate",
      direction: "lose",
    });
  });

  it("shows the computed target so the user sees the effect", async () => {
    const tools = createTools(fakeClient({ saveGoal: () => goal() }));

    const result = await call(tools, "save_goal", {
      sex: "female",
      age_years: 32,
      height_cm: 168,
      weight_kg: 64,
      activity: "moderate",
      direction: "lose",
    });

    assert.match(result.text, /1622|1\.622/);
  });

  it("forwards a pinned target so a known number does not have to be derived", async () => {
    const client = fakeClient({ saveGoal: () => goal() });
    const tools = createTools(client);

    await call(tools, "save_goal", {
      sex: "male",
      age_years: 34,
      height_cm: 180,
      weight_kg: 93.2,
      activity: "moderate",
      direction: "lose",
      target_kcal: 2500,
    });

    assert.equal(client.calls[0].args.target_kcal, 2500);
  });

  it("shows the pinned target in the reply", async () => {
    const tools = createTools(fakeClient({ saveGoal: () => goal({ target_kcal: 2500 }) }));

    const result = await call(tools, "save_goal", {
      sex: "male",
      age_years: 34,
      height_cm: 180,
      weight_kg: 93.2,
      activity: "moderate",
      direction: "lose",
      target_kcal: 2500,
    });

    assert.match(result.text, /2\.500|2500/);
  });

  it("rejects an absurd pinned target before the round trip", async () => {
    const client = fakeClient();
    const tools = createTools(client);

    await assert.rejects(
      () =>
        call(tools, "save_goal", {
          sex: "male",
          age_years: 34,
          height_cm: 180,
          weight_kg: 93.2,
          activity: "moderate",
          direction: "lose",
          target_kcal: 25000,
        }),
      /target_kcal|6000/
    );
    assert.equal(client.calls.length, 0);
  });

  it("rejects an implausible age before the round trip", async () => {
    const client = fakeClient();
    const tools = createTools(client);

    await assert.rejects(
      () =>
        call(tools, "save_goal", {
          sex: "female",
          age_years: 5,
          height_cm: 168,
          weight_kg: 64,
          activity: "moderate",
          direction: "lose",
        }),
      /age|idade/i
    );
    assert.equal(client.calls.length, 0);
  });
});

describe("set_target", () => {
  it("pins the target without touching the profile", async () => {
    const client = fakeClient({ setTarget: () => goal({ target_kcal: 2500 }) });
    const tools = createTools(client);

    await call(tools, "set_target", { target_kcal: 2500 });

    assert.deepEqual(client.calls[0], { name: "setTarget", args: { targetKcal: 2500 } });
  });

  it("confirms the new number", async () => {
    const tools = createTools(fakeClient({ setTarget: () => goal({ target_kcal: 2500 }) }));

    const result = await call(tools, "set_target", { target_kcal: 2500 });

    assert.match(result.text, /2\.500|2500/);
  });

  it("rejects an absurd target before the round trip", async () => {
    const client = fakeClient();
    const tools = createTools(client);

    await assert.rejects(() => call(tools, "set_target", { target_kcal: 25000 }), /1200|6000/);
    assert.equal(client.calls.length, 0);
  });
});

describe("get_progress", () => {
  it("reports the weekly average and adherence", async () => {
    const tools = createTools(fakeClient({ getProgress: () => progress() }));

    const result = await call(tools, "get_progress", { date: "2026-11-16" });

    assert.match(result.text, /1\.?890/);
    assert.match(result.text, /57/);
  });

  it("names the best and worst day", async () => {
    const tools = createTools(fakeClient({ getProgress: () => progress() }));

    const result = await call(tools, "get_progress", {});

    assert.match(result.text, /Seg/);
    assert.match(result.text, /Qui/);
  });
});

describe("delete_entry", () => {
  it("forwards the entry id", async () => {
    const client = fakeClient({ deleteEntry: () => null });
    const tools = createTools(client);

    await call(tools, "delete_entry", { id: 42 });

    assert.equal(client.calls[0].args.id, 42);
  });

  it("confirms the removal", async () => {
    const tools = createTools(fakeClient({ deleteEntry: () => null }));

    const result = await call(tools, "delete_entry", { id: 42 });

    assert.match(result.text, /42/);
  });
});

describe("create_food", () => {
  it("forwards the per-100g profile", async () => {
    const client = fakeClient({ createFood: () => ({ food: food({ id: 900, name: "Tapioca" }) }) });
    const tools = createTools(client);

    await call(tools, "create_food", {
      name: "Tapioca",
      kcal_100g: 230,
      protein_100g: 6,
      carb_100g: 38,
      fat_100g: 5,
      portion_grams: 120,
    });

    const args = client.calls[0].args;
    assert.equal(args.name, "Tapioca");
    assert.equal(args.kcal_100g, 230);
    assert.equal(args.portion_grams, 120);
  });

  it("rejects a zero-calorie food", async () => {
    const client = fakeClient();
    const tools = createTools(client);

    await assert.rejects(() => call(tools, "create_food", { name: "Água", kcal_100g: 0 }), /kcal/i);
    assert.equal(client.calls.length, 0);
  });
});

describe("weight tools", () => {
  it("list_weight reports the newest reading", async () => {
    const tools = createTools(
      fakeClient({
        listWeight: () => ({
          weights: [
            { date: "2026-11-16", kg: 68.4 },
            { date: "2026-11-10", kg: 69.1 },
          ],
          count: 2,
        }),
      })
    );

    const result = await call(tools, "list_weight", {});

    assert.match(result.text, /68[,.]4/);
  });

  it("save_weight forwards kg and date", async () => {
    const client = fakeClient({ saveWeight: () => ({ date: "2026-11-16", kg: 68.4 }) });
    const tools = createTools(client);

    await call(tools, "save_weight", { kg: 68.4, date: "2026-11-16" });

    assert.deepEqual(client.calls[0].args, { kg: 68.4, date: "2026-11-16" });
  });
});

describe("get_food", () => {
  it("forwards the id and reports the portion", async () => {
    const client = fakeClient({
      getFood: () => ({
        food: food({ id: 3, name: "Banana prata" }),
        portion: { grams: 80, kcal: 78 },
      }),
    });
    const tools = createTools(client);

    const result = await call(tools, "get_food", { id: 3 });

    assert.equal(client.calls[0].args.id, 3);
    assert.match(result.text, /Banana/);
    assert.match(result.text, /78/);
  });
});

describe("get_goal", () => {
  it("reports the current target", async () => {
    const tools = createTools(fakeClient({ getGoal: () => goal() }));

    const result = await call(tools, "get_goal", {});

    assert.match(result.text, /1622|1\.622/);
  });
});

describe("meal normalisation", () => {
  it("accepts the Portuguese spellings a user would type", async () => {
    const client = fakeClient({ logEntry: () => ({ entry: entry() }) });
    const tools = createTools(client);

    for (const [input, expected] of [
      ["café da manhã", "cafe_da_manha"],
      ["almoço", "almoco"],
      ["jantar", "jantar"],
      ["ceia", "ceia"],
      ["lanche", "lanche"],
    ]) {
      await call(tools, "log_meal", { food_id: 1, grams: 100, meal: input });
      const last = client.calls.at(-1);
      assert.equal(last.args.meal, expected, `${input} should normalise to ${expected}`);
    }
  });
});

describe("empty result handling", () => {
  let tools;
  beforeEach(() => {
    tools = createTools(
      fakeClient({
        getProgress: () => ({
          start: "2026-11-10",
          end: "2026-11-16",
          days: [
            { label: "Seg", kcal: 0, target_kcal: 1622 },
            { label: "Ter", kcal: 0, target_kcal: 1622 },
          ],
          average_kcal: 0,
          target_kcal: 0,
          adherence_pct: 0,
          balance_kcal: 0,
          best_day: { label: "", kcal: 0, target_kcal: 0 },
          worst_day: { label: "", kcal: 0, target_kcal: 0 },
        }),
      })
    );
  });

  it("does not print NaN for an untouched week", async () => {
    const result = await call(tools, "get_progress", {});
    assert.doesNotMatch(result.text, /NaN|undefined/);
  });
});
