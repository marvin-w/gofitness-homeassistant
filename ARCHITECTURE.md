# GoFitness — Architecture

This document explains how GoFitness is built: the package layout, the data
model, the calorie and BMI formulas with their sources, the HTTP API, how
internationalisation works, and how to add a recipe or a translation.

It is written for the next developer (or the next AI session) picking the project
up. If you only want to install and use the add-on, read the top-level
[README.md](README.md) and [gofitness/DOCS.md](gofitness/DOCS.md) instead.

---

## 1. Design principles

1. **Sustainable health over speed.** The whole point is a healthy BMI reached
   without crash dieting. Every calorie calculation is clamped by safety floors
   (§4). If the "aggressive" number and the "safe" number disagree, the safe one
   wins.
2. **Local-first.** All data lives in a single SQLite file under `/data`. Nothing
   leaves the machine unless the user opts into AI estimation, and then only the
   one food description or photo they submit.
3. **Pure Go, no CGO.** The SQLite driver is [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite),
   a cgo-free translation of SQLite, so `CGO_ENABLED=0` cross-compiles cleanly
   for the supported Home Assistant architectures (aarch64, amd64). The 32-bit
   arches (armhf, armv7, i386) were dropped when Home Assistant deprecated them
   in 2025.12.
4. **One binary.** The frontend is embedded with `go:embed`, so the add-on ships
   as a single static binary plus a thin `run.sh`.
5. **Home Assistant native.** Authentication, per-user profiles and sensor
   publishing all go through Home Assistant rather than reinventing accounts.
6. **German first, fully bilingual.** The backend never emits display prose; it
   emits translation codes that the frontend renders (§6).
7. **Optional intelligence.** AI is a bonus, never a dependency. Remove the API
   key and everything still works from a built-in food table.

---

## 2. Package layout

Module path: `github.com/marvin-w/gofitness-homeassistant/gofitness`

```
cmd/gofitness/         Entrypoint: config load, DB open, graceful shutdown, slog.
internal/
  config/              Reads /data/options.json; GOFITNESS_* env overrides.
  store/               SQLite: schema/migrations, per-user scoped queries.
  nutrition/           Mifflin–St Jeor, BMI, macros, milestones, safety floors.
  recipes/             43 recipes (DE/EN), constraint filtering, i18n vocabulary.
  mealplan/            Week generator + aggregated shopping list.
  gamify/              XP, levels, streaks, badges, milestone track.
  ai/                  Claude client (structured tool call, vision) + offline food table.
  hass/                Ingress-header auth, sensor discovery, sensor publishing.
  api/                 HTTP handlers, JSON API, static file serving.
web/                   Embedded single-page frontend (vanilla JS/CSS).
```

Dependency direction is one-way: `api` depends on everything below it; the
domain packages (`nutrition`, `recipes`, `gamify`) depend on nothing in the
project and are pure and unit-tested in isolation.

### Request lifecycle

```
Browser ──► HA ingress ──► api.Server.ServeHTTP
              (adds X-Remote-User-* and X-Ingress-Path headers)
                              │
              wrap(): resolve HA user ──► store.EnsureUser ──► load Profile
                              │
              handler(w, r, reqCtx{User, HA, Lang, Now})
                              │
              store / nutrition / recipes / mealplan / gamify / ai
                              │
              writeJSON  ◄────┘
```

`cmd/gofitness/main.go` wires the dependencies and owns the `http.Server` (with
generous read/write timeouts, because a vision request is slow). `api.New`
registers every route on a `http.ServeMux` using Go 1.22+ method-and-pattern
routing (`"POST /api/weights"`, `"DELETE /api/food/{id}"`).

---

## 3. Data model

One SQLite database, `"/data/gofitness.db"`. All user data is scoped by
`user_id`, which is the Home Assistant user id (or `local` for direct,
non-ingress access). Schema is applied idempotently from the `migrations` slice
in [`internal/store/store.go`](gofitness/internal/store/store.go); a
`schema_migrations` table records the applied version.

