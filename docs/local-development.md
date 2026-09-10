# Local development (Phase 1 live path)

This repository implements the **non-archival / live** path from `requirement.md` and `architecture.md`:

MQTT → Go backend (in-memory latest telemetry) → REST + WebSocket → React widgets.

Export / Firehose / S3 history is **not implemented** in this phase. Those remain AWS-side concerns documented in `docs/aws-setup.md`.

## Prerequisites

- Go 1.22+
- Node 20+
- PostgreSQL 16
- An MQTT broker (local Mosquitto, or AWS IoT Core)

Optional: Docker, if you prefer `docker compose up` for Postgres + Mosquitto.

## Configure

```bash
cp .env.example .env
# change JWT_SECRET
cp web/.env.example web/.env
```

`AUTH_MODE=local` enables `POST /api/dev/login`. That endpoint must stay off in production (`AUTH_MODE=cognito`).

## Database and seed

Create database `iot` owned by user `iot`, then:

```bash
go run ./cmd/seed
```

Seeded accounts (local auth only):

| Email | Role |
|---|---|
| admin@example.com | `org_admin` |
| user@example.com | `org_user` |

Organization id: `00000000-0000-4000-8000-000000000001`

MQTT topic example:

`org/00000000-0000-4000-8000-000000000001/device/line-a-01/telemetry`

## Run

Terminal 1 — Mosquitto (if not using Docker):

```bash
mosquitto -c deploy/mosquitto.conf
```

Terminal 2 — API + WebSocket:

```bash
go run ./cmd/server
```

Terminal 3 — frontend (Vite proxies `/api` and `/ws` to `:8080`):

```bash
cd web && npm install && npm run dev
```

Open http://localhost:5173 and sign in as `admin@example.com`.

Terminal 4 — publish live readings:

```bash
go run ./cmd/pub
```

Widgets stay on **Waiting for device to publish…** until the first valid MQTT message after a backend restart. That is required behavior.

## Tests

```bash
go test ./...
```

API, ACL, telemetry, and WebSocket tests use in-memory fakes and do not need PostgreSQL.

## Production auth

Set `AUTH_MODE=cognito`, `JWT_ISSUER`, and `JWT_JWKS_URL`. The browser must send the Cognito **ID token** so custom claims are present: `user_id` (or `sub` mapped to the users row), `organization_id`, `role`, `can_export`. Users still need a matching row in `users` with the same `organization_id`.

The Cognito app client used by the UI must allow `USER_PASSWORD_AUTH`, or you can replace the login form with Hosted UI later.
