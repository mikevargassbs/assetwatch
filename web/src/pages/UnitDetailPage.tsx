import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import {
  barcodeStickerUrl,
  deleteUnit,
  getReceiving,
  getStage1A,
  getStage1B,
  getUnit,
  listMetaDataFields,
  networkDefaultsForSite,
  recordReceiving,
  signOffStage1A,
  signOffStage1B,
  updateUnitIdentity,
  updateUnitMetaData,
  upsertStage1A,
  upsertStage1B,
  type FieldDefinition,
  type HardwareUnit,
  type Stage0Receiving,
  type Stage1A,
  type Stage1B,
} from '../api/hardware'
import { authFetch } from '../api/client'
import { listItems, listSiteLocations, type Item, type SiteLocation } from '../api/admin'
import { CreateFieldModal } from './AdminPage'
import { listAuditLog, type AuditLogEntry } from '../api/audit'
import { downloadAuthenticated, unitInfoSheetUrl } from '../api/reporting'
import { useConfirm } from '../components/ConfirmDialogProvider'
import { useSnackbar } from '../components/SnackbarProvider'
import { Combobox } from '../components/Combobox'
import { DataTable } from '../components/DataTable'
import { Tabs } from '../components/Tabs'
import { DynamicFields, defaultMetaValues, type MetaValues } from '../components/DynamicFields'
import { MaskedInput } from '../components/MaskedInput'
import { axisWebsiteUrl, isAxisWebsiteField } from '../utils/axis'
import { subnetAddresses } from '../utils/network'
import { getUnitLogistics, type UnitLogisticsView } from '../api/logistics'
import {
  getInstallation,
  recordSiteReceipt,
  signOffInstallation,
  upsertInstallation,
  listInstallationPhotos,
  uploadInstallationPhoto,
  deleteInstallationPhoto,
  installationPhotoUrl,
  type SiteInstallation,
  type InstallationPhoto,
} from '../api/installation'
import {
  documentDownloadUrl,
  emailClientSigningLink,
  emailHeadOfficeSigningLink,
  generateClientSigningLink,
  generateHeadOfficeSigningLink,
  getAcceptance,
  recordBSPAcceptance,
  uploadManualDocument,
  type ClientAcceptance,
} from '../api/acceptance'
import { SignaturePad } from '../components/SignaturePad'
import {
  declareDefect,
  emailSupplier,
  getDefectReport,
  markDelivered,
  markShippedBack,
  markSupplierReceived,
  recordReplacement,
  reportUrl,
  type DefectReport,
} from '../api/defective'

async function openAcceptanceDocument(unitId: string) {
  const res = await authFetch(documentDownloadUrl(unitId))
  if (!res.ok) return
  const blob = await res.blob()
  window.open(URL.createObjectURL(blob), '_blank')
}

const MAX_INSTALLATION_PHOTOS = 3

function isMobileDevice(): boolean {
  return /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent)
}

async function openBarcodeSticker(unitId: string) {
  const res = await authFetch(barcodeStickerUrl(unitId))
  if (!res.ok) return
  const blob = await res.blob()
  window.open(URL.createObjectURL(blob), '_blank')
}

export function UnitDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { roles } = useAuth()
  const confirm = useConfirm()
  const showSnackbar = useSnackbar()
  const [unit, setUnit] = useState<HardwareUnit | null>(null)
  const [stage1a, setStage1a] = useState<Stage1A | null>(null)
  const [stage1b, setStage1b] = useState<Stage1B | null>(null)
  const [fields1a, setFields1a] = useState<FieldDefinition[]>([])
  const [logistics, setLogistics] = useState<UnitLogisticsView | null>(null)
  const [installationRec, setInstallationRec] = useState<SiteInstallation | null>(null)
  const [acceptance, setAcceptance] = useState<ClientAcceptance | null>(null)
  const [defect, setDefect] = useState<DefectReport | null>(null)
  const [receiving, setReceiving] = useState<Stage0Receiving | null>(null)
  const [items, setItems] = useState<Item[]>([])
  const [showDeclareDefect, setShowDeclareDefect] = useState(false)
  const [activeTab, setActiveTab] = useState<string | null>(null)

  async function refresh() {
    if (!id) return
    const u = await getUnit(id)
    setUnit(u)
    setFields1a(await listMetaDataFields('stage1a'))
    try {
      setReceiving(await getReceiving(id))
    } catch {
      setReceiving(null)
    }

    let sa: Stage1A | null = null
    try {
      sa = await getStage1A(id)
    } catch {
      sa = null
    }
    setStage1a(sa)

    // Firmware Configuration (Stage 1-B) is what happens while the unit sits
    // in the "configuration" board column — which starts as soon as Encoded
    // is signed off, not only once all of Stage 1-A (including its own
    // Configured/QC) is done. Later stages genuinely don't exist until their
    // prior stage is signed off, so those still skip the lookup entirely.
    let sb: Stage1B | null = null
    if (u.board_column !== 'pre_deployment') {
      try {
        sb = await getStage1B(id)
      } catch {
        sb = null
      }
    }
    setStage1b(sb)

    let lg: UnitLogisticsView | null = null
    if (sb?.signed_off_at) {
      try {
        lg = await getUnitLogistics(id)
      } catch {
        lg = null
      }
    }
    setLogistics(lg)

    let inst: SiteInstallation | null = null
    if (lg?.docket?.received_at) {
      try {
        inst = await getInstallation(id)
      } catch {
        inst = null
      }
    }
    setInstallationRec(inst)

    if (inst?.signed_off_at) {
      try {
        setAcceptance(await getAcceptance(id))
      } catch {
        setAcceptance(null)
      }
    } else {
      setAcceptance(null)
    }

    if (u.status === 'defective') {
      try {
        setDefect(await getDefectReport(id))
      } catch {
        setDefect(null)
      }
    } else {
      setDefect(null)
    }
  }

  useEffect(() => {
    refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  useEffect(() => {
    listItems(true).then(setItems).catch(() => setItems([]))
  }, [])

  // Reusable success snackbar for the various stage sections below, all of
  // which just refresh the unit after a save — only the message differs.
  function onSaved(message: string) {
    return () => {
      showSnackbar(message, 'success')
      refresh()
    }
  }

  if (!unit) {
    return <div className="detail-page">Loading…</div>
  }

  const has = (role: string) => roles.includes(role) || roles.includes('admin')
  const currentUnit = unit
  const canEditDefect =
    has('pm_pc') ||
    has('encoder') ||
    has('configurator') ||
    has('qc') ||
    has('logistics') ||
    has('field_technician') ||
    has('bsp_acceptance_officer')

  async function handleDelete() {
    const ok = await confirm({
      title: `Delete "${currentUnit.alias || currentUnit.barcode}"?`,
      message: 'This retires the unit and removes it from the board. Its history is kept and can be restored by an admin if needed.',
      confirmLabel: 'Delete',
      danger: true,
    })
    if (!ok) return
    try {
      await deleteUnit(currentUnit.id)
      showSnackbar('Unit deleted.', 'success')
      navigate('/board', { replace: true })
    } catch {
      showSnackbar('Failed to delete unit.', 'error')
    }
  }

  const tabs: { key: string; label: string; status?: 'done' | 'attention'; content: ReactNode }[] = []

  tabs.push({
    key: 'receiving',
    label: 'Receiving',
    status: receiving ? 'done' : undefined,
    content: (
      <ReceivingSection
        unitId={unit.id}
        canRecord={has('pm_pc')}
        receiving={receiving}
        defaultPoRef={items.find((it) => it.id === unit.item_id)?.sales_order_number}
        onDone={onSaved('Receiving recorded.')}
      />
    ),
  })

  if (defect) {
    tabs.push({
      key: 'defect',
      label: 'Defect',
      status: 'attention',
      content: (
        <DefectSection unitId={unit.id} defect={defect} canEdit={canEditDefect} onDone={onSaved('Defect report updated.')} />
      ),
    })
  }

  tabs.push({
    key: 'stage1a',
    label: 'Pre-Deployment Config & QC',
    status: stage1a?.signed_off_at ? 'done' : undefined,
    content: (
      <Stage1ASection
        unitId={unit.id}
        unit={unit}
        stage={stage1a}
        fields={fields1a}
        canEdit={has('encoder') || has('configurator') || has('qc')}
        canEncode={has('encoder')}
        canConfigure={has('configurator')}
        canQC={has('qc')}
        onDone={onSaved('Configuration saved.')}
      />
    ),
  })

  if (unit.board_column !== 'pre_deployment') {
    tabs.push({
      key: 'stage1b',
      label: 'Firmware Configuration',
      status: stage1b?.signed_off_at ? 'done' : undefined,
      content: (
        <Stage1BSection
          unitId={unit.id}
          stage={stage1b}
          canEdit={has('configurator') || has('qc')}
          canConfigure={has('configurator')}
          canQC={has('qc')}
          canDeclareDefect={canEditDefect}
          hasDefect={!!defect}
          onDone={onSaved('Firmware configuration saved.')}
          onDefectDeclared={onSaved('Firmware update issue reported — unit marked defective.')}
        />
      ),
    })
  }

  if (stage1b?.signed_off_at) {
    tabs.push({
      key: 'logistics',
      label: 'Logistics',
      status: logistics?.docket?.received_at ? 'done' : undefined,
      content: <LogisticsSection logistics={logistics} />,
    })
  }

  if (logistics?.docket?.received_at) {
    tabs.push({
      key: 'installation',
      label: 'Installation',
      status: installationRec?.signed_off_at ? 'done' : undefined,
      content: (
        <InstallationSection
          unitId={unit.id}
          installation={installationRec}
          allocatedBranch={unit.allocated_branch}
          canEdit={has('field_technician')}
          onDone={onSaved('Installation recorded.')}
        />
      ),
    })
  }

  if (installationRec?.signed_off_at) {
    tabs.push({
      key: 'acceptance',
      label: 'Acceptance',
      status: acceptance?.signed_off_at ? 'done' : undefined,
      content: (
        <AcceptanceSection
          unitId={unit.id}
          acceptance={acceptance}
          canEdit={has('bsp_acceptance_officer')}
          onDone={onSaved('Acceptance recorded.')}
        />
      ),
    })
  }

  // Attributes and Audit History always sit at the end of the tab list —
  // they're supporting/reference info, not part of the main stage-by-stage
  // flow.
  tabs.push({
    key: 'attributes',
    label: 'Attributes',
    content: <AttributesSection unitId={unit.id} unit={unit} canEdit={canEditDefect} onDone={refresh} />,
  })

  if (has('admin')) {
    tabs.push({ key: 'audit', label: 'Audit History', content: <AuditHistorySection unitId={unit.id} /> })
  }

  // Overall lifecycle progress shown in the header — independent of which
  // tabs currently exist, since later stages only appear once their
  // predecessor is signed off (so an absent tab reads as "not started",
  // not "done").
  const lifecycleDone = [
    !!receiving,
    !!stage1a?.signed_off_at,
    !!stage1b?.signed_off_at,
    !!logistics?.docket?.received_at,
    !!installationRec?.signed_off_at,
    !!acceptance?.signed_off_at,
  ].filter(Boolean).length
  const lifecycleTotal = 6

  // Land on Receiving by default so it's easy to spot when it's still
  // outstanding — but once it's already recorded, skip straight to
  // Pre-Deployment Config & QC instead of a tab there's nothing left to do.
  const defaultTabKey = (receiving ? tabs.find((t) => t.key === 'stage1a')?.key : undefined) ?? tabs[0]?.key
  const currentTabKey = activeTab && tabs.some((t) => t.key === activeTab) ? activeTab : defaultTabKey
  const currentTab = tabs.find((t) => t.key === currentTabKey)

  return (
    <div className="detail-page">
      <header className="detail-header">
        <button className="link-button detail-back" onClick={() => navigate('/board')}>
          ← Back to board
        </button>
        <div className="detail-heading">
          <h1>{unit.alias || unit.barcode}</h1>
          {unit.alias && <span className="detail-barcode">{unit.barcode}</span>}
        </div>
        <span className="detail-column">{unit.board_column.replace(/_/g, ' ')}</span>
        {unit.is_exception && <span className="detail-column detail-column-exception">Exception</span>}
        <div className="detail-header-actions">
          <button
            type="button"
            className="btn"
            onClick={() => downloadAuthenticated(unitInfoSheetUrl(unit.id))}
          >
            Print Report
          </button>
          {canEditDefect && !defect && !installationRec?.signed_off_at && (
            <button type="button" className="btn btn-danger-outline" onClick={() => setShowDeclareDefect(true)}>
              Declare Defect
            </button>
          )}
          {has('admin') && !installationRec?.signed_off_at && (
            <button type="button" className="btn btn-danger" onClick={handleDelete}>
              Delete Unit
            </button>
          )}
        </div>
        <div className="detail-header-progress">
          <div className="detail-header-progress-bar">
            <div
              className={`detail-header-progress-fill${lifecycleDone >= lifecycleTotal ? ' detail-header-progress-fill-complete' : ''}`}
              style={{ width: `${(lifecycleDone / lifecycleTotal) * 100}%` }}
            />
          </div>
          <span className="detail-header-progress-label">
            {lifecycleDone}/{lifecycleTotal} lifecycle stages complete
          </span>
        </div>
      </header>

      {defect && (
        <div className="detail-defect-banner">
          <strong>Defect declared</strong>
          <span>
            — {defect.defect_type.replace(/_/g, ' ')}
            {defect.description ? `: ${defect.description}` : ''}
          </span>
          <button type="button" className="link-button" onClick={() => setActiveTab('defect')}>
            View defect report
          </button>
        </div>
      )}

      {showDeclareDefect && (
        <DeclareDefectModal
          unitId={unit.id}
          onClose={() => setShowDeclareDefect(false)}
          onDeclared={() => {
            setShowDeclareDefect(false)
            refresh()
          }}
        />
      )}

      {currentTabKey && (
        <Tabs
          tabs={tabs.map((t) => ({ key: t.key, label: t.label, status: t.status }))}
          active={currentTabKey}
          onChange={setActiveTab}
        />
      )}

      {currentTab?.content}
    </div>
  )
}

function AttributesSection({
  unitId,
  unit,
  canEdit,
  onDone,
}: {
  unitId: string
  unit: HardwareUnit
  canEdit: boolean
  onDone: () => void
}) {
  const { roles } = useAuth()
  const [fields, setFields] = useState<FieldDefinition[]>([])
  const [meta, setMeta] = useState<MetaValues>({})
  const [submitting, setSubmitting] = useState(false)
  const [showCreateField, setShowCreateField] = useState(false)
  const showSnackbar = useSnackbar()
  const isAdmin = roles.includes('admin')

  function loadFields() {
    listMetaDataFields('general').then(setFields).catch(() => setFields([]))
  }

  useEffect(() => {
    loadFields()
  }, [])

  useEffect(() => {
    const values = defaultMetaValues(fields, unit.meta_data)
    const axisField = fields.find(isAxisWebsiteField)
    if (axisField && !values[axisField.field_key] && unit.device_make?.trim().toUpperCase() === 'AXIS') {
      const url = axisWebsiteUrl(unit.device_model ?? '', unit.part_number ?? '')
      if (url) values[axisField.field_key] = url
    }
    setMeta(values)
  }, [fields, unit.meta_data, unit.device_make, unit.device_model, unit.part_number])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await updateUnitMetaData(unitId, meta)
      showSnackbar('Attributes saved.', 'success')
      onDone()
    } catch {
      showSnackbar('Failed to save attributes.', 'error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <section className="detail-section">
        <div className="section-toolbar">
          <h2>Attributes</h2>
          {isAdmin && (
            <button type="button" className="btn btn-primary" onClick={() => setShowCreateField(true)}>
              + New Field
            </button>
          )}
        </div>
        {fields.length === 0 ? (
          <p className="board-empty">No general attribute fields have been configured yet.</p>
        ) : canEdit ? (
          <form onSubmit={handleSave} className="detail-form">
            <DynamicFields fields={fields} values={meta} onChange={setMeta} />
            <button type="submit" disabled={submitting}>
              {submitting ? 'Saving…' : 'Save attributes'}
            </button>
          </form>
        ) : (
          <dl className="detail-readonly-list">
            {fields.map((f) => (
              <div key={f.field_key}>
                <dt>{f.label}</dt>
                <dd>{f.data_type === 'boolean' ? (meta[f.field_key] ? 'Yes' : 'No') : (meta[f.field_key] as string) || '—'}</dd>
              </div>
            ))}
          </dl>
        )}
      </section>

      {showCreateField && (
        <CreateFieldModal
          stage="general"
          onClose={() => setShowCreateField(false)}
          onCreated={() => {
            setShowCreateField(false)
            loadFields()
          }}
        />
      )}
    </>
  )
}

function AuditHistorySection({ unitId }: { unitId: string }) {
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    listAuditLog({ entity_type: 'hardware_unit', entity_id: unitId, limit: 200 })
      .then(setEntries)
      .finally(() => setLoading(false))
  }, [unitId])

  return (
    <section className="detail-section">
      <h2>Audit History</h2>
      {loading && <p className="board-empty">Loading…</p>}
      {!loading && (
        <DataTable
          rows={entries}
          getRowKey={(e) => e.id}
          emptyMessage="No audit entries for this unit."
          columns={[
            { key: 'performed_at', header: 'When', render: (e) => new Date(e.performed_at).toLocaleString() },
            { key: 'action', header: 'Action', render: (e) => e.action },
            { key: 'performed_by_name', header: 'Performed By', render: (e) => e.performed_by_name ?? 'System' },
          ]}
        />
      )}
    </section>
  )
}

