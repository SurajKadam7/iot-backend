# AWS setup notes (live path + archive configuration)

These notes describe how to host the Phase 1 live application. **This repository does not create AWS resources** and does not claim they are deployed.

Certificate / Thing provisioning is **manual and out of scope** for the app.

## First live AWS test (laptop → IoT Core → EC2 → UI)

Use this path to prove live widgets on AWS **without** Cognito, RDS, Firehose, or HTTPS.

```text
Laptop cmd/pub  --MQTT/TLS 8883 publish-->  AWS IoT Core
AWS IoT Core    --MQTT/TLS 8883 subscribe--> Go server on EC2
EC2             --WebSocket /ws------------> Browser UI
```

IoT Core is a **broker**. The laptop and the Go process both connect **out** to it. Do not open port 8883 inbound on EC2.

Keep `AUTH_MODE=local`. Seeded org id: `00000000-0000-4000-8000-000000000001`. Seeded device identifiers: `line-a-01`, `line-a-02`, `line-b-01`, `line-b-02`.

### Two different “devices”

| Registry | What it is | This test |
|---|---|---|
| **AWS IoT Thing** | MQTT identity (X.509 cert + policy) | One Thing for the laptop publisher, one Thing for the EC2 subscriber |
| **Postgres `devices` table** | What the Go backend accepts | Created by `go run ./cmd/seed`. Unknown `device_identifier` values are dropped |

`cmd/pub` uses **one MQTT connection** and publishes as the four seeded identifiers. Register **one** laptop Thing whose policy allows those topics. Do not create four AWS Things for this test unless you want per-device certs later.

### 1. Register the laptop publisher on AWS IoT Core

Replace `ap-south-1` with your region. Client id must match the publisher (`iot-dev-publisher` by default).

```bash
export AWS_REGION=ap-south-1
mkdir -p certs/publisher
cd certs/publisher

aws iot create-thing --thing-name iot-laptop-publisher --region "$AWS_REGION"

aws iot create-keys-and-certificate \
  --set-as-active \
  --certificate-pem-outfile publisher-cert.pem \
  --public-key-outfile publisher-public.key \
  --private-key-outfile publisher-private.key \
  --region "$AWS_REGION"
# Note certificateArn from the JSON output.

curl -sS -o AmazonRootCA1.pem https://www.amazontrust.com/repository/AmazonRootCA1.pem
```

Create policy `iot-laptop-publisher-policy` (IoT Core uses `*` in resource ARNs, not MQTT `+`):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "iot:Connect",
      "Resource": "arn:aws:iot:ap-south-1:ACCOUNT_ID:client/iot-dev-publisher"
    },
    {
      "Effect": "Allow",
      "Action": "iot:Publish",
      "Resource": [
        "arn:aws:iot:ap-south-1:ACCOUNT_ID:topic/org/00000000-0000-4000-8000-000000000001/device/line-a-01/telemetry",
        "arn:aws:iot:ap-south-1:ACCOUNT_ID:topic/org/00000000-0000-4000-8000-000000000001/device/line-a-02/telemetry",
        "arn:aws:iot:ap-south-1:ACCOUNT_ID:topic/org/00000000-0000-4000-8000-000000000001/device/line-b-01/telemetry",
        "arn:aws:iot:ap-south-1:ACCOUNT_ID:topic/org/00000000-0000-4000-8000-000000000001/device/line-b-02/telemetry"
      ]
    }
  ]
}
```

Save that JSON as `publisher-policy.json` (replace `ACCOUNT_ID` and the region), then attach policy and Thing:

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
CERT_ARN=$(aws iot list-certificates --query "certificates[?status=='ACTIVE'].certificateArn | [0]" --output text --region "$AWS_REGION")

# Prefer the ARN printed by create-keys-and-certificate if several certs exist.
aws iot create-policy --policy-name iot-laptop-publisher-policy --policy-document file://publisher-policy.json --region "$AWS_REGION"
aws iot attach-policy --policy-name iot-laptop-publisher-policy --target "$CERT_ARN" --region "$AWS_REGION"
aws iot attach-thing-principal --thing-name iot-laptop-publisher --principal "$CERT_ARN" --region "$AWS_REGION"
```

**Console equivalent:** AWS IoT → Manage → All devices → Things → Create thing → Create a single thing named `iot-laptop-publisher` → Auto-generate a new certificate → Attach the policy above → Download the device cert, private key, and Amazon Root CA 1.

Never commit `certs/`. Never reuse this certificate on EC2.

### 2. Register the EC2 backend subscriber

