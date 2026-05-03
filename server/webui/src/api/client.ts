const BASE = '/api/v1'

function token() {
  return sessionStorage.getItem('wg_token') ?? ''
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token() ? { Authorization: `Bearer ${token()}` } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error ?? `HTTP ${res.status}`)
  return data as T
}

export const api = {
  // Auth
  login: (username: string, password: string, totp_code?: string, push_auth_request_id?: string) =>
    request<{ token: string; user_id: string; username: string; is_admin: boolean; require_totp?: boolean; push_auth_enabled?: boolean; push_request_id?: string }>(
      'POST', '/auth/login', { username, password, totp_code, push_auth_request_id }
    ),
  pollPushStatus: (id: string) =>
    request<{ status: string }>('GET', `/auth/push-status/${id}`),
  me: () => request<User>('GET', '/auth/me'),
  serverInfo: () => request<{ vpn_name: string; push_auth_enabled: boolean }>('GET', '/info'),
  createPushAuth: (context: string) =>
    request<{ request_id: string; status: string }>('POST', '/auth/push', { context }),

  // Password change
  changePassword: (current_password: string, new_password: string) =>
    request<{ ok: boolean }>('POST', '/auth/change-password', { current_password, new_password }),

  // TOTP
  totpSetup: () => request<{ secret: string; uri: string }>('POST', '/auth/totp/setup'),
  totpConfirm: (code: string) => request<{ ok: boolean }>('POST', '/auth/totp/confirm', { code }),
  totpDisable: (code: string) => request<{ ok: boolean }>('POST', '/auth/totp/disable', { code }),

  // Passkeys
  listPasskeys: () => request<Passkey[]>('GET', '/auth/passkeys'),
  deletePasskey: (id: string) => request<{ ok: boolean }>('DELETE', `/auth/passkeys/${id}`),
  passkeyLoginBegin: (username: string) => request<unknown>('POST', '/auth/passkey/login/begin', { username }),
  passkeyLoginFinish: (username: string, assertion: unknown) =>
    request<{ token: string; username: string; is_admin: boolean }>(
      'POST', `/auth/passkey/login/finish?username=${encodeURIComponent(username)}`, assertion
    ),
  passkeyRegisterBegin: () => request<unknown>('POST', '/auth/passkey/register/begin'),
  passkeyRegisterFinish: (name: string, attestation: unknown) =>
    request<{ id: string; name: string }>('POST', `/auth/passkey/register/finish?name=${encodeURIComponent(name)}`, attestation),

  // Servers (user-facing — lists accessible servers)
  listMyServers: () => request<WGServer[]>('GET', '/servers'),

  // Sessions (user)
  mySessions: () => request<Session[]>('GET', '/sessions/mine'),
  createSession: (server_id: string, client_public_key: string) =>
    request<{ session_id: string; assigned_ip: string; server_id: string; wg_config: string; vpn_name: string }>(
      'POST', '/sessions', { server_id, client_public_key }
    ),
  deleteSession: (id: string) => request<{ ok: boolean }>('DELETE', `/sessions/${id}`),

  // Admin — users
  listUsers: () => request<User[]>('GET', '/admin/users'),
  createUser: (data: { username: string; email: string; password: string; first_name?: string; last_name?: string; is_admin: boolean }) =>
    request<User>('POST', '/admin/users', data),
  updateUser: (id: string, data: Partial<{ email: string; password: string; first_name: string; last_name: string; is_admin: boolean; is_active: boolean; disable_totp: boolean; admin_password: string }>) =>
    request<User>('PUT', `/admin/users/${id}`, data),
  deleteUser: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/users/${id}`),
  userGroups: (id: string) => request<Group[]>('GET', `/admin/users/${id}/groups`),
  addUserGroup: (userId: string, groupId: string) =>
    request<{ ok: boolean }>('POST', `/admin/users/${userId}/groups`, { group_id: groupId }),
  removeUserGroup: (userId: string, groupId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/users/${userId}/groups/${groupId}`),

  // Admin — groups
  listGroups: () => request<Group[]>('GET', '/admin/groups'),
  createGroup: (data: { name: string; description?: string }) =>
    request<Group>('POST', '/admin/groups', data),
  updateGroup: (id: string, data: Partial<{ name: string; description: string }>) =>
    request<Group>('PUT', `/admin/groups/${id}`, data),
  deleteGroup: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/groups/${id}`),
  groupAccess: (id: string) => request<ResourceGroup[]>('GET', `/admin/groups/${id}/access`),
  addGroupAccess: (groupId: string, resourceGroupId: string) =>
    request<{ ok: boolean }>('POST', `/admin/groups/${groupId}/access`, { resource_group_id: resourceGroupId }),
  removeGroupAccess: (groupId: string, rgId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/groups/${groupId}/access/${rgId}`),

  // Admin — resources
  listResources: () => request<Resource[]>('GET', '/admin/resources'),
  createResource: (data: { name: string; ip_address: string; type: string; mask?: number | null; ports?: string | null; description?: string }) =>
    request<Resource>('POST', '/admin/resources', data),
  updateResource: (id: string, data: Partial<{ name: string; ip_address: string; type: string; mask: number | null; ports: string | null; description: string }>) =>
    request<Resource>('PUT', `/admin/resources/${id}`, data),
  deleteResource: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/resources/${id}`),

  // Admin — installations
  listInstallations: () => request<Installation[]>('GET', '/admin/installations'),
  revokeInstallation: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/installations/${id}`),

  // User — my installations
  listMyInstallations: () => request<Installation[]>('GET', '/installations/mine'),

  // Admin — resource groups
  listResourceGroups: () => request<ResourceGroup[]>('GET', '/admin/resource-groups'),
  createResourceGroup: (data: { name: string; description?: string }) =>
    request<ResourceGroup>('POST', '/admin/resource-groups', data),
  updateResourceGroup: (id: string, data: Partial<{ name: string; description: string }>) =>
    request<ResourceGroup>('PUT', `/admin/resource-groups/${id}`, data),
  deleteResourceGroup: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/resource-groups/${id}`),
  getResourceGroup: (id: string) => request<{ group: ResourceGroup; resources: Resource[] }>('GET', `/admin/resource-groups/${id}`),
  addResourceGroupMember: (rgId: string, resourceId: string) =>
    request<{ ok: boolean }>('POST', `/admin/resource-groups/${rgId}/resources`, { resource_id: resourceId }),
  removeResourceGroupMember: (rgId: string, resourceId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/resource-groups/${rgId}/resources/${resourceId}`),

  // Admin — sessions
  listAllSessions: () => request<AdminSession[]>('GET', '/admin/sessions'),
  terminateSession: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/sessions/${id}`),

  // Admin — user stored configs
  adminListUserConfigs: () => request<UserConfig[]>('GET', '/admin/user-configs'),
  adminDeleteUserConfig: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/user-configs/${id}`),

  // Admin — settings
  getSettings: () => request<Record<string, string>>('GET', '/admin/settings'),
  updateSettings: (data: Record<string, string>) => request<{ ok: boolean }>('PUT', '/admin/settings', data),

  // Admin — WireGuard servers
  adminListServers: () => request<WGServer[]>('GET', '/admin/servers'),
  adminCreateServer: (data: {
    name: string; endpoint: string; port: number; interface_name: string
    subnet: string; dns?: string; external?: boolean; public_key?: string
  }) => request<WGServer>('POST', '/admin/servers', data),
  adminUpdateServer: (id: string, data: { name: string; endpoint: string; dns: string }) =>
    request<{ ok: boolean }>('PUT', `/admin/servers/${id}`, data),
  adminDeleteServer: (id: string) => request<{ ok: boolean }>('DELETE', `/admin/servers/${id}`),
  adminServerGroups: (id: string) => request<Group[]>('GET', `/admin/servers/${id}/groups`),
  adminAddServerGroup: (serverId: string, groupId: string) =>
    request<{ ok: boolean }>('POST', `/admin/servers/${serverId}/groups`, { group_id: groupId }),
  adminRemoveServerGroup: (serverId: string, groupId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/servers/${serverId}/groups/${groupId}`),
  adminServerUsers: (id: string) => request<User[]>('GET', `/admin/servers/${id}/users`),
  adminAddServerUser: (serverId: string, userId: string) =>
    request<{ ok: boolean }>('POST', `/admin/servers/${serverId}/users`, { user_id: userId }),
  adminRemoveServerUser: (serverId: string, userId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/servers/${serverId}/users/${userId}`),

  // Admin — server <-> bundle attachments (the new direct access path)
  adminServerBundles: (id: string) => request<ResourceGroup[]>('GET', `/admin/servers/${id}/bundles`),
  adminAddServerBundle: (serverId: string, bundleId: string) =>
    request<{ ok: boolean }>('POST', `/admin/servers/${serverId}/bundles`, { bundle_id: bundleId }),
  adminRemoveServerBundle: (serverId: string, bundleId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/servers/${serverId}/bundles/${bundleId}`),

  // Admin — bundle -> servers reciprocal
  adminBundleServers: (id: string) => request<{ id: string; name: string; subnet: string }[]>('GET', `/admin/resource-groups/${id}/servers`),
  adminAttachBundleServer: (bundleId: string, serverId: string) =>
    request<{ ok: boolean }>('POST', `/admin/resource-groups/${bundleId}/servers`, { server_id: serverId }),
  adminDetachBundleServer: (bundleId: string, serverId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/resource-groups/${bundleId}/servers/${serverId}`),

  // Admin — user <-> server direct access
  adminUserServers: (id: string) => request<{ id: string; name: string; subnet: string }[]>('GET', `/admin/users/${id}/servers`),
  adminAddUserServer: (userId: string, serverId: string) =>
    request<{ ok: boolean }>('POST', `/admin/users/${userId}/servers`, { server_id: serverId }),
  adminRemoveUserServer: (userId: string, serverId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/users/${userId}/servers/${serverId}`),

  // Admin — user bundle assignments (per-server)
  adminUserBundles: (userId: string, serverId: string) =>
    request<ResourceGroup[]>('GET', `/admin/users/${userId}/servers/${serverId}/bundles`),
  adminAddUserBundle: (userId: string, serverId: string, bundleId: string) =>
    request<{ ok: boolean }>('POST', `/admin/users/${userId}/servers/${serverId}/bundles`, { bundle_id: bundleId }),
  adminRemoveUserBundle: (userId: string, serverId: string, bundleId: string) =>
    request<{ ok: boolean }>('DELETE', `/admin/users/${userId}/servers/${serverId}/bundles/${bundleId}`),

  // Permissions
  permCatalog: () => request<PermDef[]>('GET', '/auth/permissions'),
  updateRolePermissions: (roleId: string, permissions: string[]) =>
    request<{ ok: boolean }>('PUT', `/admin/groups/${roleId}/permissions`, { permissions }),

  // Admin — diagnostics
  adminDiagnostics: () => request<Diagnostic[]>('GET', '/admin/diagnostics'),

  // Admin — user reach (flattened access)
  userReach: (id: string) => request<ReachRow[]>('GET', `/admin/users/${id}/reach`),

  // Admin — audit log
  adminAudit: (params: { limit?: number; offset?: number; actor?: string; action?: string; target_type?: string; target_id?: string; since?: string } = {}) => {
    const q = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v != null && v !== '') q.set(k, String(v)) })
    const qs = q.toString()
    return request<{ items: AuditRow[]; total: number; limit: number; offset: number }>(
      'GET', `/admin/audit${qs ? '?' + qs : ''}`,
    )
  },

  // Admin — denied attempts
  adminDenials: (params: { limit?: number; offset?: number; user_id?: string; dst_ip?: string; since?: string } = {}) => {
    const q = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v != null && v !== '') q.set(k, String(v)) })
    const qs = q.toString()
    return request<{ items: DenialRow[]; total: number; limit: number; offset: number }>(
      'GET', `/admin/denials${qs ? '?' + qs : ''}`,
    )
  },

  // Admin — traffic (per-flow accounting)
  trafficTop: (params: { by: 'user' | 'resource' | 'user_resource' | 'destination' | 'port'; user_id?: string; resource_id?: string; server_id?: string; limit?: number; since?: string }) => {
    const q = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v != null && v !== '') q.set(k, String(v)) })
    return request<TrafficTopRow[]>('GET', `/admin/traffic/top?${q}`)
  },
  trafficSummary: (params: { user_id?: string; resource_id?: string; server_id?: string; since?: string } = {}) => {
    const q = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v != null && v !== '') q.set(k, String(v)) })
    const qs = q.toString()
    return request<TrafficSummary>('GET', `/admin/traffic/summary${qs ? '?' + qs : ''}`)
  },

  // Admin — full access topology
  adminTopology: () => request<Topology>('GET', '/admin/topology'),
}

// Types
export interface User {
  id: string
  username: string
  email: string
  first_name: string
  last_name: string
  totp_enabled: boolean
  is_admin: boolean
  is_active: boolean
  created_at: string
  /** Effective permissions — only present on /auth/me. */
  permissions?: string[]
}

export interface PermDef {
  key: string
  label: string
  description: string
  category: string
}

export interface Passkey {
  id: string
  user_id: string
  name: string
  created_at: string
}

export interface Group {
  id: string
  name: string
  description: string | null
  created_at: string
  /** Permissions granted by this role; populated by /admin/groups list. */
  permissions?: string[]
}

export interface Resource {
  id: string
  name: string
  ip_address: string
  type: 'host' | 'network'
  mask: number | null
  ports: string | null
  description: string | null
  created_at: string
}

export interface ResourceGroup {
  id: string
  name: string
  description: string | null
  created_at: string
}

export interface Session {
  id: string
  server_id: string | null
  assigned_ip: string
  created_at: string
  last_keepalive: string
}

export interface AdminSession extends Session {
  username: string
  email: string
}

export interface UserConfig {
  id: string
  name: string
  created_at: string
  user_id: string
  username: string
  email: string
}

export interface Installation {
  id: string
  device_name: string
  user_id: string | null
  username: string
  is_active: boolean
  last_seen: string | null
  created_at: string
}

export interface WGServer {
  id: string
  name: string
  endpoint: string
  port: number
  interface_name: string
  public_key?: string
  subnet: string
  dns: string | null
  external: boolean
  is_active: boolean
  /** Present when fetched via admin endpoint — true means the WG iface is currently up. */
  running?: boolean
  created_at: string
}

export interface Diagnostic {
  id: string
  severity: 'warn' | 'error' | 'info'
  title: string
  detail?: string
  subject?: string
}

export interface Topology {
  people:         { id: string; username: string; is_admin: boolean }[]
  bundles:        { id: string; name: string }[]
  resources:      { id: string; name: string; ip_address: string; type: 'host' | 'network'; mask: number | null }[]
  servers:        { id: string; name: string; subnet: string }[]
  person_server:  { from: string; to: string }[]
  server_bundle?: { from: string; to: string }[]
  server_allowed_bundle?: { from: string; to: string }[]
  user_bundle:    { from: string; to: string }[]
  bundle_resource:{ from: string; to: string }[]
}

export interface TrafficTopRow {
  key: string
  label: string
  bytes_tx: number
  bytes_rx: number
  conns: number
}

export interface TrafficSummary {
  bytes_tx: number
  bytes_rx: number
  conns: number
}

export interface DenialRow {
  id: number
  first_ts: string
  last_ts: string
  count: number
  user_id: string | null
  username: string | null
  src_ip: string
  dst_ip: string
  dst_port: number | null
  proto: string
}

export interface AuditRow {
  id: string
  ts: string
  actor_user_id: string | null
  actor_username: string | null
  method: string
  path: string
  action: string | null
  target_type: string | null
  target_id: string | null
  target_label: string | null
  status_code: number
  success: number
  error_message: string | null
  ip: string | null
  user_agent: string | null
  detail: string | null
}

export interface ReachRow {
  resource_id: string
  resource_name: string
  ip_address: string
  type: 'host' | 'network'
  mask: number | null
  ports: string | null
  bundle_id: string
  bundle_name: string
  server_id: string
  server_name: string
  server_subnet: string
}
