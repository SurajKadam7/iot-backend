# AWS setup notes (live path + archive configuration)

These notes describe how to host the Phase 1 live application. **This repository does not create AWS resources** and does not claim they are deployed.

Certificate / Thing provisioning is **manual and out of scope** for the app.

## What Phase 1 runs on EC2

One Go process:

- MQTT subscribe (live latest state only)
- REST (`/api/health`, `/api/ready`, `/api/me`, locations, devices)
- WebSocket `/ws` with first-message JWT auth (5s timeout)

Put the binary on a single EC2 instance with a public HTTPS endpoint (or serve the UI from CloudFront and API from the instance). Architecture forbids ALB/Redis/DynamoDB/Kinesis Data Streams/SQS archive workers for MVP.

## PostgreSQL / RDS

Create an RDS PostgreSQL instance (or local Postgres for dev). Apply migrations by starting the server (it runs `migrations/*.up.sql` on boot) or by executing those files.

Every tenant table is scoped by `organization_id`. JWT `organization_id` is the only tenant selector.

## Cognito

1. User pool + app client.
2. Pre-token generation trigger (or equivalent) must embed trusted claims on the **ID token**:
   - `user_id` (UUID of `users.id`) or rely on `sub` = `users.cognito_subject`
   - `organization_id`
   - `role` = `org_admin` | `org_user`
   - `can_export` (boolean; unused by Phase 1 APIs but still required by the JWT contract)
3. Backend env:
   - `AUTH_MODE=cognito`
   - `JWT_ISSUER=https://cognito-idp.{region}.amazonaws.com/{poolId}`
   - `JWT_JWKS_URL=.../.well-known/jwks.json`
   - `JWT_AUDIENCE={clientId}` (optional; also accepts `client_id`)
4. Mirror each user in PostgreSQL (`users.cognito_subject`, `organization_id`, `role`, `status`).
5. One organization per user.

Do not put AWS credentials in the browser. Frontend only talks to Cognito (login) and this API.

## AWS IoT Core (live MQTT)

1. Create a Thing + X.509 cert for **each device** (manual).
2. Policy: publish to `org/{thatOrg}/device/{thatIdentifier}/telemetry` only.
3. Create a **backend subscriber** Thing/cert used by the Go process.
4. Policy for the backend cert: subscribe `org/+/device/+/telemetry`.
5. Backend env:
   - `MQTT_BROKER=ssl://xxxxx-ats.iot.{region}.amazonaws.com:8883`
   - `MQTT_CERT_PATH` / `MQTT_KEY_PATH` / `MQTT_CA_CERT_PATH` (Amazon Root CA 1)
6. QoS 1. Duplicates overwrite in-memory latest state.
7. Unknown `device_identifier` values are dropped. Topic `orgId` is not authorization.

After EC2 restart, in-memory state is empty until the next publish.

## Archive path (configured in AWS, not in this binary)

Architecture requires archive **independent of the Go process**:

```
IoT Rule on org/+/device/+/telemetry
  → Amazon Data Firehose
  → S3 prefix org/{organization_id}/device/{device_identifier}/date={YYYY-MM-DD}/hour={HH}/...
  → lifecycle expire after 3 months
  → rule error action: SQS DLQ or backup bucket + alarm
```

The Go backend **must not write** the historical archive. Phase 1 does **not** implement `POST /api/exports`. When export is built later, jobs must read only the caller’s `org/{organization_id}/...` prefixes.

## Frontend hosting

- Build: `cd web && npm ci && npm run build`
- Upload `web/dist` to S3 + CloudFront, or set `WEB_DIST_DIR=web/dist` and let the Go process serve the SPA.
- DNS: Route 53
- TLS: ACM on CloudFront (and/or the API endpoint)
- Frontend env at build time: `VITE_AUTH_MODE=cognito`, `VITE_COGNITO_REGION`, `VITE_COGNITO_CLIENT_ID`

## Monitoring

CloudWatch: EC2 CPU/disk, RDS, IoT rule errors, Firehose delivery errors, Go process logs (`LOG_FORMAT=json`). Do not log JWT tokens, cert material, or full telemetry payloads at info level.

## Network

TLS for MQTT, HTTPS, and WSS in production. Security group: 443 to the API/UI; 8883 is IoT Core, not opened on EC2. Postgres not public.