function ReceivingSection({
  unitId,
  canRecord,
  receiving,
  defaultPoRef,
  onDone,
}: {
  unitId: string
  canRecord: boolean
  receiving: Stage0Receiving | null
  defaultPoRef?: string
  onDone: () => void
}) {
  const [showModal, setShowModal] = useState(false)

  if (!canRecord && !receiving) return null

  return (
    <section className="detail-section receiving-summary">
      <div>
        <h2>Receiving</h2>
        {receiving ? (
          <p className="detail-status">
            {receiving.po_or_waybill_ref ? `PO/Waybill: ${receiving.po_or_waybill_ref}` : 'No PO/Waybill ref recorded'}
            {' — '}
            {receiving.items_correct === false ? (
              <span style={{ color: 'var(--danger)' }}>discrepancy reported</span>
            ) : (
              'items correct'
            )}
          </p>
        ) : (
          <p className="detail-status">Not recorded yet — optional, can be added any time.</p>
        )}
      </div>
      {canRecord && (
        <button type="button" className="btn" onClick={() => setShowModal(true)}>
          {receiving ? 'Edit Receiving' : 'Record Receiving'}
        </button>
      )}

      {showModal && (
        <ReceivingModal
          unitId={unitId}
          receiving={receiving}
          defaultPoRef={defaultPoRef}
          onClose={() => setShowModal(false)}
          onSaved={() => {
            setShowModal(false)
            onDone()
          }}
        />
      )}
    </section>
  )
}

function ReceivingModal({
  unitId,
  receiving,
  defaultPoRef,
  onClose,
  onSaved,
}: {
  unitId: string
  receiving: Stage0Receiving | null
  defaultPoRef?: string
  onClose: () => void
  onSaved: () => void
}) {
  const [waybill, setWaybill] = useState(receiving?.po_or_waybill_ref ?? defaultPoRef ?? '')
  const [itemsCorrect, setItemsCorrect] = useState(receiving?.items_correct ?? true)
  const [notes, setNotes] = useState(receiving?.discrepancy_notes ?? '')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await recordReceiving(unitId, {
        po_or_waybill_ref: waybill || undefined,
        items_correct: itemsCorrect,
        discrepancy_notes: notes || undefined,
      })
      onSaved()
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="modal-overlay">
      <form className="modal-card" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
          &times;
        </button>
        <h2>Receiving</h2>
        <label>PO / Waybill Ref</label>
        <input value={waybill} onChange={(e) => setWaybill(e.target.value)} placeholder="Optional" />
        <label className="checkbox-row">
          <input type="checkbox" checked={itemsCorrect} onChange={(e) => setItemsCorrect(e.target.checked)} />
          All items correct
        </label>
        {!itemsCorrect && (
          <>
            <label>Discrepancy notes</label>
            <textarea value={notes} onChange={(e) => setNotes(e.target.value)} />
          </>
        )}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>
            {submitting ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </div>
  )
}

const MASKED_STAGE1A_FIELDS = new Set(['default_password'])
const UPPERCASE_EXEMPT_STAGE1A_FIELDS = new Set(['default_username', 'default_password', 'frequency_hz'])

