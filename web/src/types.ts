export type Role = "org_admin" | "org_user";
export type AuthRole = Role | "platform_admin";

export type Me = {
  user_id: string;
  organization_id: string;
  organization_name: string;
  email: string;
  role: AuthRole;
  can_export: boolean;
};

export type OrgUser = {
  id: string;
  organization_id: string;
  email: string;
  role: Role;
  can_export: boolean;
  status: "active" | "disabled";
  created_at: string;
};

export type Location = {
  id: string;
  organization_id: string;
  name: string;
  created_at: string;
};

export type SubLocation = {
  id: string;
  organization_id: string;
  location_id: string;
  name: string;
  created_at: string;
};

export type Device = {
  id: string;
  organization_id: string;
  device_identifier: string;
  name: string;
  status: "active" | "inactive";
  created_at: string;
  mqtt_topic: string;
  sub_location_id: string | null;
  location_id: string | null;
  location_name: string | null;
  sub_location_name: string | null;
};

export type Reading = {
  device_id: string;
  temperature: number;
  pressure: number;
  humidity: number;
  ts: string;
  updated_at: string;
};

export type ApiError = {
  error: { code: string; message: string };
};

export type ConnectionState = "connecting" | "live" | "reconnecting" | "offline" | "paused";

export type OrgOverviewUser = {
  id: string;
  email: string;
  role: AuthRole;
  status: "active" | "disabled";
};

export type OrgOverviewSubLocation = {
  id: string;
  name: string;
  device_count: number;
};

export type OrgOverviewLocation = {
  id: string;
  name: string;
  device_count: number;
  sub_locations: OrgOverviewSubLocation[];
};

export type OrganizationOverview = {
  id: string;
  name: string;
  status: "active" | "disabled";
  user_limit: number;
  created_at: string;
  user_count: number;
  device_count: number;
  unassigned_device_count: number;
  users: OrgOverviewUser[];
  locations: OrgOverviewLocation[];
};
