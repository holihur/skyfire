import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { api, isUnauthorized, type LoginResponse } from '../lib/api'

interface LoginResult {
  ok: boolean
  networkError: boolean
  /** Server body for 2FA/enrollment continuation (present on non-network errors). */
  body?: LoginResponse
  /** Server error message (e.g. invalid two-factor code). */
  error?: string
}

interface AuthState {
  authed: boolean
  loading: boolean
  login: (username: string, password: string, totp?: string) => Promise<LoginResult>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState>({
  authed: false,
  loading: true,
  login: async () => ({ ok: false, networkError: false }),
  logout: async () => {},
})

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [loading, setLoading] = useState(true)
  const [authed, setAuthed] = useState(false)

  const verify = useCallback(async () => {
    try {
      await api.health()
      setAuthed(true)
    } catch (err) {
      if (isUnauthorized(err)) {
        setAuthed(false)
      } else {
        // daemon unreachable: still render the app so errors are visible
        setAuthed(true)
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void verify()
  }, [verify])

  useEffect(() => {
    const onUnauthorized = () => setAuthed(false)
    window.addEventListener('skyfire:unauthorized', onUnauthorized)
    return () => window.removeEventListener('skyfire:unauthorized', onUnauthorized)
  }, [])

  const login = useCallback(async (username: string, password: string, totp?: string): Promise<LoginResult> => {
    try {
      const res = await api.login(username, password, totp)
      if (res?.ok) {
        setAuthed(true)
        return { ok: true, networkError: false, body: res }
      }
      // 200 without ok:true carries a 2FA / enrollment continuation
      return { ok: false, networkError: false, body: res }
    } catch (err) {
      // only 401 means bad credentials/code; anything else is a network failure
      return {
        ok: false,
        networkError: !isUnauthorized(err),
        error: err instanceof Error ? err.message : String(err),
      }
    }
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      /* session cleared locally regardless */
    }
    setAuthed(false)
  }, [])

  const value = useMemo(
    () => ({ authed, loading, login, logout }),
    [authed, loading, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  return useContext(AuthContext)
}