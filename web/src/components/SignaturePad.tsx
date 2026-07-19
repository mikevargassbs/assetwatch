import { useRef, type PointerEvent } from 'react'

interface SignaturePadProps {
  onChange: (dataUrl: string | null) => void
}

export function SignaturePad({ onChange }: SignaturePadProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const drawing = useRef(false)
  const hasDrawn = useRef(false)

  function getContext() {
    const canvas = canvasRef.current
    if (!canvas) return null
    return canvas.getContext('2d')
  }

  function pointerPos(e: PointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current!
    const rect = canvas.getBoundingClientRect()
    // The canvas's on-screen size (rect) is scaled by CSS and can differ
    // from its internal drawing-buffer size (canvas.width/height), e.g. on
    // phones narrower than the 500px max-width. Without rescaling here, the
    // stroke lands wherever the point would fall at native 500x160 size,
    // not where the finger/pen actually touched.
    const scaleX = canvas.width / rect.width
    const scaleY = canvas.height / rect.height
    return { x: (e.clientX - rect.left) * scaleX, y: (e.clientY - rect.top) * scaleY }
  }

  function handlePointerDown(e: PointerEvent<HTMLCanvasElement>) {
    const ctx = getContext()
    if (!ctx) return
    drawing.current = true
    const { x, y } = pointerPos(e)
    ctx.beginPath()
    ctx.moveTo(x, y)
    canvasRef.current?.setPointerCapture(e.pointerId)
  }

  function handlePointerMove(e: PointerEvent<HTMLCanvasElement>) {
    if (!drawing.current) return
    const ctx = getContext()
    if (!ctx) return
    const { x, y } = pointerPos(e)
    ctx.lineWidth = 2
    ctx.lineCap = 'round'
    ctx.strokeStyle = '#18181b'
    ctx.lineTo(x, y)
    ctx.stroke()
    hasDrawn.current = true
  }

  function handlePointerUp() {
    drawing.current = false
    const canvas = canvasRef.current
    if (canvas && hasDrawn.current) {
      onChange(canvas.toDataURL('image/png'))
    }
  }

  function clear() {
    const canvas = canvasRef.current
    const ctx = getContext()
    if (canvas && ctx) {
      ctx.clearRect(0, 0, canvas.width, canvas.height)
    }
    hasDrawn.current = false
    onChange(null)
  }

  return (
    <div className="signature-pad">
      <canvas
        ref={canvasRef}
        width={500}
        height={160}
        className="signature-canvas"
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerLeave={handlePointerUp}
      />
      <button type="button" className="link-button" onClick={clear}>
        Clear signature
      </button>
    </div>
  )
}
