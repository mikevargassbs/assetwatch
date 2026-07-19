import { useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { resetPassword } from '../api/client'

export function ResetPasswordPage() {
  const { token } = useParams<{ token: string }>()
  const navigate = useNavigate()
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!token) return
    setError(null)

    if (newPassword.length < 8) {
      setError('Password must be at least 8 characters.')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }

    setSubmitting(true)
    try {
      await resetPassword(token, newPassword)
      setDone(true)
    } catch {
      setError('This reset link is invalid or has expired. Please request a new one.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-brand">
          <img className="login-logo" src="/AssetWatchLogo.png" alt="" />
          <h1>SBS AssetWatch</h1>
        </div>

        {done ? (
          <>
            <p className="login-subtitle">Your password has been reset. You can now sign in.</p>
            <button type="button" className="login-submit" onClick={() => navigate('/login', { replace: true })}>
              Go to sign in
            </button>
          </>
        ) : (
          <form onSubmit={handleSubmit}>
            <p className="login-subtitle">Choose a new password for your account.</p>

            <label htmlFor="newPassword">New Password</label>
            <div className="login-input-group">
              <input
                id="newPassword"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                autoComplete="new-password"
                required
              />
            </div>

            <label htmlFor="confirmPassword">Confirm Password</label>
            <div className="login-input-group">
              <input
                id="confirmPassword"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                autoComplete="new-password"
                required
              />
            </div>

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

            <button type="submit" className="login-submit" disabled={submitting} style={{ marginTop: 8 }}>
              {submitting && <span className="login-spinner" aria-hidden="true" />}
              {submitting ? 'Resetting…' : 'Reset password'}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