| Table                | Key columns                                                     | Purpose                                              |
| -------------------- | -------------------------------------------------------------- | ---------------------------------------------------- |
| `users`              | `id`, `name`                                                   | One row per Home Assistant user.                     |
| `profiles`           | `user_id` (PK/FK), sex, birth_date, height, weights, activity, goal, breastfeeding, `prefs_json`, `setup_done` | The wizard output + household preferences.           |
| `weights`            | `user_id`, `weight_kg`, `body_fat_pct`, `recorded_at`, `source` | Weigh-ins (manual or from a tracker).                |
| `food_logs`          | `user_id`, `day`, `meal_type`, `name`, `kcal`, macros, `source`, `recipe_id`, `confidence` | Everything eaten. `source` = manual/table/ai/plan.   |
| `workouts`           | `user_id`, `day`, `kind`, `minutes`, `kcal`, `steps`, `source` | Activity, including imported steps.                  |
| `meal_plans`         | `user_id`, `week_start` (unique per user)                      | One generated week.                                  |
| `meal_plan_entries`  | `plan_id` (FK), `day_index`, `meal_type`, `recipe_id`, `servings`, `cooked` | The meals in a plan.                                 |
| `shopping_items`     | `plan_id` (FK), `name`, `amount`, `unit`, `category`, `checked` | Aggregated shopping list for the plan.               |
| `achievements`       | `user_id`, `code` (PK together), `unlocked_at`                 | Which badges a user has unlocked.                    |
| `tracker_links`      | `user_id`, `kind`, `entity_id`                                 | Which HA sensor feeds which metric.                  |
| `app_settings`       | `key`, `value`                                                 | Small global key/value settings.                     |

Foreign keys cascade on delete, so removing a user removes all of their data.
Household preferences (`prefs_json`) — fish policy, veggie level, household size,
meals per day, cook time, excluded tags/ingredients, language, cook-once-eat-twice
— are stored as JSON on the profile rather than as columns, because they are read
and written as a unit.

---

## 4. Nutrition formulas and safety floors

All of this lives in [`internal/nutrition/nutrition.go`](gofitness/internal/nutrition/nutrition.go)
and is fully unit-tested. It is the heart of the app.

### Basal metabolic rate — Mifflin–St Jeor (1990)

```
BMR = 10·weight(kg) + 6.25·height(cm) − 5·age(yr) + s
      s = +5   (male)
      s = −161 (female)
      s = −78  (diverse: the average of the two constants)
```

Source: Mifflin MD, St Jeor ST, et al., *A new predictive equation for resting
energy expenditure in healthy individuals*, Am J Clin Nutr 1990;51:241–247.
Mifflin–St Jeor is chosen over Harris–Benedict because it is more accurate for
modern body compositions.

### Total daily energy expenditure

```
TDEE = BMR · PAL + lactation surcharge
```

Physical Activity Level multipliers: sedentary 1.2, light 1.375, moderate 1.55,
active 1.725, very active 1.9 (the standard Katch/WHO activity factors).

### Goal adjustment

- **Lose:** −20 % of TDEE, capped at −750 kcal/day.
- **Gain muscle:** +10 % of TDEE, capped at +400 kcal/day.
- **Recomp / maintain:** 0.
- A **muscle goal at BMI ≥ 27 automatically becomes recomposition** (note code
  `recomp_suggested`) — you cannot lean-bulk your way to a healthy BMI.

### Safety floors (the point of the package)

The target calories are then raised to the **highest** applicable floor:

- 1500 kcal (female) / 1800 kcal (male);
- while breastfeeding: at least 1800 (partial) or 2000 (exclusive);
- **never below the person's own BMR.**

And, while breastfeeding, the **daily deficit is capped at 300 kcal** regardless
of goal (note `bf_deficit_capped`), because a larger deficit risks milk supply.

### Lactation surcharge — DGE

`+500 kcal/day` exclusive, `+250 kcal/day` partial, added to TDEE. Follows the
Deutsche Gesellschaft für Ernährung (DGE) recommendation for lactating women.
Breastfeeding also adds +20 g protein, raises the minimum carbohydrate to 160 g
(milk production), and adds 700 ml to the water target.

### Macros

- **Protein:** 1.6 g/kg baseline, 2.0 g/kg when losing (muscle preservation),
  1.8 g/kg for muscle/recomp — scaled to `min(weight, healthy-high)` so someone
  well above their healthy range is not handed an unreachable protein number.
- **Fat:** 28 % of target kcal, with a floor of 0.8 g/kg (0.6 g/kg after carbs
  are bought back).
- **Carbs:** the remainder, with a floor of 100 g (160 g breastfeeding); if the
  floor bites, carbs are bought back from fat before ever touching protein.
- **Fibre:** ~14 g per 1000 kcal. **Water:** 35 ml/kg (+700 ml breastfeeding).

### BMI, healthy range, milestones

WHO cut-offs (`BMICategory` returns a stable code, never text). Healthy weight
range is BMI 18.5–24.9 for the person's height. When the user gives no target
weight, the app aims at **BMI 23** (comfortably inside the range, not the edge).
`Milestones` splits the journey into ~2 kg steps (max 12 for long journeys) so
there is always a small win within reach, and flags the step where a healthy BMI
is first crossed.

