import { apiFetch } from './client'

export interface DefectReport {
  id: string
  hardware_unit_id: string
  declared_by?: string
  declared_date: string
  defect_type: 'defective' | 'damaged' | 'wrong_item'
  description?: string
  report_generated_at?: string
  emailed_to_supplier_at?: string
  supplier_email?: string
  replacement_status: 'pending' | 'shipped_back' | 'replacement_received'
  replacement_hardware_unit_id?: string
  tracking_number?: string
  carrier?: string
  shipped_back_at?: string
  delivered_at?: string
  supplier_received_at?: string
  meta_data: Record<string, unknown>
}

export function getDefectReport(unitId: string) {
  return apiFetch<DefectReport>(`/api/v1/hardware-units/${unitId}/defect`)
}

export function declareDefect(unitId: string, defectType: string, description?: string) {
  return apiFetch<DefectReport>(`/api/v1/hardware-units/${unitId}/defect`, {
    method: 'POST',
    body: JSON.stringify({ defect_type: defectType, description }),
  })
}

export function reportUrl(unitId: string) {
  return `/api/v1/hardware-units/${unitId}/defect/report`
}

export function emailSupplier(unitId: string, supplierEmail: string) {
  return apiFetch<{ acceptance: DefectReport; sent: boolean }>(`/api/v1/hardware-units/${unitId}/defect/email-supplier`, {
    method: 'POST',
    body: JSON.stringify({ supplier_email: supplierEmail }),
  })
}

export function markShippedBack(unitId: string, trackingNumber?: string, carrier?: string) {
  return apiFetch<DefectReport>(`/api/v1/hardware-units/${unitId}/defect/shipped-back`, {
    method: 'POST',
    body: JSON.stringify({ tracking_number: trackingNumber || undefined, carrier: carrier || undefined }),
  })
}

export function markDelivered(unitId: string) {
  return apiFetch<DefectReport>(`/api/v1/hardware-units/${unitId}/defect/delivered`, { method: 'POST' })
}

export function markSupplierReceived(unitId: string) {
  return apiFetch<DefectReport>(`/api/v1/hardware-units/${unitId}/defect/supplier-received`, { method: 'POST' })
}

export function recordReplacement(unitId: string, replacementSerialNumber: string) {
  return apiFetch<{ defect_report: DefectReport; replacement_unit: { id: string; barcode: string } }>(
    `/api/v1/hardware-units/${unitId}/defect/replacement`,
    { method: 'POST', body: JSON.stringify({ replacement_serial_number: replacementSerialNumber }) },
  )
}