const STAGE1A_FIELD_GROUPS = [
  {
    heading: 'Device',
    fields: [
      ['device_make', 'Device Make'],
      ['device_model', 'Device Model'],
      ['serial_number', 'Serial Number'],
    ],
  },
  {
    heading: 'Network',
    fields: [
      ['device_name_dns', 'Device Name (DNS)'],
      ['mac_address', 'MAC Address'],
      ['client_ip_address', 'Client IP Address'],
      ['dns_server_1', 'DNS Server 1'],
      ['dns_server_2', 'DNS Server 2'],
      ['ntp_server', 'NTP Server'],
      ['frequency_hz', 'Frequency (Hz)'],
    ],
  },
  {
    heading: 'Credentials',
    fields: [
      ['default_username', 'Default Username'],
      ['default_password', 'Default Password'],
    ],
  },
] as const

function formatIPInput(raw: string, isDeleting: boolean): string {
  let cleaned = raw.replace(/[^0-9.]/g, '').replace(/\.{2,}/g, '.')
  if (cleaned.startsWith('.')) cleaned = cleaned.slice(1)
  const segments = cleaned
    .split('.')
    .slice(0, 4)
    .map((seg) => {
      seg = seg.slice(0, 3)
      if (seg !== '' && parseInt(seg, 10) > 255) seg = '255'
      return seg
    })
  let result = segments.join('.')
  // auto-advance to the next octet once a segment fills 3 digits, so typing
  // more than 3 digits in a row doesn't get silently dropped
  if (!isDeleting && segments.length < 4 && segments[segments.length - 1].length === 3) {
    result += '.'
  }
  return result
}

// Installation's site_ip/site_subnet/site_gateway are all IP-shaped, so they
// get the same format-as-you-type behavior as Stage 1-A's network fields.
const INSTALLATION_IP_FIELDS = new Set(['site_ip', 'site_subnet', 'site_gateway'])

function formatMACInput(raw: string, _isDeleting: boolean): string {
  const cleaned = raw
    .replace(/[^0-9a-fA-F]/g, '')
    .toUpperCase()
    .slice(0, 12)
  return (cleaned.match(/.{1,2}/g) ?? []).join(':')
}

function formatNumericInput(raw: string): string {
  return raw.replace(/[^0-9]/g, '')
}

function isValidIPv4(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) return true
  const parts = trimmed.split('.')
  if (parts.length !== 4) return false
  return parts.every((part) => /^[0-9]{1,3}$/.test(part) && Number(part) >= 0 && Number(part) <= 255)
}

function isValidMACAddress(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) return true
  return /^([0-9A-F]{2}:){5}[0-9A-F]{2}$/i.test(trimmed)
}

const STAGE1A_FIELD_FORMATTERS: Record<string, (raw: string, isDeleting: boolean) => string> = {
  client_ip_address: formatIPInput,
  dns_server_1: formatIPInput,
  dns_server_2: formatIPInput,
  mac_address: formatMACInput,
  frequency_hz: formatNumericInput,
}

// Fields that take an IP address at this site and so benefit from the
// site's subnet-derived suggestion list, on top of any history-based ones.
const IP_SUGGESTION_FIELDS = new Set(['client_ip_address', 'dns_server_1', 'dns_server_2', 'ntp_server'])

