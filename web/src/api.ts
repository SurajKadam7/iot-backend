import type { ApiError, Device, Location, Me, OrganizationOverview, OrgUser, Role, SubLocation } from "./types";

const TOKEN_KEY = "iot_jwt";

export function getToken(): string | null {
  return sessionStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null): void {
  if (token) sessionStorage.setItem(TOKEN_KEY, token);
  else sessionStorage.removeItem(TOKEN_KEY);
}

async function parse<T>(res: Response): Promise<T> {
  if (res.status === 204) return undefined as T;
  const data = (await res.json()) as T | ApiError;
  if (!res.ok) {
    const err = data as ApiError;
    throw new Error(err.error?.message || res.statusText);
  }
  return data as T;
}

function headers(json = false): HeadersInit {
  const h: Record<string, string> = {};
  const token = getToken();
  if (token) h.Authorization = `Bearer ${token}`;
  if (json) h["Content-Type"] = "application/json";
  return h;
}

export const api = {
  async loginLocal(email: string): Promise<{ token: string }> {
    return parse(
      await fetch("/api/dev/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      }),
    );
  },

  async loginCognito(email: string, password: string): Promise<string> {
    const region = import.meta.env.VITE_COGNITO_REGION;
    const clientId = import.meta.env.VITE_COGNITO_CLIENT_ID;
    if (!region || !clientId) {
      throw new Error("Cognito is not configured (VITE_COGNITO_REGION / VITE_COGNITO_CLIENT_ID).");
    }
    const res = await fetch(`https://cognito-idp.${region}.amazonaws.com/`, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-amz-json-1.1",
        "X-Amz-Target": "AWSCognitoIdentityProviderService.InitiateAuth",
      },
      body: JSON.stringify({
        AuthFlow: "USER_PASSWORD_AUTH",
        ClientId: clientId,
        AuthParameters: { USERNAME: email, PASSWORD: password },
      }),
    });
    const data = (await res.json()) as {
      AuthenticationResult?: { IdToken?: string };
      message?: string;
      __type?: string;
    };
    if (!res.ok || !data.AuthenticationResult?.IdToken) {
      throw new Error(data.message || "Cognito login failed");
    }
    return data.AuthenticationResult.IdToken;
  },

  async me(): Promise<Me> {
    return parse(await fetch("/api/me", { headers: headers() }));
  },
  async locations(): Promise<{ locations: Location[] }> {
    return parse(await fetch("/api/locations", { headers: headers() }));
  },
  async createLocation(name: string): Promise<Location> {
    return parse(await fetch("/api/locations", { method: "POST", headers: headers(true), body: JSON.stringify({ name }) }));
  },
  async subLocations(locationId: string): Promise<{ sub_locations: SubLocation[] }> {
    return parse(await fetch(`/api/locations/${locationId}/sub-locations`, { headers: headers() }));
  },
  async createSubLocation(locationId: string, name: string): Promise<SubLocation> {
    return parse(
      await fetch(`/api/locations/${locationId}/sub-locations`, {
        method: "POST",
        headers: headers(true),
        body: JSON.stringify({ name }),
      }),
    );
  },
  async devices(): Promise<{ devices: Device[] }> {
    return parse(await fetch("/api/devices", { headers: headers() }));
  },
  async createDevice(body: {
    device_identifier: string;
    name: string;
    sub_location_id?: string | null;
    status?: string;
  }): Promise<Device> {
    return parse(await fetch("/api/devices", { method: "POST", headers: headers(true), body: JSON.stringify(body) }));
  },
  async patchDevice(
    id: string,
    body: { name?: string; status?: string; sub_location_id?: string | null },
  ): Promise<Device> {
    return parse(await fetch(`/api/devices/${id}`, { method: "PATCH", headers: headers(true), body: JSON.stringify(body) }));
  },
  async deleteDevice(id: string): Promise<void> {
    return parse(await fetch(`/api/devices/${id}`, { method: "DELETE", headers: headers() }));
  },
  async users(): Promise<{ users: OrgUser[] }> {
    return parse(await fetch("/api/users", { headers: headers() }));
  },
  async createUser(body: { email: string; role: Role; can_export: boolean }): Promise<OrgUser> {
    return parse(await fetch("/api/users", { method: "POST", headers: headers(true), body: JSON.stringify(body) }));
  },
  async patchUser(
    id: string,
    body: { role?: Role; can_export?: boolean; status?: "active" | "disabled" },
  ): Promise<OrgUser> {
    return parse(await fetch(`/api/users/${id}`, { method: "PATCH", headers: headers(true), body: JSON.stringify(body) }));
  },
  async deleteUser(id: string): Promise<void> {
    return parse(await fetch(`/api/users/${id}`, { method: "DELETE", headers: headers() }));
  },
  async internalOrganizations(): Promise<{ organizations: OrganizationOverview[] }> {
    return parse(await fetch("/api/internal/organizations", { headers: headers() }));
  },
};
