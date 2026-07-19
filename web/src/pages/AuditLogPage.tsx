import { useEffect, useState, type FormEvent } from 'react'
import { Navigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { listAuditLog, type AuditLogEntry } from '../api/audit'
import { useSnackbar } from '../components/SnackbarProvider'
import { DataTable } from '../components/DataTable'

export function AuditLogPage() {
  const { roles } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [entityType, setEntityType] = useState(searchParams.get('entity_type') ?? '')
  const [entityId, setEntityId] = useState(searchParams.get('entity_id') ?? '')
  const showSnackbar = useSnackbar()

  async function load(type: string, id: string) {
    setLoading(true)
    try {
      setEntries(await listAuditLog({ entity_type: type || undefined, entity_id: id || undefined, limit: 200 }))
    } catch {
      showSnackbar('Failed to load audit log.', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(entityType, entityId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  if (!roles.includes('admin')) {
    return <Navigate to="/board" replace />
  }

  function handleFilter(e: FormEvent) {
    e.preventDefault()
    setSearchParams(
      (() => {
        const params = new URLSearchParams()
        if (entityType) params.set('entity_type', entityType)
        if (entityId) params.set('entity_id', entityId)
        return params
      })(),
    )
    load(entityType, entityId)
  }

  function handleClear() {
    setEntityType('')
    setEntityId('')
    setSearchParams(new URLSearchParams())
    load('', '')
  }

  return (
    <div className="detail-page">
      <div className="page-toolbar">
        <h1 className="page-title">Audit History</h1>
      </div>

      <section className="detail-section">
        <div className="section-toolbar">
          <form onSubmit={handleFilter} className="site-location-add-form">
            <input
              placeholder="Entity type (e.g. hardware_unit)"
              value={entityType}
              onChange={(e) => setEntityType(e.target.value)}
            />
            <input placeholder="Entity ID" value={entityId} onChange={(e) => setEntityId(e.target.value)} />
            <button type="submit" className="btn btn-primary">
              Filter
            </button>
            {(entityType || entityId) && (
              <button type="button" className="btn" onClick={handleClear}>
                Clear
              </button>
            )}
          </form>
        </div>

        {loading && <p className="board-empty">Loading…</p>}
        {!loading && (
          <DataTable
            rows={entries}
            getRowKey={(e) => e.id}
            emptyMessage="No audit entries found."
            columns={[
              { key: 'performed_at', header: 'When', render: (e) => new Date(e.performed_at).toLocaleString() },
              { key: 'entity_type', header: 'Entity Type', render: (e) => e.entity_type },
              { key: 'entity_id', header: 'Entity ID', render: (e) => e.entity_id },
              { key: 'action', header: 'Action', render: (e) => e.action },
              { key: 'performed_by_name', header: 'Performed By', render: (e) => e.performed_by_name ?? 'System' },
            ]}
          />
        )}
      </section>
    </div>
  )
}
