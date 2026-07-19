import { authFetch } from './client'

export const WIPE_CONFIRMATION_PHRASE = 'DELETE ALL TRANSACTIONS'

export async function downloadTransactionDataExport() {
  const res = await authFetch('/api/v1/data-management/export')
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  const blob = await res.blob()
  const objectUrl = URL.createObjectURL(blob)
  const contentDisposition = res.headers.get('Content-Disposition') ?? ''
  const match = contentDisposition.match(/filename="([^"]+)"/)
  const a = document.createElement('a')
  a.href = objectUrl
  a.download = match ? match[1] : 'transaction-data-export.json'
  a.click()
  URL.revokeObjectURL(objectUrl)
}

export async function wipeAllTransactionData() {
  const res = await authFetch('/api/v1/data-management/wipe', {
    method: 'POST',
    body: JSON.stringify({ confirm: WIPE_CONFIRMATION_PHRASE }),
  })
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
}