Rate of change is derived from the energy gap at **7700 kcal per kg of body fat**.

### Projection (deliberately pessimistic)

`Project()` turns the current weight, the goal and the expected weekly change
into a low-pressure forecast: the current BMI, the next ~2 kg checkpoint, and how
many weeks the checkpoint and the goal are away. Every week count is padded by a
`pessimismFactor` (1.4) because real weight change is slower than the calorie
maths — water, plateaus, the odd birthday cake. Under-promising and beating the
estimate is the point; an optimistic number that gets missed creates exactly the
pressure this app avoids. It is surfaced on the Weight screen.

### Where the recipe numbers come from

The per-portion kcal and macros in the recipe database are **not hand-entered** —
they are computed from a single documented ingredient table by
[`tools/nutrition/`](gofitness/tools/nutrition/) (`reference.py` +
`generate.py`) and written back into `recipes.json`. This keeps the numbers
internally consistent and makes every assumption visible: where a product varies
a lot (fresh tortellini, cream cheese) the table says which product was assumed.
Re-run `python3 gofitness/tools/nutrition/generate.py` after editing a recipe.

---

## 5. Recipes and meal planning

Recipes live in [`internal/recipes/data/`](gofitness/internal/recipes/data/):

- `recipes.json` — the 43 recipes with German text, ingredients, macros, prep
  time, and constraint flags (`contains_fish`, `fish_breaded`, `veggie_level`,
  `breastfeeding_safe`).
- `recipes.en.json` — English overrides keyed by recipe id (title, description,
  steps). A missing entry falls back to German rather than disappearing.
- `i18n.json` — the shared vocabulary (ingredient names, units, categories, tags)
  used to translate list items in both recipes and the shopping list.

### Constraint filtering

`Book.Matches(recipe, Filter)` enforces the household rules:

- **Fish policy** (`breaded_only` | `any` | `none`): with `breaded_only`, a
  recipe that contains fish is only allowed if the fish is breaded — this is the
  "little fish, only breaded" (*nur paniertes*) rule.
- **Veggie level** (`MaxVeggieRank`): recipes ranked low/medium/high; the default
  low setting keeps vegetables in the background.
- **Breastfeeding-safe:** when on, only recipes flagged safe (fully cooked, no
  raw fish/egg, no alcohol) are selected.

### The week generator

[`internal/mealplan/`](gofitness/internal/mealplan/) builds a week for the
household: it picks recipes that pass the filter, respects meals-per-day and cook
time, and always applies **cook once, eat twice** — a cooked portion reappears as
a leftover entry on a later day. It then **aggregates the shopping list**: every
recipe ingredient is scaled by the entry's portion count and summed, keyed on the
**German** ingredient name so switching UI language never splits a line into two.
Small pantry amounts are floored at 0.1 so they never round to "0 TL Salz".

Each recipe also carries a `portion_g` — the approximate weight of one finished
portion — so the app can answer "what is one portion" concretely (a serving
*count* alone is ambiguous), and the plan can say roughly how much to cook.

### One shared plan for the whole household

The meal plan is **shared**: there is a single plan per week for the whole home,
stored under a fixed `store.HouseholdID` pseudo-user, and every Home Assistant
user sees, cooks from and ticks off the same one. The plan is sized to the
**combined** daily calorie target of everyone who has finished setup (so there is
enough cooked for each person to eat to their own need) and is made
lactation-safe if *anyone* in the house is breastfeeding. Portion counts already
cover the household, so the shopping list scales by the portion count directly.

The **meal-planning preferences are global** (fish policy, veg level, household
size, meals per day, cook time, exclusions), stored in `app_settings` under
`household_prefs` and overlaid onto every request's profile in `wrap()`. Personal
health data — weight, age, height, goal, breastfeeding — and the interface
language stay per-user.

> The shopping list is still a shopping approximation, not a per-person nutrition
> calculation — this is documented in the code.

### Why recipe links are search links

Each recipe has a `search` field; the link is a Chefkoch (DE) / Allrecipes (EN)
**search URL** built from it, not a deep link to one page. Deep links rot; search
links do not, and the full recipe (ingredients + steps) is embedded in the app
anyway.

---

## 6. Internationalisation

German is primary; English is a full peer. The rule that keeps it maintainable:

> **The backend never returns display prose.** It returns *codes*.

- Nutrition advisories are `nutrition.Note{Code, Params}` (e.g.
  `{"bf_deficit_capped", {"kcal": 300}}`).
