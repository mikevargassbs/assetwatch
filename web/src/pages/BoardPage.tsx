import { useEffect, useRef, useState, type DragEvent, type FormEvent, type KeyboardEvent, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  createUnit,
  getStage1A,
  getStage1B,
  listDeviceMakes,
  listDeviceModels,
  listUnits,
  moveBoardColumn,
  syncBoardColumn,
  type HardwareUnit,
} from '../api/hardware'
import { getUnitLogistics } from '../api/logistics'
import { getInstallation } from '../api/installation'
import { getAcceptance } from '../api/acceptance'
import { listItems, listMetaDataFields, listSiteLocations, type FieldDefinition, type Item } from '../api/admin'
import { BarcodeScannerModal } from '../components/BarcodeScanner'
import { Combobox } from '../components/Combobox'
import { DataTable } from '../components/DataTable'
import { defaultMetaValues, type MetaValues } from '../components/DynamicFields'
import { useConfirm } from '../components/ConfirmDialogProvider'
import { useAuth } from '../auth/AuthContext'
import { axisWebsiteUrl, isAxisWebsiteField } from '../utils/axis'

const COLUMNS = [
  { key: 'pre_deployment', label: 'Pre-Deployment' },
  { key: 'configuration', label: 'Configuration' },
  { key: 'shipment', label: 'Shipment' },
  { key: 'installation', label: 'Installation' },
  { key: 'commissioning', label: 'Commissioning' },
  { key: 'completed', label: 'Completed' },
] as const

type ColumnKey = (typeof COLUMNS)[number]['key']

// What must be true before a unit is allowed to leave each column. This
// mirrors the same completion state the backend already uses to
// auto-advance board_column — a drop is only ever "missing" something
// because the backend hasn't advanced it yet.
async function missingRequirements(column: ColumnKey, unitId: string): Promise<string[]> {
  switch (column) {
    case 'pre_deployment': {
      try {
        const s = await getStage1A(unitId)
        const missing: string[] = []
        if (!s.encoded_by) missing.push('Encoding')
        return missing
      } catch {
        return ['Stage 1-A configuration']
      }
    }
    case 'configuration': {
      try {
        const s = await getStage1B(unitId)
        const missing: string[] = []
        if (!s.configured_by) missing.push('Firmware configuration')
        if (!s.configuration_qc_by) missing.push('Firmware QC sign-off')
        return missing
      } catch {
        return ['Firmware configuration']
      }
    }
    case 'shipment': {
      try {
        const view = await getUnitLogistics(unitId)
        const missing: string[] = []
        if (!view.docket?.dispatched_at) missing.push('Dispatch')
        if (!view.docket?.received_at) missing.push('Delivery confirmation')
        return missing
      } catch {
        return ['Shipment details']
      }
    }
    case 'installation': {
      try {
        const s = await getInstallation(unitId)
        const missing: string[] = []
        if (!s.installed_by) missing.push('Installation sign-off')
        if (!s.inspected_by) missing.push('Inspection sign-off')
        if (!s.fit_focus_by) missing.push('Fit & focus sign-off')
        return missing
      } catch {
        return ['Site installation']
      }
    }
    case 'commissioning': {
      try {
        const s = await getAcceptance(unitId)
        const missing: string[] = []
        if (!s.bsp_acceptance_date && !s.client_signed_at && !s.head_office_signed_at) {
          missing.push('BSP acceptance, Branch Manager, or Head Office sign-off')
        }
        return missing
      } catch {
        return ['Client acceptance']
      }
    }
    default:
      return []
  }
}

function unitLabel(u: HardwareUnit) {
  return u.alias || u.barcode
}

// Abbreviates a site/branch name to its word initials (e.g. "Port Moresby
// Branch" -> "PMB"), or the first few letters of a single-word name.
function abbreviateSiteName(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return ''
  if (words.length === 1) return words[0].slice(0, 3).toUpperCase()
  return words.map((w) => w[0]).join('').toUpperCase()
}

// Auto-generated when the user leaves Alias / Name blank: site abbreviation,
// device model, and the last 4 digits of the serial number. Renaming later
// is always possible from the unit detail page.
function generateAlias(allocatedBranch: string, deviceModel: string, serialNumber: string): string {
  const parts = [
    abbreviateSiteName(allocatedBranch),
    deviceModel.trim(),
    serialNumber.trim().slice(-4).toUpperCase(),
  ].filter(Boolean)
  return parts.join('-')
}

