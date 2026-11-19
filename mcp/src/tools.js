// MCP tool definitions for Vitalis.
//
// Each tool is a small adapter: validate the arguments, call one client method,
// and turn the JSON into a short Portuguese reply the model can relay. The
// descriptions are written for a model deciding whether to call the tool, so
// they say when to use it, not just what it does.

import { z } from "zod";

import {
  MEAL_LABELS,
  STATUS_LABELS,
  grams,
  millilitres,
  normalizeMeal,
  number,
  signed,
} from "./format.js";

const dateSchema = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/, "date deve ser YYYY-MM-DD")
  .optional()
  .describe("Dia no formato YYYY-MM-DD. Omita para hoje.");

const mealSchema = z
  .string()
  .describe(
    "Refeição: cafe_da_manha, almoco, lanche, jantar ou ceia (aceita 'café da manhã', 'almoço')."
  )
  .transform((raw, ctx) => {
    const meal = normalizeMeal(raw);
    if (!meal) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: `refeição desconhecida: ${raw}. Use cafe_da_manha, almoco, lanche, jantar ou ceia.`,
      });
      return z.NEVER;
    }
    return meal;
  });

const sexSchema = z.enum(["female", "male"], {
  errorMap: () => ({ message: "sex deve ser 'female' ou 'male'" }),
});

const activitySchema = z.enum(["sedentary", "light", "moderate", "active"], {
  errorMap: () => ({ message: "activity deve ser sedentary, light, moderate ou active" }),
});

const directionSchema = z.enum(["lose", "maintain", "gain"], {
  errorMap: () => ({ message: "direction deve ser lose, maintain ou gain" }),
});

/** Wrap a run function so a validation failure reads like a tool error. */
function tool({ name, description, schema, run }) {
  return {
    name,
    description,
    schema,
    async run(rawArgs) {
      const args = schema.parse(rawArgs ?? {});
      return run(args);
    },
  };
}

function result(text, data) {
  return { text, data };
}

