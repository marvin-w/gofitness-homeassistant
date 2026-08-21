# GoFitness

Local-first, gamified weight-loss and meal-prep coach for the whole household,
running entirely inside Home Assistant. Female and male calorie targets,
breastfeeding-safe plans, real recipes with an aggregated weekly shopping list,
per-person weight tracking tied to your Home Assistant login, and optional AI
photo calorie estimation.

Everything stays on your machine: a single SQLite database under `/data`, no
cloud account, no telemetry.

## Installation

1. In Home Assistant go to **Settings → Add-ons → Add-on Store**.
2. Open the **⋮** menu (top right) → **Repositories**.
3. Add `https://github.com/marvin-w/gofitness-homeassistant` and close the dialog.
4. Find **GoFitness** in the store, click it and press **Install**.
5. Once installed, press **Start**, then **Open Web UI**.

The first person to open the app walks through a short setup wizard (sex, height,
weight, goal, and whether they are breastfeeding) and immediately gets a daily
calorie target. Each Home Assistant user who opens the add-on gets their own
private profile automatically — no extra login.

## Configuration

All options are optional; the defaults are sensible for a German-speaking
household.

```yaml
anthropic_api_key: ""          # enables AI + photo calorie estimation
ai_model: claude-opus-5        # Claude model used for estimates
default_language: de           # de | en (new profiles start here)
publish_sensors: true          # mirror weight/BMI/calories/streak into HA
log_level: info                # debug | info | warn | error
```

### Option: `anthropic_api_key`

An [Anthropic API key](https://console.anthropic.com/). **Optional.** Without it,
ad-hoc food is estimated from a built-in offline food table (~85 common foods in
German and English) and the photo button is hidden. With a key, you can describe
food in free text or **take a photo** and Claude estimates the calories and
macros, then asks you to confirm before it is logged. If an API call fails the
app silently falls back to the offline table.

### Option: `ai_model`

The Claude model used for estimates. Leave as the default unless you have a
reason to change it.

### Option: `default_language`

`de` or `en`. The language new profiles start in; each person can switch language
in the app afterwards. German is the primary language.

### Option: `publish_sensors`

When `true` (default), GoFitness publishes each person's latest weight, BMI,
calorie target and streak back to Home Assistant as sensors, so you can use them
in dashboards and automations. Set to `false` to keep everything inside the
add-on.

### Option: `log_level`

Controls add-on log verbosity. Use `debug` when reporting a problem.

## Using GoFitness

- **Dashboard** — today's calorie budget, current weight, BMI with the healthy
  band, streak and level.
- **Weight** — log a weigh-in with date and time; the chart shows progress with
  the healthy-BMI band and your target marked.
- **Plan** — generate a household week of meals and an aggregated shopping list.
  Meals respect the "little fish, only breaded" and "little veg" constraints and
  reuse leftovers ("cook once, eat twice"). Tick off shopping items as you buy.
- **Log** — record what you actually ate. Planned meals log exactly; anything
  else is estimated by the food table or the AI (text or photo).
- **Progress** — XP, level, badges and milestones toward a healthy BMI.

### Fitness trackers

If you have a step or activity sensor in Home Assistant (a watch, a phone), you
can point GoFitness at it under **trackers** and it will import steps. Publishing
sensors must be enabled and the add-on needs the Home Assistant API (granted by
default).

## Data and privacy

- All data lives in `/data/gofitness.db` (SQLite) inside the add-on, which Home
  Assistant persists across restarts and includes in add-on backups.
- Nothing is sent anywhere unless you set an Anthropic API key, in which case
  only the food description or photo you submit is sent to Anthropic for that one
  estimate.

## Notes

- **Recipe links are search links** (Chefkoch / Allrecipes) rather than deep
  links to a single page, so they can never rot — and the full recipe with
  ingredients and steps is embedded in the app anyway.
- The shopping list scales each recipe by portions × household size, which
  assumes everyone eats a similar portion. It is a shopping approximation, not a
  per-person nutrition calculation.
- The safety floors are the point: GoFitness will not propose a crash diet. It
  targets a healthy BMI sustainably, with exercise and real food.

## Support

Please open an issue at
<https://github.com/marvin-w/gofitness-homeassistant/issues>.
