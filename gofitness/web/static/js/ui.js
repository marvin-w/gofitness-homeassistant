// DOM helpers, sheets, toasts and the two hand-written SVG charts.

import { t, num, shortDate } from './i18n.js';

const SVG_NS = 'http://www.w3.org/2000/svg';

/** Create an element. Attributes starting with "on" become listeners. */
export function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  applyAttrs(node, attrs);
  append(node, children);
  return node;
}

/** Create an SVG element. */
export function svg(tag, attrs = {}, ...children) {
  const node = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === null || v === undefined || v === false) continue;
    node.setAttribute(k, String(v));
  }
  append(node, children);
  return node;
}

function applyAttrs(node, attrs) {
  for (const [k, v] of Object.entries(attrs)) {
    if (v === null || v === undefined || v === false) continue;
    if (k === 'class') node.className = v;
    else if (k === 'text') node.textContent = v;
    else if (k === 'html') node.innerHTML = v;
    else if (k === 'dataset') Object.assign(node.dataset, v);
    else if (k.startsWith('on') && typeof v === 'function') {
      node.addEventListener(k.slice(2).toLowerCase(), v);
    } else if (k === 'value') node.value = v;
    else if (k === 'checked' || k === 'disabled' || k === 'selected') node[k] = !!v;
    else node.setAttribute(k, String(v));
  }
}

function append(node, children) {
  for (const child of children.flat(Infinity)) {
    if (child === null || child === undefined || child === false) continue;
    node.appendChild(typeof child === 'object' ? child : document.createTextNode(String(child)));
  }
}

/** Replace an element's children. */
export function fill(node, ...children) {
  node.replaceChildren();
  append(node, children);
  return node;
}

/** Show a short-lived message. */
export function toast(message, kind = 'info') {
  const host = document.getElementById('toasts');
  const node = el('div', { class: `toast toast--${kind}`, role: 'status', text: message });
  host.appendChild(node);
  // Let the entry transition run before scheduling removal.
  requestAnimationFrame(() => node.classList.add('toast--in'));
  setTimeout(() => {
    node.classList.remove('toast--in');
    setTimeout(() => node.remove(), 250);
  }, 3200);
}

/**
 * Open a bottom sheet. Returns a handle with close().
 * The sheet traps focus loosely: Escape and the backdrop both close it.
 */
export function sheet(title, content, { onClose } = {}) {
  const host = document.getElementById('sheets');

  const close = () => {
    backdrop.classList.remove('sheet-backdrop--in');
    panel.classList.remove('sheet--in');
    setTimeout(() => {
      backdrop.remove();
      document.removeEventListener('keydown', onKey);
    }, 220);
    onClose?.();
  };

  const onKey = (e) => {
    if (e.key === 'Escape') close();
  };

  const panel = el(
    'div',
    { class: 'sheet', role: 'dialog', 'aria-modal': 'true', 'aria-label': title },
    el(
      'div',
      { class: 'sheet__head' },
      el('h2', { class: 'sheet__title', text: title }),
      el('button', {
        class: 'icon-btn',
        type: 'button',
        'aria-label': t('close'),
        text: '✕',
        onClick: close,
      })
    ),
    el('div', { class: 'sheet__body' }, content)
  );

  const backdrop = el(
    'div',
    {
      class: 'sheet-backdrop',
      onClick: (e) => {
        if (e.target === backdrop) close();
      },
    },
    panel
  );

  host.appendChild(backdrop);
  requestAnimationFrame(() => {
    backdrop.classList.add('sheet-backdrop--in');
    panel.classList.add('sheet--in');
  });
  document.addEventListener('keydown', onKey);

  return { close, panel };
}

/** A labelled form field. */
export function field(label, control, hint) {
  return el(
    'label',
    { class: 'field' },
    el('span', { class: 'field__label', text: label }),
    control,
    hint ? el('span', { class: 'field__hint', text: hint }) : null
  );
}

/** A group of radio-style option cards. */
export function optionGroup(name, options, value, onChange) {
  return el(
    'div',
    { class: 'options' },
    options.map((opt) =>
      el(
        'button',
        {
          type: 'button',
          class: `option${opt.value === value ? ' option--on' : ''}`,
          'aria-pressed': opt.value === value ? 'true' : 'false',
          onClick: () => onChange(opt.value),
        },
        el('span', { class: 'option__label', text: opt.label }),
        opt.hint ? el('span', { class: 'option__hint', text: opt.hint }) : null
      )
    )
  );
}

/**
 * The calorie ring on the dashboard.
 * A single value against a target reads faster as one ring than as a chart.
 */
