CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    user_limit INTEGER NOT NULL DEFAULT 10 CHECK (user_limit > 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id),
    cognito_subject TEXT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('org_admin', 'org_user')),
    can_export BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS users_cognito_subject_idx ON users (cognito_subject);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_idx ON users (lower(email));
CREATE INDEX IF NOT EXISTS users_organization_id_idx ON users (organization_id);

CREATE TABLE IF NOT EXISTS locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS locations_organization_id_idx ON locations (organization_id);

CREATE TABLE IF NOT EXISTS sub_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id),
    location_id UUID NOT NULL REFERENCES locations (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS sub_locations_organization_id_idx ON sub_locations (organization_id);
CREATE INDEX IF NOT EXISTS sub_locations_location_id_idx ON sub_locations (location_id);

CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id),
    sub_location_id UUID REFERENCES sub_locations (id) ON DELETE SET NULL,
    device_identifier TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS devices_device_identifier_idx ON devices (device_identifier);
CREATE INDEX IF NOT EXISTS devices_organization_id_idx ON devices (organization_id);
