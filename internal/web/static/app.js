const fmt = new Intl.NumberFormat();
const usd = new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 4 });

async function getJSON(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function text(id, value) {
  document.getElementById(id).textContent = value;
}

function renderChart(series) {
  const root = document.getElementById("chart");
  root.innerHTML = "";
  const max = Math.max(1, ...series.map((p) => p.totalTokens));
  for (const point of series) {
    const bar = document.createElement("div");
    bar.className = "bar";
    bar.style.height = `${Math.max(2, (point.totalTokens / max) * 100)}%`;
    bar.title = `${point.bucket}: ${fmt.format(point.totalTokens)} tokens`;
    root.appendChild(bar);
  }
}

function renderModels(rows) {
  const body = document.getElementById("models");
  body.innerHTML = "";
  for (const row of rows) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td></td><td>${fmt.format(row.totalTokens)}</td><td>${usd.format(row.estimatedCostUsd)}</td>`;
    tr.firstChild.textContent = row.model;
    body.appendChild(tr);
  }
  if (rows.length === 0) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td colspan="3">No telemetry yet</td>`;
    body.appendChild(tr);
  }
}

async function refresh() {
  const [summary, series, models, health] = await Promise.all([
    getJSON("/api/summary"),
    getJSON("/api/series"),
    getJSON("/api/breakdown/models"),
    getJSON("/api/health"),
  ]);

  text("cost", usd.format(summary.estimatedCostUsd));
  text("tokens", fmt.format(summary.totalTokens));
  text("requests", fmt.format(summary.requests));
  text("failures", fmt.format(summary.failures));
  text("input", fmt.format(summary.inputTokens));
  text("cached", fmt.format(summary.cachedInputTokens));
  text("output", fmt.format(summary.outputTokens));
  text("reasoning", fmt.format(summary.reasoningOutputTokens));
  text("lastEvent", health.lastEventAt ? new Date(health.lastEventAt).toLocaleString() : "Never");
  text("accepted", fmt.format(health.acceptedEvents));
  text("dropped", fmt.format(health.droppedContentFields));

  renderChart(series);
  renderModels(models);
}

document.getElementById("refresh").addEventListener("click", () => refresh().catch(console.error));
refresh().catch(console.error);
