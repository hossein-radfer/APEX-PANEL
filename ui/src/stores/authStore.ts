import Cookies from 'js-cookie'
import { create } from 'zustand'

interface AuthAdmin {
  user_id: number
  username: string
  role: string
  reseller_id: number | null
}

interface AuthState {
  auth: {
    admin: AuthAdmin | null
    setAdmin: (admin: AuthAdmin) => void
    accessToken: string
    setAccessToken: (accessToken: string) => void
    resetAccessToken: () => void
    reset: () => void
  }
}

export const useAuthStore = create<AuthState>()((set) => {
  const cookieState = Cookies.get('access_token')
  const initToken = cookieState ? cookieState : ''
  const initUsername = Cookies.get('username')
  const initRole = Cookies.get('role')
  const initResellerId = Cookies.get('reseller_id')

  const initAdmin =
    initUsername && initRole
      ? {
          user_id: 0,
          username: initUsername,
          role: initRole,
          reseller_id: initResellerId ? Number(initResellerId) : null,
        }
      : null

  return {
    auth: {
      admin: initAdmin,
      setAdmin: (admin) =>
        set((state) => {
          Cookies.set('username', admin?.username)
          Cookies.set('role', admin?.role)
          if (admin?.reseller_id !== null && admin?.reseller_id !== undefined) {
            Cookies.set('reseller_id', String(admin.reseller_id))
          } else {
            Cookies.remove('reseller_id')
          }
          return { ...state, auth: { ...state.auth, admin } }
        }),
      accessToken: initToken,
      setAccessToken: (accessToken) =>
        set((state) => {
          Cookies.set('access_token', accessToken)
          return { ...state, auth: { ...state.auth, accessToken } }
        }),
      resetAccessToken: () =>
        set((state) => {
          Cookies.remove('access_token')
          return { ...state, auth: { ...state.auth, accessToken: '' } }
        }),
      reset: () =>
        set((state) => {
          Cookies.remove('access_token')
          Cookies.remove('username')
          Cookies.remove('role')
          Cookies.remove('reseller_id')
          return {
            ...state,
            auth: { ...state.auth, admin: null, accessToken: '' },
          }
        }),
    },
  }
})
