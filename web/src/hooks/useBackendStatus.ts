import { useEffect, useRef, useState } from 'react'

export type BackendStatus = 'checking' | 'online' | 'offline'

export type BackendInfo = {
  status: BackendStatus
  version: string | null
}

const POLL_INTERVAL_MS = 15000
const REQUEST_TIMEOUT_MS = 5000

async function pingBackend(): Promise<string | null> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
  try {
    const res = await fetch('/healthz', { signal: controller.signal, cache: 'no-store' })
    if (!res.ok) return null
    const body = (await res.json().catch(() => null)) as { version?: string } | null
    return body?.version ?? ''
  } catch {
    return null
  } finally {
    clearTimeout(timeout)
  }
}

// Polls the API's /healthz on an interval, plus immediately whenever the
// browser regains network connectivity, so the indicator reacts faster than
// the poll interval would alone.
export function useBackendStatus(): BackendInfo {
  const [info, setInfo] = useState<BackendInfo>({ status: 'checking', version: null })
  const mounted = useRef(true)

  useEffect(() => {
    mounted.current = true

    async function check() {
      const version = await pingBackend()
      if (mounted.current) {
        setInfo(version === null ? { status: 'offline', version: null } : { status: 'online', version })
      }
    }

    check()
    const interval = setInterval(check, POLL_INTERVAL_MS)
    window.addEventListener('online', check)
    window.addEventListener('offline', check)

    return () => {
      mounted.current = false
      clearInterval(interval)
      window.removeEventListener('online', check)
      window.removeEventListener('offline', check)
    }
  }, [])

  return info
}
