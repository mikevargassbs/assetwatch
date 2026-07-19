import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'

interface ConfirmOptions {
  title: string
  message?: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
}

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>

const ConfirmDialogContext = createContext<ConfirmFn | undefined>(undefined)

export function useConfirm(): ConfirmFn {
  const ctx = useContext(ConfirmDialogContext)
  if (!ctx) {
    throw new Error('useConfirm must be used within ConfirmDialogProvider')
  }
  return ctx
}

export function ConfirmDialogProvider({ children }: { children: ReactNode }) {
  const [options, setOptions] = useState<ConfirmOptions | null>(null)
  const resolver = useRef<(value: boolean) => void>(undefined)

  const confirm = useCallback<ConfirmFn>((opts) => {
    setOptions(opts)
    return new Promise((resolve) => {
      resolver.current = resolve
    })
  }, [])

  function settle(result: boolean) {
    setOptions(null)
    resolver.current?.(result)
    resolver.current = undefined
  }

  const value = useMemo(() => confirm, [confirm])

  return (
    <ConfirmDialogContext.Provider value={value}>
      {children}
      {options && (
        <div className="modal-overlay">
          <div className="modal-card confirm-dialog" onClick={(e) => e.stopPropagation()}>
            <h2>{options.title}</h2>
            {options.message && <p className="modal-subtitle">{options.message}</p>}
            <div className="modal-actions">
              <button type="button" className="btn" onClick={() => settle(false)}>
                {options.cancelLabel ?? 'Cancel'}
              </button>
              <button
                type="button"
                className={`btn ${options.danger ? 'btn-danger' : 'btn-primary'}`}
                onClick={() => settle(true)}
                autoFocus
              >
                {options.confirmLabel ?? 'Confirm'}
              </button>
            </div>
          </div>
        </div>
      )}
    </ConfirmDialogContext.Provider>
  )
}
