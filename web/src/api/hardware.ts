import { apiFetch } from './client'

export interface HardwareUnit {
  id: string
  barcode: string
  alias?: string
  serial_number?: string
  device_make?: string
  device_model?: string
  part_number?: string
  status: string
  current_stage: string
  board_column: string
  is_exception: boolean
  allocated_branch?: string
  meta_data: Record<string, unknown>
  created_at: string
  updated_at: string
  encoded: boolean
  configured: boolean
  qc: boolean
}

export interface Stage0Receiving {
  id: string
  hardware_unit_id: string
  received_by?: string
  received_date: string
  po_or_waybill_ref?: string
  items_correct?: boolean
  discrepancy_notes?: string
  escalated_to?: string
  escalated_at?: string
}

export interface Stage1A {
  id: string
  hardware_unit_id: string
  device_make?: string
  device_model?: string
  device_name_dns?: string
  client_ip_address?: string
  serial_number?: string
  mac_address?: string
  dns_server_1?: string
  dns_server_2?: string
  ntp_server?: string
  frequency_hz?: number
  default_username?: string
  default_password?: string
  encoded_by?: string
  encoded_by_name?: string
  encoded_at?: string
  configured_by?: string
  configured_by_name?: string
  configured_at?: string
  qc_by?: string
  qc_by_name?: string
  qc_at?: string
  barcode_printed_at?: string
  signed_off_at?: string
  meta_data: Record<string, unknown>
}

export interface Stage1B {
  id: string
  hardware_unit_id: string
  configured_by?: string
  configured_by_name?: string
  configured_date?: string
  firmware_updated: boolean
  firmware_version?: string
  configuration_qc_by?: string
  configuration_qc_by_name?: string
  qc_date?: string
  signed_off_at?: string
  meta_data: Record<string, unknown>
}

export interface FieldDefinition {
  id: number
  stage: string
  field_key: string
  label: string
  data_type: string
  is_required: boolean
  sort_order: number
  active: boolean
}

export function listUnits(includeArchived = false) {
  return apiFetch<HardwareUnit[]>(`/api/v1/hardware-units?include_archived=${includeArchived}`)
}

export function getUnit(id: string) {
  return apiFetch<HardwareUnit>(`/api/v1/hardware-units/${id}`)
}

export function deleteUnit(id: string) {
  return apiFetch<void>(`/api/v1/hardware-units/${id}`, { method: 'DELETE' })
}

export function moveBoardColumn(id: string, column: string, resetAcceptance = false) {
  return apiFetch<HardwareUnit>(`/api/v1/hardware-units/${id}/board-column`, {
    method: 'POST',
    body: JSON.stringify({ column, reset_acceptance: resetAcceptance }),
  })
}

export function syncBoardColumn(id: string) {
  return apiFetch<{ unit: HardwareUnit; moved: boolean }>(`/api/v1/hardware-units/${id}/board-column/sync`, {
    method: 'POST',
  })
}

export function restoreUnit(id: string) {
  return apiFetch<void>(`/api/v1/hardware-units/${id}/restore`, { method: 'POST' })
}

export function purgeUnit(id: string) {
  return apiFetch<void>(`/api/v1/hardware-units/${id}/purge`, { method: 'DELETE' })
}

export function listDeviceMakes() {
  return apiFetch<string[]>('/api/v1/hardware-units/device-makes')
}

export function listDeviceModels() {
  return apiFetch<string[]>('/api/v1/hardware-units/device-models')
}

export interface NewUnitInput {
  alias?: string
  serial_number: string
  device_make?: string
  device_model?: string
  part_number?: string
  barcode?: string
  allocated_branch?: string
  meta_data?: Record<string, unknown>
}

export function createUnit(input: NewUnitInput) {
  return apiFetch<HardwareUnit>('/api/v1/hardware-units', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export interface UnitIdentityInput {
  alias?: string
  serial_number?: string
  device_make?: string
  device_model?: string
  part_number?: string
  allocated_branch?: string
  barcode: string
}

export function updateUnitIdentity(unitId: string, input: UnitIdentityInput) {
  return apiFetch<HardwareUnit>(`/api/v1/hardware-units/${unitId}/identity`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function getReceiving(unitId: string) {
  return apiFetch<Stage0Receiving>(`/api/v1/hardware-units/${unitId}/receiving`)
}

export function recordReceiving(
  unitId: string,
  input: { po_or_waybill_ref?: string; items_correct: boolean; discrepancy_notes?: string },
) {
  return apiFetch<Stage0Receiving>(`/api/v1/hardware-units/${unitId}/receiving`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getStage1A(unitId: string) {
  return apiFetch<Stage1A>(`/api/v1/hardware-units/${unitId}/stage1a`)
}

export function upsertStage1A(unitId: string, input: Partial<Stage1A>) {
  return apiFetch<Stage1A>(`/api/v1/hardware-units/${unitId}/stage1a`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function signOffStage1A(unitId: string, step: 'encoded' | 'configured' | 'qc' | 'qc_fail', performedAt?: string) {
  return apiFetch<Stage1A>(`/api/v1/hardware-units/${unitId}/stage1a/sign-off`, {
    method: 'POST',
    body: JSON.stringify({ step, performed_at: performedAt ?? null }),
  })
}

export interface NetworkDefaults {
  client_ip_address: string
  dns_server_1: string
  dns_server_2: string
  ntp_server: string
  dns_server_1_options: string[]
  dns_server_2_options: string[]
  ntp_server_options: string[]
}

export function networkDefaultsForSite(site: string) {
  return apiFetch<NetworkDefaults>(
    `/api/v1/hardware-units/stage1a/network-defaults?site=${encodeURIComponent(site)}`,
  )
}

export function barcodeStickerUrl(unitId: string) {
  return `/api/v1/hardware-units/${unitId}/stage1a/barcode`
}

export function getStage1B(unitId: string) {
  return apiFetch<Stage1B>(`/api/v1/hardware-units/${unitId}/stage1b`)
}

export function upsertStage1B(unitId: string, input: Partial<Stage1B>) {
  return apiFetch<Stage1B>(`/api/v1/hardware-units/${unitId}/stage1b`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function signOffStage1B(unitId: string, step: 'configured' | 'qc' | 'qc_fail', performedAt?: string) {
  return apiFetch<Stage1B>(`/api/v1/hardware-units/${unitId}/stage1b/sign-off`, {
    method: 'POST',
    body: JSON.stringify({ step, performed_at: performedAt ?? null }),
  })
}

export function listMetaDataFields(stage: string) {
  return apiFetch<FieldDefinition[]>(`/api/v1/meta-data-fields?stage=${encodeURIComponent(stage)}`)
}

export function updateUnitMetaData(unitId: string, metaData: Record<string, unknown>) {
  return apiFetch<HardwareUnit>(`/api/v1/hardware-units/${unitId}/meta-data`, {
    method: 'PUT',
    body: JSON.stringify({ meta_data: metaData }),
  })
}
