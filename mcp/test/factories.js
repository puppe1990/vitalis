// Fixtures for the MCP tests.
//
// Every value comes from one seeded faker so a failing assertion reproduces
// with identical data. The shapes mirror exactly what /api/v1 returns, which is
// the contract the tools depend on.

import { faker } from "@faker-js/faker";

export const SEED = 20261116;

/** A faker instance pinned to SEED. */
export function makeFaker() {
  const f = new faker.constructor({ seed: SEED });
  return f;
}

/** A food row as /api/v1/foods emits it. */
export function food(overrides = {}) {
  const f = makeFaker();
  f.seed(overrides.seed ?? SEED);
  const protein = f.number.float({ min: 2, max: 30, fractionDigits: 1 });
  const carb = f.number.float({ min: 0, max: 60, fractionDigits: 1 });
  const fat = f.number.float({ min: 0.5, max: 20, fractionDigits: 1 });
  return {
    id: overrides.id ?? f.number.int({ min: 1, max: 999 }),
    name:
      overrides.name ??
      f.helpers.arrayElement([
        "Peito de frango grelhado",
        "Arroz branco cozido",
        "Banana prata",
        "Ovo de galinha cozido",
        "Aveia em flocos finos",
      ]),
    category: overrides.category ?? "Proteínas",
    kcal_100g: overrides.kcal_100g ?? Number((protein * 4 + carb * 4 + fat * 9).toFixed(1)),
    protein_100g: protein,
    carb_100g: carb,
    fat_100g: fat,
    portion_label: overrides.portion_label ?? "1 porção",
    portion_grams: overrides.portion_grams ?? 100,
    verified: true,
    ...overrides.rest,
  };
}

/** An entry row as /api/v1/diary and /api/v1/entries emit it. */
export function entry(overrides = {}) {
  const f = makeFaker();
  f.seed(overrides.seed ?? SEED);
  const grams = overrides.grams ?? f.number.int({ min: 40, max: 250 });
  return {
    id: overrides.id ?? f.number.int({ min: 1, max: 9999 }),
    food_id: overrides.food_id ?? f.number.int({ min: 1, max: 999 }),
    food_name: overrides.food_name ?? "Peito de frango grelhado",
    meal: overrides.meal ?? "almoco",
    date: overrides.date ?? "2026-11-16",
    grams,
    kcal: overrides.kcal ?? Number((grams * 1.65).toFixed(1)),
    protein_g: overrides.protein_g ?? Number((grams * 0.31).toFixed(1)),
    carb_g: overrides.carb_g ?? 0,
    fat_g: overrides.fat_g ?? Number((grams * 0.036).toFixed(1)),
  };
}

/** A day payload as /api/v1/diary emits it. */
export function diary(overrides = {}) {
  const entries = overrides.entries ?? [entry()];
  const consumed = Number(entries.reduce((sum, e) => sum + e.kcal, 0).toFixed(1));
  const target = overrides.target ?? 1622;
  return {
    date: overrides.date ?? "2026-11-16",
    totals: {
      kcal: consumed,
      protein_g: Number(entries.reduce((s, e) => s + e.protein_g, 0).toFixed(1)),
      carb_g: Number(entries.reduce((s, e) => s + e.carb_g, 0).toFixed(1)),
      fat_g: Number(entries.reduce((s, e) => s + e.fat_g, 0).toFixed(1)),
    },
    target: { kcal: target, protein_g: 142, carb_g: 142, fat_g: 54 },
    balance: {
      target_kcal: target,
      consumed_kcal: consumed,
      burned_kcal: overrides.burned ?? 0,
      net_kcal: consumed - (overrides.burned ?? 0),
      remaining_kcal: Number((target - consumed + (overrides.burned ?? 0)).toFixed(1)),
      status: "deficit_moderate",
    },
    water: { drunk_ml: overrides.water ?? 500, target_ml: 2240 },
    meals: overrides.meals ?? [
      { meal: "cafe_da_manha", entries: [], count: 0, kcal: 0 },
      { meal: "almoco", entries, count: entries.length, kcal: consumed },
      { meal: "lanche", entries: [], count: 0, kcal: 0 },
      { meal: "jantar", entries: [], count: 0, kcal: 0 },
      { meal: "ceia", entries: [], count: 0, kcal: 0 },
    ],
  };
}

/** A goal payload as /api/v1/goal emits it. */
export function goal(overrides = {}) {
  return {
    goal: {
      target_kcal: 1622,
      protein_g: 142,
      carb_g: 142,
      fat_g: 54,
      water_ml: 2240,
      sex: "female",
      age_years: 32,
      height_cm: 168,
      weight_kg: 64,
      activity: "moderate",
      direction: "lose",
      ...overrides,
    },
  };
}

/** A weekly progress payload as /api/v1/progress emits it. */
export function progress(overrides = {}) {
  const days = overrides.days ?? [
    { label: "Seg", kcal: 1710, target_kcal: 1950 },
    { label: "Ter", kcal: 1900, target_kcal: 1950 },
    { label: "Qua", kcal: 1880, target_kcal: 1950 },
    { label: "Qui", kcal: 2120, target_kcal: 1950 },
    { label: "Sex", kcal: 1790, target_kcal: 1950 },
    { label: "Sáb", kcal: 2010, target_kcal: 1950 },
    { label: "Dom", kcal: 1890, target_kcal: 1950 },
  ];
  return {
    start: "2026-11-10",
    end: "2026-11-16",
    days,
    average_kcal: 1890,
    target_kcal: 1950,
    adherence_pct: 57.1,
    balance_kcal: -420,
    best_day: { label: "Seg", kcal: 1710, target_kcal: 1950 },
    worst_day: { label: "Qui", kcal: 2120, target_kcal: 1950 },
  };
}
