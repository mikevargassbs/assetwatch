import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { requestPasswordReset } from '../api/client'
import { useBackendStatus } from '../hooks/useBackendStatus'

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const { version } = useBackendStatus()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const [showForgotPassword, setShowForgotPassword] = useState(false)
  const [resetEmail, setResetEmail] = useState('')
  const [resetSubmitting, setResetSubmitting] = useState(false)
  const [resetSent, setResetSent] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(email, password)
      navigate('/board', { replace: true })
    } catch {
      setError('Invalid email or password.')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleForgotPasswordSubmit(e: FormEvent) {
    e.preventDefault()
    setResetSubmitting(true)
    try {
      await requestPasswordReset(resetEmail)
    } finally {
      setResetSubmitting(false)
      setResetSent(true)
    }
  }

  function openForgotPassword() {
    setResetEmail(email)
    setResetSent(false)
    setShowForgotPassword(true)
  }

  if (showForgotPassword) {
    return (
      <div className="login-page">
        <form className="login-card" onSubmit={handleForgotPasswordSubmit}>
          <div className="login-brand">
            <img className="login-logo" src="/AssetWatchLogo.png" alt="" />
            <h1>SBS AssetWatch</h1>
          </div>

          {resetSent ? (
            <p className="login-subtitle">
              If an account exists for that email, we've sent a link to reset the password. Check your inbox.
            </p>
          ) : (
            <>
              <p className="login-subtitle">
                Enter your @sbs.com.pg account email and we'll send you a link to reset your password.
              </p>

              <label htmlFor="resetEmail">Email</label>
              <div className="login-input-group">
                <input
                  id="resetEmail"
                  type="text"
                  value={resetEmail}
                  onChange={(e) => setResetEmail(e.target.value)}
                  placeholder="you or you@sbs.com.pg"
                  autoComplete="username"
                  data-no-uppercase
                  required
                />
              </div>

              <button type="submit" className="login-submit" disabled={resetSubmitting}>
                {resetSubmitting && <span className="login-spinner" aria-hidden="true" />}
                {resetSubmitting ? 'Sending…' : 'Send reset link'}
              </button>
            </>
          )}

          <button
            type="button"
            className="login-submit"
            style={{ marginTop: 12, background: 'transparent', color: 'inherit' }}
            onClick={() => setShowForgotPassword(false)}
          >
            Back to sign in
          </button>
        </form>
        {version && <span className="login-version">v{version}</span>}
      </div>
    )
  }

  return (
    <div className="login-page">
      <form className="login-card" onSubmit={handleSubmit}>
        <div className="login-brand">
          <img className="login-logo" src="/AssetWatchLogo.png" alt="" />
          <h1>SBS AssetWatch</h1>
        </div>
        <p className="login-subtitle">Sign in with your @sbs.com.pg account</p>

        <label htmlFor="email">Username</label>
        <div className="login-input-group">
          <svg className="login-input-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
            <path
              d="M3 5.5A1.5 1.5 0 0 1 4.5 4h11A1.5 1.5 0 0 1 17 5.5v9a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 3 14.5v-9Z"
              stroke="currentColor"
              strokeWidth="1.4"
            />
            <path d="m4 5.5 6 5 6-5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <input
            id="email"
            type="text"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you or you@sbs.com.pg"
            autoComplete="username"
            data-no-uppercase
            required
          />
        </div>

        <label htmlFor="password">Password</label>
        <div className="login-input-group">
          <svg className="login-input-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
            <rect x="4" y="9" width="12" height="7.5" rx="1.5" stroke="currentColor" strokeWidth="1.4" />
            <path d="M6.5 9V6.5a3.5 3.5 0 1 1 7 0V9" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
          </svg>
          <input
            id="password"
            type={showPassword ? 'text' : 'password'}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
          <button
            type="button"
            className="login-password-toggle"
            onClick={() => setShowPassword((v) => !v)}
            aria-label={showPassword ? 'Hide password' : 'Show password'}
            tabIndex={-1}
          >
            {showPassword ? (
              <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path
                  d="M3 3l14 14M9.5 5.1c.16-.01.33-.01.5-.01 4 0 6.83 3.13 7.5 4.91-.28.75-1 2.06-2.15 3.2m-2.62 1.6A7.9 7.9 0 0 1 10 15c-4 0-6.83-3.13-7.5-4.91.34-.9 1.24-2.4 2.66-3.6"
                  stroke="currentColor"
                  strokeWidth="1.4"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
                <path d="M8.1 8.1a2.25 2.25 0 0 0 3.18 3.18" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
              </svg>
            ) : (
              <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path
                  d="M2.5 10S5.33 4.9 10 4.9 17.5 10 17.5 10 14.67 15.1 10 15.1 2.5 10 2.5 10Z"
                  stroke="currentColor"
                  strokeWidth="1.4"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
                <circle cx="10" cy="10" r="2.25" stroke="currentColor" strokeWidth="1.4" />
              </svg>
            )}
          </button>
        </div>

        <button
          type="button"
          className="login-forgot-password"
          style={{
            alignSelf: 'flex-end',
            background: 'none',
            border: 'none',
            padding: 0, 
            paddingTop: 4,
            marginBottom: 2,
            font: 'inherit',
            color: 'inherit',
            cursor: 'pointer',
            textDecoration: 'underline',
          }}
          onClick={openForgotPassword}
        >
          Forgot password?
        </button>

        {error && (
          <div className="login-error">
            <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <circle cx="10" cy="10" r="7.25" stroke="currentColor" strokeWidth="1.4" />
              <path d="M10 6.5v4" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
              <circle cx="10" cy="13.25" r="0.9" fill="currentColor" />
            </svg>
            {error}
          </div>
        )}

        <button type="submit" className="login-submit" disabled={submitting}>
          {submitting && <span className="login-spinner" aria-hidden="true" />}
          {submitting ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
      {version && <span className="login-version">v{version}</span>}
    </div>
  )
}
