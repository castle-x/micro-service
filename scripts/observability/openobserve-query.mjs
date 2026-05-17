#!/usr/bin/env node

const DEFAULT_AUTH_HEADER = "Basic cm9vdEBleGFtcGxlLmNvbTpDb21wbGV4cGFzczEyMw==";

export function buildConfig(env = process.env) {
  return {
    baseUrl: trimTrailingSlash(env.OPENOBSERVE_URL || "http://localhost:5080"),
    org: env.OPENOBSERVE_ORG || "default",
    stream: env.OPENOBSERVE_STREAM || "default",
    authHeader: env.OPENOBSERVE_AUTH_HEADER || DEFAULT_AUTH_HEADER,
  };
}

export function parseSinceToMillis(since = "15m") {
  const match = /^(\d+)([mh])$/.exec(String(since).trim());
  if (!match) {
    throw new Error("SINCE must look like 15m or 2h");
  }
  const value = Number(match[1]);
  const unit = match[2];
  return value * (unit === "h" ? 60 * 60 * 1000 : 60 * 1000);
}

export function buildLatestTracesUrl(cfg, opts) {
  const url = new URL(
    `/api/${encodeURIComponent(cfg.org)}/${encodeURIComponent(cfg.stream)}/traces/latest`,
    cfg.baseUrl,
  );
  url.searchParams.set("start_time", String(opts.startMicros));
  url.searchParams.set("end_time", String(opts.endMicros));
  url.searchParams.set("from", "0");
  url.searchParams.set("size", String(opts.size || 10));
  if (opts.traceId) {
    url.searchParams.set("filter", `trace_id='${escapeSqlLiteral(opts.traceId)}'`);
  }
  return url;
}

export function buildSearchUrl(cfg, streamType) {
  const url = new URL(`/api/${encodeURIComponent(cfg.org)}/_search`, cfg.baseUrl);
  if (streamType) {
    url.searchParams.set("type", streamType);
  }
  return url;
}

export function buildSearchBody({ sql, startMicros, endMicros, size = 25 }) {
  return {
    query: {
      sql,
      start_time: startMicros,
      end_time: endMicros,
      from: 0,
      size,
    },
    search_type: "ui",
    timeout: 0,
  };
}

export function isMissingStreamError(err) {
  return err instanceof Error && /Search stream not found/.test(err.message);
}

export function isTraceOverviewUnavailableError(err) {
  return isMissingStreamError(err) || (err instanceof Error && /Search field not found/.test(err.message));
}

export function summarizeTrace(traceId, overview, hits) {
  const latest = overview?.hits?.[0] || null;
  const rootSpan = hits[0] || latest?.first_event || null;
  const slowest = [...hits].sort((a, b) => Number(b.duration || 0) - Number(a.duration || 0))[0] || null;
  const errors = hits.filter((span) => String(span.span_status || span.status || "").toLowerCase() === "error");
  const overviewSpanCount = Array.isArray(latest?.spans)
    ? latest.spans.reduce((sum, count) => sum + Number(count || 0), 0)
    : 0;
  return {
    type: "trace",
    trace_id: traceId,
    overview,
    span_count: hits.length || overviewSpanCount,
    root_span: rootSpan,
    error_spans: errors,
    slowest_span: slowest,
  };
}

async function main(argv = process.argv.slice(2), env = process.env) {
  const command = argv[0];
  const cfg = buildConfig(env);
  const now = Date.now();
  const sinceMs = parseSinceToMillis(env.SINCE || "15m");
  const startMicros = (now - sinceMs) * 1000;
  const endMicros = now * 1000;

  switch (command) {
    case "trace":
      await printTrace(cfg, env.TRACE_ID, startMicros, endMicros);
      break;
    case "logs":
      await printLogs(cfg, env.TRACE_ID, startMicros, endMicros);
      break;
    case "metrics":
      await printMetrics(cfg, env.SERVICE, startMicros, endMicros);
      break;
    case "errors":
      await printErrors(cfg, startMicros, endMicros);
      break;
    default:
      usage();
      process.exitCode = 2;
  }
}

