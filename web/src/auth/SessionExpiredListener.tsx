import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { SESSION_EXPIRED_EVENT } from '../api/client'
import { useSnackbar } from '../components/SnackbarProvider'
import { useAuth } from './AuthContext'

// Mounted once near the app root: reacts to the SESSION_EXPIRED_EVENT that
// api/client.ts fires when a request's access token is rejected and the
// refresh token can't renew it either, so an expired session surfaces as a
// clear message and a redirect instead of pages just failing silently.
export function SessionExpiredListener() {
  const navigate = useNavigate()
  const showSnackbar = useSnackbar()
  const { forceLogout } = useAuth()

  useEffect(() => {
    function handleSessionExpired() {
      forceLogout()
      showSnackbar('Your session has expired. Please log in again.', 'error')
      navigate('/login', { replace: true })
    }
    window.addEventListener(SESSION_EXPIRED_EVENT, handleSessionExpired)
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, handleSessionExpired)
  }, [navigate, showSnackbar, forceLogout])

  return null
}
