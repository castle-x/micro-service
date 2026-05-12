# Local Observability

This directory contains the local OpenTelemetry observability stack for development.

## Stack

```text
services/*
  -> OTLP gRPC localhost:4317 or OTLP HTTP localhost:4318
  -> OpenTelemetry Collector
  -> OpenObserve
```

OpenObserve is the first local backend for traces, metrics, and logs. Prometheus, Grafana, Jaeger, and Loki are intentionally not required for the first version.

## Start

```bash
make dev-start
```

`dev-start` starts local infrastructure, OpenObserve, the OpenTelemetry Collector, backend services, and the web dev server. It enables OTel for backend services by default.

To start only the observability stack:

```bash
make obs-up
```

OpenObserve UI:

```text
http://localhost:5080
```

Default local login:

```text
Email: root@example.com
Password: Complexpass123
```

## Service Configuration

Development services use these values by default when started through `make dev-start`, `make dev-restart`, `make model-start`, or `make model-restart`:

```bash
OTEL_ENABLED=true
OTEL_ENDPOINT=localhost:4317
OTEL_PROTOCOL=grpc
OTEL_ENVIRONMENT=local
OTEL_INSECURE=true
```

The application still only knows about OTLP. OpenObserve-specific credentials stay in the Collector and query tooling.

## Query Helpers

```bash
make obs-trace TRACE_ID=<trace-id>
make obs-logs TRACE_ID=<trace-id>
make obs-metrics SERVICE=edge-api
make obs-errors SINCE=15m
```

The query helpers output JSON so AI agents can read them without scraping the OpenObserve UI.

## Stop

```bash
make dev-stop
```

`dev-stop` stops backend and frontend processes only. It leaves Docker infrastructure and observability containers running for faster restarts.

To stop the observability stack:

```bash
make obs-down
```

Use `make obs-ps` to inspect container status.
