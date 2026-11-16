-- Nutrition domain. Nutrients live per 100 g on `foods` so a portion is a
-- multiplication and never a re-typed label.

CREATE TABLE IF NOT EXISTS foods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    brand TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    barcode TEXT NOT NULL DEFAULT '',
    kcal_100g REAL NOT NULL,
    protein_100g REAL NOT NULL DEFAULT 0,
    carb_100g REAL NOT NULL DEFAULT 0,
    fat_100g REAL NOT NULL DEFAULT 0,
    portion_label TEXT NOT NULL DEFAULT '',
    portion_grams REAL NOT NULL DEFAULT 100,
    verified INTEGER NOT NULL DEFAULT 0 CHECK (verified IN (0, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Seed re-runs are idempotent on the food name, so `amarra-cais db seed`
    -- can run on every boot without duplicating the catalog.
    UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS idx_foods_category ON foods (category);

CREATE TABLE IF NOT EXISTS goals (
    user_id INTEGER PRIMARY KEY NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_kcal REAL NOT NULL,
    protein_g REAL NOT NULL DEFAULT 0,
    carb_g REAL NOT NULL DEFAULT 0,
    fat_g REAL NOT NULL DEFAULT 0,
    water_ml REAL NOT NULL DEFAULT 2500,
    sex TEXT NOT NULL DEFAULT 'female',
    age_years INTEGER NOT NULL DEFAULT 0,
    height_cm REAL NOT NULL DEFAULT 0,
    weight_kg REAL NOT NULL DEFAULT 0,
    activity TEXT NOT NULL DEFAULT 'moderate',
    direction TEXT NOT NULL DEFAULT 'maintain',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    food_id INTEGER NOT NULL REFERENCES foods(id),
    meal TEXT NOT NULL,
    logged_on DATE NOT NULL,
    grams REAL NOT NULL,
    kcal REAL NOT NULL,
    protein_g REAL NOT NULL DEFAULT 0,
    carb_g REAL NOT NULL DEFAULT 0,
    fat_g REAL NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_entries_user_day ON entries (user_id, logged_on);

CREATE TABLE IF NOT EXISTS day_logs (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    logged_on DATE NOT NULL,
    water_ml REAL NOT NULL DEFAULT 0,
    burned_kcal REAL NOT NULL DEFAULT 0,
    weight_kg REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, logged_on)
);

CREATE TABLE IF NOT EXISTS weights (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    logged_on DATE NOT NULL,
    kg REAL NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, logged_on)
);

-- The MCP server talks to the JSON API with a bearer token. Storing only the
-- SHA-256 keeps a DB dump or backup from being replayable.
CREATE TABLE IF NOT EXISTS api_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens (user_id);
