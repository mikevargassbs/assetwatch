import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import * as api from '../api/client'

interface AuthContextValue {
  email: string | null
  roles: string[]
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
  forceLogout: () => void
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [email, setEmail] = useState<string | null>(api.getCurrentEmail())
  const [roles, setRoles] = useState<string[]>(api.getCurrentRoles())

  const value = useMemo<AuthContextValue>(
    () => ({
      email,
      roles,
      isAuthenticated: !!api.getAccessToken(),
      login: async (emailInput, password) => {
        const auth = await api.login(emailInput, password)
        setEmail(auth.email)
        setRoles(auth.roles)
      },
      logout: async () => {
        await api.logout()
        setEmail(null)
        setRoles([])
      },
      // Used when the server has already rejected the session (expired/
      // revoked refresh token) and api/client.ts has cleared storage — just
      // resets local state without another round-trip to /auth/logout.
      forceLogout: () => {
        setEmail(null)
        setRoles([])
      },
    }),
    [email, roles],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return ctx
}