function Stage1ASection({
  unitId,
  unit,
  stage,
  fields,
  canEdit,
  canEncode,
  canConfigure,
  canQC,
  onDone,
}: {
  unitId: string
  unit: HardwareUnit
  stage: Stage1A | null
  fields: FieldDefinition[]
  canEdit: boolean
  canEncode: boolean
  canConfigure: boolean
  canQC: boolean
  onDone: () => void
}) {
  const [form, setForm] = useState<Record<string, string>>({})
  const [meta, setMeta] = useState<MetaValues>({})
  const [accessories, setAccessories] = useState({
    type: '',
    input_type: '',
    model: '',
    power_type: '',
    automatic_gain_control: false,
  })
  const [identityForm, setIdentityForm] = useState({ alias: '', part_number: '', barcode: '', allocated_branch: '' })
  const [siteLocations, setSiteLocations] = useState<SiteLocation[]>([])
  const [networkOptions, setNetworkOptions] = useState<Record<string, string[]>>({})
  const [submitting, setSubmitting] = useState(false)
  const [identityError, setIdentityError] = useState<string | null>(null)
  const showSnackbar = useSnackbar()
  const autoMacRef = useRef<string | null>(null)

  useEffect(() => {
    listSiteLocations()
      .then(setSiteLocations)
      .catch(() => setSiteLocations([]))
  }, [])

  // Many devices (e.g. Axis cameras) print the MAC address as the serial
  // number, so keep MAC Address in sync with Serial Number as it's typed —
  // unless the encoder has typed their own value into MAC Address, in which
  // case that takes precedence and auto-fill stops.
  useEffect(() => {
    if (!form.serial_number) return
    const mac = formatMACInput(form.serial_number, false)
    if (!mac) return
    const current = form.mac_address ?? ''
    if (current !== '' && current !== autoMacRef.current) return
    if (current === mac) return
    autoMacRef.current = mac
    setForm((f) => ({ ...f, mac_address: mac }))
  }, [form.serial_number, form.mac_address])

  const matchedSite = siteLocations.find(
    (l) => l.name.trim().toLowerCase() === identityForm.allocated_branch.trim().toLowerCase(),
  )

  // Suggest addresses in the site's own subnet for the fields that take an
  // IP, once we know its Gateway/Subnet Mask — on top of whatever
  // history-based suggestions networkDefaultsForSite already found.
  const subnetOptions = useMemo(
    () =>
      matchedSite?.ip_gateway && matchedSite?.subnet_mask
        ? subnetAddresses(matchedSite.ip_gateway, matchedSite.subnet_mask)
        : [],
    [matchedSite?.ip_gateway, matchedSite?.subnet_mask],
  )

  useEffect(() => {
    setForm({
      // `||`, not `??` — a saved Stage 1-A record's field can be an empty
      // string (not null), which `??` would treat as "already set" and
      // never fall back to the unit's own value.
      device_make: stage?.device_make || unit.device_make || '',
      device_model: stage?.device_model || unit.device_model || '',
      device_name_dns: stage?.device_name_dns ?? '',
      client_ip_address: stage?.client_ip_address ?? '',
      serial_number: stage?.serial_number || unit.serial_number || '',
      mac_address: stage?.mac_address ?? '',
      dns_server_1: stage?.dns_server_1 ?? '',
      dns_server_2: stage?.dns_server_2 ?? '',
      ntp_server: stage?.ntp_server ?? '',
      frequency_hz: stage?.frequency_hz != null ? String(stage.frequency_hz) : '50',
      default_username: stage?.default_username ?? 'root',
      default_password: stage?.default_password ?? '',
    })
    setMeta(defaultMetaValues(fields, stage?.meta_data))
    setAccessories({
      type: (stage?.meta_data?.accessories_type as string) ?? '',
      input_type: (stage?.meta_data?.accessories_input_type as string) ?? '',
      model: (stage?.meta_data?.accessories_model as string) ?? '',
      power_type: (stage?.meta_data?.accessories_power_type as string) ?? '',
      automatic_gain_control: Boolean(stage?.meta_data?.accessories_automatic_gain_control),
    })
  }, [stage, fields])

  useEffect(() => {
    if (!unit.allocated_branch) return
    networkDefaultsForSite(unit.allocated_branch)
      .then((defaults) => {
        setNetworkOptions({
          dns_server_1: defaults.dns_server_1_options,
          dns_server_2: defaults.dns_server_2_options,
          ntp_server: defaults.ntp_server_options,
        })
        // Only pre-fill fields the encoder hasn't already saved a value for.
        setForm((f) => ({
          ...f,
          client_ip_address: f.client_ip_address || defaults.client_ip_address,
          dns_server_1: f.dns_server_1 || defaults.dns_server_1,
          dns_server_2: f.dns_server_2 || defaults.dns_server_2,
          ntp_server: f.ntp_server || defaults.ntp_server,
        }))
      })
      .catch(() => {})
  }, [stage, unit.allocated_branch])

  useEffect(() => {
    setIdentityForm({
      alias: unit.alias ?? '',
      part_number: unit.part_number ?? '',
      barcode: unit.barcode,
      allocated_branch: unit.allocated_branch ?? '',
    })
  }, [unit])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setIdentityError(null)
    try {
      if (!isValidIPv4(form.client_ip_address ?? '')) {
        setIdentityError('Client IP Address must be a valid IPv4 address.')
        return
      }
      if (!isValidIPv4(form.dns_server_1 ?? '')) {
        setIdentityError('DNS Server 1 must be a valid IPv4 address.')
        return
      }
      if (!isValidIPv4(form.dns_server_2 ?? '')) {
        setIdentityError('DNS Server 2 must be a valid IPv4 address.')
        return
      }
      if (!isValidMACAddress(form.mac_address ?? '')) {
        setIdentityError('MAC Address must be a complete MAC address in the format AA:BB:CC:DD:EE:FF.')
        return
      }

      await upsertStage1A(unitId, {
        ...form,
        frequency_hz: form.frequency_hz ? Number(form.frequency_hz) : undefined,
        meta_data: {
          ...meta,
          accessories_type: accessories.type,
          accessories_input_type: accessories.input_type,
          accessories_model: accessories.model,
          accessories_power_type: accessories.power_type,
          accessories_automatic_gain_control: accessories.automatic_gain_control,
        },
      })
      await updateUnitIdentity(unitId, {
        alias: identityForm.alias || undefined,
        part_number: identityForm.part_number || undefined,
        serial_number: form.serial_number || unit.serial_number,
        device_make: form.device_make || unit.device_make,
        device_model: form.device_model || unit.device_model,
        allocated_branch: identityForm.allocated_branch || undefined,
        barcode: identityForm.barcode,
      })
      onDone()
    } catch (err) {
      if (err instanceof Error && err.message.startsWith('409') && err.message.toLowerCase().includes('ip')) {
        setIdentityError('That client IP address is already in use at this site.')
      } else if (err instanceof Error && err.message.startsWith('409')) {
        setIdentityError('That serial number is already in use.')
      } else {
        showSnackbar(`Failed to save configuration: ${err instanceof Error ? err.message : String(err)}`, 'error')
      }
    } finally {
      setSubmitting(false)
    }
  }

  async function handleSignOff(step: 'encoded' | 'configured' | 'qc' | 'qc_fail', performedAt?: string) {
    await signOffStage1A(unitId, step, performedAt)
    onDone()
  }

  return (
    <>
      <section className="detail-section">
        <h2>Pre-Deployment Configuration &amp; QC</h2>
        {canEdit && (
          <form onSubmit={handleSave} className="detail-form">
            {STAGE1A_FIELD_GROUPS.map((group) => (
              <div key={group.heading}>
                <h3 className="dynamic-fields-heading">{group.heading}</h3>
                <div className="detail-form-grid">
                  {group.heading === 'Device' && (
                    <>
                      <div className="form-field">
                        <label>Alias / Name</label>
                        <input
                          value={identityForm.alias}
                          onChange={(e) =>
                            setIdentityForm({ ...identityForm, alias: e.target.value.toUpperCase() })
                          }
                        />
                      </div>
                      <div className="form-field">
                        <label>Part Number</label>
                        <input
                          value={identityForm.part_number}
                          onChange={(e) =>
                            setIdentityForm({ ...identityForm, part_number: e.target.value.toUpperCase() })
                          }
                        />
                      </div>
                      <div className="form-field">
                        <label>Barcode</label>
                        <input
                          value={identityForm.barcode}
                          onChange={(e) =>
                            setIdentityForm({ ...identityForm, barcode: e.target.value.toUpperCase() })
                          }
                        />
                      </div>
                      <div className="form-field">
                        <label>Allocated Branch / Site</label>
                        <Combobox
                          value={identityForm.allocated_branch}
                          onChange={(v) => setIdentityForm({ ...identityForm, allocated_branch: v })}
                          options={siteLocations.map((l) => l.name)}
                        />
                      </div>
                    </>
                  )}
                  {group.heading === 'Network' && (
                    <>
                      <div className="form-field">
                        <label>Gateway</label>
                        <input value={matchedSite?.ip_gateway ?? ''} disabled placeholder="Set on the site's record" />
                      </div>
                      <div className="form-field">
                        <label>Subnet Mask</label>
                        <input value={matchedSite?.subnet_mask ?? ''} disabled placeholder="Set on the site's record" />
                      </div>
                    </>
                  )}
                  {group.fields.map(([key, label]) => {
                    if (MASKED_STAGE1A_FIELDS.has(key)) {
                      return (
                        <div className="form-field" key={key}>
                          <label>{label}</label>
                          <MaskedInput value={form[key] ?? ''} onChange={(v) => setForm({ ...form, [key]: v })} />
                        </div>
                      )
                    }
                    // History-based suggestions (last values used at this site) come
                    // first, backed by the site's full subnet range for IP fields.
                    const historyOptions = networkOptions[key] ?? []
                    const fieldOptions = IP_SUGGESTION_FIELDS.has(key)
                      ? [...historyOptions, ...subnetOptions.filter((a) => !historyOptions.includes(a))]
                      : historyOptions
                    return (
                      <div className="form-field" key={key}>
                        <label>{label}</label>
                        <input
                          value={form[key] ?? ''}
                          list={fieldOptions.length > 0 ? `${key}-options` : undefined}
                          onChange={(e) => {
                            const formatter = STAGE1A_FIELD_FORMATTERS[key]
                            const isDeleting =
                              e.nativeEvent instanceof InputEvent && e.nativeEvent.inputType?.startsWith('delete')
                            let value = formatter ? formatter(e.target.value, !!isDeleting) : e.target.value
                            if (!UPPERCASE_EXEMPT_STAGE1A_FIELDS.has(key)) value = value.toUpperCase()
                            setForm({ ...form, [key]: value })
                          }}
                        />
                        {fieldOptions.length > 0 && (
                          <datalist id={`${key}-options`}>
                            {fieldOptions.map((option) => (
                              <option key={option} value={option} />
                            ))}
                          </datalist>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>
            ))}

            <h3 className="dynamic-fields-heading">Accessories</h3>
            <div className="detail-form-grid">
              <div className="form-field">
                <label>Accessories Type</label>
                <select
                  value={accessories.type}
                  onChange={(e) => setAccessories({ ...accessories, type: e.target.value })}
                >
                  <option value="">Select…</option>
                  <option value="input">Input</option>
                  <option value="output">Output</option>
                </select>
              </div>
              <div className="form-field">
                <label>Input Type</label>
                <input
                  value={accessories.input_type}
                  list="accessories-input-type-options"
                  onChange={(e) => setAccessories({ ...accessories, input_type: e.target.value })}
                />
                <datalist id="accessories-input-type-options">
                  <option value="Microphone" />
                </datalist>
              </div>
              <div className="form-field">
                <label>Model</label>
                <input
                  value={accessories.model}
                  list="accessories-model-options"
                  onChange={(e) => setAccessories({ ...accessories, model: e.target.value })}
                />
                <datalist id="accessories-model-options">
                  <option value="Generic" />
                </datalist>
              </div>
              <div className="form-field">
                <label>Power Type</label>
                <input
                  value={accessories.power_type}
                  list="accessories-power-type-options"
                  onChange={(e) => setAccessories({ ...accessories, power_type: e.target.value })}
                />
                <datalist id="accessories-power-type-options">
                  <option value="5V" />
                  <option value="12V" />
                  <option value="24V" />
                  <option value="PoE" />
                </datalist>
              </div>
              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={accessories.automatic_gain_control}
                  onChange={(e) => setAccessories({ ...accessories, automatic_gain_control: e.target.checked })}
                />
                Automatic Gain Control
              </label>
            </div>

            <DynamicFields fields={fields} values={meta} onChange={setMeta} heading="Axis Settings" uppercaseText />

            {identityError && <div className="login-error">{identityError}</div>}
            <button type="submit" disabled={submitting}>
              Save configuration
            </button>
          </form>
        )}

        <div className="signoff-section">
          <div className="signoff-header">
            <p className="signoff-caption">
              {unit.board_column === 'pre_deployment'
                ? "Sign-off: Encoder confirms the fields above were entered. Once encoded, this unit advances to Configuration, where it's configured and QC'd."
                : "Sign-off: Configurator confirms device configuration is applied, QC confirms it's been checked."}
            </p>
            <SignoffProgress
              done={[!!stage?.encoded_by, unit.board_column !== 'pre_deployment' && !!stage?.configured_by, unit.board_column !== 'pre_deployment' && !!stage?.qc_by].filter(Boolean).length}
              total={unit.board_column === 'pre_deployment' ? 1 : 3}
            />
          </div>
          <div className="signoff-steps">
            <TimedSignOffButton
              label="Encoded"
              done={!!stage?.encoded_by}
              at={stage?.encoded_at}
              by={stage?.encoded_by_name}
              allowed={canEncode}
              onConfirm={(performedAt) => handleSignOff('encoded', performedAt)}
            />
            {unit.board_column !== 'pre_deployment' && (
              <>
                <TimedSignOffButton
                  label="Configured"
                  done={!!stage?.configured_by}
                  at={stage?.configured_at}
                  by={stage?.configured_by_name}
                  allowed={canConfigure}
                  onConfirm={(performedAt) => handleSignOff('configured', performedAt)}
                />
                <QCSignOffButton
                  done={!!stage?.qc_by}
                  at={stage?.qc_at}
                  by={stage?.qc_by_name}
                  allowed={canQC}
                  onPass={(performedAt) => handleSignOff('qc', performedAt)}
                  onFail={() => handleSignOff('qc_fail')}
                />
              </>
            )}
          </div>

          {stage?.signed_off_at && (
            <div className="signed-off-banner">
              <CheckIcon />
              <span>
                Signed off {new Date(stage.signed_off_at).toLocaleString()} —{' '}
                <button type="button" className="link-button" onClick={() => openBarcodeSticker(unitId)}>
                  Print barcode sticker
                </button>
              </span>
            </div>
          )}
        </div>
      </section>
    </>
  )
}

function Stage1BSection({
  unitId,
  stage,
  canEdit,
  canConfigure,
  canQC,
  canDeclareDefect,
  hasDefect,
  onDone,
  onDefectDeclared,
}: {
  unitId: string
  stage: Stage1B | null
  canEdit: boolean
  canConfigure: boolean
  canQC: boolean
  canDeclareDefect: boolean
  hasDefect: boolean
  onDone: () => void
  onDefectDeclared: () => void
}) {
  const [firmwareUpdated, setFirmwareUpdated] = useState(stage?.firmware_updated ?? false)
  const [firmwareVersion, setFirmwareVersion] = useState(stage?.firmware_version ?? '')
  const [fields, setFields] = useState<FieldDefinition[]>([])
  const [meta, setMeta] = useState<MetaValues>({})
  const [submitting, setSubmitting] = useState(false)
  const [showUpdateFailedModal, setShowUpdateFailedModal] = useState(false)

  useEffect(() => {
    listMetaDataFields('stage1b').then(setFields).catch(() => setFields([]))
  }, [])

  useEffect(() => {
    setFirmwareUpdated(stage?.firmware_updated ?? false)
    setFirmwareVersion(stage?.firmware_version ?? '')
    setMeta(defaultMetaValues(fields, stage?.meta_data))
  }, [stage, fields])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await upsertStage1B(unitId, { firmware_updated: firmwareUpdated, firmware_version: firmwareVersion, meta_data: meta })
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleSignOff(step: 'configured' | 'qc' | 'qc_fail', performedAt?: string) {
    await signOffStage1B(unitId, step, performedAt)
    onDone()
  }

  return (
    <section className="detail-section">
      <div className="section-toolbar">
        <h2>Firmware Configuration</h2>
        {canDeclareDefect && !hasDefect && (
          <button type="button" className="btn btn-danger-outline" onClick={() => setShowUpdateFailedModal(true)}>
            Report Update Issue
          </button>
        )}
      </div>
      {canEdit && (
        <form onSubmit={handleSave} className="detail-form">
          <label className="checkbox-row">
            <input type="checkbox" checked={firmwareUpdated} onChange={(e) => setFirmwareUpdated(e.target.checked)} />
            Firmware updated
          </label>
          <label>Firmware version</label>
          <input value={firmwareVersion} onChange={(e) => setFirmwareVersion(e.target.value)} />
          <DynamicFields fields={fields} values={meta} onChange={setMeta} />
          <button type="submit" disabled={submitting}>
            Save firmware configuration
          </button>
        </form>
      )}

      {showUpdateFailedModal && (
        <DeclareDefectModal
          unitId={unitId}
          initialDefectType="defective"
          initialDescription="Firmware update did not complete successfully."
          onClose={() => setShowUpdateFailedModal(false)}
          onDeclared={() => {
            setShowUpdateFailedModal(false)
            onDefectDeclared()
          }}
        />
      )}

      <div className="signoff-section">
        <div className="signoff-header">
          <p className="signoff-caption">
            Sign-off: Configurator confirms firmware was applied, QC confirms it's been checked. Once both are done,
            this unit advances to Shipment.
          </p>
          <SignoffProgress done={[!!stage?.configured_by, !!stage?.configuration_qc_by].filter(Boolean).length} total={2} />
        </div>
        <div className="signoff-steps">
          <TimedSignOffButton
            label="Configured"
            done={!!stage?.configured_by}
            at={stage?.configured_date}
            by={stage?.configured_by_name}
            allowed={canConfigure}
            onConfirm={(performedAt) => handleSignOff('configured', performedAt)}
          />
          <QCSignOffButton
            done={!!stage?.configuration_qc_by}
            at={stage?.qc_date}
            by={stage?.configuration_qc_by_name}
            allowed={canQC}
            onPass={(performedAt) => handleSignOff('qc', performedAt)}
            onFail={() => handleSignOff('qc_fail')}
          />
        </div>

        {stage?.signed_off_at && (
          <div className="signed-off-banner">
            <CheckIcon />
            <span>Signed off {new Date(stage.signed_off_at).toLocaleString()}</span>
          </div>
        )}
      </div>
    </section>
  )
}

function LogisticsSection({ logistics }: { logistics: UnitLogisticsView | null }) {
  const navigate = useNavigate()
  const docket = logistics?.docket

  return (
    <section className="detail-section">
      <h2>Logistics</h2>
      {!docket && <p className="board-empty">This unit hasn't been added to a delivery docket yet.</p>}

      {docket && (
        <>
          <div className="detail-form-grid">
            <div className="form-field">
              <label>Docket Number</label>
              <p>
                <button type="button" className="link-button" onClick={() => navigate(`/logistics/${docket.id}`)}>
                  {docket.docket_number}
                </button>
              </p>
            </div>
            <div className="form-field">
              <label>Waybill Number</label>
              <p>{docket.waybill_number ?? '—'}</p>
            </div>
            <div className="form-field">
              <label>Shipped Via</label>
              <p>{docket.shipped_via ?? '—'}</p>
            </div>
            <div className="form-field">
              <label>Shipping Provider</label>
              <p>{docket.shipping_provider ?? '—'}</p>
            </div>
            <div className="form-field">
              <label>Destination Site</label>
              <p>{docket.destination_site ?? '—'}</p>
            </div>
          </div>

          <div className="detail-form-grid">
            <div>
              <strong>Dispatched by:</strong> {docket.dispatched_by ?? '—'}{' '}
              {docket.dispatched_at && `(${new Date(docket.dispatched_at).toLocaleString()})`}
            </div>
            <div>
              <strong>Received by:</strong> {docket.received_by ?? '—'}{' '}
              {docket.received_at && `(${new Date(docket.received_at).toLocaleString()})`}
            </div>
          </div>

          <h3>Tracking History</h3>
          <DataTable
            rows={logistics?.tracking_events ?? []}
            getRowKey={(e) => e.id}
            emptyMessage="No tracking events yet."
            columns={[
              { key: 'occurred_at', header: 'When', render: (e) => new Date(e.occurred_at).toLocaleString() },
              { key: 'status', header: 'Status', render: (e) => e.status ?? '—' },
              { key: 'description', header: 'Description', render: (e) => e.description },
              { key: 'source', header: 'Source', render: (e) => (e.is_system_generated ? 'System' : 'Manual') },
            ]}
          />
        </>
      )}
    </section>
  )
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M3 8.5L6.2 11.5L13 4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

// Small "N of M complete" indicator shown in a signoff-section's header so
// the state of a multi-step sign-off is visible without reading every step.
function SignoffProgress({ done, total }: { done: number; total: number }) {
  const complete = done >= total
  return (
    <div className="signoff-progress">
      <span className={`signoff-progress-label${complete ? ' signoff-progress-label-complete' : ''}`}>
        {done}/{total} complete
      </span>
      <div className="signoff-progress-bar">
        <div
          className={`signoff-progress-fill${complete ? ' signoff-progress-fill-complete' : ''}`}
          style={{ width: `${total > 0 ? (done / total) * 100 : 0}%` }}
        />
      </div>
    </div>
  )
}

// Used only where the backend accepts a custom sign-off timestamp (Stage
// 1-A/1-B) — lets the user pick "now" or backdate/postdate the sign-off.
function TimedSignOffButton({
  label,
  done,
  at,
  by,
  allowed,
  onConfirm,
}: {
  label: string
  done: boolean
  at?: string
  by?: string
  allowed: boolean
  onConfirm: (performedAt?: string) => void | Promise<void>
}) {
  const [showModal, setShowModal] = useState(false)
  const locked = !done && !allowed

  return (
    <div className={`signoff-step${done ? ' signoff-step-done' : ''}${locked ? ' signoff-step-locked' : ''}`}>
      <span className="signoff-step-icon">{done && <CheckIcon />}</span>
      <div className="signoff-step-body">
        <button
          type="button"
          className="signoff-step-label"
          disabled={done || !allowed}
          onClick={() => setShowModal(true)}
          title={!allowed ? 'You do not have the role required for this step' : undefined}
        >
          {label}
        </button>
        <span className="signoff-step-meta">
          {done && at ? (
            <>
              {by && <>{by} — </>}
              {new Date(at).toLocaleString()}
            </>
          ) : locked ? (
            'Locked'
          ) : (
            'Pending'
          )}
        </span>
      </div>

      {showModal && (
        <SignOffTimeModal
          label={label}
          onCancel={() => setShowModal(false)}
          onConfirm={async (performedAt) => {
            await onConfirm(performedAt)
            setShowModal(false)
          }}
        />
      )}
    </div>
  )
}

function QCSignOffButton({
  done,
  at,
  by,
  allowed,
  onPass,
  onFail,
}: {
  done: boolean
  at?: string
  by?: string
  allowed: boolean
  onPass: (performedAt?: string) => void | Promise<void>
  onFail: () => void | Promise<void>
}) {
  const [showChoice, setShowChoice] = useState(false)
  const [showTimeModal, setShowTimeModal] = useState(false)
  const [failing, setFailing] = useState(false)
  const locked = !done && !allowed

  return (
    <div className={`signoff-step${done ? ' signoff-step-done' : ''}${locked ? ' signoff-step-locked' : ''}`}>
      <span className="signoff-step-icon">{done && <CheckIcon />}</span>
      <div className="signoff-step-body">
        <button
          type="button"
          className="signoff-step-label"
          disabled={done || !allowed}
          onClick={() => setShowChoice(true)}
          title={!allowed ? 'You do not have the role required for this step' : undefined}
        >
          QC
        </button>
        <span className="signoff-step-meta">
          {done && at ? (
            <>
              {by && <>{by} — </>}
              {new Date(at).toLocaleString()}
            </>
          ) : locked ? (
            'Locked'
          ) : (
            'Pending'
          )}
        </span>
      </div>

      {showChoice && (
        <div className="modal-overlay">
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <button type="button" className="modal-close" onClick={() => setShowChoice(false)} aria-label="Close">
              &times;
            </button>
            <h2>QC Result</h2>
            <p className="modal-subtitle">Did this unit pass QC?</p>
            <div className="modal-actions">
              <button
                type="button"
                className="btn"
                disabled={failing}
                onClick={async () => {
                  setFailing(true)
                  try {
                    await onFail()
                    setShowChoice(false)
                  } finally {
                    setFailing(false)
                  }
                }}
              >
                {failing ? 'Reopening…' : 'Failed — reopen Configured'}
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => {
                  setShowChoice(false)
                  setShowTimeModal(true)
                }}
              >
                Passed
              </button>
            </div>
          </div>
        </div>
      )}

      {showTimeModal && (
        <SignOffTimeModal
          label="QC"
          onCancel={() => setShowTimeModal(false)}
          onConfirm={async (performedAt) => {
            await onPass(performedAt)
            setShowTimeModal(false)
          }}
        />
      )}
    </div>
  )
}

function toDatetimeLocalValue(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function SignOffTimeModal({
  label,
  onCancel,
  onConfirm,
}: {
  label: string
  onCancel: () => void
  onConfirm: (performedAt?: string) => void | Promise<void>
}) {
  const [mode, setMode] = useState<'now' | 'manual'>('now')
  const [manualValue, setManualValue] = useState(() => toDatetimeLocalValue(new Date()))
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      if (mode === 'manual') {
        await onConfirm(new Date(manualValue).toISOString())
      } else {
        await onConfirm(undefined)
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="modal-overlay">
      <form className="modal-card" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <button type="button" className="modal-close" onClick={onCancel} aria-label="Close">
          &times;
        </button>
        <h2>Sign off &ldquo;{label}&rdquo;</h2>
        <label className="checkbox-row">
          <input type="radio" checked={mode === 'now'} onChange={() => setMode('now')} />
          Use current date &amp; time
        </label>
        <label className="checkbox-row">
          <input type="radio" checked={mode === 'manual'} onChange={() => setMode('manual')} />
          Enter date &amp; time manually
        </label>
        {mode === 'manual' && (
          <input
            type="datetime-local"
            value={manualValue}
            onChange={(e) => setManualValue(e.target.value)}
            required
          />
        )}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onCancel} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>
            {submitting ? 'Signing off…' : 'Confirm'}
          </button>
        </div>
      </form>
    </div>
  )
}

function InstallationSection({
  unitId,
  installation,
  allocatedBranch,
  canEdit,
  onDone,
}: {
  unitId: string
  installation: SiteInstallation | null
  allocatedBranch?: string
  canEdit: boolean
  onDone: () => void
}) {
  const [confirmedCorrect, setConfirmedCorrect] = useState(true)
  const [discrepancyNotes, setDiscrepancyNotes] = useState('')
  const [location, setLocation] = useState('')
  const [heightM, setHeightM] = useState('')
  const [networkAttached, setNetworkAttached] = useState(false)
  const [deviceContactable, setDeviceContactable] = useState(false)
  const [siteForm, setSiteForm] = useState<Record<string, string>>({})
  const [siteLocations, setSiteLocations] = useState<SiteLocation[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [fields, setFields] = useState<FieldDefinition[]>([])
  const [meta, setMeta] = useState<MetaValues>({})
  const [photos, setPhotos] = useState<InstallationPhoto[]>([])
  const [photoUploading, setPhotoUploading] = useState(false)
  const [photoError, setPhotoError] = useState<string | null>(null)

  useEffect(() => {
    listMetaDataFields('installation').then(setFields).catch(() => setFields([]))
    listSiteLocations()
      .then(setSiteLocations)
      .catch(() => setSiteLocations([]))
  }, [])

  const loadPhotos = useCallback(() => {
    listInstallationPhotos(unitId)
      .then(setPhotos)
      .catch(() => setPhotos([]))
  }, [unitId])

  useEffect(() => {
    loadPhotos()
  }, [loadPhotos])

  async function handlePhotoUpload(file: File | undefined | null) {
    if (!file) return
    setPhotoError(null)
    setPhotoUploading(true)
    try {
      await uploadInstallationPhoto(unitId, file)
      loadPhotos()
    } catch (err) {
      setPhotoError(err instanceof Error ? err.message.replace(/^\d+\s+/, '') : 'Failed to upload photo')
    } finally {
      setPhotoUploading(false)
    }
  }

  async function handlePhotoDelete(photoId: string) {
    setPhotoError(null)
    try {
      await deleteInstallationPhoto(unitId, photoId)
      loadPhotos()
    } catch (err) {
      setPhotoError(err instanceof Error ? err.message.replace(/^\d+\s+/, '') : 'Failed to delete photo')
    }
  }

  function handleSiteNameChange(value: string) {
    const match = siteLocations.find((l) => l.name.toLowerCase() === value.trim().toLowerCase())
    setSiteForm((prev) => ({
      ...prev,
      site_name: value,
      ...(match
        ? { site_location: match.region ?? '', site_gateway: match.ip_gateway ?? '', site_subnet: match.subnet_mask ?? '' }
        : {}),
    }))
  }

  useEffect(() => {
    setLocation(installation?.installed_location ?? '')
    setHeightM(installation?.installed_height_m != null ? String(installation.installed_height_m) : '')
    setNetworkAttached(installation?.network_attached ?? false)
    setDeviceContactable(installation?.device_contactable ?? false)
    setMeta(defaultMetaValues(fields, installation?.meta_data))
    const siteName = installation?.site_name || allocatedBranch || ''
    const match = siteLocations.find((l) => l.name.toLowerCase() === siteName.trim().toLowerCase())
    setSiteForm({
      site_name: siteName,
      site_location: installation?.site_location || match?.region || '',
      site_ip: installation?.site_ip ?? '',
      site_subnet: installation?.site_subnet || match?.subnet_mask || '',
      site_gateway: installation?.site_gateway || match?.ip_gateway || '',
      deployment_team: installation?.deployment_team ?? '',
      team_leader: installation?.team_leader ?? '',
    })
  }, [installation, fields, allocatedBranch, siteLocations])

  async function handleSiteReceipt(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await recordSiteReceipt(unitId, {
        confirmed_correct: confirmedCorrect,
        discrepancy_notes: confirmedCorrect ? undefined : discrepancyNotes || undefined,
      })
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  function installationPayload() {
    return {
      installed_location: location,
      installed_height_m: heightM ? parseFloat(heightM) : undefined,
      network_attached: networkAttached,
      device_contactable: deviceContactable,
      ...siteForm,
      meta_data: meta,
    }
  }

  async function handleSaveInstallation(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await upsertInstallation(unitId, installationPayload())
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleSignOff(step: 'installed' | 'inspected' | 'fit_focus', performedAt?: string) {
    await signOffInstallation(unitId, step, performedAt)
    onDone()
  }

  return (
    <section className="detail-section">
      <h2>Site Installation</h2>

      {canEdit && !installation?.received_on_site_at && (
        <form onSubmit={handleSiteReceipt} className="detail-form">
          <label className="checkbox-row">
            <input type="checkbox" checked={confirmedCorrect} onChange={(e) => setConfirmedCorrect(e.target.checked)} />
            Confirmed as correct on receipt
          </label>
          {!confirmedCorrect && (
            <>
              <label>Discrepancy notes</label>
              <textarea value={discrepancyNotes} onChange={(e) => setDiscrepancyNotes(e.target.value)} />
            </>
          )}
          <button type="submit" disabled={submitting}>
            Record site receipt
          </button>
        </form>
      )}

      {installation?.received_on_site_at && installation.confirmed_correct === false && (
        <p className="detail-status" style={{ color: 'var(--danger)' }}>
          Discrepancy reported — escalated to PM/PC{' '}
          {installation.escalated_to_pmpc_at && new Date(installation.escalated_to_pmpc_at).toLocaleString()}
        </p>
      )}

      {canEdit && installation?.received_on_site_at && (
        <form onSubmit={handleSaveInstallation} className="detail-form">
          <div className="detail-form-grid">
            <div className="form-field">
              <label htmlFor="inst_site_name">Site Name</label>
              <Combobox
                id="inst_site_name"
                value={siteForm.site_name ?? ''}
                onChange={handleSiteNameChange}
                options={siteLocations.map((l) => l.name)}
              />
            </div>
            {(
              [
                ['site_location', 'Site Location'],
                ['site_ip', 'Site IP'],
                ['site_subnet', 'Site Subnet'],
                ['site_gateway', 'Site Gateway'],
                ['deployment_team', 'Deployment Team'],
                ['team_leader', 'Team Leader'],
              ] as const
            ).map(([key, label]) => {
              // Site IP is the one field an installer actually needs to pick
              // a fresh address for — suggest the site's own subnet range.
              const fieldOptions =
                key === 'site_ip' ? subnetAddresses(siteForm.site_gateway ?? '', siteForm.site_subnet ?? '') : []
              return (
                <div className="form-field" key={key}>
                  <label>{label}</label>
                  <input
                    value={siteForm[key] ?? ''}
                    list={fieldOptions.length > 0 ? `${key}-options` : undefined}
                    onChange={(e) => {
                      const formatter = INSTALLATION_IP_FIELDS.has(key) ? formatIPInput : null
                      const isDeleting =
                        e.nativeEvent instanceof InputEvent && e.nativeEvent.inputType?.startsWith('delete')
                      const value = formatter ? formatter(e.target.value, !!isDeleting) : e.target.value
                      setSiteForm({ ...siteForm, [key]: value })
                    }}
                  />
                  {fieldOptions.length > 0 && (
                    <datalist id={`${key}-options`}>
                      {fieldOptions.map((option) => (
                        <option key={option} value={option} />
                      ))}
                    </datalist>
                  )}
                </div>
              )
            })}
            <div className="form-field">
              <label htmlFor="inst_location">Installed Location</label>
              <input id="inst_location" value={location} onChange={(e) => setLocation(e.target.value)} />
            </div>
            <div className="form-field">
              <label htmlFor="inst_height">Installed Height (m)</label>
              <input
                id="inst_height"
                type="number"
                value={heightM}
                onChange={(e) => setHeightM(e.target.value)}
              />
            </div>
          </div>
          <div className="checkbox-row-group">
            <label className="checkbox-row">
              <input type="checkbox" checked={networkAttached} onChange={(e) => setNetworkAttached(e.target.checked)} />
              Network attached
            </label>
            <label className="checkbox-row">
              <input
                type="checkbox"
                checked={deviceContactable}
                onChange={(e) => setDeviceContactable(e.target.checked)}
              />
              Contactable
            </label>
          </div>
          <DynamicFields fields={fields} values={meta} onChange={setMeta} />
          <button type="submit" disabled={submitting}>
            Save installation details
          </button>
        </form>
      )}

      {installation?.received_on_site_at && (
        <div className="signoff-section">
          <div className="signoff-header">
            <p className="signoff-caption">
              Sign-off: confirms the unit was installed, inspected, and focused/aligned on site. Once all three are
              done, this unit advances to Commissioning.
            </p>
            <SignoffProgress
              done={[!!installation?.installed_by, !!installation?.inspected_by, !!installation?.fit_focus_by].filter(Boolean).length}
              total={3}
            />
          </div>
          <div className="signoff-steps">
            <TimedSignOffButton
              label="Installed"
              done={!!installation?.installed_by}
              at={installation?.installed_at}
              by={installation?.installed_by_name}
              allowed={canEdit}
              onConfirm={(performedAt) => handleSignOff('installed', performedAt)}
            />
            <TimedSignOffButton
              label="Inspected"
              done={!!installation?.inspected_by}
              at={installation?.inspected_at}
              by={installation?.inspected_by_name}
              allowed={canEdit && photos.length > 0}
              onConfirm={(performedAt) => handleSignOff('inspected', performedAt)}
            />
            <TimedSignOffButton
              label="Fit & Focus"
              done={!!installation?.fit_focus_by}
              at={installation?.fit_focus_completed_at}
              by={installation?.fit_focus_by_name}
              allowed={canEdit && photos.length > 0}
              onConfirm={(performedAt) => handleSignOff('fit_focus', performedAt)}
            />
          </div>

          {installation?.installed_by && (
            <InstallationPhotos
              unitId={unitId}
              photos={photos}
              canEdit={canEdit}
              uploading={photoUploading}
              error={photoError}
              onUpload={handlePhotoUpload}
              onDelete={handlePhotoDelete}
            />
          )}

          {installation?.signed_off_at && (
            <div className="signed-off-banner">
              <CheckIcon />
              <span>Signed off {new Date(installation.signed_off_at).toLocaleString()}</span>
            </div>
          )}
        </div>
      )}
    </section>
  )
}

function InstallationPhotoThumb({ unitId, photo, canDelete, onDelete, onPreview }: {
  unitId: string
  photo: InstallationPhoto
  canDelete: boolean
  onDelete: () => void
  onPreview: (url: string) => void
}) {
  const [url, setUrl] = useState<string | null>(null)

  useEffect(() => {
    let objectUrl: string | null = null
    authFetch(installationPhotoUrl(unitId, photo.id))
      .then((res) => (res.ok ? res.blob() : null))
      .then((blob) => {
        if (!blob) return
        objectUrl = URL.createObjectURL(blob)
        setUrl(objectUrl)
      })
      .catch(() => undefined)
    return () => {
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [unitId, photo.id])

  return (
    <div className="installation-photo-thumb">
      {url ? (
        <button
          type="button"
          className="installation-photo-thumb-btn"
          onClick={() => onPreview(url)}
          title="View photo"
        >
          <img src={url} alt="Installation" />
        </button>
      ) : (
        <span className="installation-photo-loading">Loading…</span>
      )}
      {canDelete && (
        <button type="button" className="installation-photo-remove" onClick={onDelete} title="Remove photo">
          ✕
        </button>
      )}
    </div>
  )
}

function InstallationPhotoPreview({ url, onClose }: { url: string; onClose: () => void }) {
  return (
    <div className="modal-overlay">
      <div className="modal-card installation-photo-preview-card" onClick={(e) => e.stopPropagation()}>
        <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
          &times;
        </button>
        <img src={url} alt="Installation photo" className="installation-photo-preview-img" />
      </div>
    </div>
  )
}

function InstallationPhotos({
  unitId,
  photos,
  canEdit,
  uploading,
  error,
  onUpload,
  onDelete,
}: {
  unitId: string
  photos: InstallationPhoto[]
  canEdit: boolean
  uploading: boolean
  error: string | null
  onUpload: (file: File | undefined | null) => void
  onDelete: (photoId: string) => void
}) {
  const mobile = useMemo(() => isMobileDevice(), [])
  const canAddMore = canEdit && photos.length < MAX_INSTALLATION_PHOTOS
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)

  return (
    <div className="installation-photos">
      <p className="signoff-caption">
        Installation photos ({photos.length}/{MAX_INSTALLATION_PHOTOS}) — at least 1 required before Inspected or Fit
        &amp; Focus can be signed off.
      </p>

      {photos.length > 0 && (
        <div className="installation-photo-grid">
          {photos.map((photo) => (
            <InstallationPhotoThumb
              key={photo.id}
              unitId={unitId}
              photo={photo}
              canDelete={canEdit}
              onDelete={() => onDelete(photo.id)}
              onPreview={setPreviewUrl}
            />
          ))}
        </div>
      )}

      {previewUrl && <InstallationPhotoPreview url={previewUrl} onClose={() => setPreviewUrl(null)} />}

      {error && (
        <p className="detail-status" style={{ color: 'var(--danger)' }}>
          {error}
        </p>
      )}

      {canAddMore && (
        <div className="installation-photo-actions">
          {mobile && (
            <label className="signoff-btn">
              Take Photo
              <input
                type="file"
                accept="image/*"
                capture="environment"
                disabled={uploading}
                style={{ display: 'none' }}
                onChange={(e) => {
                  onUpload(e.target.files?.[0])
                  e.target.value = ''
                }}
              />
            </label>
          )}
          <label className="signoff-btn">
            {mobile ? 'Upload from Gallery' : 'Upload Photo'}
            <input
              type="file"
              accept="image/*"
              disabled={uploading}
              style={{ display: 'none' }}
              onChange={(e) => {
                onUpload(e.target.files?.[0])
                e.target.value = ''
              }}
            />
          </label>
        </div>
      )}
    </div>
  )
}

// Signature fields hold either a drawn signature (a data:image/png;... URL
// from SignaturePad) or, when the pad was left blank, the typed name as a
// plain-text fallback — render whichever was actually stored.
function SignatureImage({ data }: { data?: string }) {
  if (!data) return null
  if (data.startsWith('data:image')) {
    return <img src={data} alt="Signature" className="signature-preview" />
  }
  return <span className="signature-preview-text">{data}</span>
}

function AcceptanceSection({
  unitId,
  acceptance,
  canEdit,
  onDone,
}: {
  unitId: string
  acceptance: ClientAcceptance | null
  canEdit: boolean
  onDone: () => void
}) {
  const [signatureData, setSignatureData] = useState<string | null>(null)
  const [bspName, setBspName] = useState('')
  const [comments, setComments] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [linkResult, setLinkResult] = useState<string | null>(null)
  const [linkSubmitting, setLinkSubmitting] = useState(false)
  const [clientEmail, setClientEmail] = useState('')
  const [emailSubmitting, setEmailSubmitting] = useState(false)
  const [emailResult, setEmailResult] = useState<string | null>(null)
  const [headOfficeLinkResult, setHeadOfficeLinkResult] = useState<string | null>(null)
  const [headOfficeLinkSubmitting, setHeadOfficeLinkSubmitting] = useState(false)
  const [headOfficeEmail, setHeadOfficeEmail] = useState('')
  const [headOfficeEmailSubmitting, setHeadOfficeEmailSubmitting] = useState(false)
  const [headOfficeEmailResult, setHeadOfficeEmailResult] = useState<string | null>(null)
  const [clientSignOffError, setClientSignOffError] = useState<string | null>(null)
  const [uploadClientName, setUploadClientName] = useState('')
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [fields, setFields] = useState<FieldDefinition[]>([])
  const [meta, setMeta] = useState<MetaValues>({})

  useEffect(() => {
    listMetaDataFields('acceptance').then(setFields).catch(() => setFields([]))
  }, [])

  useEffect(() => {
    setMeta(defaultMetaValues(fields, acceptance?.meta_data))
  }, [acceptance, fields])

  async function handleBSPSignOff(e: FormEvent) {
    e.preventDefault()
    if (!signatureData) return
    setSubmitting(true)
    try {
      await recordBSPAcceptance(unitId, {
        signed_by_name: bspName || undefined,
        signature: signatureData,
        comments: comments || undefined,
        meta_data: meta,
      })
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  function describeSignOffError(err: unknown, alreadySignedMessage: string): string {
    if (err instanceof Error) {
      if (err.message.startsWith('409')) return alreadySignedMessage
      const match = err.message.match(/^\d+\s+(.*)$/)
      if (match?.[1]) return match[1]
    }
    return 'Something went wrong — please try again.'
  }

  async function handleGenerateLink() {
    if (linkSubmitting || submitting) return
    setLinkSubmitting(true)
    setClientSignOffError(null)
    try {
      const { token } = await generateClientSigningLink(unitId)
      setLinkResult(`${window.location.origin}/sign/${token}`)
      onDone()
    } catch (err) {
      setClientSignOffError(describeSignOffError(err, 'This unit has already been signed by the client — refresh the page.'))
    } finally {
      setLinkSubmitting(false)
    }
  }

  async function handleEmailLink(e: FormEvent) {
    e.preventDefault()
    if (!clientEmail || linkSubmitting || emailSubmitting) return
    setEmailSubmitting(true)
    setClientSignOffError(null)
    try {
      const { sent } = await emailClientSigningLink(unitId, clientEmail)
      setEmailResult(sent ? `Sign-off link emailed to ${clientEmail}.` : 'SMTP not configured — recorded without sending.')
      onDone()
    } catch (err) {
      setClientSignOffError(describeSignOffError(err, 'This unit has already been signed by the client — refresh the page.'))
    } finally {
      setEmailSubmitting(false)
    }
  }

  async function handleGenerateHeadOfficeLink() {
    if (headOfficeLinkSubmitting || submitting) return
    setHeadOfficeLinkSubmitting(true)
    setClientSignOffError(null)
    try {
      const { token } = await generateHeadOfficeSigningLink(unitId)
      setHeadOfficeLinkResult(`${window.location.origin}/sign/${token}`)
      onDone()
    } catch (err) {
      setClientSignOffError(describeSignOffError(err, 'This unit has already been signed by head office — refresh the page.'))
    } finally {
      setHeadOfficeLinkSubmitting(false)
    }
  }

  async function handleEmailHeadOfficeLink(e: FormEvent) {
    e.preventDefault()
    if (!headOfficeEmail || headOfficeLinkSubmitting || headOfficeEmailSubmitting) return
    setHeadOfficeEmailSubmitting(true)
    setClientSignOffError(null)
    try {
      const { sent } = await emailHeadOfficeSigningLink(unitId, headOfficeEmail)
      setHeadOfficeEmailResult(
        sent ? `Sign-off link emailed to ${headOfficeEmail}.` : 'SMTP not configured — recorded without sending.',
      )
      onDone()
    } catch (err) {
      setClientSignOffError(describeSignOffError(err, 'This unit has already been signed by head office — refresh the page.'))
    } finally {
      setHeadOfficeEmailSubmitting(false)
    }
  }

  async function handleManualUpload(e: FormEvent) {
    e.preventDefault()
    if (!uploadFile || !uploadClientName) return
    setSubmitting(true)
    setClientSignOffError(null)
    try {
      await uploadManualDocument(unitId, uploadFile, uploadClientName)
      onDone()
    } catch {
      setClientSignOffError('This unit has already been signed by the client — refresh the page.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="detail-section">
      <h2>Client Acceptance</h2>

      {clientSignOffError && (
        <p className="detail-status" style={{ color: 'var(--danger)' }}>
          {clientSignOffError}
        </p>
      )}

      <div className="acceptance-columns">
        <div className="acceptance-column">
          <h3>BSP Acceptance</h3>
          {canEdit && !acceptance?.bsp_acceptance_date && (
            <form onSubmit={handleBSPSignOff} className="detail-form">
              <label>Signed by</label>
              <input value={bspName} onChange={(e) => setBspName(e.target.value)} placeholder="Your name" />
              <label>Signature</label>
              <SignaturePad onChange={(data) => setSignatureData(data ?? bspName)} />
              <label>Comments</label>
              <textarea value={comments} onChange={(e) => setComments(e.target.value)} rows={3} />
              <DynamicFields fields={fields} values={meta} onChange={setMeta} />
              <button type="submit" disabled={submitting || !signatureData}>
                Record BSP acceptance
              </button>
            </form>
          )}
          {acceptance?.bsp_acceptance_date && (
            <>
              <p className="signed-off-note">
                Accepted{acceptance.bsp_signed_by_name ? ` by ${acceptance.bsp_signed_by_name}` : ''} —{' '}
                {new Date(acceptance.bsp_acceptance_date).toLocaleString()}
              </p>
              <SignatureImage data={acceptance.bsp_signature} />
            </>
          )}
        </div>

        <div className="acceptance-column">
          <h3>Branch Manager Sign-Off</h3>
          {canEdit && acceptance?.bsp_acceptance_date && !acceptance?.client_signed_at && (
            <>
              <p className="detail-status">Signing here closes this acceptance stage.</p>
              <div className="signoff-row">
                <button className="signoff-btn" onClick={handleGenerateLink} disabled={linkSubmitting || submitting}>
                  {linkSubmitting ? 'Generating…' : 'Generate e-signature link'}
                </button>
              </div>
              {linkResult && (
                <p className="detail-status">
                  Share this link with the branch manager: <code>{linkResult}</code>
                </p>
              )}

              <form onSubmit={handleEmailLink} className="detail-form">
                <label>Or email the sign-off link to the branch manager</label>
                <input
                  value={clientEmail}
                  onChange={(e) => setClientEmail(e.target.value)}
                  type="email"
                  placeholder="branchmanager@example.com"
                  disabled={emailSubmitting || submitting}
                />
                <button type="submit" disabled={emailSubmitting || submitting || !clientEmail}>
                  {emailSubmitting ? 'Sending…' : 'Email sign-off link'}
                </button>
              </form>
              {emailResult && <p className="detail-status">{emailResult}</p>}
              {acceptance?.client_link_emailed_at && (
                <p className="signed-off-note">
                  Emailed to {acceptance.client_email} on {new Date(acceptance.client_link_emailed_at).toLocaleString()}
                </p>
              )}

              <form onSubmit={handleManualUpload} className="detail-form">
                <label>Or upload a manually signed document</label>
                <input
                  type="file"
                  onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)}
                  accept="application/pdf,image/*"
                  disabled={linkSubmitting || submitting}
                />
                <label>Branch Manager</label>
                <input
                  value={uploadClientName}
                  onChange={(e) => setUploadClientName(e.target.value)}
                  disabled={linkSubmitting || submitting}
                />
                <button type="submit" disabled={submitting || linkSubmitting || !uploadFile || !uploadClientName}>
                  Upload signed document
                </button>
              </form>
            </>
          )}
          {acceptance?.client_signed_at && (
            <>
              <p className="signed-off-note">
                Branch Manager ({acceptance.client_name}) signed {new Date(acceptance.client_signed_at).toLocaleString()}{' '}
                via {acceptance.method === 'manual_upload' ? 'manual upload' : 'e-signature'}
                {acceptance.uploaded_document_path && (
                  <>
                    {' — '}
                    <button type="button" className="link-button" onClick={() => openAcceptanceDocument(unitId)}>
                      view document
                    </button>
                  </>
                )}
              </p>
              <SignatureImage data={acceptance.client_signature_data} />
            </>
          )}
        </div>

        <div className="acceptance-column">
          <h3>Head Office / Security Manager Sign-Off</h3>
          {canEdit && acceptance?.bsp_acceptance_date && !acceptance?.head_office_signed_at && (
            <>
              <p className="detail-status">
                Optional additional sign-off — not required if the Branch Manager has already signed.
              </p>
              <div className="signoff-row">
                <button
                  className="signoff-btn"
                  onClick={handleGenerateHeadOfficeLink}
                  disabled={headOfficeLinkSubmitting || submitting}
                >
                  {headOfficeLinkSubmitting ? 'Generating…' : 'Generate e-signature link'}
                </button>
              </div>
              {headOfficeLinkResult && (
                <p className="detail-status">
                  Share this link with head office: <code>{headOfficeLinkResult}</code>
                </p>
              )}

              <form onSubmit={handleEmailHeadOfficeLink} className="detail-form">
                <label>Or email the sign-off link to head office / security manager</label>
                <input
                  value={headOfficeEmail}
                  onChange={(e) => setHeadOfficeEmail(e.target.value)}
                  type="email"
                  placeholder="headoffice@example.com"
                  disabled={headOfficeEmailSubmitting || submitting}
                />
                <button type="submit" disabled={headOfficeEmailSubmitting || submitting || !headOfficeEmail}>
                  {headOfficeEmailSubmitting ? 'Sending…' : 'Email sign-off link'}
                </button>
              </form>
              {headOfficeEmailResult && <p className="detail-status">{headOfficeEmailResult}</p>}
              {acceptance?.head_office_link_emailed_at && (
                <p className="signed-off-note">
                  Emailed to {acceptance.head_office_email} on{' '}
                  {new Date(acceptance.head_office_link_emailed_at).toLocaleString()}
                </p>
              )}
            </>
          )}
          {acceptance?.head_office_signed_at && (
            <>
              <p className="signed-off-note">
                Head Office / Security Manager ({acceptance.head_office_name}) signed{' '}
                {new Date(acceptance.head_office_signed_at).toLocaleString()}
              </p>
              <SignatureImage data={acceptance.head_office_signature_data} />
            </>
          )}
        </div>
      </div>

      {acceptance?.signed_off_at && (
        <p className="signed-off-note">
          <strong>Fully signed off {new Date(acceptance.signed_off_at).toLocaleString()}</strong>
        </p>
      )}
    </section>
  )
}

async function openDefectReport(unitId: string) {
  const res = await authFetch(reportUrl(unitId))
  if (!res.ok) return
  const blob = await res.blob()
  window.open(URL.createObjectURL(blob), '_blank')
}

function DeclareDefectModal({
  unitId,
  onClose,
  onDeclared,
  initialDefectType,
  initialDescription,
}: {
  unitId: string
  onClose: () => void
  onDeclared: () => void
  initialDefectType?: string
  initialDescription?: string
}) {
  const [defectType, setDefectType] = useState(initialDefectType ?? 'defective')
  const [description, setDescription] = useState(initialDescription ?? '')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await declareDefect(unitId, defectType, description || undefined)
      onDeclared()
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="modal-overlay">
      <form className="modal-card" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
          &times;
        </button>
        <h2>Declare Defect</h2>
        <label>Defect Type</label>
        <select value={defectType} onChange={(e) => setDefectType(e.target.value)}>
          <option value="defective">Defective</option>
          <option value="damaged">Damaged</option>
          <option value="wrong_item">Wrong Item</option>
        </select>
        <label>Description</label>
        <textarea value={description} onChange={(e) => setDescription(e.target.value)} />
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-danger" disabled={submitting}>
            {submitting ? 'Declaring…' : 'Declare'}
          </button>
        </div>
      </form>
    </div>
  )
}

function DefectSection({
  unitId,
  defect,
  canEdit,
  onDone,
}: {
  unitId: string
  defect: DefectReport | null
  canEdit: boolean
  onDone: () => void
}) {
  const [supplierEmail, setSupplierEmail] = useState('')
  const [replacementSerialNumber, setReplacementSerialNumber] = useState('')
  const [trackingNumber, setTrackingNumber] = useState('')
  const [carrier, setCarrier] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [emailResult, setEmailResult] = useState<string | null>(null)

  async function handleEmailSupplier(e: FormEvent) {
    e.preventDefault()
    if (!supplierEmail) return
    setSubmitting(true)
    try {
      const { sent } = await emailSupplier(unitId, supplierEmail)
      setEmailResult(sent ? 'Email sent to supplier.' : 'SMTP not configured — recorded without sending.')
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleShippedBack(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await markShippedBack(unitId, trackingNumber, carrier)
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelivered() {
    setSubmitting(true)
    try {
      await markDelivered(unitId)
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleSupplierReceived() {
    setSubmitting(true)
    try {
      await markSupplierReceived(unitId)
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  async function handleReplacement(e: FormEvent) {
    e.preventDefault()
    if (!replacementSerialNumber) return
    setSubmitting(true)
    try {
      await recordReplacement(unitId, replacementSerialNumber)
      onDone()
    } finally {
      setSubmitting(false)
    }
  }

  if (!defect) return null

  return (
    <section className="detail-section detail-section-defect">
      <h2>Defective / Damaged / Wrong Item</h2>
      <p>
        <strong>{defect.defect_type}</strong> — declared {new Date(defect.declared_date).toLocaleString()}
      </p>
      {defect.description && <p>{defect.description}</p>}

      <div className="signoff-row">
        <button type="button" className="link-button" onClick={() => openDefectReport(unitId)}>
          Print return tag
        </button>
      </div>

      {canEdit && defect.replacement_status === 'pending' && (
        <form onSubmit={handleEmailSupplier} className="detail-form">
          <label>Supplier Email</label>
          <input value={supplierEmail} onChange={(e) => setSupplierEmail(e.target.value)} type="email" />
          <button type="submit" disabled={submitting || !supplierEmail}>
            Email report to supplier
          </button>
        </form>
      )}
      {emailResult && <p className="detail-status">{emailResult}</p>}
      {defect.emailed_to_supplier_at && (
        <p className="signed-off-note">
          Emailed to {defect.supplier_email} on {new Date(defect.emailed_to_supplier_at).toLocaleString()}
        </p>
      )}

      {canEdit && defect.replacement_status === 'pending' && (
        <form onSubmit={handleShippedBack} className="detail-form">
          <label>Tracking Number (optional)</label>
          <input value={trackingNumber} onChange={(e) => setTrackingNumber(e.target.value)} />
          <label>Carrier (optional)</label>
          <input value={carrier} onChange={(e) => setCarrier(e.target.value)} />
          <button type="submit" className="signoff-btn" disabled={submitting}>
            Mark shipped back to supplier
          </button>
        </form>
      )}

      {(defect.replacement_status === 'shipped_back' || defect.replacement_status === 'replacement_received') &&
        defect.shipped_back_at && (
          <div className="detail-section-defect-timeline">
            <h3>Return Shipment</h3>
            {(defect.tracking_number || defect.carrier) && (
              <p className="signed-off-note">
                {[defect.carrier, defect.tracking_number].filter(Boolean).join(' — ')}
              </p>
            )}
            <ul className="signoff-row">
              <li>Shipped back {new Date(defect.shipped_back_at).toLocaleString()}</li>
              <li>
                {defect.delivered_at
                  ? `Delivered to supplier ${new Date(defect.delivered_at).toLocaleString()}`
                  : 'Not yet delivered'}
              </li>
              <li>
                {defect.supplier_received_at
                  ? `Received by supplier ${new Date(defect.supplier_received_at).toLocaleString()}`
                  : 'Not yet confirmed received'}
              </li>
            </ul>
            {canEdit && defect.replacement_status === 'shipped_back' && (
              <div className="signoff-row">
                {!defect.delivered_at && (
                  <button type="button" className="signoff-btn" onClick={handleDelivered} disabled={submitting}>
                    Mark delivered to supplier
                  </button>
                )}
                {defect.delivered_at && !defect.supplier_received_at && (
                  <button type="button" className="signoff-btn" onClick={handleSupplierReceived} disabled={submitting}>
                    Mark received by supplier
                  </button>
                )}
              </div>
            )}
          </div>
        )}

      {defect.replacement_status === 'shipped_back' && (
        <>
          <p className="signed-off-note">Shipped back — awaiting replacement</p>
          {canEdit && (
            <form onSubmit={handleReplacement} className="detail-form">
              <label>Replacement Unit Serial Number</label>
              <input value={replacementSerialNumber} onChange={(e) => setReplacementSerialNumber(e.target.value)} />
              <button type="submit" disabled={submitting || !replacementSerialNumber}>
                Record replacement received
              </button>
            </form>
          )}
        </>
      )}

      {defect.replacement_status === 'replacement_received' && (
        <p className="signed-off-note">
          <strong>Replacement received</strong> — new unit barcode above was created and starts a fresh lifecycle.
        </p>
      )}
    </section>
  )
}
