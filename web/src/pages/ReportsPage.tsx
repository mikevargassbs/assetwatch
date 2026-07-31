import { useEffect, useState, type FormEvent } from 'react'
import { listSiteLocations, type SiteLocation } from '../api/admin'
import { listDeviceModels } from '../api/hardware'
import { DataTable, type DataTableColumn } from '../components/DataTable'
import {
  defectsReportExportUrl,
  downloadAuthenticated,
  getDefectsReport,
  getHardwareSummary,
  hardwareSummaryExportUrl,
  packingListUrl,
  type HardwareFilters,
  type HardwareSummaryRow,
} from '../api/reporting'

const STAGE_OPTIONS = [
  '', 'pre_deployment', 'configuration', 'shipment', 'installation', 'commissioning', 'completed',
]

export function ReportsPage() {
  return (
    <div className="detail-page">
      <div className="page-toolbar">
        <h1 className="page-title">Reports</h1>
      </div>

      <HardwareSummarySection />
      <DefectsReportSection />
      <PackingListSection />
    </div>
  )
}

const HARDWARE_SUMMARY_COLUMNS: DataTableColumn<HardwareSummaryRow>[] = [
  { key: 'barcode', header: 'Barcode', render: (r) => r.barcode },
  { key: 'status', header: 'Status', render: (r) => r.status },
  { key: 'stage', header: 'Stage', render: (r) => r.board_column },
  { key: 'site', header: 'Site', render: (r) => r.site_name ?? r.allocated_branch ?? '—' },
  { key: 'device_model', header: 'Device Model', render: (r) => r.device_model ?? '—' },
  { key: 'waybill', header: 'Waybill', render: (r) => r.waybill_number ?? '—' },
  { key: 'defect', header: 'Defect', render: (r) => r.defect_type ?? '—' },
]

function HardwareSummaryTable({ rows }: { rows: HardwareSummaryRow[] }) {
  return (
    <DataTable
      columns={HARDWARE_SUMMARY_COLUMNS}
      rows={rows}
      getRowKey={(r) => r.hardware_unit_id}
      rowClassName={(r) => (r.is_exception ? 'report-row-exception' : undefined)}
      searchText={(r) =>
        [r.barcode, r.status, r.board_column, r.site_name, r.allocated_branch, r.device_model, r.waybill_number, r.defect_type]
          .filter(Boolean)
          .join(' ')
      }
      searchPlaceholder="Search hardware…"
      emptyMessage="No matching hardware units."
    />
  )
}

function FilterForm({
  filters,
  setFilters,
  onSubmit,
  submitting,
}: {
  filters: HardwareFilters
  setFilters: (f: HardwareFilters) => void
  onSubmit: (e: FormEvent) => void
  submitting: boolean
}) {
  const [siteLocations, setSiteLocations] = useState<SiteLocation[]>([])
  const [deviceModels, setDeviceModels] = useState<string[]>([])

  useEffect(() => {
    listSiteLocations().then(setSiteLocations).catch(() => setSiteLocations([]))
    listDeviceModels().then(setDeviceModels).catch(() => setDeviceModels([]))
  }, [])

  return (
    <form onSubmit={onSubmit} className="report-filter-form">
      <select value={filters.stage ?? ''} onChange={(e) => setFilters({ ...filters, stage: e.target.value })}>
        {STAGE_OPTIONS.map((s) => (
          <option key={s} value={s}>
            {s === '' ? 'All stages' : s}
          </option>
        ))}
      </select>
      <select
        value={filters.site_name ?? ''}
        onChange={(e) => setFilters({ ...filters, site_name: e.target.value })}
      >
        <option value="">All sites</option>
        {siteLocations.map((s) => (
          <option key={s.id} value={s.name}>
            {s.name}
          </option>
        ))}
      </select>
      <select
        value={filters.device_model ?? ''}
        onChange={(e) => setFilters({ ...filters, device_model: e.target.value })}
      >
        <option value="">All device models</option>
        {deviceModels.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>
      <input
        type="date"
        value={filters.date_from ?? ''}
        onChange={(e) => setFilters({ ...filters, date_from: e.target.value })}
      />
      <input
        type="date"
        value={filters.date_to ?? ''}
        onChange={(e) => setFilters({ ...filters, date_to: e.target.value })}
      />
      <button type="submit" disabled={submitting}>
        Apply filters
      </button>
    </form>
  )
}

function HardwareSummarySection() {
  const [filters, setFilters] = useState<HardwareFilters>({})
  const [rows, setRows] = useState<HardwareSummaryRow[]>([])
  const [loading, setLoading] = useState(false)
  const [loaded, setLoaded] = useState(false)

  async function load(e?: FormEvent) {
    e?.preventDefault()
    setLoading(true)
    try {
      setRows(await getHardwareSummary(filters))
      setLoaded(true)
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="detail-section">
      <h2>Summary of Hardware</h2>
      <p className="detail-status">Filter by stage, site, device model, or date range — also works as "Summary of Hardware to a Site" by setting the Site filter.</p>
      <FilterForm filters={filters} setFilters={setFilters} onSubmit={load} submitting={loading} />

      <div className="signoff-row">
        <button className="signoff-btn" onClick={() => downloadAuthenticated(hardwareSummaryExportUrl(filters, 'csv'))}>
          Export CSV
        </button>
        <button className="signoff-btn" onClick={() => downloadAuthenticated(hardwareSummaryExportUrl(filters, 'pdf'))}>
          Print / Export PDF
        </button>
      </div>

      {loaded && <HardwareSummaryTable rows={rows} />}
    </section>
  )
}

function DefectsReportSection() {
  const [filters, setFilters] = useState<HardwareFilters>({})
  const [rows, setRows] = useState<HardwareSummaryRow[]>([])
  const [loading, setLoading] = useState(false)
  const [loaded, setLoaded] = useState(false)

  async function load(e?: FormEvent) {
    e?.preventDefault()
    setLoading(true)
    try {
      setRows(await getDefectsReport(filters))
      setLoaded(true)
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="detail-section">
      <h2>Defective / Damage / Wrong Item Report</h2>
      <FilterForm filters={filters} setFilters={setFilters} onSubmit={load} submitting={loading} />

      <div className="signoff-row">
        <button className="signoff-btn" onClick={() => downloadAuthenticated(defectsReportExportUrl(filters, 'csv'))}>
          Export CSV
        </button>
        <button className="signoff-btn" onClick={() => downloadAuthenticated(defectsReportExportUrl(filters, 'pdf'))}>
          Print / Export PDF
        </button>
      </div>

      {loaded && <HardwareSummaryTable rows={rows} />}
    </section>
  )
}

function PackingListSection() {
  const [siteName, setSiteName] = useState('')
  const [siteLocations, setSiteLocations] = useState<SiteLocation[]>([])

  useEffect(() => {
    listSiteLocations().then(setSiteLocations).catch(() => setSiteLocations([]))
  }, [])

  function handlePrint(e: FormEvent) {
    e.preventDefault()
    if (!siteName) return
    downloadAuthenticated(packingListUrl(siteName))
  }

  return (
    <section className="detail-section">
      <h2>Packing List Report to Each Site</h2>
      <form onSubmit={handlePrint} className="report-filter-form">
        <select value={siteName} onChange={(e) => setSiteName(e.target.value)} required>
          <option value="">Select a site</option>
          {siteLocations.map((s) => (
            <option key={s.id} value={s.name}>
              {s.name}
            </option>
          ))}
        </select>
        <button type="submit" disabled={!siteName}>
          Print packing list
        </button>
      </form>
    </section>
  )
}
