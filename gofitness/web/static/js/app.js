// Application shell: state, routing between views, and the setup wizard.

import { api, ApiError } from './api.js';
import { t, setLang, getLang, badgeText, noteText, num, shortDate, shortTime } from './i18n.js';
import { el, fill, toast, sheet, field, optionGroup, calorieRing, progressBar, weightChart, kcalChart } from './ui.js';

const LANG_KEY = 'gofitness.lang';

const state = {
  boot: null,
  view: 'today',
  planWeek: null,
  planData: null,
};

const root = () => document.getElementById('app');

// --------------------------------------------------------------------- boot

async function start() {
  // The stored language wins on first paint so the app never flashes German at
  // an English user; the profile value takes over once bootstrap returns.
  setLang(localStorage.getItem(LANG_KEY) || 'de');
  render(loadingView());

  try {
    state.boot = await api.bootstrap();
  } catch (err) {
    render(errorView(err));
    return;
  }

  const lang = state.boot.profile?.prefs?.language || state.boot.lang || 'de';
  setLang(lang);
  localStorage.setItem(LANG_KEY, lang);

  if (!state.boot.setup_done) {
    renderWizard();
    return;
  }
  renderApp();
}

function render(...nodes) {
  fill(root(), ...nodes);
}

function loadingView() {
  return el('div', { class: 'app' }, el('p', { class: 'empty', text: t('loading') }));
}

function errorView(err) {
  return el(
    'div',
    { class: 'app' },
    el('div', { class: 'card' },
      el('h2', { text: t('error_generic') }),
      el('p', { class: 'muted small', text: String(err?.message || err) }),
      el('button', { class: 'btn btn--primary', text: t('retry'), onClick: () => start() })
    )
  );
}

/** Report a failed action without losing the current screen. */
function fail(err) {
  const msg = err instanceof ApiError ? err.message : t('error_generic');
  toast(msg, 'error');
  console.error(err);
}

async function reload() {
  state.boot = await api.bootstrap();
  renderApp();
}

// ------------------------------------------------------------------- shell

function renderApp() {
  const b = state.boot;
  render(
    el(
      'div',
      { class: 'app' },
      topbar(),
      el('main', { class: 'view', id: 'view' }, viewFor(state.view))
    ),
    navbar()
  );

  // Celebrate anything unlocked by the last action.
  const fresh = b.gamify?.recently_earned || [];
  if (fresh.length) {
    const [title] = badgeText(fresh[0].code);
    toast(`${fresh[0].icon} ${t('awards_new')} ${title}`, 'success');
  }
}

function topbar() {
  const b = state.boot;
  return el(
    'header',
    { class: 'topbar' },
    el(
      'div',
      {},
      el('h1', { class: 'topbar__title', text: titleFor(state.view) }),
      state.view === 'today'
        ? el('p', {
            class: 'topbar__sub',
            text: t('hello', { name: b.profile?.display_name || b.user?.name || '' }),
          })
        : null
    ),
    langToggle()
  );
}

function langToggle() {
  const set = async (lang) => {
    if (lang === getLang()) return;
    setLang(lang);
    localStorage.setItem(LANG_KEY, lang);
    // Persist on the profile so the choice follows the user to other devices.
    if (state.boot?.setup_done) {
      const p = state.boot.profile;
      try {
        await api.saveProfile(profilePayload(p, { prefs: { ...p.prefs, language: lang } }));
      } catch (err) {
        // A failed save only costs cross-device sync, so the switch still applies.
        console.warn('language not persisted', err);
      }
      state.planData = null;
      await reload();
    } else {
      renderWizard();
    }
  };

  return el(
    'div',
    { class: 'lang-toggle', role: 'group', 'aria-label': t('field_language') },
    ['de', 'en'].map((l) =>
      el('button', {
        type: 'button',
        text: l,
        'aria-pressed': getLang() === l ? 'true' : 'false',
        onClick: () => set(l),
      })
    )
  );
}

const NAV = [
  { id: 'today', icon: '🏠', key: 'nav_today' },
  { id: 'food', icon: '🍽️', key: 'nav_food' },
  { id: 'plan', icon: '📅', key: 'nav_plan' },
  { id: 'weight', icon: '⚖️', key: 'nav_weight' },
  { id: 'awards', icon: '🏆', key: 'nav_awards' },
  { id: 'profile', icon: '⚙️', key: 'nav_profile' },
];

function navbar() {
  return el(
    'nav',
    { class: 'nav' },
    NAV.map((item) =>
      el(
        'button',
        {
          class: `nav__item${state.view === item.id ? ' nav__item--on' : ''}`,
          type: 'button',
          'aria-current': state.view === item.id ? 'page' : null,
          onClick: () => go(item.id),
        },
        el('span', { class: 'nav__icon', text: item.icon }),
        el('span', { text: t(item.key) })
      )
    )
  );
}

function titleFor(view) {
  return {
    today: t('app_name'),
    food: t('food_title'),
    plan: t('plan_title'),
    weight: t('weight_title'),
    awards: t('awards_title'),
    profile: t('profile_title'),
  }[view];
}

function go(view) {
  state.view = view;
  renderApp();
  window.scrollTo({ top: 0, behavior: 'instant' });
}

function viewFor(view) {
  switch (view) {
    case 'food': return foodView();
    case 'plan': return planView();
    case 'weight': return weightView();
    case 'awards': return awardsView();
    case 'profile': return profileView();
    default: return todayView();
  }
}

// ------------------------------------------------------------ today / home

function todayView() {
  const b = state.boot;
  const plan = b.plan || {};
  const totals = b.totals || {};
  const g = b.gamify || {};

  return el(
    'div',
    {},
    el(
      'div',
      { class: 'card' },
      el('div', { class: 'hero' },
        calorieRing(totals.kcal || 0, plan.target_kcal || 0),
        el('p', { class: 'muted small', style: 'margin-top:8px',
          text: t('of_target', { target: num(plan.target_kcal) }) })
      ),
      el(
        'div',
        { class: 'stats' },
        stat(num(totals.kcal), t('eaten')),
        stat(num(totals.workout_kcal), t('burned')),
        stat(num(Math.max(0, (plan.target_kcal || 0) - (totals.kcal || 0))), t('remaining'))
      )
    ),

    levelCard(g),

    el(
      'div',
      { class: 'card' },
      el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('macros_today') })),
      progressBar(t('protein'), Math.round(totals.protein_g || 0), plan.protein_g || 0),
      progressBar(t('carbs'), Math.round(totals.carbs_g || 0), plan.carbs_g || 0),
      progressBar(t('fat'), Math.round(totals.fat_g || 0), plan.fat_g || 0)
    ),

    el(
      'div',
      { class: 'card' },
      el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('quick_add') })),
      el(
        'div',
        { class: 'stack' },
        el('button', { class: 'btn btn--primary btn--block', text: `🍽️  ${t('log_food')}`, onClick: openFoodSheet }),
        el('div', { class: 'row' },
          el('button', { class: 'btn', style: 'flex:1', text: `⚖️  ${t('log_weight')}`, onClick: openWeightSheet }),
          el('button', { class: 'btn', style: 'flex:1', text: `💪  ${t('log_workout')}`, onClick: openWorkoutSheet })
        )
      )
    ),

    nextMilestoneCard(g),

    el(
      'div',
      { class: 'card' },
      el('div', { class: 'card__head' },
        el('h2', { class: 'card__title', text: t('todays_meals') }),
        el('span', { class: 'card__meta', text: `${num(totals.kcal)} ${t('kcal')}` })
      ),
      foodList(b.food || [], b.today)
    ),

    (b.workouts || []).length
      ? el(
          'div',
          { class: 'card' },
          el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('todays_workouts') })),
          el('ul', { class: 'list' }, (b.workouts || []).map(workoutRow))
        )
      : null,

    planNotes(plan.notes)
  );
}

