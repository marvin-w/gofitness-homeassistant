# Changelog

All notable changes to the GoFitness add-on are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 1.1.0

### Added

- **One shared household meal plan** — everyone in the home sees, cooks from and
  ticks off the same weekly plan and shopping list. It is sized to the combined
  calorie target of all household members, made breastfeeding-safe if anyone is
  nursing, and always reuses leftovers ("cook once, eat twice" is now always on).
- **Global meal-planning settings** — fish, veg, household size, meals per day and
  cook time are shared by the household; personal health data and language stay
  per-user.
- **Clear portions** — every recipe states one portion's weight; the plan shows
  how much to cook and the recipe view shows the ingredient amounts "for N
  portions".
- **Progress projection** — current BMI and a deliberately pessimistic estimate of
  the weeks to the next checkpoint and to the goal, so it never adds pressure.
- **Available to every household member**, not just admins (`panel_admin: false`).

### Changed

- **Recipe nutrition recomputed** from a single documented ingredient table
  (`tools/nutrition/`) instead of hand-entered numbers, so the values are
  consistent and every assumption is visible.
- **Protein pancakes replaced** with fluffy banana pancakes (normal milk, no egg).
- **German date format** (DD.MM.YYYY) everywhere, including the date picker.

### Fixed

- Streak text now reads "1 Tag" instead of "1 Tage" (singular).

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
  recipe search link, and "cook once, eat twice" leftovers.
- **Household meal planner** — a full week with an aggregated shopping list,
  honouring the "little fish / only breaded" and "little veg" constraints.
- **Food logging** — planned meals are logged exactly; ad-hoc food is estimated
  by an offline food table (~85 common foods, DE/EN) or, with an Anthropic API
  key, by Claude — including **photo recognition** that asks you to confirm.
- **Gamification** — XP, levels, streaks, 28 badges and a milestone track toward
  a healthy BMI.
- **Per-person weight tracking** tied to Home Assistant users via ingress auth.
- **Home Assistant sensors** — weight, BMI, calories and streak published back to
  Home Assistant, plus optional fitness-tracker step import.
- **German and English** throughout the UI and recipes (German primary).
- **Local-first storage** — a single SQLite database under `/data`, no cloud.
