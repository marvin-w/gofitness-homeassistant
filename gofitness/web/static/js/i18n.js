// Translation dictionary. German is the primary language and is always
// complete; English mirrors it. A missing key falls back to German, then to the
// key itself, so a gap shows up as odd text rather than an empty screen.

export const LANGS = ['de', 'en'];

const DICT = {
  de: {
    app_name: 'GoFitness',
    app_tagline: 'Gesund werden. Zusammen. Nachhaltig.',

    // Navigation
    nav_today: 'Heute',
    nav_food: 'Essen',
    nav_plan: 'Plan',
    nav_weight: 'Gewicht',
    nav_awards: 'Erfolge',
    nav_profile: 'Profil',

    // Generic
    save: 'Speichern',
    cancel: 'Abbrechen',
    delete: 'Löschen',
    add: 'Hinzufügen',
    back: 'Zurück',
    next: 'Weiter',
    close: 'Schließen',
    confirm: 'Passt so',
    edit: 'Bearbeiten',
    loading: 'Lädt …',
    today: 'Heute',
    yesterday: 'Gestern',
    none: 'Keine',
    optional: 'optional',
    kcal: 'kcal',
    kg: 'kg',
    cm: 'cm',
    min: 'Min.',
    protein: 'Eiweiß',
    carbs: 'Kohlenhydrate',
    fat: 'Fett',
    fiber: 'Ballaststoffe',
    water: 'Wasser',
    of: 'von',
    error_generic: 'Da ist etwas schiefgelaufen.',
    retry: 'Nochmal versuchen',
    nothing_yet: 'Noch nichts eingetragen.',

    // Setup wizard
    setup_welcome: 'Willkommen bei GoFitness',
    setup_intro: 'In einer Minute eingerichtet. Danach weißt du genau, wie viele Kalorien du am Tag brauchst – und was du diese Woche kochst.',
    setup_start: 'Los geht\'s',
    setup_step: 'Schritt {n} von {total}',
    setup_about_you: 'Über dich',
    setup_your_body: 'Deine Werte',
    setup_your_goal: 'Dein Ziel',
    setup_your_food: 'Dein Essen',
    setup_summary: 'Dein Plan',
    setup_finish: 'Plan aktivieren',

    field_name: 'Name',
    field_sex: 'Geschlecht',
    field_birth_date: 'Geburtsdatum',
    date_placeholder: 'TT.MM.JJJJ',
    field_height: 'Größe',
    field_weight: 'Aktuelles Gewicht',
    field_target_weight: 'Wunschgewicht',
    field_target_weight_hint: 'Leer lassen – wir schlagen dir ein gesundes Ziel vor.',
    field_activity: 'Alltagsbewegung',
    field_goal: 'Ziel',
    field_breastfeeding: 'Stillst du gerade?',
    field_language: 'Sprache',
    field_household: 'Personen im Haushalt',
    field_meals_per_day: 'Mahlzeiten pro Tag',
    field_cook_time: 'Maximale Kochzeit',
    field_veggie: 'Wie viel Gemüse?',
    field_fish: 'Fisch',
    field_cook_once: 'Einmal kochen, zweimal essen',
    field_cook_once_hint: 'Das Abendessen wird am nächsten Tag als Mittagessen eingeplant.',
    field_cook_once_always: '🍲 Einmal kochen, zweimal essen ist immer aktiv – das Abendessen kommt am nächsten Tag als Mittagessen zurück.',
    household_settings_hint: '👪 Diese Einstellungen gelten für den ganzen Haushalt – alle sehen denselben Wochenplan.',

    sex_female: 'Weiblich',
    sex_male: 'Männlich',
    sex_divers: 'Divers',

    activity_sedentary: 'Sitzend',
    activity_sedentary_hint: 'Büro, wenig Bewegung',
    activity_light: 'Leicht aktiv',
    activity_light_hint: 'Etwas zu Fuß, leichte Hausarbeit',
    activity_moderate: 'Mäßig aktiv',
    activity_moderate_hint: 'Viel auf den Beinen',
    activity_active: 'Aktiv',
    activity_active_hint: 'Körperliche Arbeit oder täglich Sport',
    activity_very_active: 'Sehr aktiv',
    activity_very_active_hint: 'Harte Arbeit plus Training',

    goal_lose: 'Abnehmen',
    goal_lose_hint: 'Langsam und dauerhaft',
    goal_maintain: 'Gewicht halten',
    goal_maintain_hint: 'So bleiben, wie es ist',
    goal_gain_muscle: 'Muskeln aufbauen',
    goal_gain_muscle_hint: 'Mit Krafttraining',
    goal_recomp: 'Straffen',
    goal_recomp_hint: 'Fett runter, Muskeln rauf',

    bf_none: 'Nein',
    bf_partial: 'Ja, mit Beikost',
    bf_exclusive: 'Ja, voll stillend',
    bf_hint: 'Im Stillmodus rechnen wir mehr Kalorien ein und filtern alle Rezepte auf stillfreundlich. Du kannst das jederzeit im Profil wieder ausschalten.',

    veggie_low: 'Wenig',
    veggie_low_hint: 'Gemüse bleibt im Hintergrund',
    veggie_medium: 'Normal',
    veggie_high: 'Gerne viel',

    fish_breaded_only: 'Nur paniert',
    fish_breaded_only_hint: 'Fischstäbchen und Backfisch – sonst kein Fisch',
    fish_any: 'Alles',
    fish_none: 'Gar keinen',

    // Dashboard
    hello: 'Hallo {name}',
    kcal_left: 'noch übrig',
    kcal_over: 'darüber',
    of_target: 'von {target} kcal',
    eaten: 'Gegessen',
    burned: 'Sport',
    remaining: 'Rest',
    macros_today: 'Makros heute',
    quick_add: 'Schnell eintragen',
    log_food: 'Essen eintragen',
    log_weight: 'Gewicht eintragen',
    log_workout: 'Sport eintragen',
    todays_meals: 'Heute gegessen',
    todays_workouts: 'Heute bewegt',
    streak_days: '{n} Tage am Stück',
    streak_days_one: '{n} Tag am Stück',
    level_label: 'Level {n}',
    xp_to_next: 'noch {n} XP bis Level {next}',
    next_milestone: 'Nächstes Etappenziel',
    kg_to_go: 'noch {n} kg',
    milestone_reached: 'Erreicht!',
    weekly_summary: 'Diese Woche',
    days_on_target: '{n} von 7 Tagen im Zielbereich',

    // Food
    food_title: 'Essen',
    food_describe: 'Was hast du gegessen?',
    food_describe_hint: 'Zum Beispiel: „2 Kugeln Eis" oder „Brot mit Käse"',
    food_estimate: 'Kalorien schätzen',
    food_photo: 'Foto aufnehmen',
    food_photo_hint: 'Mach ein Bild von deinem Teller – die KI rechnet die Kalorien aus.',
    food_manual: 'Selbst eintragen',
    food_search: 'In der Lebensmitteltabelle suchen',
    food_estimating: 'Wird geschätzt …',
    food_estimate_result: 'Schätzung',
    food_confirm_question: 'Passt das so?',
    food_confidence: 'Sicherheit',
    confidence_high: 'hoch',
    confidence_medium: 'mittel',
    confidence_low: 'niedrig',
    food_assumptions: 'Angenommen',
    food_adjust: 'Werte anpassen',
    food_source_ai_text: 'KI-Schätzung',
    food_source_ai_image: 'KI-Bilderkennung',
    food_source_local_db: 'Lebensmitteltabelle',
    food_source_manual: 'Selbst eingetragen',
    food_source_recipe: 'Aus dem Wochenplan',
    food_source_setup: 'Beim Einrichten',
    food_source_tracker: 'Fitnesstracker',
    food_added: 'Eingetragen.',
    ai_unavailable: 'Für die Bilderkennung fehlt ein Anthropic API-Key in den Add-on-Einstellungen. Texteingaben werden über die lokale Lebensmitteltabelle geschätzt.',
    meal_breakfast: 'Frühstück',
    meal_lunch: 'Mittagessen',
    meal_dinner: 'Abendessen',
    meal_snack: 'Snack',

    // Meal plan
    plan_title: 'Wochenplan',
    plan_generate: 'Woche planen',
    plan_shuffle: 'Neu würfeln',
    plan_week_of: 'Woche ab {date}',
    plan_prev_week: 'Vorherige Woche',
    plan_next_week: 'Nächste Woche',
    plan_unsaved: 'Vorschlag – noch nicht gespeichert',
    plan_avg: 'Ø {n} kcal/Tag für den Haushalt',
    plan_cooked: 'Gekocht',
    plan_log_meal: 'Gegessen',
    plan_leftover: 'Reste von gestern',
    plan_portions: '{n} Portionen',
    plan_amount_g: '≈ {n} g kochen',
    shopping_title: 'Einkaufsliste',
    shopping_hint: 'Für {n} Personen. Vorratsschrank-Sachen stehen unten.',
    shopping_pantry: 'Hast du wahrscheinlich schon',
    shopping_empty: 'Plane erst eine Woche, dann gibt es hier die Einkaufsliste.',
    recipe_ingredients: 'Zutaten',
    recipe_ingredients_for: 'Zutaten für {n} Portionen',
    recipe_portion_g: '1 Portion ≈ {n} g',
    recipe_steps: 'Zubereitung',
    recipe_open: 'Rezept online suchen',
    recipe_servings: 'Portionen',
    recipe_per_serving: 'pro Portion',
    recipe_time: '{n} Min.',
    recipes_browse: 'Alle Rezepte',
    recipes_search: 'Rezept suchen',

    // Weight
    weight_title: 'Gewicht',
    weight_current: 'Aktuell',
    weight_start: 'Start',
    weight_target: 'Ziel',
    weight_change: 'Veränderung',
    weight_add: 'Gewicht eintragen',
    weight_body_fat: 'Körperfett',
    weight_trend: 'Trend (7 Tage)',
    weight_healthy_range: 'Normalgewicht: {low}–{high} kg',
    weight_no_data: 'Trag dein erstes Gewicht ein, dann siehst du hier deinen Verlauf.',
    bmi_label: 'BMI',
    bmi_unknown: 'unbekannt',
    bmi_underweight: 'Untergewicht',
    bmi_normal: 'Normalgewicht',
    bmi_overweight: 'Übergewicht',
    bmi_obese_1: 'Adipositas Grad I',
    bmi_obese_2: 'Adipositas Grad II',
    bmi_obese_3: 'Adipositas Grad III',
    chart_weight: 'Gewichtsverlauf',
    chart_kcal: 'Kalorien pro Tag',

    // Workouts
    workout_title: 'Sport',
    workout_kind: 'Art',
    workout_minutes: 'Dauer',
    workout_kcal_auto: 'Kalorien werden geschätzt, wenn du nichts einträgst.',
    workout_walk: 'Spazieren',
    workout_run: 'Laufen',
    workout_cycle: 'Radfahren',
    workout_strength: 'Krafttraining',
    workout_swim: 'Schwimmen',
    workout_yoga: 'Yoga',
    workout_hiit: 'HIIT',
    workout_stroller: 'Kinderwagen schieben',
    workout_housework: 'Hausarbeit',
    workout_other: 'Sonstiges',

    // Achievements
    awards_title: 'Erfolge',
    awards_unlocked: '{n} von {total} freigeschaltet',
    awards_locked: 'Noch nicht freigeschaltet',
    awards_new: 'Neu freigeschaltet!',
    milestones_title: 'Dein Weg',
    milestone_goal: 'Ziel',
    milestone_healthy_bmi: 'Hier beginnt der Normalbereich',
    group_start: 'Erste Schritte',
    group_streak: 'Dranbleiben',
    group_weight: 'Gewicht',
    group_food: 'Ernährung',
    group_sport: 'Bewegung',
    group_mealprep: 'Meal Prep',
    group_health: 'Gesundheit',

    level_starter: 'Neuling',
    level_beginner: 'Einsteiger',
    level_routine: 'Routiniert',
    level_committed: 'Dranbleiber',
    level_strong: 'Stark',
    level_expert: 'Profi',
    level_master: 'Meister',
    level_legend: 'Legende',

    // Profile
    profile_title: 'Profil',
    profile_saved: 'Gespeichert.',
    profile_food_prefs: 'Essens-Vorlieben',
    profile_trackers: 'Fitnesstracker',
    profile_trackers_hint: 'Verbinde eine Uhr oder Waage aus Home Assistant. Schritte und verbrannte Kalorien landen dann automatisch in deinem Tag.',
    profile_tracker_none: 'Nicht verbunden',
    profile_tracker_sync: 'Jetzt synchronisieren',
    profile_tracker_synced: 'Übernommen.',
    profile_tracker_unavailable: 'Keine Verbindung zu Home Assistant – Tracker können nur innerhalb des Add-ons gelesen werden.',
    tracker_steps: 'Schritte',
    tracker_active_energy: 'Verbrannte Kalorien',
    tracker_weight: 'Waage',
    tracker_heart_rate: 'Herzfrequenz',
    tracker_sleep: 'Schlaf',
    profile_about: 'Über',
    profile_data_note: 'Alle Daten liegen lokal in deiner Home-Assistant-Installation. Es wird nichts hochgeladen – außer du nutzt die KI-Schätzung, dann geht die Beschreibung oder das Foto an die Anthropic-API.',

    // Plan notes (from the backend)
    note_recomp_suggested: 'Bei deinem aktuellen BMI bringt Straffen (Kalorien halten plus Krafttraining) mehr als eine Massephase.',
    note_bf_deficit_capped: 'Weil du stillst, ist das Defizit auf {kcal} kcal begrenzt – das schützt die Milchmenge, und du nimmst trotzdem stetig ab.',
    note_kcal_floor_raised: 'Das Kalorienziel wurde auf einen sicheren Mindestwert von {kcal} kcal angehoben. Langsamer abnehmen hält länger.',
    note_bf_active: 'Stillmodus aktiv: +{kcal} kcal pro Tag, und alle Rezepte sind stillfreundlich (durchgegart, kein roher Fisch, kein Alkohol).',
    note_underweight_warning: 'Dein BMI liegt bereits unter dem Normalbereich – Abnehmen ist hier nicht das richtige Ziel. Bitte sprich mit deiner Ärztin oder deinem Arzt.',
    note_fish_free_week: 'Diese Woche ganz ohne Fisch – passt zu euren Vorlieben.',
    note_slot_unfilled: 'Für eine Mahlzeit gab es kein passendes Rezept. Lockere die Vorlieben etwas, dann wird die Woche voll.',

    plan_energy_title: 'Deine Zahlen',
    plan_bmr: 'Grundumsatz',
    plan_tdee: 'Gesamtumsatz',
    plan_target: 'Tagesziel',
    plan_deficit: 'Defizit',
    plan_surplus: 'Überschuss',
    plan_weekly_change: 'pro Woche',
    plan_eta: 'Ziel voraussichtlich in {n} Wochen',
    plan_eta_none: 'Ziel erreicht – jetzt geht es ums Halten.',
    progress_title: 'Dein Fortschritt',
    proj_next_goal: 'Nächstes Etappenziel',
    proj_weeks_next: 'bis dahin',
    proj_weeks_goal: 'bis zum Ziel',
    proj_weeks_value: 'ca. {n} Wo.',
    proj_note: 'Entspannt geschätzt – meistens geht es schneller. Kein Stress.',
    proj_at_goal: 'Geschafft – du bist im Zielbereich! 🎉',
  },

  en: {
    app_name: 'GoFitness',
    app_tagline: 'Get healthy. Together. Sustainably.',

    nav_today: 'Today',
    nav_food: 'Food',
    nav_plan: 'Plan',
    nav_weight: 'Weight',
    nav_awards: 'Awards',
    nav_profile: 'Profile',

    save: 'Save',
    cancel: 'Cancel',
    delete: 'Delete',
    add: 'Add',
    back: 'Back',
    next: 'Next',
    close: 'Close',
    confirm: 'Looks right',
    edit: 'Edit',
    loading: 'Loading …',
    today: 'Today',
    yesterday: 'Yesterday',
    none: 'None',
    optional: 'optional',
    kcal: 'kcal',
    kg: 'kg',
    cm: 'cm',
    min: 'min',
    protein: 'Protein',
    carbs: 'Carbs',
    fat: 'Fat',
    fiber: 'Fibre',
    water: 'Water',
    of: 'of',
    error_generic: 'Something went wrong.',
    retry: 'Try again',
    nothing_yet: 'Nothing logged yet.',

    setup_welcome: 'Welcome to GoFitness',
    setup_intro: 'Set up in a minute. After that you will know exactly how many calories you need each day — and what you are cooking this week.',
    setup_start: 'Get started',
    setup_step: 'Step {n} of {total}',
    setup_about_you: 'About you',
    setup_your_body: 'Your numbers',
    setup_your_goal: 'Your goal',
    setup_your_food: 'Your food',
    setup_summary: 'Your plan',
    setup_finish: 'Activate plan',

    field_name: 'Name',
    field_sex: 'Sex',
    field_birth_date: 'Date of birth',
    date_placeholder: 'DD.MM.YYYY',
    field_height: 'Height',
    field_weight: 'Current weight',
    field_target_weight: 'Target weight',
    field_target_weight_hint: 'Leave empty and we will suggest a healthy target.',
    field_activity: 'Everyday activity',
    field_goal: 'Goal',
    field_breastfeeding: 'Are you breastfeeding?',
    field_language: 'Language',
    field_household: 'People in the household',
    field_meals_per_day: 'Meals per day',
    field_cook_time: 'Maximum cooking time',
    field_veggie: 'How much veg?',
    field_fish: 'Fish',
    field_cook_once: 'Cook once, eat twice',
    field_cook_once_hint: 'Dinner is planned again as the next day\'s lunch.',
    field_cook_once_always: '🍲 Cook once, eat twice is always on — dinner comes back as the next day\'s lunch.',
    household_settings_hint: '👪 These settings apply to the whole household — everyone sees the same weekly plan.',

    sex_female: 'Female',
    sex_male: 'Male',
    sex_divers: 'Non-binary',

    activity_sedentary: 'Sedentary',
    activity_sedentary_hint: 'Desk job, little movement',
    activity_light: 'Lightly active',
    activity_light_hint: 'Some walking, light housework',
    activity_moderate: 'Moderately active',
    activity_moderate_hint: 'On your feet a lot',
    activity_active: 'Active',
    activity_active_hint: 'Physical work or daily exercise',
    activity_very_active: 'Very active',
    activity_very_active_hint: 'Hard work plus training',

    goal_lose: 'Lose weight',
    goal_lose_hint: 'Slowly and for good',
    goal_maintain: 'Maintain',
    goal_maintain_hint: 'Stay as you are',
    goal_gain_muscle: 'Build muscle',
    goal_gain_muscle_hint: 'With strength training',
    goal_recomp: 'Recomposition',
    goal_recomp_hint: 'Fat down, muscle up',

    bf_none: 'No',
    bf_partial: 'Yes, with solids',
    bf_exclusive: 'Yes, exclusively',
    bf_hint: 'In breastfeeding mode we add calories and filter every recipe to be lactation-friendly. You can switch it off again in your profile at any time.',

    veggie_low: 'Not much',
    veggie_low_hint: 'Vegetables stay in the background',
    veggie_medium: 'Normal',
    veggie_high: 'Plenty',

    fish_breaded_only: 'Breaded only',
    fish_breaded_only_hint: 'Fish fingers and breaded fish — no other fish',
    fish_any: 'Anything',
    fish_none: 'None at all',

    hello: 'Hi {name}',
    kcal_left: 'left',
    kcal_over: 'over',
    of_target: 'of {target} kcal',
    eaten: 'Eaten',
    burned: 'Exercise',
    remaining: 'Left',
    macros_today: 'Macros today',
    quick_add: 'Quick add',
    log_food: 'Log food',
    log_weight: 'Log weight',
    log_workout: 'Log exercise',
    todays_meals: 'Eaten today',
    todays_workouts: 'Moved today',
    streak_days: '{n} days in a row',
    streak_days_one: '{n} day in a row',
    level_label: 'Level {n}',
    xp_to_next: '{n} XP to level {next}',
    next_milestone: 'Next milestone',
    kg_to_go: '{n} kg to go',
    milestone_reached: 'Reached!',
    weekly_summary: 'This week',
    days_on_target: '{n} of 7 days on target',

    food_title: 'Food',
    food_describe: 'What did you eat?',
    food_describe_hint: 'For example: "2 scoops of ice cream" or "bread with cheese"',
    food_estimate: 'Estimate calories',
    food_photo: 'Take a photo',
    food_photo_hint: 'Photograph your plate and the AI works out the calories.',
    food_manual: 'Enter manually',
    food_search: 'Search the food table',
    food_estimating: 'Estimating …',
    food_estimate_result: 'Estimate',
    food_confirm_question: 'Does this look right?',
    food_confidence: 'Confidence',
    confidence_high: 'high',
    confidence_medium: 'medium',
    confidence_low: 'low',
    food_assumptions: 'Assumed',
    food_adjust: 'Adjust values',
    food_source_ai_text: 'AI estimate',
    food_source_ai_image: 'AI photo recognition',
    food_source_local_db: 'Food table',
    food_source_manual: 'Entered manually',
    food_source_recipe: 'From the weekly plan',
    food_source_setup: 'During setup',
    food_source_tracker: 'Fitness tracker',
    food_added: 'Logged.',
    ai_unavailable: 'Photo recognition needs an Anthropic API key in the add-on settings. Text entries are estimated from the local food table.',
    meal_breakfast: 'Breakfast',
    meal_lunch: 'Lunch',
    meal_dinner: 'Dinner',
    meal_snack: 'Snack',

    plan_title: 'Weekly plan',
    plan_generate: 'Plan the week',
    plan_shuffle: 'Reshuffle',
    plan_week_of: 'Week of {date}',
    plan_prev_week: 'Previous week',
    plan_next_week: 'Next week',
    plan_unsaved: 'Proposal — not saved yet',
    plan_avg: 'Ø {n} kcal/day for the household',
    plan_cooked: 'Cooked',
    plan_log_meal: 'Ate this',
    plan_leftover: 'Leftovers from yesterday',
    plan_portions: '{n} portions',
    plan_amount_g: '≈ {n} g to cook',
    shopping_title: 'Shopping list',
    shopping_hint: 'For {n} people. Store-cupboard items are at the bottom.',
    shopping_pantry: 'You probably have these',
    shopping_empty: 'Plan a week first and the shopping list appears here.',
    recipe_ingredients: 'Ingredients',
    recipe_ingredients_for: 'Ingredients for {n} portions',
    recipe_portion_g: '1 portion ≈ {n} g',
    recipe_steps: 'Method',
    recipe_open: 'Search for this recipe online',
    recipe_servings: 'Servings',
    recipe_per_serving: 'per serving',
    recipe_time: '{n} min',
    recipes_browse: 'All recipes',
    recipes_search: 'Search recipes',

    weight_title: 'Weight',
    weight_current: 'Current',
    weight_start: 'Start',
    weight_target: 'Target',
    weight_change: 'Change',
    weight_add: 'Log weight',
    weight_body_fat: 'Body fat',
    weight_trend: 'Trend (7 days)',
    weight_healthy_range: 'Healthy range: {low}–{high} kg',
    weight_no_data: 'Log your first weight and your history will show up here.',
    bmi_label: 'BMI',
    bmi_unknown: 'unknown',
    bmi_underweight: 'Underweight',
    bmi_normal: 'Healthy weight',
    bmi_overweight: 'Overweight',
    bmi_obese_1: 'Obesity class I',
    bmi_obese_2: 'Obesity class II',
    bmi_obese_3: 'Obesity class III',
    chart_weight: 'Weight over time',
    chart_kcal: 'Calories per day',

    workout_title: 'Exercise',
    workout_kind: 'Type',
    workout_minutes: 'Duration',
    workout_kcal_auto: 'Calories are estimated if you leave this empty.',
    workout_walk: 'Walking',
    workout_run: 'Running',
    workout_cycle: 'Cycling',
    workout_strength: 'Strength training',
    workout_swim: 'Swimming',
    workout_yoga: 'Yoga',
    workout_hiit: 'HIIT',
    workout_stroller: 'Pushing the pram',
    workout_housework: 'Housework',
    workout_other: 'Other',

    awards_title: 'Awards',
    awards_unlocked: '{n} of {total} unlocked',
    awards_locked: 'Not unlocked yet',
    awards_new: 'Newly unlocked!',
    milestones_title: 'Your journey',
    milestone_goal: 'Goal',
    milestone_healthy_bmi: 'The healthy range starts here',
    group_start: 'First steps',
    group_streak: 'Keeping it up',
    group_weight: 'Weight',
    group_food: 'Nutrition',
    group_sport: 'Movement',
    group_mealprep: 'Meal prep',
    group_health: 'Health',

    level_starter: 'Newcomer',
    level_beginner: 'Beginner',
    level_routine: 'In the routine',
    level_committed: 'Committed',
    level_strong: 'Strong',
    level_expert: 'Pro',
    level_master: 'Master',
    level_legend: 'Legend',

    profile_title: 'Profile',
    profile_saved: 'Saved.',
    profile_food_prefs: 'Food preferences',
    profile_trackers: 'Fitness tracker',
    profile_trackers_hint: 'Connect a watch or scale from Home Assistant. Steps and burned calories then land in your day automatically.',
    profile_tracker_none: 'Not connected',
    profile_tracker_sync: 'Sync now',
    profile_tracker_synced: 'Imported.',
    profile_tracker_unavailable: 'No connection to Home Assistant — trackers can only be read from inside the add-on.',
    tracker_steps: 'Steps',
    tracker_active_energy: 'Calories burned',
    tracker_weight: 'Scale',
    tracker_heart_rate: 'Heart rate',
    tracker_sleep: 'Sleep',
    profile_about: 'About',
    profile_data_note: 'All data stays local in your Home Assistant installation. Nothing is uploaded — unless you use AI estimation, in which case the description or photo goes to the Anthropic API.',

    note_recomp_suggested: 'At your current BMI, recomposition (holding calories plus strength training) does more than a bulk.',
    note_bf_deficit_capped: 'Because you are breastfeeding, the deficit is capped at {kcal} kcal — that protects your milk supply and you still lose steadily.',
    note_kcal_floor_raised: 'The calorie target was raised to a safe minimum of {kcal} kcal. Losing more slowly lasts longer.',
    note_bf_active: 'Breastfeeding mode is on: +{kcal} kcal per day, and every recipe is lactation-friendly (fully cooked, no raw fish, no alcohol).',
    note_underweight_warning: 'Your BMI is already below the healthy range — losing weight is not the right goal here. Please talk to your doctor.',
    note_fish_free_week: 'A completely fish-free week — in line with your preferences.',
    note_slot_unfilled: 'One meal had no matching recipe. Relax the preferences a little and the week will fill up.',

    plan_energy_title: 'Your numbers',
    plan_bmr: 'Basal rate',
    plan_tdee: 'Total burn',
    plan_target: 'Daily target',
    plan_deficit: 'Deficit',
    plan_surplus: 'Surplus',
    plan_weekly_change: 'per week',
    plan_eta: 'Goal expected in about {n} weeks',
    plan_eta_none: 'Goal reached — now it is about holding it.',
    progress_title: 'Your progress',
    proj_next_goal: 'Next checkpoint',
    proj_weeks_next: 'to get there',
    proj_weeks_goal: 'to the goal',
    proj_weeks_value: '~{n} wks',
    proj_note: 'A relaxed estimate — it usually goes quicker. No pressure.',
    proj_at_goal: "Done — you're in your target range! 🎉",
  },
};

