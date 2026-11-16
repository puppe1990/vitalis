-- Plate photos sit on the meal, not the food entry: one photo of rice and
-- lentils covers two rows. The bytes live in Backblaze B2; this table is the
-- index the diary uses to list and authorize a download.
CREATE TABLE IF NOT EXISTS meal_photos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    meal TEXT NOT NULL,
    logged_on DATE NOT NULL,
    b2_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_meal_photos_user_day ON meal_photos (user_id, logged_on);