export function calorieRing(eaten, target) {
  const size = 200;
  const stroke = 16;
  const r = (size - stroke) / 2;
  const circumference = 2 * Math.PI * r;
  const ratio = target > 0 ? Math.min(eaten / target, 1) : 0;
  const over = target > 0 && eaten > target;
  const remaining = Math.round(target - eaten);

  return el(
    'div',
    { class: 'ring' },
    svg(
      'svg',
      { viewBox: `0 0 ${size} ${size}`, class: 'ring__svg', 'aria-hidden': 'true' },
      svg('circle', {
        cx: size / 2, cy: size / 2, r,
        class: 'ring__track', 'stroke-width': stroke, fill: 'none',
      }),
      svg('circle', {
        cx: size / 2, cy: size / 2, r,
        class: `ring__value${over ? ' ring__value--over' : ''}`,
        'stroke-width': stroke,
        fill: 'none',
        'stroke-linecap': 'round',
        'stroke-dasharray': `${circumference * ratio} ${circumference}`,
        transform: `rotate(-90 ${size / 2} ${size / 2})`,
      })
    ),
    el(
      'div',
      { class: 'ring__center' },
      el('span', { class: 'ring__number', text: num(Math.abs(remaining)) }),
      el('span', { class: 'ring__unit', text: t('kcal') }),
      el('span', {
        class: `ring__caption${over ? ' ring__caption--over' : ''}`,
        text: over ? t('kcal_over') : t('kcal_left'),
      })
    )
  );
}

/** A labelled horizontal progress bar, used for macros and badge progress. */
export function progressBar(label, value, target, unit = 'g') {
  const ratio = target > 0 ? Math.min(value / target, 1) : 0;
  return el(
    'div',
    { class: 'macro' },
    el(
      'div',
      { class: 'macro__head' },
      el('span', { class: 'macro__label', text: label }),
      el('span', { class: 'macro__value', text: `${num(value)} / ${num(target)} ${unit}` })
    ),
    el(
      'div',
      {
        class: 'macro__track',
        role: 'progressbar',
        'aria-valuenow': Math.round(value),
        'aria-valuemin': '0',
        'aria-valuemax': Math.round(target),
        'aria-label': label,
      },
      el('div', { class: 'macro__fill', style: `width:${(ratio * 100).toFixed(1)}%` })
    )
  );
}

// ---------------------------------------------------------------------------
// Charts
//
// Both charts are single-series on purpose: one entity, one hue, no legend box
// (the title names the series). Reference lines and the healthy band are
// annotations, always carrying a text label so nothing is communicated by
// colour alone.
// ---------------------------------------------------------------------------

const CHART = {
  width: 640,
  height: 240,
  padTop: 16,
  padRight: 16,
  padBottom: 28,
  padLeft: 44,
};

function scaleFns(points, yMin, yMax) {
  const innerW = CHART.width - CHART.padLeft - CHART.padRight;
  const innerH = CHART.height - CHART.padTop - CHART.padBottom;
  const span = Math.max(yMax - yMin, 0.0001);
  return {
    x: (i, n) => CHART.padLeft + (n <= 1 ? innerW / 2 : (i / (n - 1)) * innerW),
    y: (v) => CHART.padTop + innerH - ((v - yMin) / span) * innerH,
    innerW,
    innerH,
  };
}

/** "Nice" tick values for an axis. */
function ticks(min, max, count = 4) {
  const span = max - min;
  if (span <= 0) return [min];
  const raw = span / count;
  const mag = Math.pow(10, Math.floor(Math.log10(raw)));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw) || mag * 10;
  const out = [];
  for (let v = Math.ceil(min / step) * step; v <= max + 1e-9; v += step) out.push(v);
  return out;
}

/**
 * Weight over time.
 * Raw weigh-ins are drawn as recessive dots; the 7-day trend is the emphasised
 * line, because day-to-day weight is mostly water and the trend is the signal.
 */