- BMI categories are codes (`"normal"`, `"overweight"`, …).
- Badge and level titles are codes (`"level_starter"`, `"streak_7"`).
- Recipe list items translate through the shared `i18n.json` vocabulary.

The frontend dictionary in
[`web/static/js/i18n.js`](gofitness/web/static/js/i18n.js) turns codes plus
params into the final string in the active language. This means a new advisory or
badge is added once in Go (as a code) and once per language in the dictionary,
and there is no prose to accidentally hard-code in English.

Language is resolved per request in `Server.langFor`: `?lang=` query parameter →
`X-GoFitness-Lang` header → the profile's stored preference → the add-on default.

### Ingress base path

Home Assistant serves the add-on under a per-session ingress path. `index.html`
ships a `__BASE_HREF__` placeholder that the server rewrites from the
`X-Ingress-Path` header into a `<base href>` tag. **Every asset and API URL in
the frontend is relative** — introducing an absolute `/api/...` URL would break
ingress.

---

## 7. Authentication and Home Assistant integration

[`internal/hass/`](gofitness/internal/hass/) handles both directions:

- **Inbound auth:** the ingress proxy injects `X-Remote-User-Id`,
  `X-Remote-User-Name` and `X-Remote-User-Display-Name`. Each Home Assistant user
  transparently gets their own profile. Direct (non-ingress) access falls back to
  a shared `local` user. User ids are slugified with an accent-folding table so
  names like `Renée` don't collide or lose characters.
- **Outbound sensors:** with `publish_sensors` on, the client talks to
  `http://supervisor/core` using the `SUPERVISOR_TOKEN` the Supervisor injects,
  and publishes each person's weight, BMI, calorie target and streak. It can also
  discover an existing tracker/step sensor and import steps. This needs
  `hassio_api: true` and `homeassistant_api: true` in `config.yaml`.

---

## 8. AI calorie estimation

[`internal/ai/`](gofitness/internal/ai/) is optional and degrades gracefully.

- **With an Anthropic API key:** the Claude client (`claude-opus-5` by default)
  uses a **forced tool call** for structured output (so the model must return
  `{name, portion_g, kcal, protein_g, carbs_g, fat_g, confidence}` rather than
  prose), `effort: low`, and prompt caching of the system prompt and tool schema.
  For **photos**, the same tool call runs with a vision content block; the UI then
  shows the estimate and asks the user to confirm before logging.
- **Without a key (or on any API failure):** an embedded food table
  ([`data/foods.json`](gofitness/internal/ai/data/foods.json), ~85 common foods in
  German and English) answers text queries, and the photo button is hidden. A
  failed API call silently degrades to this table, so the feature never hard-fails.

Planned meals never go through the AI — they are logged from the recipe's known
macros. Only ad-hoc food is estimated.

---

## 9. HTTP API reference

All endpoints are relative to the ingress base. All responses are JSON. Identity
is resolved from ingress headers; there is no separate auth token.

| Method & path                        | Purpose                                             |
| ------------------------------------ | --------------------------------------------------- |
| `GET /healthz`                       | Liveness + recipe count, AI/HA availability.        |
| `GET /api/bootstrap`                 | Everything the SPA needs on load: profile, capabilities, language, today. |
| `POST /api/profile`                  | Save the wizard result; returns the calculated plan.|
| `POST /api/profile/preview`          | Calculate a plan **without saving** (wizard live preview). |
| `GET /api/weights`                   | List weigh-ins.                                     |
| `POST /api/weights`                  | Add a weigh-in.                                     |
| `DELETE /api/weights/{id}`           | Delete a weigh-in.                                  |
| `GET /api/food`                      | List food log (optionally `?day=YYYY-MM-DD`).       |
| `POST /api/food`                     | Log food.                                           |
| `DELETE /api/food/{id}`              | Delete a food log entry.                            |
| `POST /api/food/estimate`            | Estimate kcal/macros from text or a photo (AI or table). |
| `GET /api/food/search`               | Search the offline food table.                      |
| `GET/POST/DELETE /api/workouts[...]` | List / add / delete workouts.                       |
| `GET /api/recipes`                   | List recipes in the active language.                |
| `GET /api/recipes/{id}`              | One recipe with ingredients and steps.              |
| `GET /api/plan`                      | The current week's plan + shopping list.            |
| `POST /api/plan/generate`            | Generate a new week.                                |
| `POST /api/plan/entries/{id}/cooked` | Mark a planned meal cooked.                         |
| `POST /api/plan/entries/{id}/log`    | Log a planned meal exactly.                         |
| `POST /api/shopping/{id}/check`      | Tick a shopping item.                               |
| `GET /api/stats`                     | Aggregated stats for the dashboard/charts.          |
| `GET /api/gamify`                    | XP, level, streaks, badges, milestones.             |
| `GET/POST /api/trackers[...]`        | List / link / sync fitness-tracker sensors.         |

