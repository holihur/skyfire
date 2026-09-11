import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { api, isUnauthorized } from '../lib/api'

interface AuthState {
  authed: boolean
  loading: boolean
  login: (username: string, password: string, remember?: boolean) => Promise<boolean>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState>({
  authed: false,
  loading: true,
  login: async () => false,
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

  const login = useCallback(async (username: string, password: string) => {
    try {
      await api.login(username, password)
      setAuthed(true)
      return true
    } catch {
      return false
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