// Badge titles and descriptions live in their own table because each one needs
// two strings and the list is long.
const BADGES = {
  de: {
    setup_done: ['Startklar', 'Profil eingerichtet'],
    first_weigh_in: ['Erste Messung', 'Zum ersten Mal gewogen'],
    first_meal_logged: ['Erster Eintrag', 'Zum ersten Mal Essen eingetragen'],
    first_workout: ['Erste Einheit', 'Zum ersten Mal Sport eingetragen'],
    first_plan: ['Erster Wochenplan', 'Eine Woche durchgeplant'],
    streak_3: ['Drei am Stück', '3 Tage hintereinander getrackt'],
    streak_7: ['Eine ganze Woche', '7 Tage hintereinander getrackt'],
    streak_14: ['Zwei Wochen', '14 Tage hintereinander getrackt'],
    streak_30: ['Ein Monat', '30 Tage hintereinander getrackt'],
    streak_100: ['Hundert Tage', '100 Tage hintereinander getrackt'],
    weigh_ins_10: ['Regelmäßig', '10 Mal gewogen'],
    weigh_ins_50: ['Datensammler', '50 Mal gewogen'],
    lost_1kg: ['Das erste Kilo', '1 kg abgenommen'],
    lost_5kg: ['Fünf Kilo leichter', '5 kg abgenommen'],
    lost_10kg: ['Zehn Kilo leichter', '10 kg abgenommen'],
    goal_reached: ['Angekommen', 'Wunschgewicht erreicht'],
    healthy_bmi: ['Im grünen Bereich', 'BMI im Normalbereich'],
    meals_25: ['Angewohnheit', '25 Mahlzeiten eingetragen'],
    meals_100: ['Routine', '100 Mahlzeiten eingetragen'],
    meals_500: ['Buchhalter', '500 Mahlzeiten eingetragen'],
    protein_week: ['Eiweißwoche', '7 Tage das Eiweißziel erreicht'],
    on_target_7: ['Punktlandung', '7 Tage im Kalorien-Zielbereich'],
    workouts_5: ['In Bewegung', '5 Einheiten trainiert'],
    workouts_25: ['Am Ball', '25 Einheiten trainiert'],
    workouts_100: ['Sportskanone', '100 Einheiten trainiert'],
    cooked_10: ['Selbst gekocht', '10 geplante Gerichte gekocht'],
    cooked_50: ['Küchenchef', '50 geplante Gerichte gekocht'],
    plans_4: ['Vorausdenker', '4 Wochen im Voraus geplant'],
  },
  en: {
    setup_done: ['Ready to go', 'Profile set up'],
    first_weigh_in: ['First reading', 'Weighed in for the first time'],
    first_meal_logged: ['First entry', 'Logged food for the first time'],
    first_workout: ['First session', 'Logged exercise for the first time'],
    first_plan: ['First weekly plan', 'Planned a full week'],
    streak_3: ['Three in a row', 'Tracked 3 days running'],
    streak_7: ['A whole week', 'Tracked 7 days running'],
    streak_14: ['Two weeks', 'Tracked 14 days running'],
    streak_30: ['A month', 'Tracked 30 days running'],
    streak_100: ['One hundred days', 'Tracked 100 days running'],
    weigh_ins_10: ['Consistent', 'Weighed in 10 times'],
    weigh_ins_50: ['Data collector', 'Weighed in 50 times'],
    lost_1kg: ['The first kilo', 'Lost 1 kg'],
    lost_5kg: ['Five kilos lighter', 'Lost 5 kg'],
    lost_10kg: ['Ten kilos lighter', 'Lost 10 kg'],
    goal_reached: ['Arrived', 'Reached your target weight'],
    healthy_bmi: ['In the green', 'BMI in the healthy range'],
    meals_25: ['Becoming a habit', 'Logged 25 meals'],
    meals_100: ['Routine', 'Logged 100 meals'],
    meals_500: ['Bookkeeper', 'Logged 500 meals'],
    protein_week: ['Protein week', 'Hit your protein target 7 days running'],
    on_target_7: ['Bullseye', '7 days inside the calorie window'],
    workouts_5: ['Moving', 'Trained 5 times'],
    workouts_25: ['Sticking with it', 'Trained 25 times'],
    workouts_100: ['Athlete', 'Trained 100 times'],
    cooked_10: ['Home cook', 'Cooked 10 planned meals'],
    cooked_50: ['Head chef', 'Cooked 50 planned meals'],
    plans_4: ['Planner', 'Planned 4 weeks ahead'],
  },
};