Errors are `{"error": "..."}` with an appropriate status; 5xx are logged, 4xx are
debug-logged. Handlers return an `apiError{Status, Msg}` and the `wrap` middleware
renders it.

---

## 10. Gamification

[`internal/gamify/`](gofitness/internal/gamify/) derives everything from raw
tracking data (it stores only which badges are unlocked, in `achievements`):

- **XP:** streak days × 5, logged foods × 2 (cap 500), workouts × 5 (cap 200),
  plus badge XP.
- **Levels:** 100 levels on a gentle cumulative curve; titles are codes
  (`level_starter` … `level_legend`, then numeric).
- **Streaks:** a current daily streak and a separate weigh-in streak; yesterday
  still counts so the streak doesn't look broken before today's first action.
- **Badges:** 28 across groups `start` / `streak` / `weight` / `food` — e.g.
  `streak_3/7/14/30/100`, weight-loss milestones, logging consistency.
- **Milestones:** the ~2 kg weight steps from §4, surfaced as a progress track.

---

## 11. Packaging and build

- **`gofitness/config.yaml`** — the add-on manifest: `ingress: true`,
  `ingress_port: 8099`, `hassio_api` + `homeassistant_api` for sensors, and the
  options schema. `/data` is persisted automatically by the Supervisor.
- **`gofitness/build.yaml`** — the Home Assistant base image per architecture.
- **`gofitness/Dockerfile`** — multi-stage. The builder runs natively
  (`--platform=$BUILDPLATFORM golang:1.25-alpine`) and cross-compiles a static
  binary for the target arch (`CGO_ENABLED=0`, `BUILD_ARCH`→`GOARCH`/`GOARM`,
  `-ldflags "-s -w -X main.version=…"`), which is copied onto `${BUILD_FROM}`.
- **`gofitness/run.sh`** — a `bashio` entrypoint that logs the effective config
  and `exec`s the binary. The binary itself reads `/data/options.json`.
- **`repository.yaml`** (repo root) — makes the repo a Home Assistant add-on
  repository so it can be added by URL.
- **`.github/workflows/ci.yaml`** — Go fmt/vet/build/test (with `-race`), the
  Home Assistant add-on linter, and a one-arch Docker build to prove the
  cross-compile wiring.

---

## 12. How-to

### Add a recipe

1. Append an object to
   [`recipes.json`](gofitness/internal/recipes/data/recipes.json) with a unique
   `id`, German `title`/`description`/`steps`, an ingredient list (name, amount,
   unit — reuse existing vocabulary keys where possible), macros, `prep_minutes`,
   a `search` string, and the constraint flags (`contains_fish`, `fish_breaded`,
   `veggie_level`, `breastfeeding_safe`).
2. Add the English text for that `id` to
   [`recipes.en.json`](gofitness/internal/recipes/data/recipes.en.json).
3. If you introduced a new ingredient/unit/tag word, add it to
   [`i18n.json`](gofitness/internal/recipes/data/i18n.json) so it translates in
   the shopping list, and add the ingredient's per-100 g nutrition to
   [`tools/nutrition/reference.py`](gofitness/tools/nutrition/reference.py).
4. Run `python3 gofitness/tools/nutrition/generate.py` to recompute the
   per-portion kcal, macros and `portion_g`. Do **not** hand-edit those numbers.
5. `go test ./internal/recipes/...` checks the data loads and is translatable
   (`Book.MissingTranslations()` is also logged at startup).

### Add a translation string

Add the code and its German + English text to the dictionary in
[`web/static/js/i18n.js`](gofitness/web/static/js/i18n.js). Never return the
display string from Go — return the code and let the dictionary render it.

### Run and test locally

```bash
cd gofitness
go build ./... && go vet ./... && go test ./...
GOFITNESS_DATA_DIR=/tmp/gf GOFITNESS_OPTIONS=/dev/null go run ./cmd/gofitness
curl -s localhost:8099/healthz
```

Override behaviour for local runs with `GOFITNESS_PORT`, `GOFITNESS_DATA_DIR`,
`ANTHROPIC_API_KEY`, `GOFITNESS_LANG`, `GOFITNESS_LOG_LEVEL`, `GOFITNESS_AI_MODEL`.