export function createTools(client) {
  return [
    tool({
      name: "get_day",
      description:
        "Leia o diário de um dia: calorias consumidas e restantes, macros, água e o que foi registrado em cada refeição. Use antes de sugerir o que comer.",
      schema: z.object({ date: dateSchema }),
      async run({ date }) {
        const day = await client.getDiary({ date });
        return result(renderDay(day), day);
      },
    }),

    tool({
      name: "search_foods",
      description:
        "Busque alimentos na base por nome ou marca. Retorna id, porção padrão e valores por 100 g. Use o id retornado em log_meal.",
      schema: z.object({
        q: z
          .string()
          .min(1, "q é obrigatório")
          .describe("Texto da busca, ex.: 'frango' ou 'aveia'."),
        category: z.string().optional().describe("Filtra por categoria, ex.: 'Proteínas'."),
        limit: z
          .number()
          .int()
          .min(1)
          .max(50)
          .optional()
          .describe("Máximo de resultados (padrão 10)."),
      }),
      async run({ q, category, limit }) {
        const payload = await client.searchFoods({ q, category, limit: limit ?? 10 });
        if (!payload.foods?.length) {
          return result(
            `Nenhum alimento encontrado para "${q}". Crie um com create_food.`,
            payload
          );
        }
        const lines = payload.foods.map(
          (f) =>
            `- id ${f.id} · ${f.name}${f.brand ? ` (${f.brand})` : ""} — ${number(f.kcal_100g)} kcal/100 g · ` +
            `P ${grams(f.protein_100g)}g C ${grams(f.carb_100g)}g G ${grams(f.fat_100g)}g · ` +
            `porção ${f.portion_label || `${grams(f.portion_grams)} g`}`
        );
        return result(
          `${payload.foods.length} alimento(s) para "${q}":\n${lines.join("\n")}`,
          payload
        );
      },
    }),

    tool({
      name: "get_food",
      description:
        "Abra a ficha de um alimento pelo id: valores por 100 g e a porção padrão calculada.",
      schema: z.object({ id: z.number().int().positive().describe("Id do alimento.") }),
      async run({ id }) {
        const payload = await client.getFood({ id });
        const f = payload.food;
        const p = payload.portion ?? {};
        const text =
          `${f.name}: ${number(f.kcal_100g)} kcal/100 g ` +
          `(P ${grams(f.protein_100g)}g · C ${grams(f.carb_100g)}g · G ${grams(f.fat_100g)}g).\n` +
          `Porção padrão ${f.portion_label || `${grams(f.portion_grams)} g`}: ` +
          `${number(p.kcal ?? 0)} kcal.`;
        return result(text, payload);
      },
    }),

    tool({
      name: "create_food",
      description:
        "Cadastre um alimento que não está na base, com os valores do rótulo por 100 g. Depois use log_meal com o id retornado.",
      schema: z.object({
        name: z.string().min(1, "name é obrigatório").describe("Nome do alimento."),
        kcal_100g: z
          .number()
          .positive("kcal_100g deve ser maior que zero")
          .describe("Calorias por 100 g."),
        protein_100g: z.number().min(0).default(0).describe("Proteína por 100 g."),
        carb_100g: z.number().min(0).default(0).describe("Carboidrato por 100 g."),
        fat_100g: z.number().min(0).default(0).describe("Gordura por 100 g."),
        brand: z.string().optional().describe("Marca."),
        category: z.string().optional().describe("Categoria, ex.: 'Cereais'."),
        portion_label: z.string().optional().describe("Medida caseira, ex.: '1 fatia'."),
        portion_grams: z.number().positive().optional().describe("Tamanho da porção em gramas."),
      }),
      async run(args) {
        const payload = await client.createFood(args);
        const f = payload.food;
        return result(
          `Alimento criado: id ${f.id} · ${f.name} — ${number(f.kcal_100g)} kcal/100 g.`,
          payload
        );
      },
    }),

    tool({
      name: "log_meal",
      description:
        "Registre uma porção de um alimento no diário. Use search_foods antes para obter o food_id e a porção recomendada.",
      schema: z.object({
        food_id: z
          .number()
          .int()
          .positive()
          .describe("Id do alimento (de search_foods ou create_food)."),
        grams: z
          .number()
          .min(1, "grams deve ser entre 1 e 5000")
          .max(5000, "grams deve ser entre 1 e 5000")
          .describe("Tamanho da porção em gramas."),
        meal: mealSchema,
        date: dateSchema,
      }),
      async run({ food_id, grams: portion, meal, date }) {
        const payload = await client.logEntry({ foodId: food_id, grams: portion, meal, date });
        const e = payload.entry;
        return result(
          `Registrado em ${MEAL_LABELS[e.meal] ?? e.meal}: ${e.food_name}, ${grams(e.grams)} g — ` +
            `${number(e.kcal)} kcal (P ${grams(e.protein_g)}g · C ${grams(e.carb_g)}g · G ${grams(e.fat_g)}g).`,
          payload
        );
      },
    }),

    tool({
      name: "log_photo",
      description:
        "Salve uma ou mais fotos de um prato no diário (JPEG, PNG, WebP ou HEIC, até 8 MB cada) no Backblaze B2. Passe o caminho local, ou uma lista de caminhos. Use depois de fotografar a refeição.",
      schema: z.object({
        path: z
          .union([
            z.string().min(1, "path é obrigatório"),
            z.array(z.string().min(1)).min(1, "path é obrigatório"),
          ])
          .describe("Caminho local da foto, ou uma lista de caminhos."),
        meal: mealSchema,
        date: dateSchema,
      }),
      async run({ path, meal, date }) {
        const payload = await client.logPhoto({ path, meal, date });
        const photos = payload.photos ?? (payload.photo ? [payload.photo] : []);
        const ids = photos.map((p) => p.id).filter(Boolean);
        const label = MEAL_LABELS[meal] || meal;
        if (ids.length > 1) {
          return result(`Fotos salvas no ${label} (ids ${ids.join(", ")}).`, payload);
        }
        return result(`Foto salva no ${label} (id ${ids[0] ?? payload.photo?.id}).`, payload);
      },
    }),

    tool({
      name: "delete_photo",
      description: "Remova uma foto do diário pelo id retornado em log_photo ou get_day.",
      schema: z.object({ id: z.number().int().positive().describe("Id da foto.") }),
      async run({ id }) {
        await client.deletePhoto({ id });
        return result(`Foto ${id} removida do diário.`, { id });
      },
    }),

    tool({
      name: "delete_entry",
      description: "Remova um registro do diário pelo id. O id aparece em get_day.",
      schema: z.object({ id: z.number().int().positive().describe("Id do registro.") }),
      async run({ id }) {
        await client.deleteEntry({ id });
        return result(`Registro ${id} removido do diário.`, { id });
      },
    }),

    tool({
      name: "add_water",
      description: "Some água ao dia atual. O padrão de 250 ml é um copo.",
      schema: z.object({
        ml: z.number().positive().max(5000).default(250).describe("Mililitros (padrão 250)."),
        date: dateSchema,
      }),
      async run({ ml, date }) {
        const payload = await client.addWater({ ml, date });
        return result(
          `Hidratação: ${millilitres(payload.drunk_ml)} de ${millilitres(payload.target_ml)} ` +
            `(${number(payload.percent)}%).`,
          payload
        );
      },
    }),

    tool({
      name: "get_goal",
      description: "Leia a meta diária atual: calorias, macros, água e o perfil que a gerou.",
      schema: z.object({}),
      async run() {
        const payload = await client.getGoal();
        const g = payload.goal;
        return result(
          `Meta diária: ${number(g.target_kcal)} kcal · P ${grams(g.protein_g)}g ` +
            `C ${grams(g.carb_g)}g G ${grams(g.fat_g)}g · água ${millilitres(g.water_ml)}.`,
          payload
        );
      },
    }),

    tool({
      name: "save_goal",
      description:
        "Defina a meta diária. Sem target_kcal, recalcula pelo perfil (Mifflin-St Jeor, piso de 1.200 kcal). Com target_kcal, fixa o número informado e divide os macros a partir dele.",
      schema: z.object({
        sex: sexSchema.describe("Sexo biológico usado na fórmula."),
        age_years: z
          .number()
          .int()
          .min(14, "age_years deve ser entre 14 e 100")
          .max(100, "age_years deve ser entre 14 e 100"),
        height_cm: z
          .number()
          .min(120, "height_cm deve ser entre 120 e 230")
          .max(230, "height_cm deve ser entre 120 e 230"),
        weight_kg: z
          .number()
          .min(35, "weight_kg deve ser entre 35 e 300")
          .max(300, "weight_kg deve ser entre 35 e 300"),
        activity: activitySchema,
        direction: directionSchema,
        target_kcal: z
          .number()
          .min(1200, "target_kcal deve ser entre 1200 e 6000")
          .max(6000, "target_kcal deve ser entre 1200 e 6000")
          .optional()
          .describe("Meta diária fixa em kcal. Omita para calcular pela fórmula do perfil."),
      }),
      async run(args) {
        const payload = await client.saveGoal(args);
        const g = payload.goal;
        return result(
          `Meta recalculada: ${number(g.target_kcal)} kcal/dia · P ${grams(g.protein_g)}g ` +
            `C ${grams(g.carb_g)}g G ${grams(g.fat_g)}g · água ${millilitres(g.water_ml)}.`,
          payload
        );
      },
    }),

    tool({
      name: "set_target",
      description:
        "Fixe a meta diária de calorias sem mexer no perfil. Use quando a pessoa já sabe o número que quer; os macros são recalculados a partir dele.",
      schema: z.object({
        target_kcal: z
          .number()
          .min(1200, "target_kcal deve ser entre 1200 e 6000")
          .max(6000, "target_kcal deve ser entre 1200 e 6000")
          .describe("Meta diária em kcal."),
      }),
      async run({ target_kcal }) {
        const payload = await client.setTarget({ targetKcal: target_kcal });
        const g = payload.goal;
        return result(
          `Meta fixada em ${number(g.target_kcal)} kcal · P ${grams(g.protein_g)}g ` +
            `C ${grams(g.carb_g)}g G ${grams(g.fat_g)}g.`,
          payload
        );
      },
    }),

    tool({
      name: "get_progress",
      description:
        "Resumo da semana: média diária, aderência à meta, saldo calórico e os dias de pico e de menor consumo.",
      schema: z.object({ date: dateSchema }),
      async run({ date }) {
        const payload = await client.getProgress({ date });
        return result(renderProgress(payload), payload);
      },
    }),

    tool({
      name: "list_weight",
      description: "Liste as pesagens registradas, da mais recente para a mais antiga.",
      schema: z.object({
        limit: z
          .number()
          .int()
          .min(1)
          .max(365)
          .optional()
          .describe("Quantas pesagens trazer (padrão 30)."),
      }),
      async run({ limit }) {
        const payload = await client.listWeight({ limit });
        if (!payload.weights?.length) {
          return result("Nenhuma pesagem registrada ainda.", payload);
        }
        const lines = payload.weights.map((w) => `- ${w.date}: ${grams(w.kg)} kg`);
        return result(`${payload.count} pesagem(ns):\n${lines.join("\n")}`, payload);
      },
    }),

    tool({
      name: "save_weight",
      description: "Registre o peso do dia. Repetir no mesmo dia substitui a pesagem anterior.",
      schema: z.object({
        kg: z.number().min(35, "kg deve ser entre 35 e 300").max(300, "kg deve ser entre 35 e 300"),
        date: dateSchema,
      }),
      async run({ kg, date }) {
        const payload = await client.saveWeight({ kg, date });
        return result(`Peso registrado: ${grams(payload.kg)} kg em ${payload.date}.`, payload);
      },
    }),
  ];
}