function formatItemLabel(item: Item): string {
  const so = item.sales_order_number ? ` — SO ${item.sales_order_number}` : ''
  return `${item.make} ${item.model}${so} (qty: ${item.qty})`
}

// Barcode scanners emit their input as keystrokes ending in Enter. In a form
// with a submit button, that Enter would otherwise submit the form early —
// instead, treat it like Tab and move to the next field.
function focusNextFieldOnEnter(e: KeyboardEvent<HTMLFormElement>) {
  if (e.key !== 'Enter') return
  const target = e.target as HTMLElement
  if (target.tagName === 'BUTTON') return
  e.preventDefault()
  const fields = Array.from(
    e.currentTarget.querySelectorAll<HTMLElement>('input:not([disabled]), select:not([disabled])'),
  ).filter((el) => el.offsetParent !== null)
  const index = fields.indexOf(target)
  if (index > -1 && index < fields.length - 1) {
    fields[index + 1].focus()
  }
}

function SignOffDot({ label, done }: { label: string; done: boolean }) {
  return (
    <span
      className={`board-card-signoff${done ? ' board-card-signoff-done' : ''}`}
      title={`${label}${done ? ' signed off' : ' not yet signed off'}`}
    >
      {label[0]}
    </span>
  )
}

function matchesSearch(u: HardwareUnit, query: string): boolean {
  const haystack = [u.alias, u.barcode, u.serial_number, u.device_make, u.device_model]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
  return haystack.includes(query.toLowerCase())
}

function columnLabel(key: string): string {
  return COLUMNS.find((c) => c.key === key)?.label ?? key
}

function uniqueSorted(values: Array<string | undefined>): string[] {
  return Array.from(new Set(values.map((value) => value?.trim()).filter(Boolean) as string[])).sort((a, b) =>
    a.localeCompare(b),
  )
}

const TABLE_COLUMN_DEFS: { key: string; label: string; render: (u: HardwareUnit) => ReactNode }[] = [
  { key: 'name', label: 'Name', render: (u) => unitLabel(u) },
  { key: 'barcode', label: 'Barcode', render: (u) => u.barcode },
  { key: 'serial', label: 'Serial Number', render: (u) => u.serial_number ?? '—' },
  { key: 'make', label: 'Make', render: (u) => u.device_make ?? '—' },
  { key: 'model', label: 'Model', render: (u) => u.device_model ?? '—' },
  { key: 'part_number', label: 'Part Number', render: (u) => u.part_number ?? '—' },
  { key: 'site', label: 'Site', render: (u) => u.allocated_branch ?? '—' },
  { key: 'stage', label: 'Stage', render: (u) => columnLabel(u.board_column) },
  { key: 'status', label: 'Status', render: (u) => u.status },
  {
    key: 'signoff',
    label: 'Sign-off',
    render: (u) => (
      <span className="board-card-signoffs">
        <SignOffDot label="Encoded" done={u.encoded} />
        <SignOffDot label="Configured" done={u.configured} />
        <SignOffDot label="QC" done={u.qc} />
      </span>
    ),
  },
]

const DEFAULT_TABLE_COLUMNS = ['name', 'barcode', 'serial', 'make', 'model', 'site', 'stage']
const TABLE_COLUMNS_STORAGE_KEY = 'board-table-visible-columns'
const BOARD_VIEW_STORAGE_KEY = 'board-preferred-view'

function loadBoardViewPreference(): 'board' | 'table' {
  try {
    const raw = localStorage.getItem(BOARD_VIEW_STORAGE_KEY)
    return raw === 'table' ? 'table' : 'board'
  } catch {
    return 'board'
  }
}

function loadVisibleTableColumns(): string[] {
  try {
    const raw = localStorage.getItem(TABLE_COLUMNS_STORAGE_KEY)
    if (!raw) return DEFAULT_TABLE_COLUMNS
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return DEFAULT_TABLE_COLUMNS
    const known = new Set(TABLE_COLUMN_DEFS.map((c) => c.key))
    const filtered = parsed.filter((k) => typeof k === 'string' && known.has(k))
    return filtered.length > 0 ? filtered : DEFAULT_TABLE_COLUMNS
  } catch {
    return DEFAULT_TABLE_COLUMNS
  }
}

