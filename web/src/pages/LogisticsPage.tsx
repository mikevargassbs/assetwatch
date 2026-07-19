import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { createDocket, listDockets, type Docket } from '../api/logistics'
import { listSiteLocations } from '../api/admin'
import { Combobox } from '../components/Combobox'
import { DataTable, type DataTableColumn } from '../components/DataTable'
import { Tabs } from '../components/Tabs'
import { useAuth } from '../auth/AuthContext'
import { useSnackbar } from '../components/SnackbarProvider'

const STATUS_LABELS: Record<string, string> = {
  draft: 'Draft',
  dispatched: 'Dispatched',
  in_transit: 'In Transit',
  arrived_at_site: 'Arrived at Site',
  delivered: 'Delivered',
}

const STATUS_TABS = ['all', 'draft', 'dispatched', 'in_transit', 'arrived_at_site', 'delivered'] as const

export function LogisticsPage() {
  const navigate = useNavigate()
  const { roles } = useAuth()
  const canCreate = roles.includes('logistics') || roles.includes('admin')
  const showSnackbar = useSnackbar()

  const [dockets, setDockets] = useState<Docket[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [statusFilter, setStatusFilter] = useState<string>('all')

  async function refresh() {
    setLoading(true)
    try {
      setDockets(await listDockets())
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  const tabs = useMemo(
    () =>
      STATUS_TABS.map((status) => ({
        key: status,
        label:
          status === 'all'
            ? `All (${dockets.length})`
            : `${STATUS_LABELS[status]} (${dockets.filter((d) => d.status === status).length})`,
      })),
    [dockets],
  )

  const visibleDockets = statusFilter === 'all' ? dockets : dockets.filter((d) => d.status === statusFilter)

  const columns: DataTableColumn<Docket>[] = [
    { key: 'docket_number', header: 'Docket #', render: (d) => d.docket_number },
    { key: 'waybill_number', header: 'Waybill #', render: (d) => d.waybill_number ?? '—' },
    { key: 'destination_site', header: 'Destination', render: (d) => d.destination_site ?? '—' },
    { key: 'status', header: 'Status', render: (d) => STATUS_LABELS[d.status] ?? d.status },
    { key: 'items', header: 'Items', render: (d) => d.item_count },
    { key: 'dispatched_at', header: 'Dispatched', render: (d) => (d.dispatched_at ? new Date(d.dispatched_at).toLocaleString() : '—') },
    { key: 'received_at', header: 'Received', render: (d) => (d.received_at ? new Date(d.received_at).toLocaleString() : '—') },
  ]

  return (
    <div className="detail-page">
      <div className="page-toolbar">
        <h1 className="page-title">Logistics — Delivery Dockets</h1>
        {canCreate && !showCreate && (
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            New Docket
          </button>
        )}
        {canCreate && showCreate && (
          <CreateDocketForm
            onClose={() => setShowCreate(false)}
            onCreated={(docket) => {
              setShowCreate(false)
              showSnackbar('Docket created.', 'success')
              navigate(`/logistics/${docket.id}`)
            }}
          />
        )}
      </div>

      <Tabs tabs={tabs} active={statusFilter} onChange={setStatusFilter} />

      {loading ? (
        <p className="board-empty">Loading…</p>
      ) : (
        <DataTable
          columns={columns}
          rows={visibleDockets}
          getRowKey={(d) => d.id}
          searchPlaceholder="Search dockets…"
          searchText={(d) => `${d.docket_number} ${d.waybill_number ?? ''} ${d.destination_site ?? ''}`}
          emptyMessage="No delivery dockets in this status."
          onRowClick={(d) => navigate(`/logistics/${d.id}`)}
        />
      )}
    </div>
  )
}

function CreateDocketForm({ onClose, onCreated }: { onClose: () => void; onCreated: (docket: Docket) => void }) {
  const [destinationSite, setDestinationSite] = useState('')
  const [siteNames, setSiteNames] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    listSiteLocations()
      .then((locations) => setSiteNames(locations.map((l) => l.name)))
      .catch(() => setSiteNames([]))
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      const docket = await createDocket({ destination_site: destinationSite || undefined })
      onCreated(docket)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="inline-form" title="Waybill number, shipped via, and shipping provider are recorded when the docket is dispatched.">
      <Combobox id="destination_site" value={destinationSite} onChange={setDestinationSite} options={siteNames} placeholder="Destination site" uppercase />
      <button type="submit" className="btn btn-primary" disabled={submitting}>
        Create docket
      </button>
      <button type="button" className="btn" onClick={onClose}>
        Cancel
      </button>
    </form>
  )
}
