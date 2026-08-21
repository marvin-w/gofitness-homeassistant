// Thin wrapper around fetch.
//
// Every URL here is relative. Home Assistant ingress serves the add-on under a
// generated path, so an absolute "/api/..." would escape the add-on entirely.
// The <base> tag in index.html resolves these against the ingress path.

import { getLang } from './i18n.js';

/** Error carrying the HTTP status so callers can react to 4xx vs 5xx. */
export class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

function withLang(path) {
  const sep = path.includes('?') ? '&' : '?';
  return `${path}${sep}lang=${encodeURIComponent(getLang())}`;
}

async function parse(res) {
  let body = null;
  try {
    body = await res.json();
  } catch {
    // A non-JSON body from a proxy error page is not worth surfacing verbatim.
  }
  if (!res.ok) {
    throw new ApiError(body?.error || `HTTP ${res.status}`, res.status);
  }
  return body;
}

export async function get(path) {
  const res = await fetch(withLang(path), {
    headers: { 'X-GoFitness-Lang': getLang() },
  });
  return parse(res);
}

export async function post(path, body) {
  const res = await fetch(withLang(path), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-GoFitness-Lang': getLang(),
    },
    body: JSON.stringify(body ?? {}),
  });
  return parse(res);
}

export async function del(path) {
  const res = await fetch(withLang(path), {
    method: 'DELETE',
    headers: { 'X-GoFitness-Lang': getLang() },
  });
  return parse(res);
}

/** Upload a photo for calorie estimation. */
export async function postImage(path, file, note) {
  const form = new FormData();
  form.append('image', file);
  if (note) form.append('note', note);
  const res = await fetch(withLang(path), {
    method: 'POST',
    headers: { 'X-GoFitness-Lang': getLang() },
    body: form,
  });
  return parse(res);
}

// Endpoint helpers, so paths live in one place.
export const api = {
  bootstrap: () => get('api/bootstrap'),
  saveProfile: (p) => post('api/profile', p),
  previewProfile: (p) => post('api/profile/preview', p),

  weights: (limit = 200) => get(`api/weights?limit=${limit}`),
  addWeight: (w) => post('api/weights', w),
  deleteWeight: (id) => del(`api/weights/${id}`),

  food: (day) => get(`api/food?day=${encodeURIComponent(day)}`),
  addFood: (f) => post('api/food', f),
  deleteFood: (id, day) => del(`api/food/${id}?day=${encodeURIComponent(day)}`),
  estimateText: (text) => post('api/food/estimate', { text }),
  estimateImage: (file, note) => postImage('api/food/estimate', file, note),
  searchFood: (q) => get(`api/food/search?q=${encodeURIComponent(q)}`),

  workouts: (day) => get(`api/workouts?day=${encodeURIComponent(day)}`),
  addWorkout: (w) => post('api/workouts', w),
  deleteWorkout: (id) => del(`api/workouts/${id}`),

  recipes: (params = '') => get(`api/recipes${params ? '?' + params : ''}`),
  recipe: (id, servings) =>
    get(`api/recipes/${encodeURIComponent(id)}${servings ? `?servings=${servings}` : ''}`),

  plan: (week) => get(`api/plan${week ? `?week=${week}` : ''}`),
  generatePlan: (week, shuffle) => post('api/plan/generate', { week, shuffle }),
  setCooked: (id, cooked) => post(`api/plan/entries/${id}/cooked`, { cooked }),
  logPlannedMeal: (id, week, portions) =>
    post(`api/plan/entries/${id}/log?week=${week}`, { portions }),
  checkShopping: (id, checked) => post(`api/shopping/${id}/check`, { checked }),

  stats: (days = 90) => get(`api/stats?days=${days}`),
  gamify: () => get('api/gamify'),

  trackers: (suggest = false) => get(`api/trackers${suggest ? '?suggest=1' : ''}`),
  setTracker: (kind, entityId) => post('api/trackers', { kind, entity_id: entityId }),
  syncTrackers: () => post('api/trackers/sync', {}),
};
