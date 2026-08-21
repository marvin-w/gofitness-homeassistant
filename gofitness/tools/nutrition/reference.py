# -*- coding: utf-8 -*-
"""
Ingredient nutrition reference for GoFitness.

This is the single documented source of truth for the calorie and macro numbers
in the recipe database. `generate.py` reads it, recomputes each recipe's
per-portion values from its ingredient list, and writes them back into
recipes.json — so the numbers are internally consistent and every assumption is
visible here rather than guessed per recipe.

Values are typical German supermarket products, per 100 g (or per 100 ml for
liquids, treated as ~1 g/ml except oils, which use OIL_DENSITY). Where a product
varies a lot (e.g. fresh tortellini, cream cheese) the comment states which
product was assumed — that is deliberately the point.

Each MASS entry: name -> (kcal, protein_g, carbs_g, fat_g) per 100 g.
"""

OIL_DENSITY = 0.92  # g per ml, for oils measured in ml

# Per 100 g / 100 ml.
MASS = {
    # --- fruit & veg ---
    "Apfel": (52, 0.3, 14.0, 0.2),
    "Banane": (89, 1.1, 23.0, 0.3),
    "Champignons": (22, 3.0, 1.0, 0.3),
    "Eisbergsalat": (13, 0.9, 2.0, 0.2),
    "Kartoffeln festkochend": (70, 2.0, 15.0, 0.1),
    "Kartoffeln mehligkochend": (70, 2.0, 15.0, 0.1),
    "Kirschtomaten": (20, 1.0, 3.5, 0.2),
    "Knoblauchzehe": (140, 6.0, 30.0, 0.5),
    "Petersilie": (40, 3.0, 6.0, 0.8),
    "Salatgurke": (12, 0.6, 2.0, 0.1),
    "Schnittlauch": (30, 3.0, 4.0, 0.7),
    "Zitrone": (30, 1.0, 9.0, 0.3),
    "Zwiebel": (40, 1.1, 9.0, 0.1),
    # --- meat, sausage, fish ---
    "Backfisch paniert (TK)": (200, 12.0, 15.0, 10.0),  # breaded, frozen
    "Fischstäbchen (TK)": (200, 11.0, 18.0, 9.0),
    "Gemischtes Hackfleisch": (250, 18.0, 0.0, 20.0),   # raw, ~20% fat
    "Rinderhackfleisch": (215, 20.0, 0.0, 15.0),        # raw, ~15% fat
    "Hähnchenbrustfilet": (110, 23.0, 0.0, 2.0),        # raw
    "Putenschnitzel": (105, 24.0, 0.0, 1.0),            # raw
    "Schweinegeschnetzeltes": (150, 21.0, 0.0, 7.0),    # raw pork strips
    "Kochschinken": (110, 18.0, 1.0, 4.0),
    "Putenschinken": (100, 18.0, 1.0, 2.5),
    "Wiener Würstchen": (270, 11.0, 1.0, 24.0),
    # --- chilled / dairy ---
    "Bergkäse gerieben": (400, 27.0, 0.0, 32.0),
    "Butter": (740, 0.7, 0.6, 82.0),
    "Feta (pasteurisiert)": (260, 14.0, 1.5, 21.0),
    "Frischkäse": (250, 6.0, 3.5, 24.0),                # Doppelrahm-style
    "Gouda gerieben": (380, 25.0, 2.0, 31.0),
    "Gouda in Scheiben": (356, 24.0, 2.0, 27.0),
    "Hüttenkäse": (100, 12.0, 3.0, 4.5),
    "Magerquark": (67, 12.0, 4.0, 0.3),
    "Mayonnaise leicht": (300, 1.0, 8.0, 29.0),         # reduced-fat
    "Milch": (50, 3.4, 4.8, 1.5),                       # 1.5% fat
    "Naturjoghurt": (61, 3.5, 5.0, 3.2),                # 3.5%
    "Parmesan gerieben": (400, 36.0, 0.0, 28.0),
    "Schmand": (240, 2.5, 4.0, 24.0),
    "Skyr natur": (65, 11.0, 4.0, 0.2),
    "Spätzle (frisch)": (260, 9.0, 45.0, 4.0),
    "Tortellini (frisch)": (290, 12.0, 42.0, 8.0),      # fresh, cheese/ham-filled
    # --- frozen ---
    "Beeren (TK-Mischung)": (45, 1.0, 8.0, 0.3),
    "Erbsen (TK)": (80, 5.0, 11.0, 0.5),
    # --- dry goods ---
    "Bulgur": (350, 12.0, 63.0, 1.3),                   # raw
    "Cornflakes ungesüßt": (360, 7.0, 84.0, 0.9),
    "Erdnussmus": (600, 25.0, 12.0, 50.0),
    "Haferflocken": (370, 13.0, 59.0, 7.0),
    "Honig": (300, 0.3, 80.0, 0.0),
    "Mandeln": (600, 21.0, 5.0, 50.0),
    "Milchreis": (350, 7.0, 77.0, 1.0),                 # raw pudding rice
    "Nudeln": (350, 12.0, 70.0, 1.5),                   # raw
    "Proteinpulver Vanille": (375, 75.0, 8.0, 5.0),     # whey
    "Reis": (350, 7.0, 78.0, 1.0),                      # raw
    "Rosinen": (300, 3.0, 70.0, 0.5),
    "Röstzwiebeln": (550, 6.0, 35.0, 42.0),
    "Semmelbrösel": (380, 12.0, 72.0, 4.0),
    "Studentenfutter": (480, 14.0, 35.0, 30.0),
    "Walnüsse": (650, 15.0, 11.0, 65.0),
    "Weichweizengrieß": (350, 11.0, 70.0, 1.0),
    "Weizenmehl": (340, 10.0, 71.0, 1.0),
    "Ahornsirup": (260, 0.0, 67.0, 0.0),
    # --- cans / preserves ---
    "Ananas (Dose)": (60, 0.4, 14.0, 0.1),
    "Apfelmus": (45, 0.2, 11.0, 0.1),
    "Gehackte Tomaten": (30, 1.3, 5.0, 0.2),
    "Passierte Tomaten": (35, 1.5, 6.0, 0.2),
    "Gewürzgurken": (15, 0.5, 3.0, 0.1),
    "Kidneybohnen (Dose)": (100, 7.0, 14.0, 0.5),       # drained
    "Kokosmilch light": (90, 1.0, 3.0, 8.0),
    "Mais (Dose)": (90, 3.0, 16.0, 1.2),                # drained
    # --- oils (per 100 g; ml converted with OIL_DENSITY) ---
    "Olivenöl": (884, 0.0, 0.0, 100.0),
    "Rapsöl": (884, 0.0, 0.0, 100.0),
    # --- breads measured by weight fall back to per-100 g here; slices use UNIT ---
    "Vollkornbrot": (220, 8.0, 38.0, 2.5),
    "Vollkorntoast": (245, 9.0, 42.0, 3.5),
    "Brötchen vom Vortag": (270, 9.0, 52.0, 3.5),
    "Weizentortillas": (300, 8.0, 50.0, 7.0),
    "Reiswaffeln": (385, 8.0, 82.0, 3.0),
    "Ei": (145, 12.5, 0.7, 10.0),
    "Eier": (145, 12.5, 0.7, 10.0),
    # --- broths, sugar, water (mass basis) ---
    "Gemüsebrühe": (4, 0.0, 0.5, 0.0),
    "Hühnerbrühe": (4, 0.0, 0.5, 0.0),
    "Mineralwasser": (0, 0.0, 0.0, 0.0),
    "Zucker": (400, 0.0, 100.0, 0.0),
    "Puderzucker": (400, 0.0, 100.0, 0.0),
    "Senf": (100, 5.0, 8.0, 5.0),
    # --- spices / leavening: negligible energy, listed so nothing is "missing" ---
    "Backpulver": (0, 0.0, 0.0, 0.0),
    "Currypulver mild": (0, 0.0, 0.0, 0.0),
    "Gyrosgewürz": (0, 0.0, 0.0, 0.0),
    "Kreuzkümmel": (0, 0.0, 0.0, 0.0),
    "Muskatnuss": (0, 0.0, 0.0, 0.0),
    "Oregano": (0, 0.0, 0.0, 0.0),
    "Paprikapulver edelsüß": (0, 0.0, 0.0, 0.0),
    "Salz": (0, 0.0, 0.0, 0.0),
    "Salz und Pfeffer": (0, 0.0, 0.0, 0.0),
    "Vanillezucker": (400, 0.0, 100.0, 0.0),
    "Zimt": (0, 0.0, 0.0, 0.0),
}

