# Local Environment Files

Copy the examples in this directory to matching `.env` files and fill local
values before starting the development stack.

## First-Time Setup

Copy each example to the matching local file, then replace placeholder values.

```bash
cp deployments/env/infra.env.example deployments/env/infra.env
cp deployments/env/observability.env.example deployments/env/observability.env
cp deployments/env/asset.env.example deployments/env/asset.env
cp deployments/env/llm.env.example deployments/env/llm.env
cp deployments/env/secrets.env.example deployments/env/secrets.env
```

`LLM_ENCRYPT_KEY` lives in `deployments/env/llm.env` and is required before any
LLM service start or restart. Generate a stable local value with:

```bash
openssl rand -base64 32
```

Keep this value stable for the same local database. Changing it requires
re-entering stored LLM provider API keys.

## Load Order

Files are loaded in this order. Later files override earlier files.

1. `.env` at the repo root, for legacy compatibility only.
2. `deployments/env/infra.env`
3. `deployments/env/observability.env`
4. `deployments/env/asset.env`
5. `deployments/env/llm.env`
6. `deployments/env/secrets.env`
7. `deployments/env/overrides.env`

Use root `.env` only during migration. New values should live under
`deployments/env/`.

## Files

- `infra.env`: local infrastructure, registry, discovery, and service ports.
- `observability.env`: OpenTelemetry and OpenObserve settings.
- `asset.env`: asset service and OSS non-secret settings.
- `llm.env`: llm service settings, including the stable `LLM_ENCRYPT_KEY`.
- `secrets.env`: credentials and third-party secrets. Never commit it.
- `overrides.env`: personal local overrides such as temporary log levels.

Run `bash scripts/dev/check-env.sh` before `make dev-start`. It prints JSON to
stdout and exits non-zero when required keys are missing or still use
placeholder values, or when `JWT_SECRET` / `LLM_ENCRYPT_KEY` is shorter than 32
bytes.

## Kong and Konga

Kong runs in DB-less mode and loads `deployments/kong/declarative.local.yaml`.
That local file is generated from `deployments/kong/declarative.yml` plus the
local `JWT_SECRET`. Do not commit the generated file.

Konga is available only as a local observer for the Kong Admin API. Use it to
inspect services, routes, plugins, consumers, and JWT credentials; do not use it
as the source of truth for configuration changes. Kong proxy, Kong Admin API,
and Konga are bound to `127.0.0.1` in local Compose.

`make infra-up` runs `scripts/dev/bootstrap-konga.sh` after Compose starts. The
script creates or activates Konga's local connection `local-kong` with Kong Admin
URL `http://kong:8001`. If Konga shows `Connected to N/A`, run
`make konga-bootstrap` once and refresh the browser.

Kong also enables the OpenTelemetry plugin in DB-less config. Local Kong traces
are exported to the collector over OTLP HTTP at `http://otel-collector:4318/v1/traces`
with `service.name=kong-gateway`.

The Vite web dev server proxies `/api` to Kong proxy `http://localhost:8000`, so
browser traffic follows the same gateway route and JWT pre-authentication path as
direct API calls.
