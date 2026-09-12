# Architecture

## Design choice (Phase 1)

| Path | Purpose | Durability if backend is down |
|---|---|---|
| MQTT → Go backend | Live latest state + WebSocket widgets | Best-effort (memory cleared on process restart) |
| MQTT → Go backend → S3 CSV | Phase 1 historical archive + exports | **Pauses** until the subscriber is back |
| IoT Rule → Firehose → S3 | Later-phase durable archive | **Yes** — independent of EC2 (**delayed**, not in Phase 1) |

**Phase 1 load:** ~50 devices × ~1 event/s ≈ **50 msg/s**. The Go MQTT subscription is both the live path and the archive path. **Firehose is delayed** until a later phase. Deleting old S3 objects is **out of scope** for Phase 1.

## High-level flow

```text
IoT Devices
    │ MQTT/TLS (QoS 1)  ~50 msg/s
    │ fixed JSON schema
    │ topic: org/{orgId}/device/{deviceId}/telemetry
    ▼
AWS IoT Core
    └──────── MQTT subscription ───────► Go backend on EC2
                                           ├─ latest state in memory (overwrite)
                                           ├─ append CSV rows to S3 (batched)
                                           ├─ REST API (admin + export)
                                           └─ WebSocket (first-message JWT auth)

S3
    ├─ archive: org/{orgId}/device/{deviceId}/date={YYYY-MM-DD}/hour={HH}/...csv
    └─ export files: short-lived CSV + pre-signed download

Browser
  ├─ HTTPS → Go REST API
  └─ WSS → Go WebSocket → scrollable live widgets

Cognito login → JWT claims: user_id, organization_id, role, can_export
Go backend → PostgreSQL (org / user / location / device metadata)
Frontend → S3/CloudFront
DNS → Route 53 | TLS → ACM | Monitoring → CloudWatch
```

## Auth & ACL

- **One org per user.** `users.organization_id` is required and unique membership model for MVP.
- Cognito issues JWT with at least: `user_id` (or Cognito `sub` mapped), `organization_id`, `role` (`org_admin` | `org_user` | `platform_admin`), `can_export` (boolean).
- REST: validate JWT on every request; scope tenant queries by claim `organization_id`. `platform_admin` may call `GET /api/internal/organizations` across tenants (read-only).
- **Export:** `POST /api/exports` and `GET /api/exports/{id}` require `can_export`. The frontend shows the Export button **only** when `can_export` is true.
- WebSocket **first-message auth** (over WSS):
  1. Client connects.
  2. Client must send auth message with JWT within **5 seconds** or server closes.
  3. Until authenticated: accept no subscribe/data interest; send no telemetry.
  4. On success: bind connection to `user_id` + `organization_id` + role/ACL; do not allow re-auth as another user on the same connection.
  5. Push only devices belonging to that organization.
- Never trust browser-supplied org/device IDs for authorization.

## Telemetry (live)

Fixed schema:

```json
{
  "temperature": 0.0,
  "pressure": 0.0,
  "humidity": 0.0,
  "ts": "2026-09-10T00:00:00Z"
}
```

In-memory map: `device_id → { temperature, pressure, humidity, ts, updated_at }`.
- Overwrite only when a newer valid message arrives (or always overwrite with latest valid payload for MVP simplicity).
- Do not remove entries on a timer.
- On process start: map empty → UI “Waiting for device to publish…” until first message per device.
- Before updating memory or writing S3: resolve `device_identifier` / topic device id to a row in `devices` for a known `organization_id`; drop unknown devices.

## Archive (Go MQTT → S3 CSV)

One MQTT subscription (`org/+/device/+/telemetry`) feeds both live memory and archive.

After a payload passes schema + device-table authorization, the process appends a CSV row:

```text
organization_id,device_identifier,temperature,pressure,humidity,ts
```

Object key prefix:

`org/{organization_id}/device/{device_identifier}/date={YYYY-MM-DD}/hour={HH}/...csv`

- Buffer into batched objects (not one tiny object per message).
- **Do not expire or delete archive objects in MVP.**
- Export jobs list/read only prefixes for the caller’s `organization_id` and requested device(s)/date range.

## Storage

| Data | Store | Notes |
|---|---|---|
| Organizations | PostgreSQL | Tenant root + `user_limit` |
| Users | PostgreSQL + Cognito | One org each; `role`, `can_export` |
| Locations / sub-locations | PostgreSQL | Optional grouping for filters |
| Devices | PostgreSQL | Belongs to org (+ optional sub_location) |
| Latest telemetry | Process memory | No `last_seen` column |
| Raw telemetry | S3 CSV via Go MQTT subscriber | Prefixed as above; no deletion in MVP |
| Export files | S3 | Short-lived pre-signed download |

