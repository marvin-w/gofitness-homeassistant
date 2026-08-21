# Changelog

All notable changes to the GoFitness add-on are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 1.0.0

First release.

### Added

- **Setup wizard** — sex, height, weight, goal and breastfeeding status produce a
  daily calorie target on first visit.
- **Female and male calorie targets** using the Mifflin–St Jeor equation, with
  safety floors that never drop below BMR and never below 1500 kcal (female) /
  1800 kcal (male).
- **Breastfeeding-safe mode** — raises the floor to 1800 (partial) / 2000
  (exclusive), adds the DGE lactation surcharge (+250 / +500 kcal) and caps the
  daily deficit at 300 kcal. Every recipe carries a breastfeeding-safe flag.
- **Sustainable, healthy-BMI targeting** — no crash diets; a muscle-building goal
  at BMI ≥ 27 automatically falls back to recomposition.
- **43 real recipes** in German and English with ingredient lists, a durable
  recipe search link, and "cook once, eat twice" leftovers. Every recipe states
  one portion's weight, and its kcal/macros are computed from a single documented
  ingredient table (`tools/nutrition/`) rather than guessed.
- **One shared household meal plan** — everyone in the home sees, cooks from and
  ticks off the same weekly plan and shopping list. It is sized to the combined
  calorie target of all household members, made breastfeeding-safe if anyone is
  nursing, and always reuses leftovers. The plan shows how much to cook, honouring
  the "little fish / only breaded" and "little veg" constraints.
- **Global meal-planning settings** — fish, veg, household size, meals per day and
  cook time are shared by the household; personal health data and language stay
  per-user.
- **Progress projection** — current BMI and a deliberately pessimistic estimate of
  the weeks to the next checkpoint and to the goal, so it never adds pressure.
- **Food logging** — planned meals are logged exactly; ad-hoc food is estimated
  by an offline food table (~85 common foods, DE/EN) or, with an Anthropic API
  key, by Claude — including **photo recognition** that asks you to confirm.
- **Gamification** — XP, levels, streaks, 28 badges and a milestone track toward
  a healthy BMI.
- **Per-person weight tracking** tied to Home Assistant users via ingress auth.
- **Available to every household member**, not just admins (`panel_admin: false`),
  with each Home Assistant user getting their own private profile.
- **Home Assistant sensors** — weight, BMI, calories and streak published back to
  Home Assistant, plus optional fitness-tracker step import.
- **German and English** throughout the UI and recipes (German primary).
- **Local-first storage** — a single SQLite database under `/data`, no cloud.
