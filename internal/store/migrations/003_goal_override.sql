-- A user who already knows their calorie target must be able to pin it. The
-- calculated path stays the default (0 means "derive from the profile"); a
-- positive value wins and the macros are split from it with the same ratio.
ALTER TABLE goals ADD COLUMN target_override REAL NOT NULL DEFAULT 0;
