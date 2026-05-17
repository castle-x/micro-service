# Local Environment Files

Copy the examples in this directory to matching `.env` files and fill local
values before starting the development stack.

## Load Order

Files are loaded in this order. Later files override earlier files.

1. `.env` at the repo root, for legacy compatibility only.
2. `deployments/env/infra.env`
3. `deployments/env/observability.env`
4. `deployments/env/asset.env`
5. `deployments/env/model.env`
6. `deployments/env/secrets.env`
7. `deployments/env/overrides.env`

Use root `.env` only during migration. New values should live under
`deployments/env/`.

## Files

- `infra.env`: local infrastructure, registry, discovery, and service ports.
- `observability.env`: OpenTelemetry and OpenObserve settings.
- `asset.env`: asset service and OSS non-secret settings.
- `model.env`: model service settings when they are added.
- `secrets.env`: credentials and third-party secrets. Never commit it.
- `overrides.env`: personal local overrides such as temporary log levels.

Run `bash scripts/dev/check-env.sh` before `make dev-start`. It prints JSON to
stdout and exits non-zero when required keys are missing or still use
placeholder values.

## Konga

`make infra-up` runs `scripts/dev/bootstrap-konga.sh` after Compose starts. The
script creates or activates Konga's local connection `local-kong` with Kong Admin
URL `http://kong:8001`. If Konga shows `Connected to N/A`, run
`make konga-bootstrap` once and refresh the browser.