function stat(value, label) {
  return el('div', { class: 'stat' },
    el('div', { class: 'stat__value', text: value }),
    el('div', { class: 'stat__label', text: label })
  );
}

function levelCard(g) {
  if (!g || !g.level) return null;
  return el(
    'div',
    { class: 'card card--tight' },
    el(
      'div',
      { class: 'row row--between' },
      el(
        'div',
        { class: 'level', style: 'flex:1' },
        el('div', { class: 'level__badge', text: String(g.level) }),
        el(
          'div',
          { class: 'level__body' },
          el('div', { class: 'level__title', text: t(g.level_title) }),
          el('div', { class: 'level__hint',
            text: t('xp_to_next', { n: num(Math.max(0, g.xp_for_next - g.xp_into_level)), next: g.level + 1 }) }),
          el('div', { class: 'level__track' },
            el('div', { class: 'level__fill', style: `width:${((g.level_progress || 0) * 100).toFixed(1)}%` }))
        )
      )
    ),
    g.current_streak > 0
      ? el('div', { style: 'margin-top:10px' },
          el('span', { class: 'streak' },
            el('span', { class: 'streak__flame', text: '🔥' }),
            t('streak_days', { n: g.current_streak })))
      : null
  );
}

function nextMilestoneCard(g) {
  if (!g?.next_milestone) return null;
  const m = g.next_milestone;
  return el(
    'div',
    { class: 'card card--tight' },
    el('div', { class: 'card__meta', text: t('next_milestone') }),
    el('div', { class: 'row row--between', style: 'margin-top:4px' },
      el('div', { class: 'level__title', text: `${num(m.weight_kg, 1)} ${t('kg')}` }),
      el('div', { class: 'muted small', text: t('kg_to_go', { n: num(g.kg_to_next, 1) }) })
    ),
    el('div', { class: 'level__track', style: 'margin-top:8px' },
      el('div', { class: 'level__fill', style: `width:${((g.goal_progress || 0) * 100).toFixed(1)}%` }))
  );
}

function planNotes(notes) {
  if (!notes?.length) return null;
  return el('div', {}, notes.map((n) =>
    el('div', {
      class: `note${n.code === 'underweight_warning' ? ' note--warn' : ''}`,
      text: noteText(n),
    })
  ));
}

// -------------------------------------------------------------------- food

function foodList(entries, day) {
  if (!entries.length) return el('p', { class: 'empty', text: t('nothing_yet') });
  return el('ul', { class: 'list' }, entries.map((e) => foodRow(e, day)));
}

function foodRow(e, day) {
  return el(
    'li',
    { class: 'item' },
    el(
      'div',
      { class: 'item__body' },
      el('div', { class: 'item__title', text: e.name }),
      el(
        'div',
        { class: 'item__meta' },
        el('span', { class: `tag${e.source?.startsWith('ai_') ? ' tag--ai' : ''}`,
          text: t('food_source_' + e.source) }),
        [t('meal_' + e.meal_type), e.amount, shortTime(e.logged_at)].filter(Boolean).join(' · ')
      )
    ),
    el('div', { class: 'item__value', text: `${num(e.kcal)} ${t('kcal')}` }),
    el('button', {
      class: 'icon-btn',
      text: '🗑',
      'aria-label': t('delete'),
      onClick: async () => {
        try {
          await api.deleteFood(e.id, day);
          await reload();
        } catch (err) { fail(err); }
      },
    })
  );
}

function workoutRow(w) {
  return el(
    'li',
    { class: 'item' },
    el(
      'div',
      { class: 'item__body' },
      el('div', { class: 'item__title', text: t('workout_' + w.kind) || w.kind }),
      el('div', { class: 'item__meta',
        text: [w.minutes ? `${num(w.minutes)} ${t('min')}` : null,
               w.steps ? `${num(w.steps)} ${t('tracker_steps')}` : null].filter(Boolean).join(' · ') })
    ),
    el('div', { class: 'item__value', text: `${num(w.kcal)} ${t('kcal')}` }),
    w.source === 'manual'
      ? el('button', {
          class: 'icon-btn',
          text: '🗑',
          'aria-label': t('delete'),
          onClick: async () => {
            try { await api.deleteWorkout(w.id); await reload(); } catch (err) { fail(err); }
          },
        })
      : null
  );
}

function foodView() {
  const b = state.boot;
  return el(
    'div',
    {},
    el(
      'div',
      { class: 'card' },
      el('div', { class: 'stack' },
        el('button', { class: 'btn btn--primary btn--block', text: `🍽️  ${t('log_food')}`, onClick: openFoodSheet }),
        !b.capabilities?.ai ? el('p', { class: 'empty', text: t('ai_unavailable') }) : null
      )
    ),
    el(
      'div',
      { class: 'card' },
      el('div', { class: 'card__head' },
        el('h2', { class: 'card__title', text: t('todays_meals') }),
        el('span', { class: 'card__meta', text: `${num(b.totals?.kcal)} / ${num(b.plan?.target_kcal)} ${t('kcal')}` })
      ),
      foodList(b.food || [], b.today)
    ),
    el(
      'div',
      { class: 'card' },
      el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('recipes_browse') })),
      el('button', { class: 'btn btn--block', text: t('recipes_search'), onClick: openRecipeBrowser })
    )
  );
}

