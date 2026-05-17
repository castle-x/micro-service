import assert from "node:assert/strict";
import test from "node:test";

import {
  buildConfig,
  buildLatestTracesUrl,
  buildSearchUrl,
  buildSearchBody,
  isMissingStreamError,
  isTraceOverviewUnavailableError,
  parseSinceToMillis,
  summarizeTrace,
} from "./openobserve-query.mjs";

test("buildConfig uses local OpenObserve defaults", () => {
  const cfg = buildConfig({});

  assert.equal(cfg.baseUrl, "http://localhost:5080");
  assert.equal(cfg.org, "default");
  assert.equal(cfg.stream, "default");
  assert.equal(cfg.authHeader, "Basic cm9vdEBleGFtcGxlLmNvbTpDb21wbGV4cGFzczEyMw==");
});

test("buildLatestTracesUrl builds the OpenObserve traces latest endpoint", () => {
  const cfg = buildConfig({
    OPENOBSERVE_URL: "http://localhost:5080/",
    OPENOBSERVE_ORG: "micro",
    OPENOBSERVE_STREAM: "otel",
  });

  const url = buildLatestTracesUrl(cfg, {
    traceId: "abc123",
    startMicros: 1000,
    endMicros: 2000,
    size: 5,
  });

  assert.equal(
    url.toString(),
    "http://localhost:5080/api/micro/otel/traces/latest?start_time=1000&end_time=2000&from=0&size=5&filter=trace_id%3D%27abc123%27",
  );
});

test("buildSearchUrl can target OpenObserve trace streams", () => {
  const cfg = buildConfig({
    OPENOBSERVE_URL: "http://localhost:5080/",
    OPENOBSERVE_ORG: "micro",
  });

  assert.equal(
    buildSearchUrl(cfg, "traces").toString(),
    "http://localhost:5080/api/micro/_search?type=traces",
  );
});

test("buildSearchBody emits a bounded SQL search request", () => {
  const body = buildSearchBody({
    stream: "default",
    sql: "SELECT * FROM default WHERE trace_id = 'abc123' ORDER BY _timestamp ASC",
    startMicros: 1000,
    endMicros: 2000,
    size: 25,
  });

  assert.deepEqual(body, {
    query: {
      sql: "SELECT * FROM default WHERE trace_id = 'abc123' ORDER BY _timestamp ASC",
      start_time: 1000,
      end_time: 2000,
      from: 0,
      size: 25,
    },
    search_type: "ui",
    timeout: 0,
  });
});

test("parseSinceToMillis supports minutes and hours", () => {
  assert.equal(parseSinceToMillis("15m"), 15 * 60 * 1000);
  assert.equal(parseSinceToMillis("2h"), 2 * 60 * 60 * 1000);
  assert.throws(() => parseSinceToMillis("yesterday"), /SINCE must look like/);
});

test("isMissingStreamError detects OpenObserve empty-stream search responses", () => {
  assert.equal(
    isMissingStreamError(
      new Error('OpenObserve API 400: {"code":20002,"message":"Search stream not found: default"}'),
    ),
    true,
  );
  assert.equal(isMissingStreamError(new Error("OpenObserve API 500")), false);
});

test("isTraceOverviewUnavailableError treats latest-trace schema misses as non-fatal", () => {
  assert.equal(
    isTraceOverviewUnavailableError(
      new Error('OpenObserve API 400: {"code":20004,"message":"Search field not found: Schema error"}'),
    ),
    true,
  );
  assert.equal(isTraceOverviewUnavailableError(new Error("OpenObserve API 500")), false);
});

test("summarizeTrace falls back to latest trace overview when span search is empty", () => {
  const overview = {
    hits: [
      {
        first_event: {
          operation_name: "HTTP POST /api/v1/auth/login",
          service_name: "edge-api",
        },
        spans: [4, 2],
      },
    ],
  };

  const summary = summarizeTrace("trace-123", overview, []);

  assert.equal(summary.trace_id, "trace-123");
  assert.equal(summary.span_count, 6);
  assert.deepEqual(summary.root_span, overview.hits[0].first_event);
});
