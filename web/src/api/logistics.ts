import { apiFetch } from './client'

export type DocketStatus = 'draft' | 'dispatched' | 'in_transit' | 'arrived_at_site' | 'delivered'

export interface Docket {
  id: string
  docket_number: string
  waybill_number?: string
  shipped_via?: 'air' | 'sea' | 'land' | 'others'
  shipping_provider?: string
  destination_site?: string
  status: DocketStatus
  dispatched_by?: string
  dispatched_at?: string
  received_by?: string
  received_at?: string
  notes?: string
  meta_data: Record<string, unknown>
  item_count: number
  created_at: string
  updated_at: string
}

export interface DocketItem {
  id: string
  docket_id: string
  hardware_unit_id: string
  barcode: string
  serial_number?: string
  device_make?: string
  device_model?: string
  part_number?: string
  serial_number_snapshot?: string
  added_at: string
}

export interface TrackingEvent {
  id: string
  docket_id: string
  status?: string
  description: string
  occurred_at: string
  is_system_generated: boolean
  created_at: string
}

export interface DocketDetail extends Docket {
  items: DocketItem[]
  tracking_events: TrackingEvent[]
}

export interface UnitLogisticsView {
  docket: Docket | null
  tracking_events: TrackingEvent[]
}

export function listDockets(status?: string) {
  const query = status ? `?status=${encodeURIComponent(status)}` : ''
  return apiFetch<Docket[]>(`/api/v1/delivery-dockets${query}`)
}

export function createDocket(input: { destination_site?: string; notes?: string }) {
  return apiFetch<Docket>('/api/v1/delivery-dockets', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getDocket(docketId: string) {
  return apiFetch<DocketDetail>(`/api/v1/delivery-dockets/${docketId}`)
}

export function addDocketItem(docketId: string, identifier: string) {
  return apiFetch<DocketItem>(`/api/v1/delivery-dockets/${docketId}/items`, {
    method: 'POST',
    body: JSON.stringify({ identifier }),
  })
}

export function removeDocketItem(docketId: string, itemId: string) {
  return apiFetch<void>(`/api/v1/delivery-dockets/${docketId}/items/${itemId}`, {
    method: 'DELETE',
  })
}

export function dispatchDocket(
  docketId: string,
  input: {
    dispatched_by: string
    dispatched_at?: string
    waybill_number?: string
    shipped_via?: string
    shipping_provider?: string
  },
) {
  return apiFetch<DocketDetail>(`/api/v1/delivery-dockets/${docketId}/dispatch`, {
    method: 'POST',
    body: JSON.stringify({
      dispatched_by: input.dispatched_by,
      dispatched_at: input.dispatched_at || undefined,
      waybill_number: input.waybill_number || undefined,
      shipped_via: input.shipped_via || undefined,
      shipping_provider: input.shipping_provider || undefined,
    }),
  })
}

export function receiveDocket(docketId: string, receivedBy: string, receivedAt?: string) {
  return apiFetch<DocketDetail>(`/api/v1/delivery-dockets/${docketId}/receive`, {
    method: 'POST',
    body: JSON.stringify({ received_by: receivedBy, received_at: receivedAt || undefined }),
  })
}

export function addTrackingEvent(
  docketId: string,
  input: { status?: string; description: string; occurred_at?: string },
) {
  return apiFetch<TrackingEvent>(`/api/v1/delivery-dockets/${docketId}/tracking-events`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function docketWaybillUrl(docketId: string) {
  return `/api/v1/delivery-dockets/${docketId}/waybill`
}

export function getUnitLogistics(unitId: string) {
  return apiFetch<UnitLogisticsView>(`/api/v1/hardware-units/${unitId}/logistics`)
}