export function weightChart({ points, trend, healthyLow, healthyHigh, target }) {
  if (!points.length) {
    return el('p', { class: 'empty', text: t('weight_no_data') });
  }

  const values = points.map((p) => p.weight);
  const candidates = [...values, ...trend.map((p) => p.weight)];
  if (target) candidates.push(target);
  if (healthyHigh) candidates.push(healthyHigh);

  let lo = Math.min(...candidates);
  let hi = Math.max(...candidates);
  const pad = Math.max((hi - lo) * 0.12, 1);
  lo -= pad;
  hi += pad;

  const s = scaleFns(points, lo, hi);
  const n = points.length;
  const yTicks = ticks(lo, hi, 4);

  const line = (list) =>
    list
      .map((p, i) => `${i === 0 ? 'M' : 'L'}${s.x(i, list.length).toFixed(1)},${s.y(p.weight).toFixed(1)}`)
      .join(' ');

  const bandTop = healthyHigh ? s.y(Math.min(healthyHigh, hi)) : null;
  const bandBottom = healthyLow ? s.y(Math.max(healthyLow, lo)) : null;

  const chart = svg(
    'svg',
    {
      viewBox: `0 0 ${CHART.width} ${CHART.height}`,
      class: 'chart',
      role: 'img',
      'aria-label': `${t('chart_weight')}: ${num(values[values.length - 1], 1)} kg`,
    },

    // Healthy weight band — an annotation, labelled in text below the chart.
    bandTop !== null && bandBottom !== null && bandBottom > bandTop
      ? svg('rect', {
          x: CHART.padLeft,
          y: bandTop,
          width: s.innerW,
          height: bandBottom - bandTop,
          class: 'chart__band',
        })
      : null,

    // Recessive horizontal gridlines.
    yTicks.map((v) =>
      svg('line', {
        x1: CHART.padLeft, x2: CHART.width - CHART.padRight,
        y1: s.y(v), y2: s.y(v), class: 'chart__grid',
      })
    ),
    yTicks.map((v) =>
      svg('text', {
        x: CHART.padLeft - 8, y: s.y(v) + 4,
        class: 'chart__tick', 'text-anchor': 'end',
      }, num(v, 0))
    ),

    // Target weight reference line.
    target
      ? svg('line', {
          x1: CHART.padLeft, x2: CHART.width - CHART.padRight,
          y1: s.y(target), y2: s.y(target), class: 'chart__ref',
        })
      : null,
    target
      ? svg('text', {
          x: CHART.width - CHART.padRight, y: s.y(target) - 6,
          class: 'chart__ref-label', 'text-anchor': 'end',
        }, `${t('weight_target')} ${num(target, 1)} kg`)
      : null,

    // Raw weigh-ins, recessive.
    points.map((p, i) =>
      svg('circle', {
        cx: s.x(i, n), cy: s.y(p.weight), r: 3, class: 'chart__dot',
      })
    ),

    // The trend line carries the message.
    trend.length > 1 ? svg('path', { d: line(trend), class: 'chart__line' }) : null,

    // Latest value gets a direct label instead of labelling every point.
    svg('circle', {
      cx: s.x(n - 1, n), cy: s.y(values[n - 1]), r: 5, class: 'chart__last',
    })
  );

  const first = points[0];
  const last = points[points.length - 1];

  return el(
    'div',
    { class: 'chart-wrap' },
    chart,
    el(
      'div',
      { class: 'chart-axis' },
      el('span', { text: shortDate(first.date) }),
      el('span', { text: shortDate(last.date) })
    ),
    el(
      'div',
      { class: 'chart-key' },
      el('span', { class: 'chart-key__item' },
        el('i', { class: 'dot dot--line' }), t('weight_trend')),
      healthyLow && healthyHigh
        ? el('span', { class: 'chart-key__item' },
            el('i', { class: 'dot dot--band' }),
            t('weight_healthy_range', { low: num(healthyLow, 1), high: num(healthyHigh, 1) }))
        : null
    )
  );
}

/**
 * Daily calories against the target.
 * Bars are one hue; the target is a dashed reference line with a text label, so
 * "above target" is never read from colour alone.
 */
export function kcalChart(days, target) {
  if (!days.length) {
    return el('p', { class: 'empty', text: t('nothing_yet') });
  }

  const values = days.map((d) => d.kcal);
  const hi = Math.max(...values, target || 0) * 1.15;
  const s = scaleFns(days, 0, hi);
  const yTicks = ticks(0, hi, 3);

  const n = days.length;
  // A 2px gap between bars keeps adjacent days from reading as one block.
  const slot = s.innerW / n;
  const barW = Math.max(Math.min(slot - 2, 28), 2);

  const chart = svg(
    'svg',
    {
      viewBox: `0 0 ${CHART.width} ${CHART.height}`,
      class: 'chart',
      role: 'img',
      'aria-label': t('chart_kcal'),
    },
    yTicks.map((v) =>
      svg('line', {
        x1: CHART.padLeft, x2: CHART.width - CHART.padRight,
        y1: s.y(v), y2: s.y(v), class: 'chart__grid',
      })
    ),
    yTicks.map((v) =>
      svg('text', {
        x: CHART.padLeft - 8, y: s.y(v) + 4,
        class: 'chart__tick', 'text-anchor': 'end',
      }, num(v, 0))
    ),

    days.map((d, i) => {
      const x = CHART.padLeft + i * slot + (slot - barW) / 2;
      const y = s.y(d.kcal);
      const h = Math.max(CHART.padTop + s.innerH - y, 0);
      return svg('rect', {
        x, y, width: barW, height: h,
        rx: Math.min(4, barW / 2),
        class: 'chart__bar',
      }, svg('title', {}, `${shortDate(d.day)}: ${num(d.kcal)} kcal`));
    }),

    target
      ? svg('line', {
          x1: CHART.padLeft, x2: CHART.width - CHART.padRight,
          y1: s.y(target), y2: s.y(target), class: 'chart__ref',
        })
      : null,
    target
      ? svg('text', {
          x: CHART.width - CHART.padRight, y: s.y(target) - 6,
          class: 'chart__ref-label', 'text-anchor': 'end',
        }, `${t('plan_target')} ${num(target)} kcal`)
      : null
  );

  return el(
    'div',
    { class: 'chart-wrap' },
    chart,
    el(
      'div',
      { class: 'chart-axis' },
      el('span', { text: shortDate(days[0].day) }),
      el('span', { text: shortDate(days[days.length - 1].day) })
    )
  );
}

/** Escape user-supplied text for safe insertion into markup. */
export function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])
  );
}
