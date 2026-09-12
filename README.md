# IoT Live Feed

Secure, multi-tenant live telemetry dashboard with CSV archive and export.

**Hierarchy:** Organization → Location → Sub-location → Device

```
Devices --MQTT/TLS QoS1--> AWS IoT Core --subscribe--> Go process
                                              ├─ latest state in memory → WebSocket widgets
                                              ├─ CSV rows → S3 archive
                                              └─ REST admin + export (can_export)
```

Source of truth: `requirement.md`, `architecture.md`. Agent instructions: `agent-build-prompt.md`.

The Go MQTT subscriber is the archive path (not Firehose). Export is available only to users with `can_export`. MVP does not delete old S3 objects.

## What shipped so far (live path)

- PostgreSQL metadata: organizations, users, locations, sub-locations, devices
- Cognito-compatible JWT auth (`user_id`, `organization_id`, `role`, `can_export`) plus local login for development
- MQTT subscriber with reconnect, schema validation, device-table authorization
- Concurrency-safe in-memory latest reading per device (no timer deletion, no `last_seen` column)
- REST: health/ready, me, locations, sub-locations, admin device CRUD, admin user CRUD, internal org overview
- WebSocket: first-message JWT within 5s, org-scoped snapshot + live pushes
- React UI: client live board + admin pages; operator console at `/internal`
- Tests for tenant isolation, ACL, telemetry ingest, WebSocket auth
- Local Mosquitto publisher and seed data

**Specified for MVP, not in the live-path code yet:** MQTT → S3 CSV archive, `POST/GET /api/exports`, Export button (`can_export` only).

**Deferred:** email invitations, per-device ACL, cert provisioning UI, S3 lifecycle/deletion, Firehose.

## Quick start

See [docs/local-development.md](docs/local-development.md). AWS notes: [docs/aws-setup.md](docs/aws-setup.md).

```bash
cp .env.example .env
go run ./cmd/seed
go run ./cmd/server
cd web && npm install && npm run dev
go run ./cmd/pub
```

Sign in at http://localhost:5173 as `admin@example.com` (client, `can_export`) or `ops@example.com` (operator console). `user@example.com` is a viewer without export.

## Implementation decisions (docs were silent)

These are required to have a working system; they do not add product features beyond the two docs.

| Topic | Decision |
|---|---|
| Primary keys | UUID |
| JSON | snake_case |
| MQTT topic device id | `devices.device_identifier` (globally unique) |
| Latest-state overwrite | Always replace with the latest valid payload (duplicate-safe) |
| Units shown | °C, hPa, %RH — “as published by the device” |
| Local auth | `AUTH_MODE=local` + `POST /api/dev/login` (users table has no passwords; Cognito is the password store) |
| WS protocol | `{type:auth,token}` then `{type:auth_ok}`, `{type:snapshot,readings}`, `{type:reading,reading}` |
| Device identifier after create | Immutable via API; recreate to change |
| Location delete | Not exposed (API sketch is GET+POST only) |
| SPA | Vite in `web/`; Go serves `web/dist` if present |
| Archive | Same MQTT subscription writes batched CSV to S3 |
| Export UI | Rendered only when JWT/`me.can_export` is true |

## Tests

```bash
go test ./...
```

## Security

- Tenant identity only from JWT, never from query/body
- No AWS keys in the browser
- Do not commit `.env` or certificates
- `AUTH_MODE=local` is for development only