async function printTrace(cfg, traceId, startMicros, endMicros) {
  if (!traceId) {
    throw new Error("TRACE_ID is required");
  }
  const overviewUrl = buildLatestTracesUrl(cfg, {
    traceId,
    startMicros,
    endMicros,
    size: 5,
  });
  let overview = null;
  try {
    overview = await getJson(overviewUrl, cfg);
  } catch (err) {
    if (!isTraceOverviewUnavailableError(err)) {
      throw err;
    }
  }
  const spans = await search(cfg, {
    sql: `SELECT * FROM ${cfg.stream} WHERE trace_id = '${escapeSqlLiteral(traceId)}' ORDER BY start_time ASC`,
    startMicros,
    endMicros,
    size: -1,
    streamType: "traces",
  });
  printJson(summarizeTrace(traceId, overview, spans.hits || []));
}

async function printLogs(cfg, traceId, startMicros, endMicros) {
  if (!traceId) {
    throw new Error("TRACE_ID is required");
  }
  const result = await search(cfg, {
    sql: `SELECT * FROM ${cfg.stream} WHERE trace_id = '${escapeSqlLiteral(traceId)}' ORDER BY _timestamp ASC`,
    startMicros,
    endMicros,
    size: 50,
  });
  printJson({
    type: "logs",
    trace_id: traceId,
    status: (result.hits || []).length === 0 ? "no_logs_found_or_logs_not_ingested" : "ok",
    hits: result.hits || [],
  });
}

async function printMetrics(cfg, service, startMicros, endMicros) {
  if (!service) {
    throw new Error("SERVICE is required");
  }
  const result = await search(cfg, {
    sql: `SELECT service_name, COUNT(*) AS samples FROM ${cfg.stream} WHERE service_name = '${escapeSqlLiteral(service)}' GROUP BY service_name`,
    startMicros,
    endMicros,
    size: 10,
  });
  printJson({
    type: "metrics",
    service,
    status: "raw_openobserve_query",
    hits: result.hits || [],
  });
}

async function printErrors(cfg, startMicros, endMicros) {
  const result = await search(cfg, {
    sql: `SELECT trace_id, service_name, operation_name, span_status, _timestamp FROM ${cfg.stream} WHERE span_status = 'ERROR' ORDER BY _timestamp DESC`,
    startMicros,
    endMicros,
    size: 25,
    streamType: "traces",
  });
  printJson({
    type: "errors",
    hits: result.hits || [],
  });
}

async function search(cfg, { sql, startMicros, endMicros, size, streamType }) {
  const url = buildSearchUrl(cfg, streamType);
  try {
    return await postJson(url, cfg, buildSearchBody({ sql, startMicros, endMicros, size }));
  } catch (err) {
    if (isMissingStreamError(err)) {
      return { hits: [], status: "stream_not_found" };
    }
    throw err;
  }
}

async function getJson(url, cfg) {
  const res = await fetch(url, { headers: authHeaders(cfg) });
  return readResponse(res);
}

async function postJson(url, cfg, body) {
  const res = await fetch(url, {
    method: "POST",
    headers: { ...authHeaders(cfg), "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  return readResponse(res);
}

async function readResponse(res) {
  const text = await res.text();
  let body;
  try {
    body = text ? JSON.parse(text) : {};
  } catch {
    body = { raw: text };
  }
  if (!res.ok) {
    throw new Error(`OpenObserve API ${res.status}: ${JSON.stringify(body)}`);
  }
  return body;
}

function authHeaders(cfg) {
  return { authorization: cfg.authHeader };
}

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, "");
}

function escapeSqlLiteral(value) {
  return String(value).replaceAll("'", "''");
}

function printJson(value) {
  process.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
}

function usage() {
  process.stderr.write(`Usage:
  TRACE_ID=<id> node scripts/observability/openobserve-query.mjs trace
  TRACE_ID=<id> node scripts/observability/openobserve-query.mjs logs
  SERVICE=edge-api node scripts/observability/openobserve-query.mjs metrics
  SINCE=15m node scripts/observability/openobserve-query.mjs errors
`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    process.stderr.write(`${err.message}\n`);
    process.exitCode = 1;
  });
}