/** The main food-logging sheet: photo, text estimate, or manual entry. */
function openFoodSheet() {
  const hasAI = !!state.boot.capabilities?.ai;

  const textInput = el('input', { type: 'text', placeholder: t('food_describe_hint') });
  const fileInput = el('input', {
    type: 'file',
    accept: 'image/*',
    capture: 'environment',
    style: 'display:none',
  });
  const result = el('div', {});

  const showEstimate = (estimate) => {
    fill(result, estimateCard(estimate, () => handle.close()));
    result.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  };

  const runText = async () => {
    const text = textInput.value.trim();
    if (!text) return;
    fill(result, el('p', { class: 'empty', text: t('food_estimating') }));
    try {
      const res = await api.estimateText(text);
      showEstimate(res.estimate);
    } catch (err) {
      fill(result, el('p', { class: 'empty', text: err.message }));
    }
  };

  fileInput.addEventListener('change', async () => {
    const file = fileInput.files?.[0];
    if (!file) return;
    fill(result, el('p', { class: 'empty', text: t('food_estimating') }));
    try {
      const res = await api.estimateImage(file, textInput.value.trim());
      showEstimate(res.estimate);
    } catch (err) {
      fill(result, el('p', { class: 'empty', text: err.message }));
    } finally {
      fileInput.value = '';
    }
  });

  textInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); runText(); }
  });

  const handle = sheet(
    t('food_describe'),
    el(
      'div',
      {},
      field(t('food_describe'), textInput, t('food_describe_hint')),
      el(
        'div',
        { class: 'stack' },
        el('button', { class: 'btn btn--primary btn--block', text: `✨  ${t('food_estimate')}`, onClick: runText }),
        hasAI
          ? el('button', { class: 'btn btn--block', text: `📷  ${t('food_photo')}`, onClick: () => fileInput.click() })
          : null,
        el('button', { class: 'btn btn--ghost btn--block', text: t('food_manual'), onClick: () => {
          handle.close();
          openManualFoodSheet();
        } })
      ),
      hasAI ? el('p', { class: 'field__hint', text: t('food_photo_hint') }) : null,
      fileInput,
      result
    )
  );
}

/** The confirmation card shown after an estimate — nothing is saved until the
 *  user agrees the numbers look right. */
function estimateCard(estimate, done) {
  const mealSelect = mealTypeSelect(estimate.meal_type);
  const kcalInput = el('input', { type: 'number', value: Math.round(estimate.kcal), min: '0', step: '1' });
  const nameInput = el('input', {
    type: 'text',
    value: estimate.items?.map((i) => i.name).join(', ') || '',
  });

  const save = async () => {
    try {
      await api.addFood({
        name: nameInput.value.trim() || '—',
        amount: estimate.items?.map((i) => i.amount).filter(Boolean).join(', ') || '',
        meal_type: mealSelect.value,
        kcal: Number(kcalInput.value) || 0,
        protein_g: estimate.protein_g,
        carbs_g: estimate.carbs_g,
        fat_g: estimate.fat_g,
        source: estimate.source,
        confidence: estimate.confidence,
      });
      toast(t('food_added'), 'success');
      done?.();
      await reload();
    } catch (err) { fail(err); }
  };

  return el(
    'div',
    { class: 'card confetti-pop', style: 'margin-top:14px' },
    el('div', { class: 'card__head' },
      el('h3', { class: 'card__title', text: t('food_estimate_result') }),
      el('span', { class: 'card__meta',
        text: `${t('food_confidence')}: ${t('confidence_' + (estimate.confidence || 'medium'))}` })
    ),

    el('ul', { class: 'list' }, (estimate.items || []).map((i) =>
      el('li', { class: 'item' },
        el('div', { class: 'item__body' },
          el('div', { class: 'item__title', text: i.name }),
          el('div', { class: 'item__meta', text: i.amount })
        ),
        el('div', { class: 'item__value', text: `${num(i.kcal)} ${t('kcal')}` })
      )
    )),

    el('div', { class: 'row row--between', style: 'margin:10px 0' },
      el('strong', { text: `${num(estimate.kcal)} ${t('kcal')}` }),
      el('span', { class: 'muted small',
        text: `${t('protein')} ${num(estimate.protein_g)} g · ${t('carbs')} ${num(estimate.carbs_g)} g · ${t('fat')} ${num(estimate.fat_g)} g` })
    ),

    estimate.assumptions
      ? el('div', { class: 'note', text: `${t('food_assumptions')}: ${estimate.assumptions}` })
      : null,

    el('details', { style: 'margin-bottom:10px' },
      el('summary', { class: 'muted small', style: 'cursor:pointer', text: t('food_adjust') }),
      el('div', { style: 'margin-top:10px' },
        field(t('field_name'), nameInput),
        field(t('kcal'), kcalInput)
      )
    ),

    field(t('meal_breakfast') + ' / ' + t('meal_lunch'), mealSelect),
    el('button', { class: 'btn btn--primary btn--block', text: `✓  ${t('food_confirm_question')}`, onClick: save })
  );
}

function mealTypeSelect(value) {
  return el('select', {},
    ['breakfast', 'lunch', 'dinner', 'snack'].map((m) =>
      el('option', { value: m, text: t('meal_' + m), selected: m === value })
    )
  );
}

function openManualFoodSheet() {
  const name = el('input', { type: 'text' });
  const amount = el('input', { type: 'text', placeholder: '150 g' });
  const kcal = el('input', { type: 'number', min: '0', step: '1' });
  const protein = el('input', { type: 'number', min: '0', step: '0.1' });
  const carbs = el('input', { type: 'number', min: '0', step: '0.1' });
  const fat = el('input', { type: 'number', min: '0', step: '0.1' });
  const meal = mealTypeSelect('snack');
  const results = el('div', {});

  // Searching the built-in table pre-fills the form, so manual entry is rarely
  // fully manual.
  let searchTimer;
  name.addEventListener('input', () => {
    clearTimeout(searchTimer);
    const q = name.value.trim();
    if (q.length < 2) { fill(results); return; }
    searchTimer = setTimeout(async () => {
      try {
        const res = await api.searchFood(q);
        fill(results, (res.results || []).slice(0, 6).map((r) =>
          el('button', {
            class: 'btn btn--sm',
            type: 'button',
            style: 'margin:3px 3px 0 0',
            text: `${r.name} · ${num(r.kcal)} kcal / ${r.portion}`,
            onClick: () => {
              name.value = r.name;
              amount.value = r.portion;
              kcal.value = Math.round(r.kcal);
              protein.value = r.protein_g;
              carbs.value = r.carbs_g;
              fat.value = r.fat_g;
              fill(results);
            },
          })
        ));
      } catch { fill(results); }
    }, 250);
  });

  const handle = sheet(
    t('food_manual'),
    el(
      'div',
      {},
      field(t('field_name'), name, t('food_search')),
      results,
      field(t('food_estimate'), amount),
      field(t('kcal'), kcal),
      el('div', { class: 'grid-2' },
        field(t('protein') + ' (g)', protein),
        field(t('carbs') + ' (g)', carbs)
      ),
      field(t('fat') + ' (g)', fat),
      field(t('meal_lunch'), meal),
      el('button', {
        class: 'btn btn--primary btn--block',
        text: t('save'),
        onClick: async () => {
          if (!name.value.trim()) return;
          try {
            await api.addFood({
              name: name.value.trim(),
              amount: amount.value.trim(),
              meal_type: meal.value,
              kcal: Number(kcal.value) || 0,
              protein_g: Number(protein.value) || 0,
              carbs_g: Number(carbs.value) || 0,
              fat_g: Number(fat.value) || 0,
              source: 'manual',
            });
            toast(t('food_added'), 'success');
            handle.close();
            await reload();
          } catch (err) { fail(err); }
        },
      })
    )
  );
}

