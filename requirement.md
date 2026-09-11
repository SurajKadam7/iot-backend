# Requirements

## Product
Multi-tenant IoT live-feed dashboard with 3-month telemetry export.

## Hierarchy
`Organization → Location → Sub-location → Device`

MVP UI may present devices as a **scrollable live widget board** (optionally filterable by location). Deep hierarchy navigation pages are secondary.

## Roles & simple ACL (MVP)
- **Single organization per user.**
- JWT (from Cognito login) must include trusted claims: `user_id`, `organization_id`, `role`, and `can_export`.
- Backend derives tenant context **only** from JWT claims — never from request body/query.
- Roles:
  - `org_admin`: manage locations/devices/users (CRUD as exposed by APIs); view live dashboard; export if `can_export`.
  - `org_user`: view live dashboard for their org; export only if `can_export` is true.
  - `platform_admin`: internal operator console only (all organizations, read-only). Not a client login.
- **Per-device ACL is deferred** (post-MVP).
- **Email invites are deferred** (post-MVP). Org admins can add/remove users in their organization and set `role` / `can_export`, respecting org `user_limit`. Cognito pool users are still created outside the app.
- Platform operators (`platform_admin`) get a separate read-only console of all organizations, users (email + role), locations, and device counts. They cannot create orgs or use the client live board.

## Core behavior
1. Devices publish telemetry to AWS IoT Core over MQTT (**QoS 1**). Expected MVP load: **~50 devices at ~1 event/s each (~50 msg/s)**.
2. Go backend on EC2 subscribes to IoT Core for **live state only**.
3. Backend keeps the **latest telemetry in memory per device** (overwrite on new message; do not timer-delete). No DynamoDB/Redis/time-series DB for live state. No `last_seen` DB column — UI uses in-memory timestamp.
4. After backend restart, memory is empty until the next MQTT message; UI shows **“Waiting for device to publish…”** until data arrives.
5. Backend exposes REST APIs for auth/me, admin device (and location) management, and exports.
6. Frontend shows live readings (temperature, pressure, etc.) over **WebSocket** as scrollable widgets — not REST polling.
7. Raw telemetry is archived for 3 months via AWS IoT Rule → **Amazon Data Firehose** → **S3**. Do not use MQTT as the historical queue; do not route archive through the Go backend.
8. Users with `can_export` can request/export up to 3 months of data for their organization.
9. Organization data must be strictly isolated.
10. Authentication uses Amazon Cognito (or compatible). Cognito login/token issuance embeds `organization_id` and `user_id` into the JWT.

## Fixed telemetry schema (MVP)
Devices publish JSON matching this schema (field names stable for MVP):

```json
{
  "temperature": 0.0,
  "pressure": 0.0,
  "humidity": 0.0,
  "ts": "2026-09-10T00:00:00Z"
}
```

- `temperature`, `pressure`, `humidity`: numbers (device units as agreed; document in API/UI).
- `ts`: ISO-8601 timestamp from the device when available; if missing/invalid, backend may stamp receive time for live state only.
- Reject/ignore payloads that fail validation; do not update memory with malformed data.
- Topic: `org/{orgId}/device/{deviceId}/telemetry`  
  Ownership comes from the **devices** table + authenticated MQTT identity — do not trust `orgId` in the topic/payload alone for authorization.

## Durability & cost goals
- Archive continues when the backend is down (Firehose → S3). Configure a rule **error action** and monitor delivery.
- At ~50 msg/s, Firehose is the chosen archive path (cheaper/simpler than SQS workers at this rate).
- Live UI may show waiting/stale after backend restart until the next publish; archive remains complete in S3.

## Non-functional
- Secure MQTT device authentication with X.509 certificates (provisioning of certs/Things is **out of scope** for the app MVP — configured outside the product).
- TLS for all network traffic (HTTPS + WSS).
- Validate device belongs to a known org before accepting live telemetry into memory.
- Handle MQTT reconnects, malformed messages, duplicate delivery, and graceful shutdown.
- Export jobs must tolerate duplicate/at-least-once archive records.
- Log operational errors; avoid sensitive payload/secret logging.
- Keep MVP simple for ~50 devices @ ~50 msg/s.

## Out of scope / deferred
- Analytics, historical query DBs, alerts, Redis, DynamoDB, Kinesis Data Streams, SQS archive workers, Kubernetes, multi-instance HA (unless required later).
- Per-device ACL, email invitations, multi-org users, IoT certificate/Thing provisioning UI/APIs.
- Amazon Data Firehose **is required** for archive delivery to S3.