# Grams per single unit for count/spoon-measured ingredients. A (name, unit)
# override wins over a plain unit default.
UNIT_G = {
    "Prise": 0.4,
    "TL": 5.0,     # level teaspoon of a dry spice
    "EL": 15.0,    # tablespoon
    "Päckchen": 12.0,
    "Scheiben": 45.0,
    "Stück": 100.0,
}

# (name, unit) -> grams, for items whose piece weight matters.
PIECE_G = {
    ("Apfel", "Stück"): 140.0,
    ("Banane", "Stück"): 120.0,
    ("Ei", "Stück"): 55.0,
    ("Eier", "Stück"): 55.0,
    ("Zwiebel", "Stück"): 100.0,
    ("Knoblauchzehe", "Stück"): 5.0,
    ("Zitrone", "Stück"): 60.0,
    ("Salatgurke", "Stück"): 350.0,
    ("Fischstäbchen (TK)", "Stück"): 30.0,
    ("Reiswaffeln", "Stück"): 9.0,
    ("Weizentortillas", "Stück"): 60.0,
    ("Brötchen vom Vortag", "Stück"): 60.0,
    ("Vollkornbrot", "Scheiben"): 45.0,
    ("Vollkorntoast", "Scheiben"): 35.0,
    ("Backpulver", "Päckchen"): 16.0,
    ("Vanillezucker", "Päckchen"): 8.0,
    ("Backpulver", "TL"): 4.0,
    ("Senf", "EL"): 15.0,
    ("Senf", "TL"): 5.0,
}

# Cooking water absorbed from *unlisted* cooking water, used only to estimate the
# weight of a finished portion (portion_g). Grains boiled in plain water gain
# weight; anything cooked in an ingredient that is already in the list (milk for
# porridge/rice pudding) keeps factor 1 so the liquid is not double-counted.
COOK_FACTOR = {
    "Reis": 2.6,
    "Nudeln": 2.4,
    "Bulgur": 2.8,
    "Tortellini (frisch)": 1.15,
}