// ------------------------------------------------------------------ weight

function openWeightSheet() {
  const last = state.boot.current_weight || state.boot.profile?.start_weight_kg || 70;
  const weight = el('input', { type: 'number', step: '0.1', min: '30', max: '350', value: last });
  const fatPct = el('input', { type: 'number', step: '0.1', min: '3', max: '70' });

  const handle = sheet(
    t('weight_add'),
    el(
      'div',
      {},
      field(`${t('field_weight')} (${t('kg')})`, weight),
      field(`${t('weight_body_fat')} (%)`, fatPct, t('optional')),
      el('button', {
        class: 'btn btn--primary btn--block',
        text: t('save'),
        onClick: async () => {
          try {
            await api.addWeight({
              weight_kg: Number(weight.value),
              body_fat_pct: fatPct.value ? Number(fatPct.value) : null,
            });
            toast(t('profile_saved'), 'success');
            handle.close();
            await reload();
          } catch (err) { fail(err); }
        },
      })
    )
  );
}

function weightView() {
  const container = el('div', {}, el('p', { class: 'empty', text: t('loading') }));

  api.stats(180)
    .then((data) => {
      const plan = data.plan || {};
      const points = data.weights || [];
      const current = points.length ? points[points.length - 1].weight : plan.target_weight_kg;
      const start = state.boot.profile?.start_weight_kg || current;

      fill(
        container,
        el(
          'div',
          { class: 'card' },
          el('div', { class: 'stats' },
            stat(`${num(current, 1)}`, t('weight_current')),
            stat(`${num(start, 1)}`, t('weight_start')),
            stat(`${num(plan.target_weight_kg, 1)}`, t('weight_target'))
          ),
          el('div', { class: 'row row--between', style: 'margin-top:12px' },
            el('span', { class: 'muted small',
              text: `${t('bmi_label')} ${num(plan.bmi, 1)} · ${t('bmi_' + (plan.bmi_category || 'unknown'))}` }),
            el('span', { class: 'muted small',
              text: `${t('weight_change')}: ${current - start > 0 ? '+' : ''}${num(current - start, 1)} ${t('kg')}` })
          ),
          el('button', { class: 'btn btn--primary btn--block', style: 'margin-top:12px',
            text: t('weight_add'), onClick: openWeightSheet })
        ),

        el(
          'div',
          { class: 'card' },
          el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('chart_weight') })),
          weightChart({
            points,
            trend: data.weight_trend || [],
            healthyLow: plan.healthy_low_kg,
            healthyHigh: plan.healthy_high_kg,
            target: plan.target_weight_kg,
          })
        ),

        el(
          'div',
          { class: 'card' },
          el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('chart_kcal') })),
          kcalChart((data.daily_totals || []).slice(-30), plan.target_kcal)
        ),

        el(
          'div',
          { class: 'card' },
          el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('plan_energy_title') })),
          el('div', { class: 'summary-grid' },
            summaryItem(num(plan.bmr), t('plan_bmr')),
            summaryItem(num(plan.tdee), t('plan_tdee')),
            summaryItem(num(plan.target_kcal), t('plan_target')),
            summaryItem(`${plan.weekly_change_kg > 0 ? '+' : ''}${num(plan.weekly_change_kg, 2)} ${t('kg')}`,
              t('plan_weekly_change'))
          ),
          plan.estimated_weeks
            ? el('p', { class: 'muted small', text: t('plan_eta', { n: plan.estimated_weeks }) })
            : el('p', { class: 'muted small', text: t('plan_eta_none') })
        ),

        weightHistory(points)
      );
    })
    .catch((err) => fill(container, el('p', { class: 'empty', text: err.message })));

  return container;
}

function summaryItem(value, label) {
  return el('div', { class: 'summary-item' },
    el('div', { class: 'summary-item__value', text: value }),
    el('div', { class: 'summary-item__label', text: label })
  );
}

function weightHistory(points) {
  if (!points.length) return null;
  const recent = [...points].reverse().slice(0, 20);
  return el(
    'div',
    { class: 'card' },
    el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('weight_title') })),
    el('ul', { class: 'list' }, recent.map((p) =>
      el('li', { class: 'item' },
        el('div', { class: 'item__body' },
          el('div', { class: 'item__title', text: `${num(p.weight, 1)} ${t('kg')}` }),
          el('div', { class: 'item__meta', text: shortDate(p.date) })
        )
      )
    ))
  );
}

// ---------------------------------------------------------------- workouts

const WORKOUT_KINDS = ['walk', 'run', 'cycle', 'strength', 'swim', 'yoga', 'hiit', 'stroller', 'housework', 'other'];

function openWorkoutSheet() {
  const kind = el('select', {}, WORKOUT_KINDS.map((k) =>
    el('option', { value: k, text: t('workout_' + k) })
  ));
  const minutes = el('input', { type: 'number', min: '1', max: '600', step: '1', value: '30' });
  const kcal = el('input', { type: 'number', min: '0', step: '1' });

  const handle = sheet(
    t('log_workout'),
    el(
      'div',
      {},
      field(t('workout_kind'), kind),
      field(`${t('workout_minutes')} (${t('min')})`, minutes),
      field(t('kcal'), kcal, t('workout_kcal_auto')),
      el('button', {
        class: 'btn btn--primary btn--block',
        text: t('save'),
        onClick: async () => {
          try {
            await api.addWorkout({
              kind: kind.value,
              minutes: Number(minutes.value) || 0,
              kcal: Number(kcal.value) || 0,
            });
            toast(t('profile_saved'), 'success');
            handle.close();
            await reload();
          } catch (err) { fail(err); }
        },
      })
    )
  );
}

// -------------------------------------------------------------- meal plan

function planView() {
  const container = el('div', {}, el('p', { class: 'empty', text: t('loading') }));
  loadPlan(container);
  return container;
}

async function loadPlan(container, week) {
  try {
    const data = await api.plan(week || state.planWeek || '');
    state.planWeek = data.week_start;
    state.planData = data;
    fill(container, planContent(container, data));
  } catch (err) {
    fill(container, el('p', { class: 'empty', text: err.message }));
  }
}