function renderDay(day) {
  const b = day.balance;
  const status = STATUS_LABELS[b.status_text] ?? STATUS_LABELS[b.status] ?? b.status;
  const lines = [
    `Diário de ${day.date}`,
    `Consumido: ${number(day.totals.kcal)} kcal de ${number(b.target_kcal)} kcal ` +
      `(${number(Math.max(b.remaining_kcal, 0))} restantes · ${status})`,
    `Macros: P ${grams(day.totals.protein_g)}g / ${grams(day.target.protein_g)}g · ` +
      `C ${grams(day.totals.carb_g)}g / ${grams(day.target.carb_g)}g · ` +
      `G ${grams(day.totals.fat_g)}g / ${grams(day.target.fat_g)}g`,
    `Água: ${millilitres(day.water.drunk_ml)} de ${millilitres(day.water.target_ml)}`,
  ];
  if (b.burned_kcal > 0) {
    lines.push(`Energia ativa queimada: ${number(b.burned_kcal)} kcal`);
  }

  lines.push("", "Refeições:");
  for (const meal of day.meals ?? []) {
    const label = MEAL_LABELS[meal.meal] ?? meal.meal;
    const photos = (meal.photos ?? []).map((p) => `foto ${p.id}`).join(", ");
    if (!meal.count && !photos) {
      lines.push(`- ${label}: nada registrado`);
      continue;
    }
    const items = (meal.entries ?? [])
      .map((e) => `id ${e.id} ${e.food_name} ${grams(e.grams)} g (${number(e.kcal)} kcal)`)
      .join("; ");
    const bits = [];
    if (items) bits.push(items);
    if (photos) bits.push(photos);
    const detail = bits.length ? `: ${bits.join(" · ")}` : "";
    lines.push(`- ${label} — ${number(meal.kcal)} kcal${detail}`);
  }
  return lines.join("\n");
}

function renderProgress(p) {
  const lines = [
    `Semana de ${p.start} a ${p.end}`,
    `Média diária: ${number(p.average_kcal)} kcal (meta ${number(p.target_kcal)} kcal)`,
    `Aderência: ${number(p.adherence_pct)}% dos dias dentro de ±10% da meta`,
    `Saldo da semana: ${signed(p.balance_kcal)} kcal`,
  ];
  if (p.best_day?.label) {
    lines.push(`Menor consumo: ${p.best_day.label} (${number(p.best_day.kcal)} kcal)`);
  }
  if (p.worst_day?.label) {
    lines.push(`Pico: ${p.worst_day.label} (${number(p.worst_day.kcal)} kcal)`);
  }
  return lines.join("\n");
}