let current = 'de';

/** Set the active language. */
export function setLang(lang) {
  current = LANGS.includes(lang) ? lang : 'de';
  document.documentElement.lang = current;
}

/** Get the active language. */
export function getLang() {
  return current;
}

/**
 * Translate a key, interpolating {placeholders} from params.
 * Falls back to German, then to the key itself.
 */
export function t(key, params) {
  // Pluralization: when a count {n} is exactly 1, prefer a "<key>_one" variant
  // if one is defined, so "1 Tage" becomes "1 Tag" / "1 days" becomes "1 day".
  if (params && params.n === 1 &&
    (DICT[current]?.[key + '_one'] ?? DICT.de[key + '_one']) !== undefined) {
    key = key + '_one';
  }
  let s = DICT[current]?.[key];
  if (s === undefined) s = DICT.de[key];
  if (s === undefined) return key;
  if (!params) return s;
  return s.replace(/\{(\w+)\}/g, (m, name) =>
    params[name] !== undefined ? String(params[name]) : m
  );
}

/** Translate a badge code into [title, description]. */
export function badgeText(code) {
  return BADGES[current]?.[code] || BADGES.de[code] || [code, ''];
}

/** Translate a backend note object ({code, params}). */
export function noteText(note) {
  if (!note) return '';
  if (typeof note === 'string') return t('note_' + note);
  return t('note_' + note.code, note.params || {});
}