function shiftWeek(days) {
  const d = new Date(state.planWeek + 'T12:00:00');
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

function planContent(container, data) {
  const plan = data.plan || {};
  const days = plan.days || [];

  return el(
    'div',
    {},
    el(
      'div',
      { class: 'card card--tight' },
      el('div', { class: 'row row--between' },
        el('button', { class: 'icon-btn', text: '‹', 'aria-label': t('plan_prev_week'),
          onClick: () => loadPlan(container, shiftWeek(-7)) }),
        el('div', { style: 'text-align:center' },
          el('div', { style: 'font-weight:650', text: t('plan_week_of', { date: shortDate(data.week_start) }) }),
          el('div', { class: 'muted small', text: t('plan_avg', { n: num(plan.avg_kcal) }) })
        ),
        el('button', { class: 'icon-btn', text: '›', 'aria-label': t('plan_next_week'),
          onClick: () => loadPlan(container, shiftWeek(7)) })
      ),
      !data.saved ? el('p', { class: 'empty', style: 'text-align:center', text: t('plan_unsaved') }) : null,
      el('div', { class: 'row', style: 'margin-top:10px' },
        el('button', {
          class: 'btn btn--primary', style: 'flex:1',
          text: data.saved ? t('plan_shuffle') : t('plan_generate'),
          onClick: async (e) => {
            e.target.disabled = true;
            try {
              const res = await api.generatePlan(state.planWeek, Math.floor(Math.random() * 100000));
              state.planData = res;
              fill(container, planContent(container, res));
              toast(t('profile_saved'), 'success');
            } catch (err) { fail(err); e.target.disabled = false; }
          },
        })
      )
    ),

    planNotes(plan.notes),

    el('div', { class: 'card' },
      days.map((day) => planDay(container, day, data))),

    shoppingCard(data)
  );
}

function planDay(container, day, data) {
  if (!day.entries?.length) return null;
  return el(
    'div',
    { class: 'day' },
    el('div', { class: 'day__head' },
      el('span', { class: 'day__name', text: `${day.name}, ${shortDate(day.date)}` }),
      el('span', { class: 'day__kcal', text: `${num(day.kcal)} ${t('kcal')}` })
    ),
    day.entries.map((entry) => planMeal(container, entry, data))
  );
}

function planMeal(container, entry, data) {
  return el(
    'div',
    { class: 'meal' },
    el('div', { class: 'meal__slot', text: t('meal_' + entry.meal_type) }),
    el(
      'div',
      { class: 'meal__body' },
      el('button', {
        class: 'meal__title',
        type: 'button',
        text: entry.title,
        onClick: () => openRecipe(entry.recipe_id),
      }),
      el('div', { class: 'meal__meta',
        text: [`${num(entry.kcal)} ${t('kcal')}`,
               t('plan_portions', { n: num(entry.portions, 2) }),
               entry.leftover ? t('plan_leftover') : null].filter(Boolean).join(' · ') }),
      data.saved && entry.id
        ? el('div', { class: 'meal__actions' },
            el('button', {
              class: `btn btn--sm${entry.cooked ? ' btn--primary' : ''}`,
              text: entry.cooked ? `✓ ${t('plan_cooked')}` : t('plan_cooked'),
              onClick: async () => {
                try {
                  await api.setCooked(entry.id, !entry.cooked);
                  await loadPlan(container, state.planWeek);
                } catch (err) { fail(err); }
              },
            }),
            el('button', {
              class: 'btn btn--sm',
              text: `🍽 ${t('plan_log_meal')}`,
              onClick: async () => {
                try {
                  await api.logPlannedMeal(entry.id, state.planWeek, entry.portions);
                  toast(t('food_added'), 'success');
                  state.boot = await api.bootstrap();
                  await loadPlan(container, state.planWeek);
                } catch (err) { fail(err); }
              },
            })
          )
        : null
    )
  );
}

function shoppingCard(data) {
  const items = data.shopping || [];
  if (!items.length) {
    return el('div', { class: 'card' },
      el('h2', { class: 'card__title', text: t('shopping_title') }),
      el('p', { class: 'empty', text: t('shopping_empty') })
    );
  }

  // Group by aisle, keeping the order the backend already sorted them into.
  const groups = [];
  for (const item of items) {
    let g = groups.find((x) => x.name === item.category);
    if (!g) { g = { name: item.category, items: [] }; groups.push(g); }
    g.items.push(item);
  }

  const household = state.boot.profile?.prefs?.household_size || 1;

  return el(
    'div',
    { class: 'card' },
    el('div', { class: 'card__head' },
      el('h2', { class: 'card__title', text: t('shopping_title') }),
      el('span', { class: 'card__meta', text: `${items.length}` })
    ),
    el('p', { class: 'field__hint', style: 'margin:0 0 10px',
      text: t('shopping_hint', { n: household }) }),
    groups.map((g) =>
      el('div', { class: 'shop-group' },
        el('div', { class: 'shop-group__title', text: g.name }),
        g.items.map(shoppingRow)
      )
    )
  );
}

function shoppingRow(item) {
  const row = el('label', { class: `shop-item${item.checked ? ' shop-item--done' : ''}` });
  const box = el('input', {
    type: 'checkbox',
    checked: item.checked,
    onChange: async () => {
      try {
        await api.checkShopping(item.id, box.checked);
        row.classList.toggle('shop-item--done', box.checked);
      } catch (err) {
        box.checked = !box.checked;
        fail(err);
      }
    },
  });
  return fill(row,
    box,
    el('span', { class: 'shop-item__name', text: item.name }),
    el('span', { class: 'shop-item__amount',
      text: item.amount ? `${num(item.amount, item.amount % 1 ? 1 : 0)} ${item.unit}` : '' })
  );
}

async function openRecipe(id) {
  try {
    const servings = state.boot.profile?.prefs?.household_size || 2;
    const data = await api.recipe(id, servings);
    const r = data.recipe;
    sheet(
      r.title,
      el(
        'div',
        {},
        el('p', { class: 'muted', text: r.description }),
        el('div', { class: 'recipe__meta' },
          el('span', { text: `🔥 ${num(r.kcal)} ${t('kcal')} ${t('recipe_per_serving')}` }),
          el('span', { text: `⏱ ${t('recipe_time', { n: r.prep_minutes })}` }),
          el('span', { text: `🍽 ${data.servings} ${t('recipe_servings')}` })
        ),
        el('div', { style: 'margin-bottom:12px' },
          (r.tags || []).map((tag) => el('span', { class: 'tag', text: tag }))),
        el('div', { class: 'row', style: 'margin-bottom:14px' },
          el('span', { class: 'muted small',
            text: `${t('protein')} ${num(r.protein_g)} g · ${t('carbs')} ${num(r.carbs_g)} g · ${t('fat')} ${num(r.fat_g)} g` })
        ),

        el('h3', { text: t('recipe_ingredients') }),
        el('ul', { class: 'list' }, (data.scaled_ingredients || []).map((i) =>
          el('li', { class: 'item' },
            el('div', { class: 'item__body' }, el('div', { class: 'item__title', text: i.name })),
            el('div', { class: 'item__value',
              text: i.amount ? `${num(i.amount, i.amount % 1 ? 1 : 0)} ${i.unit}` : '' })
          )
        )),

        el('h3', { style: 'margin-top:16px', text: t('recipe_steps') }),
        el('ol', { class: 'recipe__steps' }, (r.steps || []).map((s) => el('li', { text: s }))),

        r.url
          ? el('a', { class: 'btn btn--block', style: 'margin-top:14px;display:block;text-align:center;text-decoration:none',
              href: r.url, target: '_blank', rel: 'noopener noreferrer', text: t('recipe_open') })
          : null
      )
    );
  } catch (err) { fail(err); }
}

function openRecipeBrowser() {
  const search = el('input', { type: 'search', placeholder: t('recipes_search') });
  const list = el('div', {});

  const load = async () => {
    fill(list, el('p', { class: 'empty', text: t('loading') }));
    try {
      const params = new URLSearchParams();
      if (search.value.trim()) params.set('q', search.value.trim());
      const data = await api.recipes(params.toString());
      fill(list, el('ul', { class: 'list' }, (data.recipes || []).map((r) =>
        el('li', { class: 'item' },
          el('div', { class: 'item__body' },
            el('button', { class: 'meal__title', type: 'button', text: r.title,
              onClick: () => openRecipe(r.id) }),
            el('div', { class: 'item__meta',
              text: `${num(r.kcal)} ${t('kcal')} · ${t('recipe_time', { n: r.prep_minutes })}` })
          )
        )
      )));
    } catch (err) {
      fill(list, el('p', { class: 'empty', text: err.message }));
    }
  };

  let timer;
  search.addEventListener('input', () => {
    clearTimeout(timer);
    timer = setTimeout(load, 250);
  });

  sheet(t('recipes_browse'), el('div', {}, field(t('recipes_search'), search), list));
  load();
}

// ------------------------------------------------------------------ awards

function awardsView() {
  const g = state.boot.gamify || {};
  const badges = g.badges || [];

  const groups = ['start', 'streak', 'weight', 'food', 'sport', 'mealprep', 'health'];

  return el(
    'div',
    {},
    levelCard(g),
    el(
      'div',
      { class: 'card' },
      el('div', { class: 'card__head' },
        el('h2', { class: 'card__title', text: t('awards_title') }),
        el('span', { class: 'card__meta',
          text: t('awards_unlocked', { n: g.unlocked_count, total: g.total_count }) })
      ),
      groups.map((group) => {
        const inGroup = badges.filter((b) => b.group === group);
        if (!inGroup.length) return null;
        return el('div', { style: 'margin-bottom:18px' },
          el('div', { class: 'shop-group__title', text: t('group_' + group) }),
          el('div', { class: 'badges' }, inGroup.map(badgeCard))
        );
      })
    ),
    milestonesCard(g)
  );
}

function badgeCard(b) {
  const [title, desc] = badgeText(b.code);
  return el(
    'div',
    {
      class: `badge ${b.unlocked ? 'badge--on' : 'badge--locked'}`,
      title: b.unlocked ? title : `${title} — ${desc}`,
    },
    el('span', { class: 'badge__icon', text: b.unlocked ? b.icon : '🔒' }),
    el('div', { class: 'badge__title', text: title }),
    el('div', { class: 'badge__desc', text: desc }),
    !b.unlocked && b.goal > 1
      ? el('div', { class: 'badge__track' },
          el('div', { class: 'badge__fill', style: `width:${((b.progress || 0) * 100).toFixed(0)}%` }))
      : null
  );
}

function milestonesCard(g) {
  const list = g.milestones || [];
  if (!list.length) return null;
  const nextIndex = list.findIndex((m) => !m.reached);

  return el(
    'div',
    { class: 'card' },
    el('div', { class: 'card__head' }, el('h2', { class: 'card__title', text: t('milestones_title') })),
    el('ul', { class: 'milestones' }, list.map((m, i) =>
      el(
        'li',
        { class: `milestone${m.reached ? ' milestone--done' : ''}${i === nextIndex ? ' milestone--next' : ''}` },
        el('span', { class: 'milestone__dot', text: m.reached ? '✓' : '' }),
        el('div', {},
          el('div', { class: 'milestone__label',
            text: `${m.is_goal ? t('milestone_goal') + ': ' : ''}${num(m.weight_kg, 1)} ${t('kg')}` }),
          m.is_bmi_healthy
            ? el('div', { class: 'milestone__note', text: t('milestone_healthy_bmi') })
            : m.reached ? el('div', { class: 'milestone__note', text: t('milestone_reached') }) : null
        )
      )
    ))
  );
}

// ----------------------------------------------------------------- profile

/** Build a full profile payload, applying overrides. The API replaces the whole
 *  profile, so every field must be present on every save. */
function profilePayload(p, overrides = {}) {
  return {
    display_name: p.display_name,
    sex: p.sex,
    birth_date: p.birth_date,
    height_cm: p.height_cm,
    weight_kg: state.boot.current_weight || p.start_weight_kg,
    target_weight_kg: p.target_weight_kg,
    activity: p.activity,
    goal: p.goal,
    breastfeeding: p.breastfeeding,
    prefs: p.prefs,
    ...overrides,
  };
}

function profileView() {
  const p = state.boot.profile;
  const draft = JSON.parse(JSON.stringify(p));
  const container = el('div', {});

  const save = async () => {
    try {
      await api.saveProfile(profilePayload(draft, {
        display_name: draft.display_name,
        sex: draft.sex,
        birth_date: draft.birth_date,
        height_cm: Number(draft.height_cm),
        target_weight_kg: Number(draft.target_weight_kg) || 0,
        activity: draft.activity,
        goal: draft.goal,
        breastfeeding: draft.breastfeeding,
        prefs: draft.prefs,
      }));
      toast(t('profile_saved'), 'success');
      state.planData = null;
      await reload();
    } catch (err) { fail(err); }
  };

  const rerender = () => fill(container, body());

  const body = () => el(
    'div',
    {},
    el(
      'div',
      { class: 'card' },
      el('h2', { class: 'card__title', text: t('profile_title') }),
      field(t('field_name'),
        el('input', { type: 'text', value: draft.display_name,
          onInput: (e) => { draft.display_name = e.target.value; } })),
      field(t('field_birth_date'),
        el('input', { type: 'date', value: draft.birth_date,
          onInput: (e) => { draft.birth_date = e.target.value; } })),
      field(`${t('field_height')} (${t('cm')})`,
        el('input', { type: 'number', value: draft.height_cm, min: '120', max: '230',
          onInput: (e) => { draft.height_cm = e.target.value; } })),
      field(`${t('field_target_weight')} (${t('kg')})`,
        el('input', { type: 'number', step: '0.1', value: draft.target_weight_kg || '',
          onInput: (e) => { draft.target_weight_kg = e.target.value; } }),
        t('field_target_weight_hint')),

      el('div', { class: 'field__label', text: t('field_sex') }),
      optionGroup('sex', [
        { value: 'female', label: t('sex_female') },
        { value: 'male', label: t('sex_male') },
        { value: 'divers', label: t('sex_divers') },
      ], draft.sex, (v) => { draft.sex = v; rerender(); }),

      el('div', { class: 'field__label', style: 'margin-top:14px', text: t('field_activity') }),
      optionGroup('activity', ACTIVITY_OPTIONS(), draft.activity, (v) => { draft.activity = v; rerender(); }),

      el('div', { class: 'field__label', style: 'margin-top:14px', text: t('field_goal') }),
      optionGroup('goal', GOAL_OPTIONS(), draft.goal, (v) => { draft.goal = v; rerender(); }),

      el('div', { class: 'field__label', style: 'margin-top:14px', text: t('field_breastfeeding') }),
      optionGroup('bf', BF_OPTIONS(), draft.breastfeeding, (v) => { draft.breastfeeding = v; rerender(); }),
      el('p', { class: 'field__hint', text: t('bf_hint') })
    ),

    el(
      'div',
      { class: 'card' },
      el('h2', { class: 'card__title', text: t('profile_food_prefs') }),
      prefsEditor(draft.prefs, rerender)
    ),

    el('button', { class: 'btn btn--primary btn--block', text: t('save'), onClick: save }),

    trackerCard(),

    el(
      'div',
      { class: 'card', style: 'margin-top:14px' },
      el('h2', { class: 'card__title', text: t('profile_about') }),
      el('p', { class: 'muted small', text: t('profile_data_note') }),
      el('p', { class: 'muted small',
        text: `${state.boot.capabilities?.recipes} ${t('recipes_browse')} · ${state.boot.capabilities?.ai ? state.boot.capabilities.ai_model : t('none')}` })
    )
  );

  return fill(container, body());
}

function prefsEditor(prefs, rerender) {
  return el(
    'div',
    {},
    el('div', { class: 'field__label', text: t('field_fish') }),
    optionGroup('fish', [
      { value: 'breaded_only', label: t('fish_breaded_only'), hint: t('fish_breaded_only_hint') },
      { value: 'any', label: t('fish_any') },
      { value: 'none', label: t('fish_none') },
    ], prefs.fish_policy, (v) => { prefs.fish_policy = v; rerender(); }),

    el('div', { class: 'field__label', style: 'margin-top:14px', text: t('field_veggie') }),
    optionGroup('veg', [
      { value: 'low', label: t('veggie_low'), hint: t('veggie_low_hint') },
      { value: 'medium', label: t('veggie_medium') },
      { value: 'high', label: t('veggie_high') },
    ], prefs.veggie_level, (v) => { prefs.veggie_level = v; rerender(); }),

    el('div', { class: 'grid-2', style: 'margin-top:14px' },
      field(t('field_household'),
        el('input', { type: 'number', min: '1', max: '10', value: prefs.household_size,
          onInput: (e) => { prefs.household_size = Number(e.target.value) || 1; } })),
      field(t('field_meals_per_day'),
        el('select', {
          onChange: (e) => { prefs.meals_per_day = Number(e.target.value); },
        },
          [3, 4].map((n) => el('option', { value: n, text: String(n), selected: prefs.meals_per_day === n }))
        ))
    ),
    field(`${t('field_cook_time')} (${t('min')})`,
      el('input', { type: 'number', min: '10', max: '180', step: '5', value: prefs.max_cook_minutes,
        onInput: (e) => { prefs.max_cook_minutes = Number(e.target.value) || 45; } })),

    el('label', { class: 'switch' },
      el('input', { type: 'checkbox', checked: prefs.cook_once_eat_twice,
        onChange: (e) => { prefs.cook_once_eat_twice = e.target.checked; } }),
      el('span', {},
        el('div', { style: 'font-weight:600;font-size:.9rem', text: t('field_cook_once') }),
        el('div', { class: 'field__hint', text: t('field_cook_once_hint') })
      )
    )
  );
}

function trackerCard() {
  const container = el('div', { class: 'card' },
    el('h2', { class: 'card__title', text: t('profile_trackers') }),
    el('p', { class: 'empty', text: t('loading') })
  );

  api.trackers(true).then((data) => {
    if (!data.available) {
      fill(container,
        el('h2', { class: 'card__title', text: t('profile_trackers') }),
        el('p', { class: 'empty', text: t('profile_tracker_unavailable') })
      );
      return;
    }

    const rows = (data.kinds || []).map((kind) => {
      const suggestions = (data.suggestions || []).filter((s) => s.kind === kind);
      const current = data.links?.[kind] || '';
      const options = [{ value: '', label: t('profile_tracker_none') }];
      for (const s of suggestions) options.push({ value: s.entity_id, label: s.name });
      // Keep a saved entity selectable even when discovery no longer sees it.
      if (current && !options.some((o) => o.value === current)) {
        options.push({ value: current, label: current });
      }

      return field(t('tracker_' + kind),
        el('select', {
          onChange: async (e) => {
            try {
              await api.setTracker(kind, e.target.value);
              toast(t('profile_saved'), 'success');
            } catch (err) { fail(err); }
          },
        }, options.map((o) =>
          el('option', { value: o.value, text: o.label, selected: o.value === current })
        ))
      );
    });

    fill(container,
      el('h2', { class: 'card__title', text: t('profile_trackers') }),
      el('p', { class: 'field__hint', style: 'margin-bottom:12px', text: t('profile_trackers_hint') }),
      rows,
      el('button', {
        class: 'btn btn--block',
        text: t('profile_tracker_sync'),
        onClick: async (e) => {
          e.target.disabled = true;
          try {
            await api.syncTrackers();
            toast(t('profile_tracker_synced'), 'success');
            await reload();
          } catch (err) { fail(err); } finally { e.target.disabled = false; }
        },
      })
    );
  }).catch(() => {
    fill(container,
      el('h2', { class: 'card__title', text: t('profile_trackers') }),
      el('p', { class: 'empty', text: t('profile_tracker_unavailable') })
    );
  });

  return container;
}

// ------------------------------------------------------------------ wizard

const ACTIVITY_OPTIONS = () => [
  { value: 'sedentary', label: t('activity_sedentary'), hint: t('activity_sedentary_hint') },
  { value: 'light', label: t('activity_light'), hint: t('activity_light_hint') },
  { value: 'moderate', label: t('activity_moderate'), hint: t('activity_moderate_hint') },
  { value: 'active', label: t('activity_active'), hint: t('activity_active_hint') },
  { value: 'very_active', label: t('activity_very_active'), hint: t('activity_very_active_hint') },
];

const GOAL_OPTIONS = () => [
  { value: 'lose', label: t('goal_lose'), hint: t('goal_lose_hint') },
  { value: 'maintain', label: t('goal_maintain'), hint: t('goal_maintain_hint') },
  { value: 'gain_muscle', label: t('goal_gain_muscle'), hint: t('goal_gain_muscle_hint') },
  { value: 'recomp', label: t('goal_recomp'), hint: t('goal_recomp_hint') },
];

const BF_OPTIONS = () => [
  { value: 'none', label: t('bf_none') },
  { value: 'partial', label: t('bf_partial') },
  { value: 'exclusive', label: t('bf_exclusive') },
];

const WIZARD_STEPS = 5;

function renderWizard() {
  const draft = {
    display_name: state.boot.user?.name || '',
    sex: 'female',
    birth_date: '',
    height_cm: 170,
    weight_kg: 70,
    target_weight_kg: 0,
    activity: 'light',
    goal: 'lose',
    breastfeeding: 'none',
    prefs: {
      fish_policy: 'breaded_only',
      max_fish_per_week: 1,
      veggie_level: 'low',
      household_size: 2,
      meals_per_day: 4,
      max_cook_minutes: 45,
      language: getLang(),
      cook_once_eat_twice: true,
    },
  };

  let step = 0;
  let preview = null;

  const host = el('div', { class: 'app' });

  const draw = () => {
    fill(host,
      el('header', { class: 'topbar' },
        el('div', {},
          el('h1', { class: 'topbar__title', text: t('app_name') }),
          el('p', { class: 'topbar__sub', text: t('app_tagline') })
        ),
        langToggleWizard(draft, draw)
      ),
      el('div', { class: 'wizard' },
        step > 0
          ? el('div', {},
              el('div', { class: 'wizard__step', text: t('setup_step', { n: step, total: WIZARD_STEPS - 1 }) }),
              el('div', { class: 'wizard__bar' },
                el('div', { class: 'wizard__fill', style: `width:${(step / (WIZARD_STEPS - 1)) * 100}%` }))
            )
          : null,
        stepBody()
      )
    );
  };

  const goStep = async (n) => {
    step = n;
    if (step === WIZARD_STEPS - 1) {
      // The summary step needs the calculated plan, so fetch it before drawing.
      try {
        preview = await api.previewProfile(payload());
      } catch (err) {
        fail(err);
        step = WIZARD_STEPS - 2;
      }
    }
    draw();
  };

  const payload = () => ({
    ...draft,
    height_cm: Number(draft.height_cm),
    weight_kg: Number(draft.weight_kg),
    target_weight_kg: Number(draft.target_weight_kg) || 0,
  });

  const navButtons = (nextLabel, onNext) =>
    el('div', { class: 'wizard__nav' },
      step > 1 ? el('button', { class: 'btn', text: t('back'), onClick: () => goStep(step - 1) }) : null,
      el('button', { class: 'btn btn--primary', text: nextLabel, onClick: onNext })
    );

  function stepBody() {
    switch (step) {
      case 0:
        return el('div', { class: 'card' },
          el('h2', { text: t('setup_welcome') }),
          el('p', { class: 'muted', text: t('setup_intro') }),
          el('button', { class: 'btn btn--primary btn--block', text: t('setup_start'),
            onClick: () => goStep(1) })
        );

      case 1:
        return el('div', { class: 'card' },
          el('h2', { text: t('setup_about_you') }),
          field(t('field_name'),
            el('input', { type: 'text', value: draft.display_name,
              onInput: (e) => { draft.display_name = e.target.value; } })),
          field(t('field_birth_date'),
            el('input', { type: 'date', value: draft.birth_date,
              onInput: (e) => { draft.birth_date = e.target.value; } })),
          el('div', { class: 'field__label', text: t('field_sex') }),
          optionGroup('sex', [
            { value: 'female', label: t('sex_female') },
            { value: 'male', label: t('sex_male') },
            { value: 'divers', label: t('sex_divers') },
          ], draft.sex, (v) => { draft.sex = v; draw(); }),
          draft.sex === 'female'
            ? el('div', { style: 'margin-top:16px' },
                el('div', { class: 'field__label', text: t('field_breastfeeding') }),
                optionGroup('bf', BF_OPTIONS(), draft.breastfeeding, (v) => { draft.breastfeeding = v; draw(); }),
                el('p', { class: 'field__hint', text: t('bf_hint') })
              )
            : null,
          navButtons(t('next'), () => goStep(2))
        );

      case 2:
        return el('div', { class: 'card' },
          el('h2', { text: t('setup_your_body') }),
          field(`${t('field_height')} (${t('cm')})`,
            el('input', { type: 'number', min: '120', max: '230', value: draft.height_cm,
              onInput: (e) => { draft.height_cm = e.target.value; } })),
          field(`${t('field_weight')} (${t('kg')})`,
            el('input', { type: 'number', step: '0.1', min: '30', max: '350', value: draft.weight_kg,
              onInput: (e) => { draft.weight_kg = e.target.value; } })),
          el('div', { class: 'field__label', text: t('field_activity') }),
          optionGroup('activity', ACTIVITY_OPTIONS(), draft.activity, (v) => { draft.activity = v; draw(); }),
          navButtons(t('next'), () => goStep(3))
        );

      case 3:
        return el('div', { class: 'card' },
          el('h2', { text: t('setup_your_goal') }),
          optionGroup('goal', GOAL_OPTIONS(), draft.goal, (v) => { draft.goal = v; draw(); }),
          field(`${t('field_target_weight')} (${t('kg')})`,
            el('input', { type: 'number', step: '0.1', value: draft.target_weight_kg || '',
              onInput: (e) => { draft.target_weight_kg = e.target.value; } }),
            t('field_target_weight_hint')),
          el('h3', { style: 'margin-top:20px', text: t('setup_your_food') }),
          prefsEditor(draft.prefs, draw),
          navButtons(t('next'), () => goStep(4))
        );

      default: {
        const plan = preview?.plan || {};
        return el('div', { class: 'card' },
          el('h2', { text: t('setup_summary') }),
          el('div', { class: 'summary-grid' },
            summaryItem(num(plan.target_kcal), t('plan_target')),
            summaryItem(num(plan.tdee), t('plan_tdee')),
            summaryItem(`${num(plan.protein_g)} g`, t('protein')),
            summaryItem(`${num(plan.target_weight_kg, 1)} ${t('kg')}`, t('weight_target'))
          ),
          el('p', { class: 'muted small',
            text: `${t('bmi_label')} ${num(plan.bmi, 1)} · ${t('bmi_' + (plan.bmi_category || 'unknown'))} · ${t('weight_healthy_range', { low: num(plan.healthy_low_kg, 1), high: num(plan.healthy_high_kg, 1) })}` }),
          plan.estimated_weeks
            ? el('p', { class: 'muted small', text: t('plan_eta', { n: plan.estimated_weeks }) })
            : null,
          planNotes(plan.notes),
          navButtons(t('setup_finish'), async () => {
            try {
              await api.saveProfile(payload());
              localStorage.setItem(LANG_KEY, draft.prefs.language);
              await reload();
              toast(`🚀 ${t('profile_saved')}`, 'success');
            } catch (err) { fail(err); }
          })
        );
      }
    }
  }

  draw();
  render(host);
}

/** Language toggle inside the wizard: nothing is saved yet, so it only updates
 *  the draft and redraws. */
function langToggleWizard(draft, redraw) {
  return el(
    'div',
    { class: 'lang-toggle', role: 'group', 'aria-label': t('field_language') },
    ['de', 'en'].map((l) =>
      el('button', {
        type: 'button',
        text: l,
        'aria-pressed': getLang() === l ? 'true' : 'false',
        onClick: () => {
          setLang(l);
          localStorage.setItem(LANG_KEY, l);
          draft.prefs.language = l;
          redraw();
        },
      })
    )
  );
}

start();
