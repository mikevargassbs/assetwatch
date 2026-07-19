import { apiFetch, authFetch } from './client'

export interface ClientAcceptance {
  id: string
  hardware_unit_id: string
  method?: 'e_signature' | 'manual_upload'
  bsp_acceptance_by?: string
  bsp_signed_by_name?: string
  bsp_signature?: string
  bsp_acceptance_date?: string
  client_signature_link_expires_at?: string
  client_email?: string
  client_link_emailed_at?: string
  client_name?: string
  client_signed_at?: string
  client_signature_data?: string
  uploaded_document_path?: string
  head_office_signature_link_expires_at?: string
  head_office_email?: string
  head_office_link_emailed_at?: string
  head_office_name?: string
  head_office_signed_at?: string
  head_office_signature_data?: string
  comments?: string
  signed_off_at?: string
  meta_data: Record<string, unknown>
}

export function getAcceptance(unitId: string) {
  return apiFetch<ClientAcceptance>(`/api/v1/hardware-units/${unitId}/acceptance`)
}

export function recordBSPAcceptance(
  unitId: string,
  input: {
    signed_by_name?: string
    signature: string
    comments?: string
    meta_data?: Record<string, unknown>
  },
) {
  return apiFetch<ClientAcceptance>(`/api/v1/hardware-units/${unitId}/acceptance/bsp-signoff`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function generateClientSigningLink(unitId: string) {
  return apiFetch<{ token: string }>(`/api/v1/hardware-units/${unitId}/acceptance/generate-client-link`, {
    method: 'POST',
  })
}

export function emailClientSigningLink(unitId: string, clientEmail: string) {
  return apiFetch<{ acceptance: ClientAcceptance; sent: boolean }>(
    `/api/v1/hardware-units/${unitId}/acceptance/email-client-link`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ client_email: clientEmail }),
    },
  )
}

export function generateHeadOfficeSigningLink(unitId: string) {
  return apiFetch<{ token: string }>(`/api/v1/hardware-units/${unitId}/acceptance/generate-head-office-link`, {
    method: 'POST',
  })
}

export function emailHeadOfficeSigningLink(unitId: string, clientEmail: string) {
  return apiFetch<{ acceptance: ClientAcceptance; sent: boolean }>(
    `/api/v1/hardware-units/${unitId}/acceptance/email-head-office-link`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ client_email: clientEmail }),
    },
  )
}

export async function uploadManualDocument(unitId: string, file: File, clientName: string) {
  const form = new FormData()
  form.append('document', file)
  form.append('client_name', clientName)
  const res = await authFetch(`/api/v1/hardware-units/${unitId}/acceptance/manual-upload`, {
    method: 'POST',
    body: form,
  })
  if (!res.ok) {
    throw new Error(`${res.status} ${await res.text()}`)
  }
  return res.json() as Promise<ClientAcceptance>
}

export function documentDownloadUrl(unitId: string) {
  return `/api/v1/hardware-units/${unitId}/acceptance/document`
}

// ---- Public (no auth) ----

export interface PublicSigningInfo {
  hardware_barcode: string
  stage: 'branch_manager' | 'head_office'
  already_signed: boolean
  expired: boolean
}

export async function getPublicSigningInfo(token: string): Promise<PublicSigningInfo> {
  const res = await fetch(`/api/v1/public/acceptance/${token}`)
  if (!res.ok) {
    throw new Error(`${res.status} ${await res.text()}`)
  }
  return res.json()
}

export async function submitClientSignature(token: string, clientName: string, signatureData: string): Promise<void> {
  const res = await fetch(`/api/v1/public/acceptance/${token}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ client_name: clientName, signature_data: signatureData }),
  })
  if (!res.ok) {
    throw new Error(`${res.status} ${await res.text()}`)
  }
}
