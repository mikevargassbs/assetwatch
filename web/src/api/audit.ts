import { apiFetch } from './client'

export interface AuditLogEntry {
  id: number
  entity_type: string
  entity_id: string
  action: string
  performed_by?: string
  performed_by_name?: string
  performed_at: string
  old_value?: unknown
  new_value?: unknown
  ip_address?: string
  notes?: string
}

export interface AuditLogFilter {
  entity_type?: string
  entity_id?: string
  limit?: number
  offset?: number
}

export function listAuditLog(filter: AuditLogFilter = {}) {
  const params = new URLSearchParams()
  if (filter.entity_type) params.set('entity_type', filter.entity_type)
  if (filter.entity_id) params.set('entity_id', filter.entity_id)
  if (filter.limit) params.set('limit', String(filter.limit))
  if (filter.offset) params.set('offset', String(filter.offset))
  return apiFetch<AuditLogEntry[]>(`/api/v1/audit-log?${params.toString()}`)
}
