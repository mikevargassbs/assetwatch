import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  addDocketItem,
  addTrackingEvent,
  dispatchDocket,
  docketWaybillUrl,
  getDocket,
  receiveDocket,
  removeDocketItem,
  type DocketDetail,
} from '../api/logistics'
import { authFetch } from '../api/client'
import { DataTable } from '../components/DataTable'
import { Tabs } from '../components/Tabs'
import { useAuth } from '../auth/AuthContext'
import { useSnackbar } from '../components/SnackbarProvider'
import { useConfirm } from '../components/ConfirmDialogProvider'

const STATUS_LABELS: Record<string, string> = {
  draft: 'Draft',
  dispatched: 'Dispatched',
  in_transit: 'In Transit',
  arrived_at_site: 'Arrived at Site',
  delivered: 'Delivered',
}

async function openWaybill(docketId: string) {
  const res = await authFetch(docketWaybillUrl(docketId))
  if (!res.ok) return
  const blob = await res.blob()
  window.open(URL.createObjectURL(blob), '_blank')
}

export function DocketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { roles } = useAuth()
  const canEdit = roles.includes('logistics') || roles.includes('admin')
  const showSnackbar = useSnackbar()
  const confirm = useConfirm()

  const [docket, setDocket] = useState<DocketDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [itemInput, setItemInput] = useState('')
  const [addingItem, setAddingItem] = useState(false)
  const [dispatchedBy, setDispatchedBy] = useState('')
  const [waybillNumber, setWaybillNumber] = useState('')
  const [shippedVia, setShippedVia] = useState('')
  const [shippingProvider, setShippingProvider] = useState('')
  const [receivedBy, setReceivedBy] = useState('')
  const [manualDescription, setManualDescription] = useState('')
  const [manualOccurredAt, setManualOccurredAt] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [activeTab, setActiveTab] = useState<string | null>(null)

  async function refresh() {
    if (!id) return
    setLoading(true)
    try {
      setDocket(await getDocket(id))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  if (!id) return null

  async function handleAddItem(e: FormEvent) {
    e.preventDefault()
    if (!itemInput.trim()) return
    setAddingItem(true)
    try {
      await addDocketItem(id!, itemInput.trim())
      setItemInput('')
      await refresh()
    } catch (err) {
      const message = err instanceof Error ? err.message : ''
      if (message.startsWith('404')) {
        showSnackbar('No unit found with that barcode or serial number.', 'error')
      } else if (message.includes("hasn't reached the Shipment stage")) {
        showSnackbar("That unit hasn't reached the Shipment stage yet — it can't be added to a docket.", 'error')
      } else if (message.startsWith('409')) {
        showSnackbar('That unit is already on this docket.', 'error')
      } else {
        showSnackbar('Failed to add item.', 'error')
      }
    } finally {
      setAddingItem(false)
    }
  }

  async function handleRemoveItem(itemId: string) {
    if (!(await confirm({ title: 'Remove item', message: 'Remove this unit from the docket?' }))) return
    await removeDocketItem(id!, itemId)
    await refresh()
  }

  async function handleDispatch(e: FormEvent) {
    e.preventDefault()
    if (!dispatchedBy.trim()) return
    setSubmitting(true)
    try {
      await dispatchDocket(id!, {
        dispatched_by: dispatchedBy.trim(),
        waybill_number: waybillNumber || undefined,
        shipped_via: shippedVia || undefined,
        shipping_provider: shippingProvider || undefined,
      })
      showSnackbar('Docket dispatched.', 'success')
      await refresh()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleReceive(e: FormEvent) {
    e.preventDefault()
    if (!receivedBy.trim()) return
    setSubmitting(true)
    try {
      await receiveDocket(id!, receivedBy.trim())
      showSnackbar('Docket received. Units advanced to Installation.', 'success')
      await refresh()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleStageButton(status: string, description: string) {
    await addTrackingEvent(id!, { status, description })
    await refresh()
  }

  async function handleManualEvent(e: FormEvent) {
    e.preventDefault()
    if (!manualDescription.trim()) return
    setSubmitting(true)
    try {
      await addTrackingEvent(id!, {
        description: manualDescription.trim(),
        occurred_at: manualOccurredAt ? new Date(manualOccurredAt).toISOString() : undefined,
      })
      setManualDescription('')
      setManualOccurredAt('')
      await refresh()
    } finally {
      setSubmitting(false)
    }
  }

  if (loading || !docket) {
    return <div className="detail-page">{loading ? <p className="board-empty">Loading…</p> : <p>Docket not found.</p>}</div>
  }

  const tabs: { key: string; label: string; content: ReactNode }[] = [
    {
      key: 'items',
      label: `Items (${docket.items.length})`,
      content: (
        <section className="detail-section">
          <h2>Items ({docket.items.length})</h2>
          {canEdit && docket.status === 'draft' && (
            <form onSubmit={handleAddItem} className="signoff-row">
              <input
                autoFocus
                value={itemInput}
                onChange={(e) => setItemInput(e.target.value.toUpperCase())}
                placeholder="Scan or type barcode / serial number"
                style={{ minWidth: 260 }}
              />
              <button type="submit" className="btn btn-primary" disabled={addingItem}>
                Add item
              </button>
            </form>
          )}
          <DataTable
            rows={docket.items}
            getRowKey={(i) => i.id}
            emptyMessage="No items added yet."
            columns={[
              { key: 'barcode', header: 'Barcode', render: (i) => i.barcode },
              { key: 'serial_number', header: 'Serial Number', render: (i) => i.serial_number ?? '—' },
              { key: 'make', header: 'Make', render: (i) => i.device_make ?? '—' },
              { key: 'model', header: 'Model', render: (i) => i.device_model ?? '—' },
              { key: 'part_number', header: 'Part Number', render: (i) => i.part_number ?? '—' },
              { key: 'added_at', header: 'Added', render: (i) => new Date(i.added_at).toLocaleString() },
              ...(canEdit && docket.status === 'draft'
                ? [
                    {
                      key: 'actions',
                      header: '',
                      render: (i: (typeof docket.items)[number]) => (
                        <button type="button" className="link-button" onClick={() => handleRemoveItem(i.id)}>
                          Remove
                        </button>
                      ),
                    },
                  ]
                : []),
            ]}
          />
        </section>
      ),
    },
    {
      key: 'signoff',
      label: 'Sign-off',
      content: (
        <section className="detail-section">
          <h2>Sign-off</h2>
          <div className="detail-form-grid">
            <div className="signoff-summary">
              <span className="signoff-summary-label">Dispatched by</span>
              <span className="signoff-summary-value">
                {docket.dispatched_by ?? '—'}
                {docket.dispatched_at && ` (${new Date(docket.dispatched_at).toLocaleString()})`}
              </span>
            </div>
            <div className="signoff-summary">
              <span className="signoff-summary-label">Received by</span>
              <span className="signoff-summary-value">
                {docket.received_by ?? '—'}
                {docket.received_at && ` (${new Date(docket.received_at).toLocaleString()})`}
              </span>
            </div>
          </div>

          {canEdit && !docket.dispatched_at && (
            <form onSubmit={handleDispatch} className="detail-form">
              <div className="detail-form-grid">
                <div className="form-field">
                  <label>Waybill Number</label>
                  <input value={waybillNumber} onChange={(e) => setWaybillNumber(e.target.value.toUpperCase())} />
                </div>
                <div className="form-field">
                  <label>Shipped Via</label>
                  <select value={shippedVia} onChange={(e) => setShippedVia(e.target.value)}>
                    <option value="">—</option>
                    <option value="air">Air</option>
                    <option value="sea">Sea</option>
                    <option value="land">Land</option>
                    <option value="others">Others</option>
                  </select>
                </div>
                <div className="form-field">
                  <label>Shipping Provider</label>
                  <input value={shippingProvider} onChange={(e) => setShippingProvider(e.target.value.toUpperCase())} />
                </div>
              </div>
              <div className="signoff-row">
                <input
                  value={dispatchedBy}
                  onChange={(e) => setDispatchedBy(e.target.value.toUpperCase())}
                  placeholder="Dispatched by"
                />
                <button type="submit" className="signoff-btn" disabled={submitting || docket.items.length === 0}>
                  Mark Dispatched
                </button>
              </div>
            </form>
          )}

          {canEdit && docket.dispatched_at && !docket.received_at && (
            <form onSubmit={handleReceive} className="signoff-row">
              <input value={receivedBy} onChange={(e) => setReceivedBy(e.target.value.toUpperCase())} placeholder="Received by" />
              <button type="submit" className="signoff-btn" disabled={submitting}>
                Mark Received
              </button>
            </form>
          )}
        </section>
      ),
    },
    {
      key: 'tracking',
      label: `Tracking History (${docket.tracking_events.length})`,
      content: (
        <section className="detail-section">
          <h2>Tracking History</h2>
          {canEdit && docket.dispatched_at && (
            <div className="signoff-row">
              <button type="button" className="btn btn-primary" onClick={() => handleStageButton('in_transit', 'In Transit')}>
                In Transit
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => handleStageButton('arrived_at_site', 'Arrived at Site')}
              >
                Arrived at Site
              </button>
            </div>
          )}
          {canEdit && (
            <form onSubmit={handleManualEvent} className="detail-form">
              <div className="detail-form-grid">
                <div className="form-field">
                  <label>Description</label>
                  <input value={manualDescription} onChange={(e) => setManualDescription(e.target.value.toUpperCase())} />
                </div>
                <div className="form-field">
                  <label>Date/Time (optional, defaults to now)</label>
                  <input type="datetime-local" value={manualOccurredAt} onChange={(e) => setManualOccurredAt(e.target.value)} />
                </div>
              </div>
              <button type="submit" disabled={submitting}>
                Add tracking entry
              </button>
            </form>
          )}
          <DataTable
            rows={docket.tracking_events}
            getRowKey={(e) => e.id}
            emptyMessage="No tracking events yet."
            columns={[
              { key: 'occurred_at', header: 'When', render: (e) => new Date(e.occurred_at).toLocaleString() },
              { key: 'status', header: 'Status', render: (e) => (e.status ? STATUS_LABELS[e.status] ?? e.status : '—') },
              { key: 'description', header: 'Description', render: (e) => e.description },
              { key: 'source', header: 'Source', render: (e) => (e.is_system_generated ? 'System' : 'Manual') },
            ]}
          />
        </section>
      ),
    },
  ]

  const currentTabKey = activeTab && tabs.some((t) => t.key === activeTab) ? activeTab : tabs[0].key
  const currentTab = tabs.find((t) => t.key === currentTabKey)

  return (
    <div className="detail-page">
      <div className="page-toolbar">
        <button type="button" className="btn" onClick={() => navigate('/logistics')}>
          ← Back to Logistics
        </button>
      </div>

      <section className="detail-section">
        <div className="detail-header">
          <h2>Docket {docket.docket_number}</h2>
          <span className="status-badge">{STATUS_LABELS[docket.status] ?? docket.status}</span>
          <div className="detail-header-actions">
            <button type="button" className="btn" onClick={() => openWaybill(id)}>
              Print waybill
            </button>
          </div>
        </div>
        <div className="detail-form-grid">
          <div className="form-field">
            <label>Waybill Number</label>
            <p>{docket.waybill_number ?? '—'}</p>
          </div>
          <div className="form-field">
            <label>Destination Site</label>
            <p>{docket.destination_site ?? '—'}</p>
          </div>
          <div className="form-field">
            <label>Shipped Via</label>
            <p>{docket.shipped_via ?? '—'}</p>
          </div>
          <div className="form-field">
            <label>Shipping Provider</label>
            <p>{docket.shipping_provider ?? '—'}</p>
          </div>
        </div>
      </section>

      <Tabs tabs={tabs.map((t) => ({ key: t.key, label: t.label }))} active={currentTabKey} onChange={setActiveTab} />

      {currentTab?.content}
    </div>
  )
}
