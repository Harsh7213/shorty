const API_BASE_URL = import.meta.env.VITE_API_BASE_URL;
const TOKEN_STORAGE_KEY = import.meta.env.VITE_TOKEN_STORAGE_KEY;
if (!API_BASE_URL || !TOKEN_STORAGE_KEY) {
  throw new Error(
    "VITE_API_BASE_URL and VITE_TOKEN_STORAGE_KEY must be configured",
  );
}

export interface User {
  id: number;
  email: string;
  display_name: string;
}

export interface Organization {
  id: number;
  name: string;
  owner_user_id: number;
  permissions: OrganizationPermission[];
  created_at: string;
}

export type OrganizationPermission = "link.manage" | "organization.manage";

export interface OrganizationMember {
  user_id: number;
  email: string;
  display_name: string;
  permissions: OrganizationPermission[];
  joined_at: string;
  is_owner: boolean;
}

export interface ShortLink {
  id: number;
  long_url: string;
  short_url_code: string;
  short_url: string;
  status: "active" | "disabled" | "revoked";
  expires_at: string | null;
  created_at: string;
  created_by: {
    user_id: number | null;
    email: string;
    display_name: string;
  };
  total_clicks: number;
  country_codes: string[];
}

export interface OrganizationAnalytics {
  period: "week" | "month";
  links: {
    link_id: number;
    clicks: { date: string; clicks: number }[];
  }[];
  countries: {
    country_code: string;
    clicks: number;
  }[];
}

interface LoginResponse {
  access_token: string;
  token_type: string;
  expires_at: string;
}

function getErrorMessage(message: string, status: number) {
  const trimmed = message.trim();
  if (trimmed.startsWith("<")) return `Request failed (${status})`;
  return trimmed || `Request failed (${status})`;
}

export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function request<T>(
  path: string,
  options: { method?: string; body?: unknown } = {},
): Promise<T> {
  const headers = new Headers();
  const token = localStorage.getItem(TOKEN_STORAGE_KEY);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (options.body !== undefined)
    headers.set("Content-Type", "application/json");

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: options.method ?? "GET",
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!response.ok) {
    if (response.status === 401 && token && !path.endsWith("/auth/login")) {
      localStorage.removeItem(TOKEN_STORAGE_KEY);
      window.dispatchEvent(new Event("shorty:unauthorized"));
    }
    throw new ApiError(
      getErrorMessage(await response.text(), response.status),
      response.status,
    );
  }

  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export function saveToken(token: string) {
  localStorage.setItem(TOKEN_STORAGE_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_STORAGE_KEY);
}

export function hasSavedToken() {
  return Boolean(localStorage.getItem(TOKEN_STORAGE_KEY));
}

export async function signIn(email: string, password: string) {
  return request<LoginResponse>("/auth/login", {
    method: "POST",
    body: { email, password },
  });
}

export async function registerAccount(
  email: string,
  password: string,
  displayName: string,
) {
  return request<{ message: string }>("/auth/register", {
    method: "POST",
    body: { email, password, display_name: displayName },
  });
}

export function getCurrentUser() {
  return request<User>("/auth/me");
}

export function getOrganizations() {
  return request<Organization[]>("/org");
}

export function createOrganization(name: string) {
  return request<Organization>("/org", { method: "POST", body: { name } });
}

export function getOrganizationMembers(organizationId: number) {
  return request<OrganizationMember[]>(`/org/${organizationId}/members`);
}

export function saveOrganizationMember(
  organizationId: number,
  member: {
    email: string;
    display_name: string;
    password?: string;
    permissions: OrganizationPermission[];
  },
) {
  return request<{ message: string } | undefined>(
    `/org/${organizationId}/members`,
    {
      method: "POST",
      body: {
        email: member.email,
        display_name: member.display_name,
        ...(member.password ? { password: member.password } : {}),
        permissions: member.permissions,
      },
    },
  );
}

export function removeOrganizationMember(
  organizationId: number,
  userId: number,
) {
  return request<void>(`/org/${organizationId}/members/${userId}`, {
    method: "DELETE",
  });
}

export function getOrganizationLinks(organizationId: number) {
  return request<ShortLink[]>(`/org/${organizationId}/links`);
}

export function getOrganizationAnalytics(
  organizationId: number,
  period: OrganizationAnalytics["period"],
) {
  return request<OrganizationAnalytics>(
    `/org/${organizationId}/analytics?period=${period}`,
  );
}

export function createShortLink(
  organizationId: number,
  longUrl: string,
  expiresAt?: string,
) {
  return request<{ short_url: string; reused: boolean }>("/link", {
    method: "POST",
    body: {
      organization_id: organizationId,
      long_url: longUrl,
      ...(expiresAt ? { expires_at: expiresAt } : {}),
    },
  });
}

export function updateShortLink(code: string, status: "active" | "disabled") {
  return request<{
    status: "active" | "disabled" | "revoked";
    expires_at: string | null;
  }>(`/link/${code}`, {
    method: "PATCH",
    body: { status },
  });
}

export function revokeShortLink(code: string) {
  return request<{ message: string }>(`/link/${code}`, { method: "DELETE" });
}

export async function getQRCode(code: string) {
  const token = localStorage.getItem(TOKEN_STORAGE_KEY);
  const response = await fetch(
    `${API_BASE_URL}/qr/${encodeURIComponent(code)}`,
    {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    },
  );

  if (!response.ok) {
    if (response.status === 401 && token) {
      localStorage.removeItem(TOKEN_STORAGE_KEY);
      window.dispatchEvent(new Event("shorty:unauthorized"));
    }
    throw new Error(getErrorMessage(await response.text(), response.status));
  }

  return response.blob();
}
