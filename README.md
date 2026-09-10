# IoT Live Feed

Secure, multi-tenant live telemetry dashboard.

**Hierarchy:** Organization → Location → Sub-location → Device  
**Phase 1 (this PR):** non-archival live path only.

```
Devices --MQTT/TLS QoS1--> AWS IoT Core --subscribe--> Go process (latest state in memory)
                                              REST admin + WebSocket widgets
```

The archive path (IoT Rule → Firehose → S3, 3-month export) is specified in `architecture.md` but **is not implemented here**. The backend never writes historical archive data.

Source of truth: `requirement.md`, `architecture.md`. Agent instructions: `agent-build-prompt.md`.

## What shipped in Phase 1

- PostgreSQL metadata: organizations, users, locations, sub-locations, devices
- Cognito-compatible JWT auth (`user_id`, `organization_id`, `role`, `can_export`) plus local login for development
- MQTT subscriber with reconnect, schema validation, device-table authorization
- Concurrency-safe in-memory latest reading per device (no timer deletion, no `last_seen` column)
- REST: health/ready, me, locations, sub-locations, admin device CRUD
- WebSocket: first-message JWT within 5s, org-scoped snapshot + live pushes
- React UI: login, scrollable live widgets, admin devices and locations
- Tests for tenant isolation, ACL, telemetry ingest, WebSocket auth
- Local Mosquitto publisher and seed data

**Not in Phase 1:** `POST/GET /api/exports`, export UI, Firehose worker, invitations, per-device ACL, cert provisioning UI.

## Quick start

See [docs/local-development.md](docs/local-development.md). AWS notes: [docs/aws-setup.md](docs/aws-setup.md).

```bash
cp .env.example .env
go run ./cmd/seed
go run ./cmd/server
cd web && npm install && npm run dev
go run ./cmd/pub
```

Sign in at http://localhost:5173 as `admin@example.com`.

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

## Tests

```bash
go test ./...
```

## Security

- Tenant identity only from JWT, never from query/body
- No AWS keys in the browser
- Do not commit `.env` or certificates
- `AUTH_MODE=local` is for development only
