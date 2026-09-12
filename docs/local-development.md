# Local development (live path + CSV archive)

This repository implements the Phase 1 path from `requirement.md` and `architecture.md`:

MQTT → Go backend (in-memory latest telemetry **and** batched CSV archive) → REST + WebSocket → React widgets.

Locally the archive is a directory (`ARCHIVE_DIR=data/archive`) unless `S3_BUCKET` is set. Users with `can_export` can call `POST /api/exports` and `GET /api/exports/{id}`. The Export UI button is still frontend work.

**Firehose is delayed** past Phase 1. Do **not** delete old archive objects in Phase 1.

## Prerequisites

- Go 1.22+
- Node 20+
- PostgreSQL 16
- An MQTT broker (local Mosquitto, or AWS IoT Core)

Optional: Docker, if you prefer `docker compose up` for Postgres + Mosquitto. For S3 locally, set `S3_BUCKET` (and optionally `S3_ENDPOINT` for MinIO). With an empty bucket the server writes CSV under `data/archive`.

## Configure

```bash
cp .env.example .env
# change JWT_SECRET
cp web/.env.example web/.env
```

`AUTH_MODE=local` enables `POST /api/dev/login`. That endpoint must stay off in production (`AUTH_MODE=cognito`).

When `S3_BUCKET` is empty, archive files land in `ARCHIVE_DIR` (default `data/archive`). Leave AWS keys out of the browser.

## Database and seed

Create database `iot` owned by user `iot`, then:

```bash
go run ./cmd/seed
```

Seeded accounts (local auth only):

| Email | Role | Export (`can_export`) |
|---|---|---|
| admin@example.com | `org_admin` | yes — Export button visible |
| user@example.com | `org_user` | no — Export button hidden |
| ops@example.com | `platform_admin` | no |

Org admins can add, edit, and remove users at **Users** in the UI (`GET/POST/PATCH/DELETE /api/users`). Access is Viewer or Admin; **export is a separate `can_export` flag**. The Export control must appear only when that flag is true. The org `user_limit` is enforced. Local-mode users sign in with email only. Client admins cannot create `platform_admin`.

`ops@example.com` opens the **operator console** (`/internal`): every organization, user emails and roles, locations, and device counts. Read-only.

For Cognito, still create a matching pool user outside this app; the UI only writes the Postgres `users` row.

Organization id: `00000000-0000-4000-8000-000000000001`

MQTT topic example:

`org/00000000-0000-4000-8000-000000000001/device/line-a-01/telemetry`

Archive CSV prefix:

`org/{organization_id}/device/{device_identifier}/date={YYYY-MM-DD}/hour={HH}/`

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
make web-dev
```

Or: `cd web && npm install && npm run dev`.

Open http://localhost:5173 and sign in as `admin@example.com` for the client app, or `ops@example.com` for the operator console. `make web` only **builds** the SPA (`web/dist`) for the Go server to serve.

Terminal 4 — publish live readings:

```bash
go run ./cmd/pub
```

`cmd/pub` uses client id `iot-dev-publisher` locally (override `MQTT_PUB_CLIENT_ID`). For AWS IoT, copy [`.env.aws.publisher.example`](../.env.aws.publisher.example) into `.env` and attach [docs/aws-iot-laptop-policy.json](aws-iot-laptop-policy.json) to the Thing. See [docs/aws-setup.md](aws-setup.md).

Widgets stay on **Waiting for device to publish…** until the first valid MQTT message after a backend restart. That is required behavior.

## Tests

```bash
go test ./...
```

API, ACL, telemetry, and WebSocket tests use in-memory fakes and do not need PostgreSQL.

## Production auth

Set `AUTH_MODE=cognito`, `JWT_ISSUER`, and `JWT_JWKS_URL`. The browser must send the Cognito **ID token** so custom claims are present: `user_id` (or `sub` mapped to the users row), `organization_id`, `role`, `can_export`. Users still need a matching row in `users` with the same `organization_id`.

The Cognito app client used by the UI must allow `USER_PASSWORD_AUTH`, or you can replace the login form with Hosted UI later.