function TableColumnPicker({
  visible,
  onChange,
}: {
  visible: string[]
  onChange: (visible: string[]) => void
}) {
  const [open, setOpen] = useState(false)
  const visibleSet = new Set(visible)

  function toggle(key: string) {
    const next = visibleSet.has(key) ? visible.filter((k) => k !== key) : [...visible, key]
    onChange(next)
  }

  return (
    <div className="combobox" style={{ width: 'auto' }}>
      <button type="button" className="btn" onClick={() => setOpen((o) => !o)}>
        Columns
      </button>
      {open && (
        <>
          <div
            style={{ position: 'fixed', inset: 0, zIndex: 19 }}
            onClick={() => setOpen(false)}
          />
          <ul className="combobox-menu" style={{ minWidth: 200, right: 0, left: 'auto' }}>
            {TABLE_COLUMN_DEFS.map((c) => (
              <li key={c.key}>
                <label className="checkbox-row" style={{ padding: '6px 8px' }}>
                  <input type="checkbox" checked={visibleSet.has(c.key)} onChange={() => toggle(c.key)} />
                  {c.label}
                </label>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  )
}

export function BoardPage() {
  const navigate = useNavigate()
  const confirm = useConfirm()
  const { roles } = useAuth()
  const isAdmin = roles.includes('admin')
  const [units, setUnits] = useState<HardwareUnit[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showNewUnit, setShowNewUnit] = useState(false)
  const [search, setSearch] = useState('')
  const [stageFilter, setStageFilter] = useState('')
  const [siteFilter, setSiteFilter] = useState('')
  const [makeFilter, setMakeFilter] = useState('')
  const [modelFilter, setModelFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [view, setView] = useState<'board' | 'table'>(loadBoardViewPreference)
  const [visibleTableColumns, setVisibleTableColumns] = useState<string[]>(loadVisibleTableColumns)
  const [dropNotice, setDropNotice] = useState<{ kind: 'error' | 'info'; message: string } | null>(null)
  const [draggingUnitId, setDraggingUnitId] = useState<string | null>(null)
  const [dragOverColumn, setDragOverColumn] = useState<ColumnKey | null>(null)
  const [validating, setValidating] = useState(false)
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  async function refresh() {
    setLoading(true)
    try {
      const data = await listUnits()
      setUnits(data ?? [])
      setError(null)
    } catch {
      setError('Failed to load hardware units.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  useEffect(() => {
    localStorage.setItem(TABLE_COLUMNS_STORAGE_KEY, JSON.stringify(visibleTableColumns))
  }, [visibleTableColumns])

  useEffect(() => {
    localStorage.setItem(BOARD_VIEW_STORAGE_KEY, view)
  }, [view])

  function showNotice(kind: 'error' | 'info', message: string) {
    setDropNotice({ kind, message })
    if (noticeTimer.current) clearTimeout(noticeTimer.current)
    noticeTimer.current = setTimeout(() => setDropNotice(null), 6000)
  }

  function handleDragStart(unitId: string) {
    setDraggingUnitId(unitId)
  }

  function handleDragEnd() {
    setDraggingUnitId(null)
    setDragOverColumn(null)
  }

  function nextColumnFor(unit: HardwareUnit): ColumnKey | null {
    const idx = COLUMNS.findIndex((c) => c.key === unit.board_column)
    if (idx < 0 || idx === COLUMNS.length - 1) return null
    return COLUMNS[idx + 1].key
  }

  function prevColumnFor(unit: HardwareUnit): ColumnKey | null {
    const idx = COLUMNS.findIndex((c) => c.key === unit.board_column)
    if (idx <= 0) return null
    return COLUMNS[idx - 1].key
  }

  // Forward moves go through the normal readiness check (they mirror what
  // the backend already auto-advances on sign-off). Backward moves are an
  // admin-only manual override — there's no sign-off to "undo".
  function dropDirection(unit: HardwareUnit, columnKey: ColumnKey): 'forward' | 'backward' | null {
    if (nextColumnFor(unit) === columnKey) return 'forward'
    if (isAdmin && prevColumnFor(unit) === columnKey) return 'backward'
    return null
  }

  function handleDragOver(e: DragEvent, columnKey: ColumnKey) {
    const unit = units.find((u) => u.id === draggingUnitId)
    if (!unit || !dropDirection(unit, columnKey)) return
    e.preventDefault()
    setDragOverColumn(columnKey)
  }

  async function handleDrop(e: DragEvent, columnKey: ColumnKey) {
    e.preventDefault()
    setDragOverColumn(null)
    const unitId = draggingUnitId
    setDraggingUnitId(null)
    if (!unitId) return
    const unit = units.find((u) => u.id === unitId)
    if (!unit) return
    const direction = dropDirection(unit, columnKey)
    if (!direction) return

    if (direction === 'backward') {
      const targetLabel = COLUMNS.find((c) => c.key === columnKey)?.label ?? columnKey
      const ok = await confirm({
        title: 'Move this unit backward?',
        message: `Moving "${unitLabel(unit)}" back to ${targetLabel} will clear its acceptance sign-off (BSP acceptance, Branch Manager and Head Office signatures) so it can be redone. This cannot be undone.`,
        confirmLabel: 'Move back & clear sign-off',
        danger: true,
      })
      if (!ok) return

      setValidating(true)
      try {
        await moveBoardColumn(unitId, columnKey, true)
        await refresh()
      } catch {
        showNotice('error', 'Could not move this unit. Please try again.')
      } finally {
        setValidating(false)
      }
      return
    }

    setValidating(true)
    try {
      const missing = await missingRequirements(unit.board_column as ColumnKey, unitId)
      if (missing.length > 0) {
        showNotice(
          'error',
          `Cannot move "${unitLabel(unit)}" to ${COLUMNS.find((c) => c.key === columnKey)?.label} — still missing: ${missing.join(', ')}.`,
        )
        return
      }
      // Requirements are met client-side, but board_column only advances
      // server-side when something actually re-checks it (e.g. after a
      // manual backward move, nothing else will trigger that) — sync it now.
      const { moved } = await syncBoardColumn(unitId)
      if (!moved) {
        showNotice('error', `Could not move "${unitLabel(unit)}" to ${COLUMNS.find((c) => c.key === columnKey)?.label}.`)
        return
      }
      await refresh()
    } catch {
      showNotice('error', 'Could not move this unit. Please try again.')
    } finally {
      setValidating(false)
    }
  }

  // Exception units (declared defective/damaged/wrong item) have their own
  // dedicated Exceptions page — they don't belong on the normal lifecycle
  // board, since they're a branch off the main flow, not a stage within it.
  const nonExceptionUnits = units.filter((u) => !u.is_exception)
  const siteOptions = uniqueSorted(nonExceptionUnits.map((u) => u.allocated_branch))
  const makeOptions = uniqueSorted(nonExceptionUnits.map((u) => u.device_make))
  const modelOptions = uniqueSorted(nonExceptionUnits.map((u) => u.device_model))
  const statusOptions = uniqueSorted(nonExceptionUnits.map((u) => u.status))
  const hasActiveFilters = !!(stageFilter || siteFilter || makeFilter || modelFilter || statusFilter || search.trim())
  const visibleUnits = nonExceptionUnits.filter((u) => {
    if (search.trim() && !matchesSearch(u, search)) return false
    if (stageFilter && u.board_column !== stageFilter) return false
    if (siteFilter && (u.allocated_branch ?? '') !== siteFilter) return false
    if (makeFilter && (u.device_make ?? '') !== makeFilter) return false
    if (modelFilter && (u.device_model ?? '') !== modelFilter) return false
    if (statusFilter && u.status !== statusFilter) return false
    return true
  })

  return (
    <div className="board-page">
      <div className="page-toolbar">
        <h1 className="page-title">Hardware Lifecycle Board</h1>
        <div className="page-toolbar-actions">
          <div className="view-toggle">
            <button
              type="button"
              className={`view-toggle-btn${view === 'board' ? ' view-toggle-btn-active' : ''}`}
              onClick={() => setView('board')}
            >
              Board
            </button>
            <button
              type="button"
              className={`view-toggle-btn${view === 'table' ? ' view-toggle-btn-active' : ''}`}
              onClick={() => setView('table')}
            >
              Table
            </button>
          </div>
          {view === 'table' && (
            <TableColumnPicker visible={visibleTableColumns} onChange={setVisibleTableColumns} />
          )}
          <input
            className="board-search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search alias, barcode, serial, make/model…"
          />
          <button type="button" className="btn btn-primary" onClick={() => setShowNewUnit(true)}>
            + New Unit
          </button>
        </div>
      </div>

      <div className="board-filters">
        <select className="board-filter-select" value={stageFilter} onChange={(e) => setStageFilter(e.target.value)}>
          <option value="">All stages</option>
          {COLUMNS.map((column) => (
            <option key={column.key} value={column.key}>
              {column.label}
            </option>
          ))}
        </select>
        <select className="board-filter-select" value={siteFilter} onChange={(e) => setSiteFilter(e.target.value)}>
          <option value="">All sites</option>
          {siteOptions.map((site) => (
            <option key={site} value={site}>
              {site}
            </option>
          ))}
        </select>
        <select className="board-filter-select" value={makeFilter} onChange={(e) => setMakeFilter(e.target.value)}>
          <option value="">All makes</option>
          {makeOptions.map((make) => (
            <option key={make} value={make}>
              {make}
            </option>
          ))}
        </select>
        <select className="board-filter-select" value={modelFilter} onChange={(e) => setModelFilter(e.target.value)}>
          <option value="">All models</option>
          {modelOptions.map((model) => (
            <option key={model} value={model}>
              {model}
            </option>
          ))}
        </select>
        <select className="board-filter-select" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
          <option value="">All statuses</option>
          {statusOptions.map((status) => (
            <option key={status} value={status}>
              {status}
            </option>
          ))}
        </select>
        <button
          type="button"
          className="btn"
          onClick={() => {
            setSearch('')
            setStageFilter('')
            setSiteFilter('')
            setMakeFilter('')
            setModelFilter('')
            setStatusFilter('')
          }}
          disabled={!hasActiveFilters}
        >
          Clear filters
        </button>
      </div>

      {error && <div className="board-error">{error}</div>}
      {dropNotice && <div className={`board-notice board-notice-${dropNotice.kind}`}>{dropNotice.message}</div>}
      {validating && <div className="board-notice board-notice-info">Checking readiness…</div>}

      {view === 'board' && (
        <div className="board-columns">
          {COLUMNS.map((col) => (
            <section
              key={col.key}
              className={`board-column${dragOverColumn === col.key ? ' board-column-droppable' : ''}`}
              onDragOver={(e) => handleDragOver(e, col.key)}
              onDragLeave={() => setDragOverColumn((c) => (c === col.key ? null : c))}
              onDrop={(e) => handleDrop(e, col.key)}
            >
              <h2>
                {col.label}
                <span className="board-column-count">{visibleUnits.filter((u) => u.board_column === col.key).length}</span>
              </h2>
              <div className="board-column-cards">
                {loading && <p className="board-empty">Loading…</p>}
                {!loading &&
                  visibleUnits
                    .filter((u) => u.board_column === col.key)
                    .map((u) => (
                      <div
                        key={u.id}
                        className={`board-card${draggingUnitId === u.id ? ' board-card-dragging' : ''}`}
                        draggable
                        onDragStart={() => handleDragStart(u.id)}
                        onDragEnd={handleDragEnd}
                        onClick={() => navigate(`/units/${u.id}`)}
                        role="button"
                        tabIndex={0}
                      >
                        <span className="board-card-title">{unitLabel(u)}</span>
                        <span className="board-card-meta">{u.serial_number || '—'}</span>
                        <span className="board-card-barcode">{u.allocated_branch || '—'}</span>
                        <span className="board-card-signoffs">
                          <SignOffDot label="Encoded" done={u.encoded} />
                          <SignOffDot label="Configured" done={u.configured} />
                          <SignOffDot label="QC" done={u.qc} />
                        </span>
                      </div>
                    ))}
                {!loading && visibleUnits.filter((u) => u.board_column === col.key).length === 0 && (
                  <p className="board-empty">{hasActiveFilters ? 'No matches' : 'No units yet'}</p>
                )}
              </div>
            </section>
          ))}
        </div>
      )}

      {view === 'table' && !loading && (
        <DataTable
          rows={visibleUnits}
          getRowKey={(u) => u.id}
          onRowClick={(u) => navigate(`/units/${u.id}`)}
          emptyMessage={hasActiveFilters ? 'No matches' : 'No units yet'}
          columns={TABLE_COLUMN_DEFS.filter((c) => visibleTableColumns.includes(c.key)).map((c) => ({
            key: c.key,
            header: c.label,
            render: c.render,
          }))}
        />
      )}
      {view === 'table' && loading && <p className="board-empty">Loading…</p>}

      {showNewUnit && (
        <NewUnitModal
          onClose={() => setShowNewUnit(false)}
          onCreated={() => {
            setShowNewUnit(false)
            refresh()
          }}
        />
      )}
    </div>
  )
}

function NewUnitModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [serialNumber, setSerialNumber] = useState('')
  const [deviceMake, setDeviceMake] = useState('')
  const [deviceModel, setDeviceModel] = useState('')
  const [partNumber, setPartNumber] = useState('')
  const [barcode, setBarcode] = useState('')
  const [allocatedBranch, setAllocatedBranch] = useState('')
  const [makes, setMakes] = useState<string[]>([])
  const [models, setModels] = useState<string[]>([])
  const [siteLocations, setSiteLocations] = useState<string[]>([])
  const [items, setItems] = useState<Item[]>([])
  const [itemQuery, setItemQuery] = useState('')
  const [generalFields, setGeneralFields] = useState<FieldDefinition[]>([])
  const [metaValues, setMetaValues] = useState<MetaValues>({})
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [scanTarget, setScanTarget] = useState<'serial' | 'barcode' | null>(null)
  const autoAxisUrlRef = useRef<string | null>(null)
  const metaValuesRef = useRef<MetaValues>({})
  metaValuesRef.current = metaValues

  useEffect(() => {
    listDeviceMakes().then(setMakes).catch(() => setMakes([]))
    listDeviceModels().then(setModels).catch(() => setModels([]))
    listSiteLocations()
      .then((locations) => setSiteLocations(locations.map((l) => l.name)))
      .catch(() => setSiteLocations([]))
    listItems().then(setItems).catch(() => setItems([]))
    listMetaDataFields('general')
      .then((fields) => {
        setGeneralFields(fields)
        setMetaValues(defaultMetaValues(fields))
      })
      .catch(() => setGeneralFields([]))
  }, [])

  const selectedItem = items.find((it) => formatItemLabel(it) === itemQuery) ?? null
  const selectedItemId = selectedItem?.id

  // Selecting an item from the Items master file auto-fills Device
  // Make/Model — the unit stays linked to the item so its Sales Order
  // Number can default the PO/Waybill Reference later, at Receiving.
  useEffect(() => {
    if (!selectedItem) return
    setDeviceMake(selectedItem.make)
    setDeviceModel(selectedItem.model)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedItemId])

  // Keep an admin-defined "Axis Website" field filled in with a link to
  // Axis's product search for this make/model/part — as long as the user
  // hasn't typed their own value over it.
  useEffect(() => {
    const axisField = generalFields.find(isAxisWebsiteField)
    if (!axisField || deviceMake.trim().toUpperCase() !== 'AXIS') return
    const url = axisWebsiteUrl(deviceModel, partNumber)
    if (!url) return
    const current = (metaValuesRef.current[axisField.field_key] as string) ?? ''
    if (current !== '' && current !== autoAxisUrlRef.current) return
    if (current === url) return
    autoAxisUrlRef.current = url
    setMetaValues((prev) => ({ ...prev, [axisField.field_key]: url }))
  }, [generalFields, deviceMake, deviceModel, partNumber])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      const effectiveAlias = generateAlias(allocatedBranch, deviceModel, serialNumber)
      await createUnit({
        alias: effectiveAlias || undefined,
        serial_number: serialNumber,
        device_make: deviceMake || undefined,
        device_model: deviceModel || undefined,
        part_number: partNumber || undefined,
        item_id: selectedItem?.id,
        barcode: barcode || undefined,
        allocated_branch: allocatedBranch || undefined,
        meta_data: Object.keys(metaValues).length > 0 ? metaValues : undefined,
      })
      onCreated()
    } catch (err) {
      if (err instanceof Error && err.message.startsWith('409')) {
        setError(
          err.message.includes('quantity remaining')
            ? 'That item has no quantity remaining in stock.'
            : 'That serial number is already in use.',
        )
      } else {
        setError('Failed to create unit. Check that your account has the Encoder or PM/PC role.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const itemOutOfStock = !!selectedItem && selectedItem.qty <= 0

  return (
    <div className="modal-overlay">
      <form
        className="modal-card modal-card-lg"
        onClick={(e) => e.stopPropagation()}
        onSubmit={handleSubmit}
        onKeyDown={focusNextFieldOnEnter}
      >
        <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
          &times;
        </button>
        <div className="modal-header">
          <h2>New Hardware Unit</h2>
          <p className="modal-subtitle">
            A barcode is generated automatically once you save. Full device configuration is captured later, at
            Stage 1-A.
          </p>
        </div>

        <div className="modal-body">
          <div className="form-section">
            <h3 className="form-section-title">Device</h3>
            <label htmlFor="item-lookup">Item (optional)</label>
            <Combobox
              id="item-lookup"
              value={itemQuery}
              onChange={setItemQuery}
              options={items.map(formatItemLabel)}
              placeholder="Look up an item to auto-fill Make/Model…"
            />
            <p className="modal-subtitle">
              Selecting an item from the Items master file fills in Make/Model below and links this unit to that
              item's Sales Order Number, used as the default PO/Waybill Reference at Receiving.
            </p>
            {selectedItem && (
              <p className={itemOutOfStock ? 'login-error' : 'modal-subtitle'}>
                {itemOutOfStock
                  ? 'This item has no quantity remaining in stock — pick a different item or update its qty in Admin → Items.'
                  : `${selectedItem.qty} in stock.`}
              </p>
            )}
            <div className="modal-field-row">
              <div>
                <label htmlFor="make">Device Make</label>
                <Combobox id="make" value={deviceMake} onChange={setDeviceMake} options={makes} uppercase />
              </div>
              <div>
                <label htmlFor="model">Device Model</label>
                <Combobox id="model" value={deviceModel} onChange={setDeviceModel} options={models} uppercase />
              </div>
            </div>

            <div className="modal-field-row">
              <div>
                <label htmlFor="part-number">Part Number</label>
                <input
                  id="part-number"
                  value={partNumber}
                  onChange={(e) => setPartNumber(e.target.value.toUpperCase())}
                />
              </div>
              <div>
                <label htmlFor="barcode">Barcode</label>
                <div className="input-with-scan">
                  <input
                    id="barcode"
                    value={barcode}
                    onChange={(e) => setBarcode(e.target.value.toUpperCase())}
                    placeholder="Leave blank to auto-generate"
                  />
                  <button type="button" className="btn" onClick={() => setScanTarget('barcode')}>
                    📷 Scan
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div className="form-section">
            <h3 className="form-section-title">Identity</h3>
            <label htmlFor="serial">Serial Number</label>
            <div className="input-with-scan">
              <input
                id="serial"
                value={serialNumber}
                onChange={(e) => setSerialNumber(e.target.value.toUpperCase())}
                required
              />
              <button type="button" className="btn" onClick={() => setScanTarget('serial')}>
                📷 Scan
              </button>
            </div>
          </div>

          <div className="form-section">
            <h3 className="form-section-title">Assignment</h3>
            <label htmlFor="allocated-branch">Allocated Branch / Site</label>
            <Combobox
              id="allocated-branch"
              value={allocatedBranch}
              onChange={setAllocatedBranch}
              options={siteLocations}
            />
            <p className="modal-subtitle">
              An early intended-destination assignment, before shipment details are known.
            </p>
          </div>

          {error && <div className="login-error">{error}</div>}
        </div>

        <div className="modal-footer">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting || !serialNumber || itemOutOfStock}>
            {submitting ? 'Creating…' : 'Create'}
          </button>
        </div>
      </form>
      {scanTarget && (
        <BarcodeScannerModal
          onClose={() => setScanTarget(null)}
          onDetected={(value) => {
            if (scanTarget === 'serial') setSerialNumber(value.toUpperCase())
            else setBarcode(value.toUpperCase())
            setScanTarget(null)
          }}
        />
      )}
    </div>
  )
}
