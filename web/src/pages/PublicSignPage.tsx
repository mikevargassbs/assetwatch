import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { SignaturePad } from '../components/SignaturePad'
import { getPublicSigningInfo, submitClientSignature, type PublicSigningInfo } from '../api/acceptance'

export function PublicSignPage() {
  const { token } = useParams<{ token: string }>()
  const [info, setInfo] = useState<PublicSigningInfo | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [clientName, setClientName] = useState('')
  const [signatureData, setSignatureData] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)

  useEffect(() => {
    if (!token) return
    getPublicSigningInfo(token)
      .then(setInfo)
      .catch(() => setError('This signing link is invalid or has expired.'))
  }, [token])

  async function handleSubmit() {
    if (!token || !clientName || !signatureData) return
    setSubmitting(true)
    setError(null)
    try {
      await submitClientSignature(token, clientName, signatureData)
      setSubmitted(true)
    } catch (err) {
      if (err instanceof Error && err.message.startsWith('409')) {
        setError('This unit has already been signed off.')
      } else {
        setError('Failed to submit signature. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const roleLabel = info?.stage === 'head_office' ? 'Head Office / Security Manager' : 'Branch Manager'

  return (
    <div className="login-page">
      <div className="login-card" style={{ maxWidth: 520 }}>
        <h1>{roleLabel} Acceptance Sign-Off</h1>

        {error && <div className="login-error">{error}</div>}

        {!error && !info && <p className="login-subtitle">Loading…</p>}

        {info && info.expired && <p className="login-subtitle">This signing link has expired.</p>}

        {info && (info.already_signed || submitted) && !info.expired && (
          <p className="login-subtitle">
            Thank you — acceptance for unit <strong>{info.hardware_barcode}</strong> has been recorded.
          </p>
        )}

        {info && !info.already_signed && !info.expired && !submitted && (
          <>
            <p className="login-subtitle">
              Please review and sign as <strong>{roleLabel}</strong> to confirm acceptance of unit{' '}
              <strong>{info.hardware_barcode}</strong>.
            </p>

            <label htmlFor="clientName">Your Name</label>
            <input id="clientName" value={clientName} onChange={(e) => setClientName(e.target.value)} required />

            <label>Signature</label>
            <SignaturePad onChange={setSignatureData} />

            <button
              type="button"
              className="btn btn-primary"
              disabled={submitting || !clientName || !signatureData}
              onClick={handleSubmit}
              style={{ marginTop: 20 }}
            >
              {submitting ? 'Submitting…' : 'Submit Acceptance'}
            </button>
          </>
        )}
      </div>
    </div>
  )
}
