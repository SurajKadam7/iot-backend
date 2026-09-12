# Local development (live path + upcoming CSV archive)

This repository currently implements the **live** path from `requirement.md` and `architecture.md`:

MQTT → Go backend (in-memory latest telemetry) → REST + WebSocket → React widgets.

MVP also specifies: the **same Go MQTT subscription** writes CSV history to S3, and users with `can_export` get an Export button. That archive/export code is not in the live-path binary yet; follow `architecture.md` when implementing it.

Do **not** use Firehose for MVP. Do **not** delete old S3 objects in MVP.

## Prerequisites

- Go 1.22+
- Node 20+
- PostgreSQL 16
- An MQTT broker (local Mosquitto, or AWS IoT Core)

Optional: Docker, if you prefer `docker compose up` for Postgres + Mosquitto. For archive/export locally, an S3-compatible bucket (AWS S3 or MinIO) once that code lands.

## Configure

```bash
cp .env.example .env
# change JWT_SECRET
cp web/.env.example web/.env
```

`AUTH_MODE=local` enables `POST /api/dev/login`. That endpoint must stay off in production (`AUTH_MODE=cognito`).

When archive/export is implemented, set S3 bucket/region/prefix in `.env` (see `.env.example`). Leave AWS keys out of the browser.

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

Archive CSV prefix (when implemented):

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