Same pattern, different Thing, cert, client id, and policy. Use [docs/aws-iot-backend-policy.json](aws-iot-backend-policy.json) after substituting `{region}` and `{account}`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "iot:Connect",
      "Resource": "arn:aws:iot:{region}:{account}:client/iot-backend-live"
    },
    {
      "Effect": "Allow",
      "Action": "iot:Subscribe",
      "Resource": "arn:aws:iot:{region}:{account}:topicfilter/org/+/device/+/telemetry"
    },
    {
      "Effect": "Allow",
      "Action": "iot:Receive",
      "Resource": "arn:aws:iot:{region}:{account}:topic/org/*/device/*/telemetry"
    }
  ]
}
```

Thing name: `iot-backend-live`. Client id: `iot-backend-live`. Local certs live in `secrets/subscriber/` (`cert.pem`, `private.key`, `AmazonRootCA1.pem`). On EC2 copy those to `/opt/iot/certs/`.

### 3. Endpoint and EC2

```bash
aws iot describe-endpoint --endpoint-type iot:Data-ATS --region "$AWS_REGION"
# ssl://xxxxx-ats.iot.ap-south-1.amazonaws.com:8883
```

EC2 (Amazon Linux 2023, t3.small is enough):

- Security group inbound: 22 and **8080 from your IP only**
- Outbound: 443 and 8883
- Do **not** open 8883 inbound
- Run Postgres 16 on the instance (`docker compose up -d postgres` from this repo; skip Mosquitto)

On EC2 `.env`:

```
AUTH_MODE=local
MQTT_ENABLED=true
MQTT_BROKER=ssl://xxxxx-ats.iot.ap-south-1.amazonaws.com:8883
MQTT_SUB_CLIENT_ID=iot-backend-live
MQTT_SUB_CA_CERT_PATH=secrets/subscriber/AmazonRootCA1.pem
MQTT_SUB_CERT_PATH=secrets/subscriber/cert.pem
MQTT_SUB_KEY_PATH=secrets/subscriber/private.key
FRONTEND_ORIGIN=http://<ec2-public-ip>:8080
DATABASE_URL=postgres://iot:iot@127.0.0.1:5432/iot?sslmode=disable
```

```bash
go run ./cmd/seed
cd web && npm ci && npm run build && cd ..
go run ./cmd/server
```

Open `http://<ec2-public-ip>:8080` and sign in as `admin@example.com`. Widgets stay on “Waiting for device to publish…” until the first valid MQTT message after a backend restart.

### 4. Publish from the laptop

Use the Thing certs under `secrets/publisher/` (`cert.pem`, `private.key`, `AmazonRootCA1.pem`). Replace the Thing policy in the AWS console with [docs/aws-iot-laptop-policy.json](aws-iot-laptop-policy.json) after substituting `{region}` and `{account}` (publish-only telemetry).

Copy MQTT settings from [`.env.aws.publisher.example`](../.env.aws.publisher.example) into laptop `.env` (keep existing `JWT_SECRET` / `AUTH_MODE`):

```
MQTT_BROKER=ssl://xxxxx-ats.iot.{region}.amazonaws.com:8883
MQTT_PUB_CA_CERT_PATH=secrets/publisher/AmazonRootCA1.pem
MQTT_PUB_CERT_PATH=secrets/publisher/cert.pem
MQTT_PUB_KEY_PATH=secrets/publisher/private.key
MQTT_PUB_CLIENT_ID=iot-dev-publisher
PUB_ORG_ID=00000000-0000-4000-8000-000000000001
```

Then:

```bash
go run ./cmd/pub
```

Do not point `cmd/server` at these laptop certs. The EC2 subscriber needs its own Thing.

### 5. Verify

1. AWS IoT console → MQTT test client → subscribe to `org/+/device/+/telemetry` — messages appear when `cmd/pub` is running.
2. EC2 logs: `mqtt connected` then no repeated `mqtt connect` errors.
3. UI widgets update about once per second.

If the test client sees messages but the UI stays waiting, the topic `device_identifier` is not in Postgres (run seed) or the backend cert cannot `Subscribe`/`Receive`.

### Later: one AWS Thing per physical device

Production policy for a single device should allow only that identifier, for example publish `org/{thatOrg}/device/{thatIdentifier}/telemetry`. Still insert a matching row in `devices` (admin UI or seed). Thing provisioning remains outside this application.

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
   - `can_export` (boolean; org admins can change the Postgres flag; CSV export APIs are not in Phase 1)
3. Backend env:
   - `AUTH_MODE=cognito`
   - `JWT_ISSUER=https://cognito-idp.{region}.amazonaws.com/{poolId}`
   - `JWT_JWKS_URL=.../.well-known/jwks.json`
   - `JWT_AUDIENCE={clientId}` (optional; also accepts `client_id`)
4. Mirror each user in PostgreSQL (`users.cognito_subject`, `organization_id`, `role`, `can_export`, `status`). Org-admin user CRUD in the app writes Postgres only; create the Cognito user out of band.
5. One organization per user.

Do not put AWS credentials in the browser. Frontend only talks to Cognito (login) and this API.

## AWS IoT Core (live MQTT)

For the laptop test, see **First live AWS test** above (one publisher Thing covering the four seeded identifiers).

Production / per-device:

1. Create a Thing + X.509 cert for **each device** (manual).
2. Policy: publish to `org/{thatOrg}/device/{thatIdentifier}/telemetry` only.
3. Create a **backend subscriber** Thing/cert used by the Go process.
4. Policy for the backend cert: `iot:Subscribe` on `org/+/device/+/telemetry` and `iot:Receive` on `org/*/device/*/telemetry`.
5. Backend env:
   - `MQTT_BROKER=ssl://xxxxx-ats.iot.{region}.amazonaws.com:8883`
   - `MQTT_SUB_CERT_PATH` / `MQTT_SUB_KEY_PATH` / `MQTT_SUB_CA_CERT_PATH` (Amazon Root CA 1)
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
