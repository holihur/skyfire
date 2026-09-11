import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { api, isUnauthorized } from '../lib/api'

interface LoginResult {
  ok: boolean
  networkError: boolean
}

interface AuthState {
  authed: boolean
  loading: boolean
  login: (username: string, password: string, remember?: boolean) => Promise<LoginResult>
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

  const login = useCallback(async (username: string, password: string): Promise<LoginResult> => {
    try {
      const res = await api.login(username, password)
      if (res?.ok) {
        setAuthed(true)
        return { ok: true, networkError: false }
      }
      // 200 without ok:true (e.g. empty response): the daemon is not answering properly
      return { ok: false, networkError: true }
    } catch (err) {
      // only 401 means bad credentials; anything else is a network/daemon failure
      return { ok: false, networkError: !isUnauthorized(err) }
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