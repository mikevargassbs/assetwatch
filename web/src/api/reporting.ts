import { apiFetch, authFetch } from './client'

export interface HardwareSummaryRow {
  hardware_unit_id: string
  barcode: string
  status: string
  current_stage: string
  board_column: string
  is_exception: boolean
  allocated_branch?: string
  created_at: string
  device_make?: string
  device_model?: string
  serial_number?: string
  mac_address?: string
  site_name?: string
  site_location?: string
  deployment_team?: string
  shipped_via?: string
  waybill_number?: string
  defect_type?: string
  defect_replacement_status?: string
}

export interface HardwareFilters {
  unit_id?: string
  stage?: string
  status?: string
  site_name?: string
  device_model?: string
  date_from?: string
  date_to?: string
}

function buildQuery(filters: HardwareFilters, extra?: Record<string, string>): string {
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(filters)) {
    if (v) params.set(k, v)
  }
  if (extra) {
    for (const [k, v] of Object.entries(extra)) {
      if (v) params.set(k, v)
    }
  }
  return params.toString()
}

export function getHardwareSummary(filters: HardwareFilters) {
  return apiFetch<HardwareSummaryRow[]>(`/api/v1/reports/hardware-summary?${buildQuery(filters)}`)
}

export function hardwareSummaryExportUrl(filters: HardwareFilters, format: 'csv' | 'pdf') {
  return `/api/v1/reports/hardware-summary?${buildQuery(filters, { format })}`
}

export function getDefectsReport(filters: HardwareFilters) {
  return apiFetch<HardwareSummaryRow[]>(`/api/v1/reports/defects?${buildQuery(filters)}`)
}

export function defectsReportExportUrl(filters: HardwareFilters, format: 'csv' | 'pdf') {
  return `/api/v1/reports/defects?${buildQuery(filters, { format })}`
}

export function unitInfoSheetUrl(unitId: string) {
  return `/api/v1/hardware-units/${unitId}/report`
}

export function packingListUrl(siteName: string) {
  return `/api/v1/reports/packing-list?${buildQuery({}, { site_name: siteName })}`
}

export async function downloadAuthenticated(url: string) {
  const res = await authFetch(url)
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  const blob = await res.blob()
  const objectUrl = URL.createObjectURL(blob)
  const contentDisposition = res.headers.get('Content-Disposition') ?? ''
  const match = contentDisposition.match(/filename="([^"]+)"/)
  if (match && (res.headers.get('Content-Type') ?? '').includes('csv')) {
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = match[1]
    a.click()
  } else {
    window.open(objectUrl, '_blank')
  }
}
