const ACCESS_TOKEN_KEY = 'sbs_access_token'
const REFRESH_TOKEN_KEY = 'sbs_refresh_token'

// Dispatched on window when a request fails auth and the refresh token
// couldn't renew the session either — the app shell listens for this to show
// a snackbar and redirect to /login, instead of pages failing silently.
export const SESSION_EXPIRED_EVENT = 'sbs:session-expired'

export interface AuthResponse {
  access_token: string
  refresh_token: string
  email: string
  roles: string[]
}

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_TOKEN_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY)
}

export function storeSession(auth: AuthResponse) {
  localStorage.setItem(ACCESS_TOKEN_KEY, auth.access_token)
  localStorage.setItem(REFRESH_TOKEN_KEY, auth.refresh_token)
  localStorage.setItem('sbs_email', auth.email)
  localStorage.setItem('sbs_roles', JSON.stringify(auth.roles))
}

export function clearSession() {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
  localStorage.removeItem('sbs_email')
  localStorage.removeItem('sbs_roles')
}

export function getCurrentEmail(): string | null {
  return localStorage.getItem('sbs_email')
}

export function getCurrentRoles(): string[] {
  const raw = localStorage.getItem('sbs_roles')
  return raw ? JSON.parse(raw) : []
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  const res = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!res.ok) {
    throw new Error('Invalid email or password')
  }
  const data: AuthResponse = await res.json()
  storeSession(data)
  return data
}

export async function requestPasswordReset(email: string): Promise<void> {
  await fetch('/api/v1/auth/forgot-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  })
}

export async function resetPassword(token: string, newPassword: string): Promise<void> {
  const res = await fetch('/api/v1/auth/reset-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, new_password: newPassword }),
  })
  if (!res.ok) {
    throw new Error(await res.text())
  }
}

export async function logout(): Promise<void> {
  const refreshToken = getRefreshToken()
  clearSession()
  if (refreshToken) {
    await fetch('/api/v1/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    }).catch(() => undefined)
  }
}

function notifySessionExpired() {
  clearSession()
  window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT))
}

// Access tokens are short-lived (15 minutes); refreshPromise dedupes
// concurrent 401s so a page that fires several requests at once only
// triggers one refresh call instead of a burst of them.
let refreshPromise: Promise<boolean> | null = null

async function refreshAccessToken(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      const refreshToken = getRefreshToken()
      if (!refreshToken) return false
      try {
        const res = await fetch('/api/v1/auth/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: refreshToken }),
        })
        if (!res.ok) return false
        const data: AuthResponse = await res.json()
        storeSession(data)
        return true
      } catch {
        return false
      } finally {
        refreshPromise = null
      }
    })()
  }
  return refreshPromise
}

function withAuthHeaders(init?: RequestInit): RequestInit {
  // Skip the default JSON content type for FormData bodies (e.g. file
  // uploads) — fetch needs to set its own multipart boundary header instead.
  const defaultHeaders: Record<string, string> = { Authorization: `Bearer ${getAccessToken() ?? ''}` }
  if (!(init?.body instanceof FormData)) {
    defaultHeaders['Content-Type'] = 'application/json'
  }
  return {
    ...init,
    headers: { ...defaultHeaders, ...(init?.headers ?? {}) },
  }
}

// authFetch is the single place that knows how to recover from an expired
// access token: on a 401 it silently refreshes and retries once, and only
// gives up (clearing the session and notifying the app) if the refresh
// token itself is missing, expired, or revoked.
export async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  let res = await fetch(path, withAuthHeaders(init))
  if (res.status === 401 && getRefreshToken() && (await refreshAccessToken())) {
    res = await fetch(path, withAuthHeaders(init))
  }
  if (res.status === 401) {
    notifySessionExpired()
  }
  return res
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await authFetch(path, init)
  if (!res.ok) {
    throw new Error(`${res.status} ${await res.text()}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export async function apiFetchBlob(path: string, init?: RequestInit): Promise<Blob> {
  const res = await authFetch(path, init)
  if (!res.ok) {
    throw new Error(`${res.status} ${await res.text()}`)
  }
  return res.blob()
}
