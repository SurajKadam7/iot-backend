export type Role = "org_admin" | "org_user";

export type Me = {
  user_id: string;
  organization_id: string;
  organization_name: string;
  email: string;
  role: Role;
  can_export: boolean;
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
