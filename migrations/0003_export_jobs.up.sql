CREATE TABLE IF NOT EXISTS export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id),
    requested_by UUID NOT NULL REFERENCES users (id),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    from_ts TIMESTAMPTZ NOT NULL,
    to_ts TIMESTAMPTZ NOT NULL,
    device_ids TEXT[] NOT NULL DEFAULT '{}',
    object_key TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (from_ts < to_ts)
);

CREATE INDEX IF NOT EXISTS export_jobs_organization_id_idx ON export_jobs (organization_id);
CREATE INDEX IF NOT EXISTS export_jobs_status_created_idx ON export_jobs (status, created_at);
