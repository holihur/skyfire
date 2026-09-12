import type { InterfacePatch, PeerInput, Settings, WireGuardInterface } from './types'

// WireGuard public keys are standard base64 (contains `+` and `/`), which is
// not URL safe. The API decides paths with the unpadded URL-safe form.
export function urlKey(k: string): string {
  try {
    return btoa(atob(k))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '')
  } catch {
    return k
  }
}

class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export function isUnauthorized(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { ...(init?.headers as Record<string, string>) }
  if (init?.body && typeof init.body === 'string') headers['Content-Type'] = 'application/json'

  let res: Response
  try {
    res = await fetch(path, { ...init, headers, credentials: 'same-origin' })
  } catch {
    throw new ApiError(0, 'Cannot reach the Skyfire daemon')
  }
  if (res.status === 401) window.dispatchEvent(new Event('skyfire:unauthorized'))
  if (res.status === 204) return undefined as T
  const ct = res.headers.get('content-type') ?? ''
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    if (ct.includes('application/json')) {
      try {
        const body = await res.json()
        if (body?.error) msg = body.error
      } catch {
        /* ignore */
      }
    }
    throw new ApiError(res.status, msg)
  }
  if (ct.includes('application/json')) return (await res.json()) as T
  return (await res.text()) as unknown as T
}

export interface LoginResponse {
  ok?: boolean
  /** TOTP is enabled and a code is required to finish signing in. */
  totpRequired?: boolean
  /** TOTP is enabled but not yet bound; enroll an authenticator. */
  enroll?: boolean
  secret?: string
  uri?: string
  /** data: URL of the enrollment QR code. */
  qr?: string
}

export interface AuthStatus {
  passwordLogin: boolean
  totpEnabled: boolean
  totpBound: boolean
}

export const api = {
  login: (username: string, password: string, totp?: string) =>
    request<LoginResponse>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ username, password, totp: totp ?? '' }),
    }),
  authStatus: () => request<AuthStatus>('/api/auth'),
  logout: () => request<{ ok: boolean }>('/api/logout', { method: 'POST' }),
  health: () => request<{ status: string; driver: string; dryRun: boolean; version: string }>('/api/health'),
  getText: (url: string) => request<string>(url),
  settings: () => request<Settings>('/api/settings'),
  saveSettings: (s: Settings) =>
    request<Settings>('/api/settings', { method: 'PUT', body: JSON.stringify(s) }),

  interfaces: () => request<WireGuardInterface[]>('/api/interfaces'),
  interface: (name: string) => request<WireGuardInterface>(`/api/interfaces/${name}`),
  createInterface: (body: Partial<WireGuardInterface>) =>
    request<WireGuardInterface>('/api/interfaces', { method: 'POST', body: JSON.stringify(body) }),
  updateInterface: (name: string, patch: InterfacePatch) =>
    request<WireGuardInterface>(`/api/interfaces/${name}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    }),
  deleteInterface: (name: string) =>
    request<void>(`/api/interfaces/${name}`, { method: 'DELETE' }),
  setInterfaceUp: (name: string, up: boolean) =>
    request<WireGuardInterface>(`/api/interfaces/${name}/up`, {
      method: 'POST',
      body: JSON.stringify({ up }),
    }),

  addPeer: (iface: string, input: PeerInput) =>
    request<WireGuardInterface['peers'][number]>(`/api/interfaces/${iface}/peers`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updatePeer: (iface: string, pub: string, input: PeerInput) =>
    request<WireGuardInterface['peers'][number]>(`/api/interfaces/${iface}/peers/${urlKey(pub)}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    }),
  deletePeer: (iface: string, pub: string) =>
    request<void>(`/api/interfaces/${iface}/peers/${urlKey(pub)}`, { method: 'DELETE' }),

  peerConfigUrl: (iface: string, pub: string) =>
    `/api/interfaces/${iface}/peers/${urlKey(pub)}/config`,
  peerQrUrl: (iface: string, pub: string) =>
    `/api/interfaces/${iface}/peers/${urlKey(pub)}/config.png`,
  // Token-scoped client config: the connection string handed to the desktop
  // client. It works without a login and exposes only this peer.
  tokenConfigUrl: (token: string) => `/api/p/${token}/wg.conf`,
  serverConfigUrl: (iface: string) => `/api/interfaces/${iface}/config`,
  privateKey: (iface: string) => request<{ privateKey: string }>(`/api/interfaces/${iface}/private-key`),
}

export const openDownload = (url: string) => {
  const a = document.createElement('a')
  a.href = url
  a.download = ''
  document.body.appendChild(a)
  a.click()
  a.remove()
}