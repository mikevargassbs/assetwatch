import { useEffect, useRef, useState } from 'react'

interface BarcodeScannerModalProps {
  onDetected: (value: string) => void
  onClose: () => void
}

// Uses the browser's native Barcode Detection API against a live camera
// feed. Support is Chrome/Edge/Android WebView only (no Safari, no
// Firefox) — on unsupported browsers we say so up front instead of
// showing a dead camera preview.
export function BarcodeScannerModal({ onDetected, onClose }: BarcodeScannerModalProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const rafRef = useRef<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const supported = typeof window !== 'undefined' && 'BarcodeDetector' in window

  useEffect(() => {
    if (!supported) return

    let cancelled = false
    const detector = new BarcodeDetector()

    async function start() {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: 'environment' },
        })
        if (cancelled) {
          stream.getTracks().forEach((t) => t.stop())
          return
        }
        streamRef.current = stream
        if (videoRef.current) {
          videoRef.current.srcObject = stream
          await videoRef.current.play()
        }
        scan()
      } catch {
        if (!cancelled) setError('Could not access the camera. Check permissions and try again.')
      }
    }

    async function scan() {
      if (cancelled || !videoRef.current) return
      try {
        const codes = await detector.detect(videoRef.current)
        if (codes.length > 0) {
          onDetected(codes[0].rawValue)
          return
        }
      } catch {
        // Transient decode errors (e.g. frame mid-transition) are expected — keep scanning.
      }
      rafRef.current = requestAnimationFrame(() => {
        scan()
      })
    }

    start()

    return () => {
      cancelled = true
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current)
      streamRef.current?.getTracks().forEach((t) => t.stop())
      streamRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [supported])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card barcode-scanner-card" onClick={(e) => e.stopPropagation()}>
        <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
          &times;
        </button>
        <div className="modal-header">
          <h2>Scan Barcode</h2>
        </div>
        <div className="modal-body">
          {!supported && (
            <p className="login-error">
              Camera scanning isn't supported in this browser. Try Chrome or Edge on Android, or enter the code
              manually.
            </p>
          )}
          {supported && error && <p className="login-error">{error}</p>}
          {supported && !error && (
            <div className="barcode-scanner-viewport">
              <video ref={videoRef} muted playsInline className="barcode-scanner-video" />
              <div className="barcode-scanner-reticle" />
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}