## Database rules
Every tenant-owned table includes `organization_id`. Always filter by JWT org. Device id alone is not authorization.

Core tables (MVP):
- `organizations(id, name, user_limit, status, created_at)`
- `users(id, organization_id, cognito_subject, email, role, can_export, status, created_at)`
- `locations(id, organization_id, name, created_at)`
- `sub_locations(id, organization_id, location_id, name, created_at)`
- `devices(id, organization_id, sub_location_id nullable, device_identifier, name, status, created_at)`
- `export_jobs` (or equivalent) for async export status

Deferred tables/features: `invitations`, per-device ACL.

Constraints:
- `users.cognito_subject` unique.
- `devices.device_identifier` unique (at least per org; prefer globally unique for MQTT clarity).
- Useful indexes on `(organization_id)` and device lookup by `device_identifier`.

## Backend modules
- `auth`: JWT validation + tenant/ACL context
- `iot`: MQTT subscribe/reconnect (live + archive)
- `telemetry`: schema validate + org device check + memory update
- `archive`: buffer validated readings and write CSV to S3
- `state`: concurrency-safe map (`sync.RWMutex` or equivalent)
- `api`: REST (me, locations, devices admin, exports, health)
- `ws`: first-message auth hub + org-scoped push
- `export`: S3 prefix-scoped CSV job + pre-signed URL (`can_export` only)
- `repository`, `config`, `observability`

## API sketch (MVP)

### REST
- `GET /api/health` / `GET /api/ready` (unauthenticated)
- `GET /api/me`
- `GET /api/locations`
- `POST /api/locations` (admin)
- `GET /api/locations/{id}/sub-locations`
- `POST /api/locations/{id}/sub-locations` (admin)
- `GET /api/devices`
- `GET /api/devices/{id}`
- `POST /api/devices` (admin) — register device into org
- `PATCH /api/devices/{id}` (admin)
- `DELETE /api/devices/{id}` (admin)
- `GET /api/users` (admin)
- `POST /api/users` (admin) — add org user; enforce `user_limit`; no email invite
- `PATCH /api/users/{id}` (admin) — `role`, `can_export`, `status`
- `DELETE /api/users/{id}` (admin)
- `GET /api/internal/organizations` (`platform_admin`) — all orgs, users (email + role), location tree, device counts
- `POST /api/exports` (requires `can_export`)
- `GET /api/exports/{id}` (requires `can_export`)

Deferred: email invitation APIs, per-device ACL APIs.

### WebSocket
- `GET /ws` — upgrade to WSS
- First client message: authenticate with JWT (see Auth & ACL)
- After auth: server may send snapshot of current in-memory states for org devices
- Server pushes updates as MQTT updates memory
- Client may filter/subscribe to a subset of org device ids (still org-scoped)

## Frontend (MVP)
- Login (Cognito)
- Live dashboard: **scrollable widgets** for temperature/pressure/humidity + timestamp; show waiting state when no in-memory value
- Admin: add/edit/remove devices, locations, and org users (role + `can_export`)
- Operator console (`platform_admin`): read-only organizations overview at `/internal`
- **Export button and page only when `can_export` is true.** Range + status + download/failure. Hidden for everyone else (including admins with `can_export` false).
- No AWS credentials in the browser

## Export flow
1. User with `can_export` requests export for org devices/date range.
2. Backend verifies JWT org + `can_export`. Reject others.
3. Background job reads only `org/{organization_id}/...` prefixes for the range.
4. Stream a downloadable CSV to S3 (do not hold the entire file in memory).
5. Return status + short-lived pre-signed URL.

## Deployment
- One EC2 Go process: MQTT live + CSV archive to S3 + REST + WebSocket
- RDS PostgreSQL (local PG for dev OK)
- S3 bucket for archive CSV + export files; EC2 IAM: `s3:PutObject` on archive prefix, `s3:GetObject`/`ListBucket` for export jobs, `s3:PutObject` on export prefix
- Frontend S3 + CloudFront; Route 53; ACM; CloudWatch
- Device cert/Thing provisioning: manual/out of band for MVP
- No ALB/Redis/DynamoDB/Kinesis Data Streams/SQS archive path
- **Firehose delayed:** do not configure IoT Rule → Firehose in Phase 1

## Later phase (delayed): Firehose archive
When Firehose is added, IoT Core should fan out independently of EC2:

```text
AWS IoT Core
  ├─ MQTT subscribe → Go (live + Phase 1 CSV, until retired)
  └─ IoT Rule → Firehose → S3  (archive continues if EC2 is down)
```

Phase 1 must not require that path.

## Scaling path
`ALB → multiple Go instances` for API/WS when needed; keep a **single active MQTT subscriber** (live + archive) or redesign fan-out carefully so CSV is not duplicated. Keep a future Firehose path independent of extra API instances.
