import { writable } from 'svelte/store';
import { latestPacket } from './ws.js';
import { RULES } from './rules.js';

// ─── Evaluator factory ────────────────────────────────────────────────────────
// Each instance has independent state so playback and live don't interfere.
export function createRuleEvaluator() {
  const state = new Map(
    RULES.map(r => [r.id, { violationStart: null, latched: false, firedAt: null }])
  );

  // Returns { active: Map, newFires: [{rule, firedAt}], newClears: [{rule, clearedAt}] }
  function evaluate(values, nowMs) {
    const active = new Map();
    const newFires = [];
    const newClears = [];

    for (const rule of RULES) {
      const st = state.get(rule.id);
      const triggered = rule.condition(values);

      if (triggered) {
        if (st.violationStart === null) st.violationStart = nowMs;
        const elapsed = nowMs - st.violationStart;
        const fired = rule.timing_ms === 0 || elapsed >= rule.timing_ms;

        if (fired) {
          if (st.firedAt === null) {
            st.firedAt = nowMs;
            st.latched = rule.latch;
            newFires.push({ rule, firedAt: nowMs });
          }
          active.set(rule.id, { rule, firedAt: st.firedAt, latched: st.latched });
        }
      } else if (st.latched) {
        const cleared = rule.latch_clear_fn ? rule.latch_clear_fn(values) : true;
        if (cleared) {
          newClears.push({ rule, clearedAt: nowMs });
          st.latched = false;
          st.violationStart = null;
          st.firedAt = null;
        } else {
          active.set(rule.id, { rule, firedAt: st.firedAt, latched: true });
        }
      } else {
        if (st.firedAt !== null) newClears.push({ rule, clearedAt: nowMs });
        st.violationStart = null;
        st.firedAt = null;
      }
    }

    return { active, newFires, newClears };
  }

  return { evaluate };
}

// ─── Live engine instance ─────────────────────────────────────────────────────
const liveEvaluator = createRuleEvaluator();

export const activeAlerts    = writable(new Map());
export const liveFaultEvents = writable([]); // { ts: number (seconds), rule }
export const alertHistory    = writable([]); // { rule, firedAt: ms, clearedAt: ms|null }

let _notifyPermission = typeof Notification !== 'undefined' ? Notification.permission : 'denied';

export function requestNotifications() {
  if (typeof Notification === 'undefined') return;
  Notification.requestPermission().then(p => { _notifyPermission = p; });
}

function notify(rule) {
  if (_notifyPermission !== 'granted') return;
  new Notification(`⚠ ${rule.rule_ref}: ${rule.title}`, {
    body: rule.description,
    tag: rule.id,
  });
}

latestPacket.subscribe($pkt => {
  if (!$pkt) return;
  const nowMs = $pkt.ts * 1000;
  const { active, newFires, newClears } = liveEvaluator.evaluate($pkt.data, nowMs);

  activeAlerts.set(active);

  if (newFires.length) {
    liveFaultEvents.update(evts => [
      ...evts,
      ...newFires.map(f => ({ ts: $pkt.ts, rule: f.rule })),
    ]);
    alertHistory.update(h => [
      ...newFires.map(f => ({ rule: f.rule, firedAt: f.firedAt, clearedAt: null })),
      ...h.slice(0, 49 - newFires.length),
    ]);
    for (const f of newFires) notify(f.rule);
  }

  if (newClears.length) {
    alertHistory.update(h =>
      h.map(e =>
        newClears.some(c => c.rule.id === e.rule.id) && e.clearedAt === null
          ? { ...e, clearedAt: nowMs }
          : e
      )
    );
  }
});
