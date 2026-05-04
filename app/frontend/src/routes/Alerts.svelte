<script>
  import { onDestroy } from 'svelte';
  import { activeAlerts, alertHistory, requestNotifications } from '../lib/ruleEngine.js';
  import { RULES } from '../lib/rules.js';

  let now = Date.now();
  const ticker = setInterval(() => { now = Date.now(); }, 250);
  onDestroy(() => clearInterval(ticker));

  let notifPermission = typeof Notification !== 'undefined' ? Notification.permission : 'denied';

  function handleEnableNotifications() {
    requestNotifications();
    setTimeout(() => {
      notifPermission = typeof Notification !== 'undefined' ? Notification.permission : 'denied';
    }, 500);
  }

  function fmtDuration(ms) {
    if (ms < 1000) return `${ms} ms`;
    if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
    return `${Math.floor(ms / 60_000)}m ${Math.floor((ms % 60_000) / 1000)}s`;
  }

  function fmtTime(ts) {
    return new Date(ts).toLocaleTimeString();
  }
</script>

<div class="alerts-page">
  <div class="header">
    <h2>Alerts</h2>
    <div class="header-actions">
      {#if notifPermission === 'default'}
        <button class="notif-btn" on:click={handleEnableNotifications}>
          Enable Notifications
        </button>
      {:else if notifPermission === 'granted'}
        <span class="notif-on">Notifications on</span>
      {/if}
    </div>
  </div>

  <!-- Rule status board -->
  <div class="board">
    {#each RULES as rule (rule.id)}
      {@const violation = $activeAlerts.get(rule.id)}
      {@const active = !!violation}
      <div
        class="rule-card"
        class:active-critical={active && rule.severity === 'critical'}
        class:active-warning={active && rule.severity === 'warning'}
        class:latched={violation?.latched}
      >
        <div class="card-top">
          <span class="ref-badge">{rule.rule_ref}</span>
          <span class="sev-dot sev-{rule.severity}"></span>
          <span class="status-chip" class:ok={!active} class:viol={active}>
            {#if violation?.latched}
              LATCHED
            {:else if active}
              ACTIVE
            {:else}
              OK
            {/if}
          </span>
        </div>
        <div class="card-title">{rule.title}</div>
        <div class="card-desc">{rule.description}</div>
        <div class="sensors">
          {#each rule.sensors as s}
            <span class="sensor-chip">{s}</span>
          {/each}
        </div>
        {#if active}
          <div class="card-duration">
            {#if violation.latched}
              Latched — clear by dropping APPS below 5%
            {:else}
              Active for {fmtDuration(now - violation.firedAt)}
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <!-- History log -->
  <div class="history-section">
    <h3>Alert History</h3>
    {#if $alertHistory.length === 0}
      <p class="no-history">No alerts this session.</p>
    {:else}
      <table class="history-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Ref</th>
            <th>Rule</th>
            <th>Severity</th>
            <th>Duration</th>
          </tr>
        </thead>
        <tbody>
          {#each $alertHistory as entry (entry.firedAt + entry.rule.id)}
            <tr class="hist-row sev-row-{entry.rule.severity}">
              <td class="mono">{fmtTime(entry.firedAt)}</td>
              <td><span class="ref-badge small">{entry.rule.rule_ref}</span></td>
              <td>{entry.rule.title}</td>
              <td><span class="sev-label sev-{entry.rule.severity}">{entry.rule.severity}</span></td>
              <td class="mono">
                {#if entry.clearedAt}
                  {fmtDuration(entry.clearedAt - entry.firedAt)}
                {:else}
                  <span class="ongoing">ongoing</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<style>
  .alerts-page {
    padding: 16px 20px;
    overflow-y: auto;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .header h2 { margin: 0; font-size: 1rem; color: #ccc; font-weight: 600; }

  .notif-btn {
    background: #1e1e38;
    border: 1px solid #4FC3F7;
    color: #4FC3F7;
    padding: 5px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.78rem;
  }
  .notif-on { font-size: 0.78rem; color: #81C784; }

  /* Board */
  .board {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 12px;
  }

  .rule-card {
    background: #141428;
    border: 1px solid #2a2a4a;
    border-radius: 6px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    transition: border-color 0.2s, background 0.2s;
  }

  .rule-card.active-critical {
    border-color: #E57373;
    background: #1e0f0f;
    animation: pulse-red 1.4s ease-in-out infinite;
  }
  .rule-card.active-warning {
    border-color: #FFB74D;
    background: #1e1600;
  }
  .rule-card.latched {
    animation: none;
    border-color: #FFB74D;
    background: #1e1200;
  }

  @keyframes pulse-red {
    0%, 100% { box-shadow: 0 0 0 0 rgba(229, 115, 115, 0); }
    50%       { box-shadow: 0 0 8px 2px rgba(229, 115, 115, 0.35); }
  }

  .card-top {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .ref-badge {
    font-size: 0.68rem;
    padding: 1px 6px;
    background: #222240;
    border: 1px solid #3a3a5c;
    border-radius: 3px;
    color: #aaa;
    font-family: monospace;
  }
  .ref-badge.small { font-size: 0.65rem; }

  .sev-dot {
    width: 8px; height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .sev-dot.sev-critical { background: #E57373; }
  .sev-dot.sev-warning  { background: #FFB74D; }

  .status-chip {
    margin-left: auto;
    font-size: 0.68rem;
    font-weight: 700;
    padding: 1px 7px;
    border-radius: 3px;
    letter-spacing: 0.05em;
  }
  .status-chip.ok   { background: #1a2e1a; color: #81C784; }
  .status-chip.viol { background: #2e1a1a; color: #E57373; }

  .card-title {
    font-size: 0.85rem;
    font-weight: 600;
    color: #ddd;
  }
  .card-desc {
    font-size: 0.75rem;
    color: #777;
    line-height: 1.4;
  }

  .sensors {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .sensor-chip {
    font-size: 0.65rem;
    padding: 1px 5px;
    background: #1e1e3a;
    border: 1px solid #2a2a4a;
    border-radius: 3px;
    color: #4FC3F7;
    font-family: monospace;
  }

  .card-duration {
    font-size: 0.72rem;
    color: #E57373;
    margin-top: 2px;
  }

  /* History */
  .history-section h3 {
    margin: 0 0 10px;
    font-size: 0.85rem;
    color: #888;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .no-history { font-size: 0.8rem; color: #555; margin: 0; }

  .history-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
  }
  .history-table th {
    text-align: left;
    padding: 5px 10px;
    color: #555;
    border-bottom: 1px solid #2a2a4a;
    font-weight: 600;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .history-table td {
    padding: 6px 10px;
    border-bottom: 1px solid #1a1a2e;
    color: #ccc;
    vertical-align: middle;
  }
  .hist-row:hover td { background: #1a1a2e; }

  .sev-label {
    font-size: 0.68rem;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 3px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .sev-label.sev-critical { background: #2e1a1a; color: #E57373; }
  .sev-label.sev-warning  { background: #2e2200; color: #FFB74D; }

  .mono { font-family: monospace; }
  .ongoing { color: #E57373; font-style: italic; }
</style>
