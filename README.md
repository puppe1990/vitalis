# Vitalis

Contador de calorias em Go + [Amarra-cais](https://github.com/puppe1990/amarra-cais): diário por refeição, metas calculadas, hidratação, progresso semanal — e um servidor **MCP** para registrar refeições conversando com um agente.

Design: sistema **Vitalis Nutrição** (Stitch) — canvas `#f9f9ff`, emerald `#006947`, Plus Jakarta Sans, cartões `rounded-2xl` com sombra tonal.

Host: `vitalis.apps.gestaobem.com`. noindex.

## Rodar local

```bash
npm install                     # .npmrc tem include=dev, então instala mesmo com NODE_ENV=production
npm run build                   # Tailwind → web/static/css/styles.css
go run ./cmd/server             # http://127.0.0.1:4258
```

Demo em development: `demo@example.com` / `password`. A primeira subida cria o catálogo TACO inicial e imprime o token MCP de demo no log.

## Testes — um comando cada

```bash
go test ./... -race -count=1    # Go: domínio, store, handlers, API
cd mcp && npm test              # MCP: cliente, ferramentas, protocolo (node --test)
npm run format:check            # Prettier
```

Fixtures vêm de `internal/testdata` (`gofakeit`, seed `20261116`) e, no MCP, de `mcp/test/factories.js` (`@faker-js/faker`, mesma seed).

## Como as metas são calculadas

`internal/nutrition` é puro (sem HTTP/SQL). Mifflin-St Jeor → fator de atividade → ajuste de ±500 kcal pelo objetivo → piso de 1.200 kcal → macros 35/35/30 (emagrecer), 30/40/30 (manter) ou 25/45/30 (ganhar). Água: 35 ml/kg, entre 1,5 L e 4 L.

O mesmo pacote alimenta a tela e a API, então o número do dashboard e o número que o agente lê nunca divergem.

## API JSON e MCP

O app expõe `/api/v1` autenticado por **Bearer token** (criado em _Sua meta → Acesso por API (MCP)_). Só o hash SHA-256 é gravado; o token aparece uma vez.

| Método   | Rota                                | Uso                                                  |
| -------- | ----------------------------------- | ---------------------------------------------------- |
| GET      | `/api/v1/diary?d=YYYY-MM-DD`        | totais, saldo, macros, água, refeições               |
| GET      | `/api/v1/foods?q=&category=&limit=` | busca no catálogo                                    |
| GET      | `/api/v1/foods/{id}`                | ficha + porção padrão                                |
| POST     | `/api/v1/foods`                     | cria alimento manual (valores por 100 g)             |
| POST     | `/api/v1/entries`                   | registra porção (`food_id`, `grams`, `meal`, `date`) |
| DELETE   | `/api/v1/entries/{id}`              | remove registro                                      |
| POST     | `/api/v1/photos`                    | salva foto do prato (multipart `photo` + `meal`)     |
| GET      | `/api/v1/photos/{id}`               | baixa a foto (bytes)                                 |
| DELETE   | `/api/v1/photos/{id}`               | remove a foto                                        |
| POST     | `/api/v1/water`                     | soma hidratação                                      |
| GET/POST | `/api/v1/goal`                      | lê / recalcula a meta                                |
| GET      | `/api/v1/progress?d=`               | resumo semanal                                       |
| GET/POST | `/api/v1/weight`                    | pesagens                                             |

O servidor MCP em `mcp/` fala stdio e é um adaptador fino sobre essa API:

```bash
cd mcp && npm install
VITALIS_URL=http://127.0.0.1:4258 VITALIS_TOKEN=vit_... node src/index.js
```

Ferramentas: `get_day`, `search_foods`, `get_food`, `create_food`, `log_meal`, `log_photo`, `delete_photo`, `delete_entry`, `add_water`, `get_goal`, `save_goal`, `set_target`, `get_progress`, `list_weight`, `save_weight`.

Registro em um cliente MCP:

```json
{
  "mcpServers": {
    "vitalis": {
      "command": "node",
      "args": ["/caminho/para/vitalis/mcp/src/index.js"],
      "env": {
        "VITALIS_URL": "https://vitalis.apps.gestaobem.com",
        "VITALIS_TOKEN": "vit_..."
      }
    }
  }
}
```

## Estrutura

| Caminho                | Responsabilidade                                                                       |
| ---------------------- | -------------------------------------------------------------------------------------- |
| `internal/nutrition/`  | matemática pura: escala, macros, saldo, metas, semana                                  |
| `internal/photos/`     | fotos do prato: sniff, MemoryBucket, cliente B2                                        |
| `internal/store/`      | SQLite + migrations + catálogo TACO                                                    |
| `internal/handlers/`   | páginas HTML (`view.Write`)                                                            |
| `internal/api/`        | API JSON `/api/v1`                                                                     |
| `internal/testdata/`   | fixtures com `gofakeit`                                                                |
| `web/templates/pages/` | `diario`, `alimentos`, `alimento`, `alimento_novo`, `progresso`, `metas`, `api_tokens` |
| `mcp/`                 | servidor MCP (Node, stdio)                                                             |

## Deploy

`cleat__deploy app=vitalis ref=main` — porta 4258.

Envs de produção: `ENV=production`, `PORT`, `APP_URL`, `ADMIN_TOKEN`, `LOCALE=pt`, `DB_PATH`.

Fotos do prato (opcional): `B2_APPLICATION_KEY_ID`, `B2_APPLICATION_KEY`, `B2_BUCKET`. Sem as três, o diário esconde o formulário de foto. Balde público; o `<img>` aponta para a URL nativa do B2 (`{downloadUrl}/file/{bucket}/{key}`). `GET /diario/fotos/{id}` continua como proxy autenticado. JPEG, PNG, WebP e HEIC, até 8 MB.

## Decisões que valem saber

- **Dia do diário = dia local do servidor.** A API aceita `date` explícita para não depender de fuso em sessões longas.
- **Status do saldo some quando o dia não tem registros.** "Déficit acentuado" às 8h da manhã é leitura errada, não informação.
- **Semana ignora dias vazios** na média, aderência e saldo. Um dia sem registro é ausência de dado, não zero kcal.
- **Sem `appearance: base-select` nem select nativo nos formulários.** O `amarra.js` aprimora todo `<select>` e as regras do componente não acompanham o app; os formulários usam pills de rádio.
- **Barra de macro é safelisted no Tailwind.** As classes são montadas como `vitalis-bar-{{ .Color }}`, que o scanner não enxerga — sem o safelist o build de produção deixa todas as barras vazias.
