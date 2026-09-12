# Requirements

## Product
Multi-tenant IoT live-feed dashboard with CSV telemetry archive and export.

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
- **Export UI and APIs are gated on `can_export`.** The Export control is shown only when that claim/flag is true. Role alone is not enough.
- **Per-device ACL is deferred** (post-MVP).
- **Email invites are deferred** (post-MVP). Org admins can add/remove users in their organization and set `role` / `can_export`, respecting org `user_limit`. Cognito pool users are still created outside the app.
- Platform operators (`platform_admin`) get a separate read-only console of all organizations, users (email + role), locations, and device counts. They cannot create orgs or use the client live board.

## Core behavior
1. Devices publish telemetry to AWS IoT Core over MQTT (**QoS 1**). Expected MVP load: **~50 devices at ~1 event/s each (~50 msg/s)**.
2. Go backend on EC2 **subscribes** to IoT Core on `org/+/device/+/telemetry`.
3. For each valid message the backend:
   - overwrites **latest telemetry in memory** for the live dashboard (no timer-delete; no DynamoDB/Redis/time-series; no `last_seen` DB column — UI uses in-memory timestamp);
   - **appends the reading as CSV to S3** (same process, same subscription). Do not use Firehose or an IoT Rule archive path for MVP.
4. After backend restart, memory is empty until the next MQTT message; UI shows **“Waiting for device to publish…”** until data arrives. CSV archive also pauses until the process is up again and receiving MQTT.
5. Backend exposes REST APIs for auth/me, admin device (and location) management, and exports.
6. Frontend shows live readings (temperature, pressure, etc.) over **WebSocket** as scrollable widgets — not REST polling.
7. Historical CSV objects live in S3 under org-scoped prefixes. **MVP does not delete or expire older archive objects.**
8. Users with `can_export` can request an export of archived CSV for their organization (date range). Users without `can_export` must not see the Export button and must not call export APIs.
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
- `ts`: ISO-8601 timestamp from the device when available; if missing/invalid, backend may stamp receive time for live state and archive.
- Reject/ignore payloads that fail validation; do not update memory or write S3 with malformed data.
- Topic: `org/{orgId}/device/{deviceId}/telemetry`  
  Ownership comes from the **devices** table + authenticated MQTT identity — do not trust `orgId` in the topic/payload alone for authorization.

## Archive CSV (MVP)
S3 object rows use a stable header:

`organization_id,device_identifier,temperature,pressure,humidity,ts`

Recommended key prefix:

`org/{organization_id}/device/{device_identifier}/date={YYYY-MM-DD}/hour={HH}/...csv`

Buffer/batch lines into objects (do not require one S3 object per MQTT message). Retention/deletion of old objects is **out of scope for MVP**.

## Durability & cost goals
- Live UI may show waiting/stale after backend restart until the next publish.
- CSV archive is written by the same Go MQTT subscriber. If the process is down, new history is not stored until it reconnects.
- Keep MVP simple for ~50 devices @ ~50 msg/s.
- No Amazon Data Firehose, Kinesis, or SQS archive worker for MVP.

## Non-functional
- Secure MQTT device authentication with X.509 certificates (provisioning of certs/Things is **out of scope** for the app MVP — configured outside the product).
- TLS for all network traffic (HTTPS + WSS).
- Validate device belongs to a known org before accepting live telemetry into memory or writing S3.
- Handle MQTT reconnects, malformed messages, duplicate delivery, and graceful shutdown.
- Export jobs must tolerate duplicate/at-least-once archive records.
- Log operational errors; avoid sensitive payload/secret logging.

## Out of scope / deferred
- Analytics, historical query DBs, alerts, Redis, DynamoDB, Kinesis Data Streams, SQS archive workers, Kubernetes, multi-instance HA (unless required later).
- Per-device ACL, email invitations, multi-org users, IoT certificate/Thing provisioning UI/APIs.
- Automatic deletion / lifecycle expiry of old S3 archive objects.
- Amazon Data Firehose (not used for MVP archive).
