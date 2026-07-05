const fmt = new Intl.NumberFormat();
const compact = new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 1 });
const pct = new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 1 });
const usd = new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 4 });

async function getJSON(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function byId(id) {
  return document.getElementById(id);
}

function text(id, value) {
  byId(id).textContent = value;
}

function setRangeLabels(label) {
  for (const el of document.querySelectorAll(".range-label")) {
    el.textContent = label;
  }
}

function selectedFilters() {
  const rangeValue = byId("range").value;
  const source = byId("source").value;
  const params = new URLSearchParams();
  let label = `Last ${rangeValue} days`;
  if (rangeValue === "all") {
    params.set("range", "all");
    label = "All time";
  } else {
    params.set("days", rangeValue);
  }
  if (source !== "all") {
    params.set("source", source);
  }
  return { query: params.toString(), label };
}

function apiPath(path, query) {
  return query ? `${path}?${query}` : path;
}

function sourceLabel(source) {
  if (source === "claude-code") return "Claude Code";
  if (source === "codex") return "Codex";
  return source || "Unknown";
}

function sourceClass(source) {
  return `source-pill ${source === "claude-code" ? "claude-code" : ""}`;
}

function formatDuration(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return "0 ms";
  if (ms >= 1000) return `${(ms / 1000).toFixed(ms >= 10000 ? 0 : 1)} s`;
  return `${Math.round(ms)} ms`;
}

function setStatus(kind, label) {
  const pill = byId("healthState");
  pill.className = `status-pill ${kind}`;
  pill.textContent = label;
}

function emptyRow(colspan, message) {
  const tr = document.createElement("tr");
  const td = document.createElement("td");
  td.colSpan = colspan;
  td.className = "empty-row";
  td.textContent = message;
  tr.appendChild(td);
  return tr;
}

function renderChart(series) {
  const root = byId("chart");
  root.innerHTML = "";
  root.classList.toggle("chart-scroll", series.length > 14);

  const max = Math.max(1, ...series.map((p) => p.totalTokens));
  const peak = series.reduce((best, point) => (point.totalTokens > best.totalTokens ? point : best), { totalTokens: 0 });
  text("peakDay", peak.totalTokens > 0 ? `Peak: ${peak.bucket.slice(5).replace("-", "/")} / ${compact.format(peak.totalTokens)}` : "Peak: none");
  text("chartSubtitle", `${series.length} daily buckets in the selected range`);

  for (const point of series) {
    const item = document.createElement("div");
    item.className = "chart-item";

    const track = document.createElement("div");
    track.className = "bar-track";

    const bar = document.createElement("div");
    bar.className = point.totalTokens > 0 ? "bar" : "bar empty";
    bar.style.height = point.totalTokens > 0 ? `${Math.max(8, (point.totalTokens / max) * 100)}%` : "0";
    bar.title = `${point.bucket}: ${fmt.format(point.totalTokens)} tokens, ${fmt.format(point.requests)} requests, ${usd.format(point.estimatedCostUsd)}`;

    const label = document.createElement("span");
    label.className = "chart-label";
    const date = document.createElement("strong");
    date.textContent = point.bucket.slice(5).replace("-", "/");
    const value = document.createTextNode(compact.format(point.totalTokens));
    label.append(date, value);

    track.appendChild(bar);
    item.append(track, label);
    root.appendChild(item);
  }
}

function renderModels(rows) {
  const body = byId("models");
  body.innerHTML = "";
  if (rows.length === 0) {
    body.appendChild(emptyRow(4, "No telemetry in this range"));
    return;
  }

  const maxTokens = Math.max(1, ...rows.map((row) => row.totalTokens));
  for (const row of rows) {
    const tr = document.createElement("tr");

    const source = document.createElement("td");
    const sourcePill = document.createElement("span");
    sourcePill.className = sourceClass(row.source);
    sourcePill.textContent = sourceLabel(row.source);
    source.appendChild(sourcePill);

    const model = document.createElement("td");
    const modelName = document.createElement("span");
    modelName.className = "model-name";
    modelName.textContent = row.model || "unknown";
    const events = document.createElement("span");
    events.className = "cell-subtext";
    events.textContent = `${fmt.format(row.events)} events`;
    model.append(modelName, events);

    const tokens = document.createElement("td");
    tokens.append(fmt.format(row.totalTokens), inlineBar(row.totalTokens / maxTokens));

    const cost = document.createElement("td");
    cost.textContent = usd.format(row.estimatedCostUsd);

    tr.append(source, model, tokens, cost);
    body.appendChild(tr);
  }
}

function renderSources(rows) {
  const body = byId("sources");
  body.innerHTML = "";
  if (rows.length === 0) {
    body.appendChild(emptyRow(4, "No telemetry in this range"));
    return;
  }

  const maxTokens = Math.max(1, ...rows.map((row) => row.totalTokens));
  for (const row of rows) {
    const tr = document.createElement("tr");

    const source = document.createElement("td");
    const sourcePill = document.createElement("span");
    sourcePill.className = sourceClass(row.source);
    sourcePill.textContent = sourceLabel(row.source);
    source.appendChild(sourcePill);

    const requests = document.createElement("td");
    requests.textContent = fmt.format(row.requests);

    const tokens = document.createElement("td");
    tokens.append(fmt.format(row.totalTokens), inlineBar(row.totalTokens / maxTokens));

    const cost = document.createElement("td");
    cost.textContent = usd.format(row.estimatedCostUsd);

    tr.append(source, requests, tokens, cost);
    body.appendChild(tr);
  }
}

function inlineBar(ratio) {
  const track = document.createElement("span");
  track.className = "inline-bar";
  const fill = document.createElement("span");
  fill.style.width = `${Math.max(2, Math.min(100, ratio * 100))}%`;
  track.appendChild(fill);
  return track;
}

function renderTokenMix(summary) {
  const root = byId("tokenMix");
  root.innerHTML = "";
  const rows = [
    ["Input", summary.inputTokens, "mix-input"],
    ["Cached input", summary.cachedInputTokens, "mix-cached"],
    ["Cache creation", summary.cacheCreationTokens, "mix-cache-creation"],
    ["Output", summary.outputTokens, "mix-output"],
    ["Reasoning", summary.reasoningOutputTokens, "mix-reasoning"],
  ];
  const total = Math.max(0, rows.reduce((sum, [, value]) => sum + value, 0));

  for (const [label, value, fillClass] of rows) {
    const ratio = total > 0 ? value / total : 0;
    const row = document.createElement("div");
    row.className = "mix-row";

    const head = document.createElement("div");
    head.className = "mix-head";
    const name = document.createElement("span");
    name.textContent = label;
    const amount = document.createElement("span");
    amount.className = "mix-value";
    amount.textContent = `${fmt.format(value)} / ${pct.format(ratio)}`;
    head.append(name, amount);

    const track = document.createElement("div");
    track.className = "mix-track";
    const fill = document.createElement("span");
    fill.className = `mix-fill ${fillClass}`;
    fill.style.width = `${Math.max(0, Math.min(100, ratio * 100))}%`;
    track.appendChild(fill);

    row.append(head, track);
    root.appendChild(row);
  }
}

function renderSummary(summary) {
  const failureRate = summary.requests > 0 ? summary.failures / summary.requests : 0;
  const costPerRequest = summary.requests > 0 ? summary.estimatedCostUsd / summary.requests : 0;

  text("cost", usd.format(summary.estimatedCostUsd));
  text("tokens", fmt.format(summary.totalTokens));
  text("requests", fmt.format(summary.requests));
  text("failures", `${fmt.format(summary.failures)} failures`);
  text("failureRate", pct.format(failureRate));
  text("avgDuration", formatDuration(summary.avgDurationMs));
  text("costPerRequest", usd.format(costPerRequest));
  renderTokenMix(summary);
}

function renderHealth(health) {
  text("lastEvent", health.lastEventAt ? new Date(health.lastEventAt).toLocaleString() : "Never");
  text("accepted", fmt.format(health.acceptedEvents));
  text("dropped", fmt.format(health.droppedContentFields));
  setStatus(health.acceptedEvents > 0 ? "" : "loading", health.acceptedEvents > 0 ? "Receiving" : "Waiting");
}

async function refresh() {
  const filters = selectedFilters();
  setRangeLabels(filters.label);
  const refreshButton = byId("refresh");
  refreshButton.disabled = true;
  refreshButton.textContent = "Refreshing";
  setStatus("loading", "Refreshing");

  try {
    const [summary, series, models, sources, health] = await Promise.all([
      getJSON(apiPath("/api/summary", filters.query)),
      getJSON(apiPath("/api/series", filters.query)),
      getJSON(apiPath("/api/breakdown/models", filters.query)),
      getJSON(apiPath("/api/breakdown/sources", filters.query)),
      getJSON("/api/health"),
    ]);

    renderSummary(summary);
    renderChart(series);
    renderModels(models);
    renderSources(sources);
    renderHealth(health);
    text("lastUpdated", `Updated ${new Date().toLocaleTimeString()}`);
  } catch (err) {
    console.error(err);
    setStatus("error", "Error");
    text("lastUpdated", err instanceof Error ? err.message : "Refresh failed");
    byId("chart").innerHTML = '<div class="error-message">Could not load dashboard data.</div>';
  } finally {
    refreshButton.disabled = false;
    refreshButton.textContent = "Refresh";
  }
}

function refreshDashboard() {
  refresh().catch(console.error);
}

function refreshWhenVisible() {
  if (document.visibilityState === "visible") {
    refreshDashboard();
  }
}

byId("refresh").addEventListener("click", refreshDashboard);
byId("range").addEventListener("change", refreshDashboard);
byId("source").addEventListener("change", refreshDashboard);
document.addEventListener("visibilitychange", refreshWhenVisible);
refreshDashboard();
