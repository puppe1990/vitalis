// HTTP client for the Vitalis JSON API.
//
// This is the only module that knows the transport. Tool handlers call methods
// on it, so a change to /api/v1 never reaches the model-facing prose code.

/** An API failure with the status the server returned. */
export class VitalisError extends Error {
  constructor(message, status, body) {
    super(message);
    this.name = "VitalisError";
    this.status = status;
    this.body = body;
  }
}

export class VitalisClient {
  constructor({ baseUrl, token, fetchImpl = globalThis.fetch, timeoutMs = 15000 }) {
    if (!baseUrl) throw new Error("VitalisClient requires a baseUrl");
    if (!token) throw new Error("VitalisClient requires a token");

    // Trailing slashes would produce //api/v1 in the URL, which the router
    // treats as a different path.
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    this.token = token;
    this.fetchImpl = fetchImpl;
    this.timeoutMs = timeoutMs;
  }

  async request(method, path, { query, body } = {}) {
    const url = new URL(`${this.baseUrl}/api/v1${path}`);
    for (const [key, value] of Object.entries(query ?? {})) {
      if (value !== undefined && value !== null && value !== "") {
        url.searchParams.set(key, String(value));
      }
    }

    const init = {
      method,
      headers: {
        Authorization: `Bearer ${this.token}`,
        Accept: "application/json",
      },
      signal: AbortSignal.timeout(this.timeoutMs),
    };
    if (body !== undefined) {
      init.headers["Content-Type"] = "application/json";
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(url, init);

    // 204 has no body; reading it as JSON would throw on a successful delete.
    if (response.status === 204) {
      if (!response.ok)
        throw new VitalisError(`Vitalis API returned ${response.status}`, response.status, null);
      return null;
    }

    const raw = await response.text();
    let parsed = null;
    try {
      parsed = raw ? JSON.parse(raw) : null;
    } catch {
      parsed = null;
    }

    if (!response.ok) {
      const detail = parsed?.error ?? (raw ? raw.slice(0, 200) : `HTTP ${response.status}`);
      throw new VitalisError(`Vitalis API ${response.status}: ${detail}`, response.status, parsed);
    }

    if (parsed === null) {
      throw new VitalisError(
        `Vitalis API returned an unparseable body for ${path}`,
        response.status,
        raw
      );
    }
    return parsed;
  }

  getDiary({ date } = {}) {
    return this.request("GET", "/diary", { query: { d: date } });
  }

  searchFoods({ q, category, limit } = {}) {
    return this.request("GET", "/foods", { query: { q, category, limit } });
  }

  getFood({ id }) {
    return this.request("GET", `/foods/${encodeURIComponent(id)}`);
  }

  createFood(food) {
    return this.request("POST", "/foods", { body: food });
  }

  logEntry({ foodId, grams, meal, date }) {
    return this.request("POST", "/entries", {
      body: { food_id: foodId, grams, meal, date },
    });
  }

  deleteEntry({ id }) {
    return this.request("DELETE", `/entries/${encodeURIComponent(id)}`);
  }

  addWater({ ml, date }) {
    return this.request("POST", "/water", { body: { ml, date } });
  }

  getGoal() {
    return this.request("GET", "/goal");
  }

  saveGoal(profile) {
    return this.request("POST", "/goal", { body: profile });
  }

  getProgress({ date } = {}) {
    return this.request("GET", "/progress", { query: { d: date } });
  }

  listWeight({ limit } = {}) {
    return this.request("GET", "/weight", { query: { limit } });
  }

  saveWeight({ kg, date }) {
    return this.request("POST", "/weight", { body: { kg, date } });
  }
}
