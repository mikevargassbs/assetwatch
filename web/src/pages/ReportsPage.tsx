import { useState, type FormEvent } from 'react'
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

function HardwareSummaryTable({ rows }: { rows: HardwareSummaryRow[] }) {
  if (rows.length === 0) {
    return <p className="board-empty">No matching hardware units.</p>
  }
  return (
    <div style={{ overflowX: 'auto' }}>
      <table className="report-table">
        <thead>
          <tr>
            <th>Barcode</th>
            <th>Status</th>
            <th>Stage</th>
            <th>Site</th>
            <th>Device Model</th>
            <th>Waybill</th>
            <th>Defect</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.hardware_unit_id} className={r.is_exception ? 'report-row-exception' : ''}>
              <td>{r.barcode}</td>
              <td>{r.status}</td>
              <td>{r.board_column}</td>
              <td>{r.site_name ?? '—'}</td>
              <td>{r.device_model ?? '—'}</td>
              <td>{r.waybill_number ?? '—'}</td>
              <td>{r.defect_type ?? '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
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
  return (
    <form onSubmit={onSubmit} className="report-filter-form">
      <select value={filters.stage ?? ''} onChange={(e) => setFilters({ ...filters, stage: e.target.value })}>
        {STAGE_OPTIONS.map((s) => (
          <option key={s} value={s}>
            {s === '' ? 'All stages' : s}
          </option>
        ))}
      </select>
      <input
        placeholder="Site name"
        value={filters.site_name ?? ''}
        onChange={(e) => setFilters({ ...filters, site_name: e.target.value })}
      />
      <input
        placeholder="Device model"
        value={filters.device_model ?? ''}
        onChange={(e) => setFilters({ ...filters, device_model: e.target.value })}
      />
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

  function handlePrint(e: FormEvent) {
    e.preventDefault()
    if (!siteName) return
    downloadAuthenticated(packingListUrl(siteName))
  }

  return (
    <section className="detail-section">
      <h2>Packing List Report to Each Site</h2>
      <form onSubmit={handlePrint} className="report-filter-form">
        <input placeholder="Site name" value={siteName} onChange={(e) => setSiteName(e.target.value)} required />
        <button type="submit" disabled={!siteName}>
          Print packing list
        </button>
      </form>
    </section>
  )
}
