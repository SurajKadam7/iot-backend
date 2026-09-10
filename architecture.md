# Architecture

## Design choice (cost + no archive loss)

| Path | Purpose | Durability if backend is down |
|---|---|---|
| MQTT → Go backend | Live latest state + WebSocket widgets | Best-effort (memory cleared on process restart) |
| IoT Rule → Firehose → S3 | 3-month raw history / exports | **Yes** — independent of EC2 |

**MVP load:** ~50 devices × ~1 event/s ≈ **50 msg/s**. Firehose is the archive path. MQTT is not the historical queue.

## High-level flow

```text
IoT Devices
    │ MQTT/TLS (QoS 1)  ~50 msg/s
    │ fixed JSON schema
    ▼
AWS IoT Core
    ├──────── MQTT subscription ───────► Go backend on EC2 (live)
    │                                      ├─ latest state in memory (overwrite)
    │                                      ├─ REST API (admin + export)
    │                                      └─ WebSocket (first-message JWT auth)
    │
    └─ IoT Rule ──► Firehose ──► S3
         prefix: org/{orgId}/device/{deviceId}/date={YYYY-MM-DD}/...
         lifecycle: 3 months
         error action → DLQ / backup S3

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
- Cognito issues JWT with at least: `user_id` (or Cognito `sub` mapped), `organization_id`, `role` (`org_admin` | `org_user`), `can_export` (boolean).
- REST: validate JWT on every request; scope all queries by claim `organization_id`.
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
- Before updating memory: resolve `device_identifier` / topic device id to a row in `devices` for a known `organization_id`; drop unknown devices.

## Archive (Firehose → S3)

Recommended object key prefix:

`org/{organization_id}/device/{device_identifier}/date={YYYY-MM-DD}/hour={HH}/...`

- Firehose buffering should produce batched files (not one tiny object per message).
- S3 lifecycle: expire/delete raw telemetry after **3 months**.
- Export jobs list/read only prefixes for the caller’s `organization_id` and requested device(s)/date range.
- Rule error action required (SQS DLQ or secondary bucket); alert on failures.

## Storage

| Data | Store | Notes |
|---|---|---|
| Organizations | PostgreSQL | Tenant root + `user_limit` |
| Users | PostgreSQL + Cognito | One org each; `role`, `can_export` |
| Locations / sub-locations | PostgreSQL | Optional grouping for filters |
| Devices | PostgreSQL | Belongs to org (+ optional sub_location) |
| Latest telemetry | Process memory | No `last_seen` column |
| Raw telemetry | S3 via Firehose | Prefixed as above; 3-month lifecycle |
| Export files | S3 | Short-lived pre-signed download |

## Database rules
Every tenant-owned table includes `organization_id`. Always filter by JWT org. Device id alone is not authorization.

Core tables (MVP):
- `organizations(id, name, user_limit, status, created_at)`
- `users(id, organization_id, cognito_subject, email, role, can_export, status, created_at)`
- `locations(id, organization_id, name, created_at)`
- `sub_locations(id, organization_id, location_id, name, created_at)`
- `devices(id, organization_id, sub_location_id nullable, device_identifier, name, status, created_at)`

Deferred tables/features: `invitations`, per-device ACL.

Constraints:
- `users.cognito_subject` unique.
- `devices.device_identifier` unique (at least per org; prefer globally unique for MQTT clarity).
- Useful indexes on `(organization_id)` and device lookup by `device_identifier`.

## Backend modules
- `auth`: JWT validation + tenant/ACL context
- `iot`: MQTT subscribe/reconnect (live)
- `telemetry`: schema validate + org device check + memory update
- `state`: concurrency-safe map (`sync.RWMutex` or equivalent)
- `api`: REST (me, locations, devices admin, exports, health)
- `ws`: first-message auth hub + org-scoped push
- `export`: S3 prefix-scoped CSV job + pre-signed URL
- `repository`, `config`, `observability`

Go backend does **not** write the historical archive.

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
- `POST /api/exports` (requires `can_export`)
- `GET /api/exports/{id}` (requires `can_export`)

Deferred: invite/user-management email APIs, per-device ACL APIs.

### WebSocket
- `GET /ws` — upgrade to WSS
- First client message: authenticate with JWT (see Auth & ACL)
- After auth: server may send snapshot of current in-memory states for org devices
- Server pushes updates as MQTT updates memory
- Client may filter/subscribe to a subset of org device ids (still org-scoped)

## Frontend (MVP)
- Login (Cognito)
- Live dashboard: **scrollable widgets** for temperature/pressure/humidity + timestamp; show waiting state when no in-memory value
- Admin: add/edit/remove devices (and basic locations if needed)
- Export: range + status + download (if `can_export`)
- No AWS credentials in the browser

## Export flow
1. User with `can_export` requests export for org devices/date range (≤ 3 months).
2. Backend verifies JWT org + ACL.
3. Background job reads only `org/{organization_id}/...` prefixes for the range.
4. Stream CSV to S3 (do not hold entire file in memory).
5. Return status + short-lived pre-signed URL.

## Deployment
- One EC2 Go process: MQTT live + REST + WebSocket
- RDS PostgreSQL (local PG for dev OK)
- Firehose → telemetry S3 bucket with prefix scheme + lifecycle
- IoT Rule on `org/+/device/+/telemetry` → Firehose + error action
- Frontend S3 + CloudFront; Route 53; ACM; CloudWatch
- Device cert/Thing provisioning: manual/out of band for MVP
- No ALB/Redis/DynamoDB/Kinesis Data Streams/SQS archive path

## Scaling path
`ALB → multiple Go instances` for API/WS when needed; keep a single active MQTT live subscriber or redesign fan-out carefully. Keep Firehose archive path independent.
