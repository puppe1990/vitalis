// Portuguese formatting for the text the model reads back.
//
// The numbers in the reply must match what the web UI shows, otherwise a user
// who checks the app after a chat sees two different figures.

/** Round to one decimal without the trailing `.0`. */
export function grams(value) {
  const rounded = Math.round(value * 10) / 10;
  if (Number.isInteger(rounded)) return String(rounded);
  return rounded.toFixed(1).replace(".", ",");
}

/** Brazilian thousands separator: 1890 → "1.890". */
export function number(value) {
  const rounded = Math.round(value);
  const sign = rounded < 0 ? "-" : "";
  const digits = String(Math.abs(rounded));
  let out = "";
  for (let i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 === 0) out += ".";
    out += digits[i];
  }
  return sign + out;
}

/** Hydration switches to litres past 1000 ml, like the dashboard card. */
export function millilitres(value) {
  if (value < 1000) return `${number(value)} ml`;
  return `${grams(value / 1000)}L`;
}

/** Always print the sign, matching the "Déficit Total −2.450" tile. */
export function signed(value) {
  const rounded = Math.round(value);
  return rounded > 0 ? `+${number(rounded)}` : number(rounded);
}

export const MEAL_LABELS = {
  cafe_da_manha: "Café da Manhã",
  almoco: "Almoço",
  lanche: "Lanche da Tarde",
  jantar: "Jantar",
  ceia: "Ceia",
};

const MEAL_ALIASES = {
  cafe_da_manha: "cafe_da_manha",
  cafedamanha: "cafe_da_manha",
  "café da manhã": "cafe_da_manha",
  "cafe da manha": "cafe_da_manha",
  breakfast: "cafe_da_manha",
  almoco: "almoco",
  almoço: "almoco",
  lunch: "almoco",
  lanche: "lanche",
  snack: "lanche",
  jantar: "jantar",
  dinner: "jantar",
  ceia: "ceia",
  supper: "ceia",
};

/**
 * Resolve free text to a diary slot.
 *
 * Mirrors nutrition.NormalizeMeal on the server, except it returns null instead
 * of silently defaulting: an MCP client that typed "brunch" should be told, not
 * have the meal filed under a slot it never chose.
 */
export function normalizeMeal(raw) {
  if (typeof raw !== "string") return null;
  const key = raw.trim().toLowerCase();
  return MEAL_ALIASES[key] ?? null;
}

export const STATUS_LABELS = {
  // Display-only buckets from nutrition.DisplayStatus. A day that has not
  // started, or is still in progress, must not be described as a deficit.
  not_started: "dia não iniciado",
  in_progress: "dia em andamento",
  deficit_high: "déficit acentuado",
  deficit_moderate: "déficit moderado",
  deficit_light: "déficit leve",
  on_target: "dentro da meta",
  surplus: "acima da meta",
};
