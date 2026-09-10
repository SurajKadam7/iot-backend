You are a senior Go/AWS full-stack engineer. Build the MVP described by `requirement.md` and `architecture.md` in this repository.

Read both files first and treat them as the source of truth. Inspect the existing repository before changing anything. Do not invent requirements.

## Goal
Build a secure, multi-tenant IoT live-feed web application:
Organization → Location → Sub-location → Device (UI: scrollable live widgets).
~50 devices / ~50 msg/s. Go backend on EC2: MQTT live state in memory, REST for admin/export, WebSocket for live readings. Archive: IoT Rule → Firehose → S3 (3 months) with prefix `org/{orgId}/device/{deviceId}/date=...`. Fixed telemetry schema. Simple org ACL. Single org per user.

## Implementation requirements
- Backend: Go. Idiomatic layout, context, structured logging, graceful shutdown, env config, tests for authz/state.
- MQTT: AWS IoT Core TLS/X.509 for backend subscribe. Reconnect/backoff, malformed handling, duplicate-safe latest-state overwrite. Validate device against `devices` table before memory update.
- In-memory state: concurrency-safe map of latest fixed-schema telemetry per device. No timer deletion. No Redis/DynamoDB/time-series. No DB `last_seen` column.
- After restart: empty memory; frontend shows “Waiting for device to publish…” until first message.
- Database: PostgreSQL tables in `architecture.md` (no invitations table for MVP). Tenant scope everything by `organization_id`. One org per user.
- Auth: Cognito-compatible JWT with `user_id`, `organization_id`, `role`, `can_export`. Never trust body/query for tenant identity.
- ACL: `org_admin` vs `org_user` + `can_export`. Per-device ACL and email invites are deferred — do not build them.
- API: implement MVP REST surface in `architecture.md` including **admin device CRUD**. Health/readiness unauthenticated. Consistent JSON errors.
- WebSocket: WSS; **first-message JWT auth** with ≤5s timeout; no telemetry until authenticated; bind connection to org; push org-scoped updates/snapshots.
- Frontend: React + TypeScript unless stack already exists. Login, scrollable live widgets (temperature/pressure/humidity), admin device management, export UI if allowed. No AWS credentials in browser.
- Export: async job, ≤3 months, read only caller’s S3 org prefixes, stream CSV, pre-signed download. Requires `can_export`.
- Historical ingestion: Firehose path is configured in AWS (document it). Backend must not write archive. No SQS/Kinesis archive worker.
- Device certificate/Thing provisioning: out of scope (document manual setup).
- Security: TLS, validation, least privilege, no secrets in git, no sensitive logs.
- Cost discipline: no Redis/DynamoDB/Kinesis Data Streams/OpenSearch/Kubernetes/ALB/SQS archive unless required by the two docs. Firehose is required for archive.

## Data model
Implement MVP tables/constraints/indexes/migrations from `architecture.md`. Unique `device_identifier`. No invitations migration for MVP.

## Frontend behavior
- After login, only the JWT org is visible.
- Live board of scrollable device widgets; waiting state when no in-memory reading.
- Reconnect WebSocket with backoff; pause when page hidden if practical.
- Admin can create/update/delete devices in their org.
- Export UI only when `can_export`; show range, status, download/failure.

## Deliverables
1. Working backend.
2. Working frontend.
3. Database migrations/schema.
4. Tests for tenant isolation, ACL, state updates, key API/WS auth behavior.
5. `.env.example` with no secrets.
6. Local development instructions.
7. AWS setup notes: EC2, IoT Core, Firehose, S3 prefix+lifecycle, PostgreSQL/RDS, Cognito claim mapping, CloudFront, Route 53, ACM, CloudWatch; note cert provisioning is manual.
8. Keep `requirement.md` and `architecture.md`; update only for necessary corrections and call them out.
9. Do not claim AWS resources are deployed unless verified.

## Working style
- Inspect repo conventions first; implement in small increments.
- Prefer simple production-quality code.
- Do not build analytics, invites, per-device ACL, or provisioning portals.
- Do not weaken tenant isolation.
- End with: what shipped, tests, remaining config, assumptions.
