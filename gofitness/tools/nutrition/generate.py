#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Recompute recipe nutrition and portion sizes from the documented ingredient
reference, and apply the banana-pancake swap.

Run from the repo root:

    python3 gofitness/tools/nutrition/generate.py

It rewrites:
  - internal/recipes/data/recipes.json     (kcal/macros per portion, portion_g)
  - internal/recipes/data/recipes.en.json  (english text for swapped recipe)
  - internal/recipes/data/i18n.json         (vocabulary for any new ingredient)

Everything else in those files is preserved. The point is that the numbers come
from one visible table (reference.py), not per-recipe guesses.
"""
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from reference import MASS, UNIT_G, PIECE_G, COOK_FACTOR, OIL_DENSITY  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
DATA = os.path.join(ROOT, "internal", "recipes", "data")
RECIPES = os.path.join(DATA, "recipes.json")
RECIPES_EN = os.path.join(DATA, "recipes.en.json")
I18N = os.path.join(DATA, "i18n.json")

OILS = {"Olivenöl", "Rapsöl"}


def grams(ing):
    """Weight in grams of one ingredient line."""
    name, unit, amt = ing["name"], ing["unit"], float(ing["amount"])
    if unit in ("g",):
        return amt
    if unit in ("ml",):
        return amt * (OIL_DENSITY if name in OILS else 1.0)
    if (name, unit) in PIECE_G:
        return amt * PIECE_G[(name, unit)]
    if unit in UNIT_G:
        return amt * UNIT_G[unit]
    raise KeyError(f"no gram conversion for {name!r} unit {unit!r}")


def macros(ing):
    """(kcal, p, c, f) contributed by one ingredient line."""
    name = ing["name"]
    if name not in MASS:
        raise KeyError(f"no nutrition for ingredient {name!r}")
    g = grams(ing)
    k, p, c, f = MASS[name]
    factor = g / 100.0
    return k * factor, p * factor, c * factor, f * factor


def cooked_grams(ing):
    return grams(ing) * COOK_FACTOR.get(ing["name"], 1.0)


def recompute(recipe):
    servings = recipe.get("servings") or 1
    tk = tp = tc = tf = 0.0
    cooked = 0.0
    for ing in recipe["ingredients"]:
        k, p, c, f = macros(ing)
        tk += k
        tp += p
        tc += c
        tf += f
        cooked += cooked_grams(ing)
    recipe["kcal"] = round(tk / servings)
    recipe["protein_g"] = round(tp / servings)
    recipe["carbs_g"] = round(tc / servings)
    # Round fat to one decimal only when it is tiny (snacks), else whole grams.
    fat = tf / servings
    recipe["fat_g"] = round(fat, 1) if fat < 5 else round(fat)
    portion = cooked / servings
    # Round to a friendly 10 g; keep small snacks honest with 5 g steps.
    step = 5 if portion < 120 else 10
    recipe["portion_g"] = int(round(portion / step) * step)
    return recipe


# The banana pancakes that replace the old protein pancakes. Normal milk (the
# user asked to swap the plant milk out), no egg — the mashed banana binds. From
# einfachbacken.de, scaled to 2 portions (~14 small pancakes, ~7 per portion).
BANANA_PANCAKES = {
    "id": "bananen-pancakes",
    "title": "Fluffige Bananen-Pancakes",
    "description": "Nur Banane, Mehl und Milch – fluffig, süß und ohne Ei. Ein Frühstück, das nach Wochenende schmeckt.",
    "search": "fluffige bananen pancakes",
    "servings": 2,
    "prep_minutes": 20,
    "kcal": 0, "protein_g": 0, "carbs_g": 0, "fat_g": 0, "portion_g": 0,
    "meal_types": ["breakfast"],
    "tags": ["süß", "familienessen", "günstig"],
    "veggie_level": "low",
    "contains_fish": False,
    "fish_breaded": False,
    "breastfeeding_safe": True,
    "kid_friendly": True,
    "meal_prep": False,
    "freezable": True,
    "ingredients": [
        {"name": "Banane", "amount": 2, "unit": "Stück", "category": "Obst & Gemüse"},
        {"name": "Weizenmehl", "amount": 150, "unit": "g", "category": "Trockenwaren"},
        {"name": "Milch", "amount": 150, "unit": "ml", "category": "Kühlregal"},
        {"name": "Backpulver", "amount": 1, "unit": "TL", "category": "Backen & Gewürze", "pantry": True},
        {"name": "Ahornsirup", "amount": 40, "unit": "g", "category": "Backen & Gewürze"},
        {"name": "Butter", "amount": 15, "unit": "g", "category": "Kühlregal", "pantry": True},
        {"name": "Zimt", "amount": 1, "unit": "Prise", "category": "Backen & Gewürze", "pantry": True},
    ],
    "steps": [
        "Bananen schälen und mit einer Gabel möglichst fein zu Mus zerdrücken.",
        "Milch dazugeben. Mehl mit Backpulver und Zimt mischen, zugeben und mit dem Schneebesen zügig zu einem dicklichen Teig rühren. 5 Minuten ruhen lassen.",
        "Etwas Butter in einer beschichteten Pfanne erhitzen. Pro Pancake ca. 1 gehäuften EL Teig hineingeben und bei mittlerer Hitze 1–2 Minuten je Seite goldbraun backen.",
        "Pancakes stapeln und mit Ahornsirup servieren.",
    ],
}

BANANA_PANCAKES_EN = {
    "title": "Fluffy banana pancakes",
    "description": "Just banana, flour and milk — fluffy, sweet and egg-free. A breakfast that tastes like the weekend.",
    "search": "fluffy banana pancakes",
    "steps": [
        "Peel the bananas and mash them as smoothly as you can with a fork.",
        "Add the milk. Mix the flour with the baking powder and cinnamon, stir it in with a whisk to a thick batter and let it rest for 5 minutes.",
        "Heat a little butter in a non-stick pan. Add about 1 heaped tablespoon of batter per pancake and fry over medium heat for 1–2 minutes per side until golden.",
        "Stack the pancakes and serve with maple syrup.",
    ],
}

OLD_PANCAKE_ID = "protein-pancakes"


def main():
    recipes = json.load(open(RECIPES, encoding="utf-8"))
    en = json.load(open(RECIPES_EN, encoding="utf-8"))
    i18n = json.load(open(I18N, encoding="utf-8"))

    # 1. Swap the pancake recipe.
    recipes = [BANANA_PANCAKES if r["id"] == OLD_PANCAKE_ID else r for r in recipes]
    en.pop(OLD_PANCAKE_ID, None)
    en[BANANA_PANCAKES["id"]] = BANANA_PANCAKES_EN

    # 2. New ingredient vocabulary.
    i18n["ingredients"].setdefault("Ahornsirup", "Maple syrup")

    # 3. Recompute every recipe from the reference table.
    for r in recipes:
        recompute(r)

    json.dump(recipes, open(RECIPES, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
    open(RECIPES, "a", encoding="utf-8").write("\n")
    # keep en/i18n key order stable-ish
    json.dump(en, open(RECIPES_EN, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
    open(RECIPES_EN, "a", encoding="utf-8").write("\n")
    json.dump(i18n, open(I18N, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
    open(I18N, "a", encoding="utf-8").write("\n")

    print(f"updated {len(recipes)} recipes")


if __name__ == "__main__":
    main()
