import { apiFetch, apiFetchBlob } from './client'

// ---- Users ----

export interface UserSummary {
  id: string
  email: string
  full_name: string
  is_active: boolean
  roles: string[]
}

export const ALL_ROLES = [
  'admin',
  'pm_pc',
  'encoder',
  'configurator',
  'qc',
  'logistics',
  'field_technician',
  'bsp_acceptance_officer',
  'reports_viewer',
] as const

export function listUsers() {
  return apiFetch<UserSummary[]>('/api/v1/users')
}

export function createUser(input: { email: string; full_name: string; password: string; roles: string[] }) {
  return apiFetch<{ id: string }>('/api/v1/users', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function setUserRoles(userId: string, roles: string[]) {
  return apiFetch<void>(`/api/v1/users/${userId}/roles`, {
    method: 'PUT',
    body: JSON.stringify({ roles }),
  })
}

export function updateUser(userId: string, input: { full_name: string; is_active: boolean }) {
  return apiFetch<void>(`/api/v1/users/${userId}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function setUserPassword(userId: string, newPassword: string) {
  return apiFetch<void>(`/api/v1/users/${userId}/password`, {
    method: 'PUT',
    body: JSON.stringify({ new_password: newPassword }),
  })
}

// ---- Meta-data field definitions ----

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

// "general" is a pseudo-stage: fields under it aren't tied to any lifecycle
// stage. They're attached directly to the hardware unit (stored in its own
// meta_data column) and rendered on the New Unit form instead of a stage
// form. The other entries are real lifecycle stages with their own forms.
export const META_DATA_STAGES = ['general', 'stage1a', 'stage1b', 'shipment', 'installation', 'acceptance'] as const

export function listMetaDataFields(stage: string, includeInactive = false) {
  return apiFetch<FieldDefinition[]>(
    `/api/v1/meta-data-fields?stage=${encodeURIComponent(stage)}&include_inactive=${includeInactive}`,
  )
}

export function createMetaDataField(input: {
  stage: string
  field_key: string
  label: string
  data_type: string
  is_required: boolean
  sort_order: number
}) {
  return apiFetch<FieldDefinition>('/api/v1/meta-data-fields', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateMetaDataField(id: number, input: { label: string; is_required: boolean; sort_order: number }) {
  return apiFetch<FieldDefinition>(`/api/v1/meta-data-fields/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deactivateMetaDataField(id: number) {
  return apiFetch<void>(`/api/v1/meta-data-fields/${id}`, { method: 'DELETE' })
}

export function reactivateMetaDataField(id: number) {
  return apiFetch<void>(`/api/v1/meta-data-fields/${id}/reactivate`, { method: 'POST' })
}

// ---- Site locations ----

export interface SiteLocation {
  id: number
  name: string
  active: boolean
  region?: string
  ip_gateway?: string
  subnet_mask?: string
}

export interface SiteLocationInput {
  name: string
  region?: string
  ip_gateway?: string
  subnet_mask?: string
}

export function listSiteLocations(includeInactive = false) {
  return apiFetch<SiteLocation[]>(`/api/v1/site-locations?include_inactive=${includeInactive}`)
}

export function createSiteLocation(input: SiteLocationInput) {
  return apiFetch<SiteLocation>('/api/v1/site-locations', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateSiteLocation(id: number, input: SiteLocationInput) {
  return apiFetch<SiteLocation>(`/api/v1/site-locations/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deactivateSiteLocation(id: number) {
  return apiFetch<void>(`/api/v1/site-locations/${id}`, { method: 'DELETE' })
}

export function reactivateSiteLocation(id: number) {
  return apiFetch<void>(`/api/v1/site-locations/${id}/reactivate`, { method: 'POST' })
}

// ---- Items (master file) ----

export interface Item {
  id: number
  make: string
  model: string
  description?: string
  qty: number
  sales_order_number?: string
  active: boolean
}

export interface ItemInput {
  make: string
  model: string
  description?: string
  qty: number
  sales_order_number?: string
}

export function listItems(includeInactive = false) {
  return apiFetch<Item[]>(`/api/v1/items?include_inactive=${includeInactive}`)
}

export function createItem(input: ItemInput) {
  return apiFetch<Item>('/api/v1/items', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateItem(id: number, input: ItemInput) {
  return apiFetch<Item>(`/api/v1/items/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deactivateItem(id: number) {
  return apiFetch<void>(`/api/v1/items/${id}`, { method: 'DELETE' })
}

export function reactivateItem(id: number) {
  return apiFetch<void>(`/api/v1/items/${id}/reactivate`, { method: 'POST' })
}

// ---- Barcode label settings ----

export interface BarcodeLabelSettings {
  barcode_label_width_mm: number
  barcode_label_height_mm: number
  barcode_label_fields: string[]
}

export interface BarcodeLabelField {
  key: string
  label: string
}

export function getBarcodeLabelSettings() {
  return apiFetch<BarcodeLabelSettings>('/api/v1/settings/barcode-label')
}

export function updateBarcodeLabelSettings(input: BarcodeLabelSettings) {
  return apiFetch<BarcodeLabelSettings>('/api/v1/settings/barcode-label', {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

// The fields selectable for the sticker's text lines, in canonical display
// order — driven by the backend so this list doesn't need to be duplicated
// here.
export function listBarcodeLabelFields() {
  return apiFetch<BarcodeLabelField[]>('/api/v1/settings/barcode-label/fields')
}

// Renders a sample sticker PDF at the given (possibly unsaved) settings, for
// a live preview before saving.
export function previewBarcodeLabel(input: BarcodeLabelSettings) {
  return apiFetchBlob('/api/v1/settings/barcode-label/preview', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
