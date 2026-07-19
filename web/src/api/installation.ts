import { apiFetch, authFetch } from './client'

export interface SiteInstallation {
  id: string
  hardware_unit_id: string
  received_on_site_at?: string
  confirmed_correct?: boolean
  discrepancy_notes?: string
  escalated_to_pmpc_at?: string
  date_installed?: string
  installed_by?: string
  installed_by_name?: string
  installed_at?: string
  inspected_by?: string
  inspected_by_name?: string
  inspected_at?: string
  fit_focus_by?: string
  fit_focus_by_name?: string
  fit_focus_completed_at?: string
  network_attached?: boolean
  device_contactable?: boolean
  ping_checked_at?: string
  installed_location?: string
  installed_height_m?: number
  signed_off_at?: string
  site_name?: string
  site_location?: string
  site_ip?: string
  site_subnet?: string
  site_gateway?: string
  deployment_date?: string
  deployment_team?: string
  team_leader?: string
  meta_data: Record<string, unknown>
}

export function getInstallation(unitId: string) {
  return apiFetch<SiteInstallation>(`/api/v1/hardware-units/${unitId}/installation`)
}

export function recordSiteReceipt(unitId: string, input: { confirmed_correct: boolean; discrepancy_notes?: string }) {
  return apiFetch<SiteInstallation>(`/api/v1/hardware-units/${unitId}/installation/site-receipt`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function upsertInstallation(
  unitId: string,
  input: {
    installed_location?: string
    installed_height_m?: number
    network_attached?: boolean
    device_contactable?: boolean
    site_name?: string
    site_location?: string
    site_ip?: string
    site_subnet?: string
    site_gateway?: string
    deployment_date?: string
    deployment_team?: string
    team_leader?: string
    meta_data: Record<string, unknown>
  },
) {
  return apiFetch<SiteInstallation>(`/api/v1/hardware-units/${unitId}/installation`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function checkContactability(unitId: string) {
  return apiFetch<SiteInstallation>(`/api/v1/hardware-units/${unitId}/installation/contactability-check`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export function signOffInstallation(unitId: string, step: 'installed' | 'inspected' | 'fit_focus', performedAt?: string) {
  return apiFetch<SiteInstallation>(`/api/v1/hardware-units/${unitId}/installation/sign-off`, {
    method: 'POST',
    body: JSON.stringify({ step, performed_at: performedAt ?? null }),
  })
}

export interface InstallationPhoto {
  id: string
  hardware_unit_id: string
  uploaded_by?: string
  uploaded_at: string
}

export function listInstallationPhotos(unitId: string) {
  return apiFetch<InstallationPhoto[]>(`/api/v1/hardware-units/${unitId}/installation/photos`)
}

export async function uploadInstallationPhoto(unitId: string, file: File): Promise<InstallationPhoto> {
  const form = new FormData()
  form.append('photo', file)
  const res = await authFetch(`/api/v1/hardware-units/${unitId}/installation/photos`, { method: 'POST', body: form })
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  return res.json() as Promise<InstallationPhoto>
}

export async function deleteInstallationPhoto(unitId: string, photoId: string): Promise<void> {
  const res = await authFetch(`/api/v1/hardware-units/${unitId}/installation/photos/${photoId}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
}

export function installationPhotoUrl(unitId: string, photoId: string) {
  return `/api/v1/hardware-units/${unitId}/installation/photos/${photoId}`
}
