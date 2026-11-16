// Tests for the Vitalis HTTP client.
//
// The client is the only place that knows the API shape, so these tests pin the
// request it builds (path, method, auth header, body) and how it reports an
// error body — the two things a tool handler must not get wrong.

import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";

import { VitalisClient, VitalisError } from "../src/client.js";
import { entry, food } from "./factories.js";

/** Replace global fetch for one test and record every call. */
function stubFetch(t, responder) {
  const calls = [];
  const original = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init });
    return responder(String(url), init);
  };
  t.after(() => {
    globalThis.fetch = original;
  });
  return calls;
}

function jsonResponse(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("VitalisClient", () => {
  it("requires a base URL and a token", () => {
    assert.throws(() => new VitalisClient({ token: "x" }), /baseUrl/);
    assert.throws(() => new VitalisClient({ baseUrl: "http://x" }), /token/);
  });

  it("sends the bearer token on every request", async (t) => {
    const calls = stubFetch(t, () => jsonResponse({ date: "2026-11-16" }));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "vit_abc" });

    await client.getDiary({ date: "2026-11-16" });

    assert.equal(calls.length, 1);
    assert.equal(calls[0].init.headers.Authorization, "Bearer vit_abc");
    assert.match(calls[0].url, /\/api\/v1\/diary\?d=2026-11-16$/);
  });

  it("does not double a trailing slash in the base URL", async (t) => {
    const calls = stubFetch(t, () => jsonResponse({ foods: [], count: 0 }));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258/", token: "t" });

    await client.searchFoods({ q: "arroz" });

    assert.match(calls[0].url, /4258\/api\/v1\/foods/);
    assert.doesNotMatch(calls[0].url, /\/\/api/);
  });

  it("URL-encodes the search query", async (t) => {
    const calls = stubFetch(t, () => jsonResponse({ foods: [], count: 0 }));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    await client.searchFoods({ q: "pão de queijo" });

    assert.match(calls[0].url, /q=p%C3%A3o\+de\+queijo|q=p%C3%A3o%20de%20queijo/);
  });

  it("posts an entry as JSON with the portion", async (t) => {
    const calls = stubFetch(t, () => jsonResponse({ entry: entry({ food_id: 7 }) }, 201));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    const result = await client.logEntry({
      foodId: 7,
      grams: 150,
      meal: "almoco",
      date: "2026-11-16",
    });

    assert.equal(calls[0].init.method, "POST");
    assert.equal(calls[0].init.headers["Content-Type"], "application/json");
    const body = JSON.parse(calls[0].init.body);
    assert.deepEqual(body, { food_id: 7, grams: 150, meal: "almoco", date: "2026-11-16" });
    assert.equal(result.entry.food_id, 7);
  });

  it("posts a photo as multipart without a JSON content type", async (t) => {
    const calls = stubFetch(t, () => jsonResponse({ photo: { id: 9, meal: "almoco" } }, 201));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    const result = await client.logPhoto({
      meal: "almoco",
      date: "2026-11-16",
      filename: "prato.jpg",
      body: Buffer.from([0xff, 0xd8, 0xff]),
    });

    assert.equal(calls[0].init.method, "POST");
    assert.equal(calls[0].init.headers["Content-Type"], undefined);
    assert.ok(calls[0].init.body instanceof FormData);
    assert.match(calls[0].url, /\/api\/v1\/photos\?d=2026-11-16$/);
    assert.equal(result.photo.id, 9);
  });

  it("deletes a photo by id", async (t) => {
    const calls = stubFetch(t, () => new Response(null, { status: 204 }));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    const result = await client.deletePhoto({ id: 9 });

    assert.equal(calls[0].init.method, "DELETE");
    assert.match(calls[0].url, /\/api\/v1\/photos\/9$/);
    assert.equal(result, null);
  });

  it("omits an undefined date instead of sending null", async (t) => {
    const calls = stubFetch(t, () => jsonResponse({ entry: entry() }, 201));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    await client.logEntry({ foodId: 1, grams: 100, meal: "lanche" });

    const body = JSON.parse(calls[0].init.body);
    assert.equal("date" in body, false);
  });

  it("surfaces the API error message with its status", async (t) => {
    stubFetch(t, () => jsonResponse({ error: "food_id does not exist" }, 422));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    await assert.rejects(
      () => client.logEntry({ foodId: 99999, grams: 100, meal: "almoco" }),
      (err) => {
        assert.ok(err instanceof VitalisError);
        assert.equal(err.status, 422);
        assert.match(err.message, /food_id does not exist/);
        return true;
      }
    );
  });

  it("reports 401 as an authentication problem", async (t) => {
    stubFetch(t, () => jsonResponse({ error: "invalid or missing bearer token" }, 401));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "bad" });

    await assert.rejects(
      () => client.getDiary({ date: "2026-11-16" }),
      (err) => {
        assert.equal(err.status, 401);
        return true;
      }
    );
  });

  it("keeps the body of a non-JSON error readable", async (t) => {
    stubFetch(t, () => new Response("502 bad gateway", { status: 502 }));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    await assert.rejects(
      () => client.getGoal(),
      (err) => {
        assert.equal(err.status, 502);
        assert.match(err.message, /502/);
        return true;
      }
    );
  });

  it("returns the parsed food list", async (t) => {
    const payload = { foods: [food({ name: "Arroz branco cozido" })], count: 1 };
    stubFetch(t, () => jsonResponse(payload));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    const result = await client.searchFoods({ q: "arroz", limit: 5 });

    assert.equal(result.foods[0].name, "Arroz branco cozido");
    assert.equal(result.count, 1);
  });

  it("deletes an entry with DELETE", async (t) => {
    const calls = stubFetch(t, () => new Response(null, { status: 204 }));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    await client.deleteEntry({ id: 42 });

    assert.equal(calls[0].init.method, "DELETE");
    assert.match(calls[0].url, /\/api\/v1\/entries\/42$/);
  });

  it("does not blow up parsing an empty 204 body", async (t) => {
    stubFetch(t, () => new Response(null, { status: 204 }));
    const client = new VitalisClient({ baseUrl: "http://localhost:4258", token: "t" });

    assert.equal(await client.deleteEntry({ id: 1 }), null);
  });
});
