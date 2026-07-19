import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'

type SnackbarKind = 'success' | 'error' | 'info'

interface Snackbar {
  id: number
  message: string
  kind: SnackbarKind
}

type ShowSnackbarFn = (message: string, kind?: SnackbarKind) => void

const SnackbarContext = createContext<ShowSnackbarFn | undefined>(undefined)

export function useSnackbar(): ShowSnackbarFn {
  const ctx = useContext(SnackbarContext)
  if (!ctx) {
    throw new Error('useSnackbar must be used within SnackbarProvider')
  }
  return ctx
}

const AUTO_DISMISS_MS = 5000

export function SnackbarProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Snackbar[]>([])
  const nextId = useRef(0)

  const show = useCallback<ShowSnackbarFn>((message, kind = 'info') => {
    const id = nextId.current++
    setItems((prev) => [...prev, { id, message, kind }])
    setTimeout(() => {
      setItems((prev) => prev.filter((s) => s.id !== id))
    }, AUTO_DISMISS_MS)
  }, [])

  const value = useMemo(() => show, [show])

  return (
    <SnackbarContext.Provider value={value}>
      {children}
      <div className="snackbar-stack">
        {items.map((s) => (
          <div key={s.id} className={`snackbar snackbar-${s.kind}`}>
            {s.message}
            <button
              type="button"
              className="snackbar-dismiss"
              onClick={() => setItems((prev) => prev.filter((i) => i.id !== s.id))}
              aria-label="Dismiss"
            >
              &times;
            </button>
          </div>
        ))}
      </div>
    </SnackbarContext.Provider>
  )
}