/** Format a number with the locale's decimal separator. */
export function num(value, decimals = 0) {
  if (value === null || value === undefined || Number.isNaN(value)) return '–';
  return Number(value).toLocaleString(current === 'de' ? 'de-DE' : 'en-GB', {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

/** Format a YYYY-MM-DD or ISO timestamp as a German date, DD.MM.YYYY. The
 *  German date format is used regardless of the interface language. */
export function shortDate(value) {
  if (!value) return '';
  const d = new Date(value.length === 10 ? value + 'T12:00:00' : value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleDateString('de-DE', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  });
}

/** Format an ISO timestamp as a 24-hour time of day (German convention). */
export function shortTime(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleTimeString('de-DE', {
    hour: '2-digit',
    minute: '2-digit',
  });
}

/** Convert an ISO date (YYYY-MM-DD) to the German format DD.MM.YYYY. Returns an
 *  empty string when unset; passes through anything it cannot parse. */
export function isoToDE(iso) {
  if (!iso) return '';
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
  return m ? `${m[3]}.${m[2]}.${m[1]}` : iso;
}

/** Parse a German date (DD.MM.YYYY, single digits allowed) into an ISO date.
 *  Returns '' when the text is incomplete or not a real calendar date, so the
 *  caller can keep the last valid value while the user is still typing. */
export function deToISO(text) {
  if (!text) return '';
  const m = /^\s*(\d{1,2})\.(\d{1,2})\.(\d{4})\s*$/.exec(text);
  if (!m) return '';
  const day = +m[1], mon = +m[2], year = +m[3];
  if (mon < 1 || mon > 12 || day < 1 || day > 31) return '';
  const iso = `${year}-${String(mon).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
  const d = new Date(iso + 'T12:00:00');
  // Reject impossible days like 31.02. (JS would roll them over).
  return Number.isNaN(d.getTime()) || d.getUTCDate() !== day ? '' : iso;
}
