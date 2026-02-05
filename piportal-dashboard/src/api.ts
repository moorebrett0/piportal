const BASE = '/api/v1';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    ...options,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }

  return res.json();
}

export interface UserInfo {
  id: string;
  email: string;
  created_at: string;
  device_count: number;
}

export interface OrgInfo {
  id: string;
  name: string;
  created_at: string;
}

export interface DeviceInfo {
  id: string;
  subdomain: string;
  url: string;
  tier: string;
  is_online: boolean;
  tunnel_enabled: boolean;
  created_at: string;
  last_seen_at?: string;
  bytes_in: number;
  bytes_out: number;
  bytes_total: number;
  limit: number;
  org_id?: string;
  org_name?: string;
  cpu_temp?: number;
  mem_total?: number;
  mem_free?: number;
  disk_total?: number;
  disk_free?: number;
  uptime?: number;
  load_avg?: number;
}

export interface AuthResponse {
  success: boolean;
  user: { id: string; email: string };
  token: string;
}

export interface CreateDeviceResponse {
  success: boolean;
  id: string;
  token: string;
  subdomain: string;
  url: string;
}

export interface ClaimResponse {
  success: boolean;
  id: string;
  subdomain: string;
  url: string;
}

export interface CommandResult {
  device_id: string;
  subdomain: string;
  exit_code: number;
  output: string;
  error: string;
}

export interface RunCommandResponse {
  results: CommandResult[];
}

export const api = {
  signup: (email: string, password: string) =>
    request<AuthResponse>('/signup', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  login: (email: string, password: string) =>
    request<AuthResponse>('/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  logout: () =>
    request<{ success: boolean }>('/logout', { method: 'POST' }),

  me: () => request<UserInfo>('/me'),

  listDevices: (orgId?: string) =>
    request<DeviceInfo[]>(orgId ? `/devices?org_id=${orgId}` : '/devices'),

  getDevice: (id: string) => request<DeviceInfo>(`/devices/${id}`),

  createDevice: (subdomain: string) =>
    request<CreateDeviceResponse>('/devices', {
      method: 'POST',
      body: JSON.stringify({ subdomain }),
    }),

  claimDevice: (token: string) =>
    request<ClaimResponse>('/devices/claim', {
      method: 'POST',
      body: JSON.stringify({ token }),
    }),

  deleteDevice: (id: string) =>
    request<{ success: boolean }>(`/devices/${id}`, { method: 'DELETE' }),

  rebootDevice: (id: string) =>
    request<{ success: boolean }>(`/devices/${id}/reboot`, { method: 'POST' }),

  setTunnelEnabled: (id: string, enabled: boolean) =>
    request<{ success: boolean; tunnel_enabled: boolean }>(`/devices/${id}/tunnel`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    }),

  setDeviceOrg: (deviceId: string, orgId: string | null) =>
    request<{ success: boolean; org_id: string | null }>(`/devices/${deviceId}/org`, {
      method: 'PUT',
      body: JSON.stringify({ org_id: orgId }),
    }),

  listOrgs: () => request<OrgInfo[]>('/organizations'),

  createOrg: (name: string) =>
    request<OrgInfo>('/organizations', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  updateOrg: (id: string, name: string) =>
    request<{ success: boolean; id: string; name: string }>(`/organizations/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    }),

  deleteOrg: (id: string) =>
    request<{ success: boolean }>(`/organizations/${id}`, { method: 'DELETE' }),

  runCommand: (command: string, orgId: string, dryRun: boolean) =>
    request<RunCommandResponse>('/commands/run', {
      method: 'POST',
      body: JSON.stringify({ command, org_id: orgId, dry_run: dryRun }),
    }),
};
