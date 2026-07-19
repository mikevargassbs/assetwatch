import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listUnits, type HardwareUnit } from '../api/hardware'
import { DataTable, type DataTableColumn } from '../components/DataTable'

function unitLabel(u: HardwareUnit) {
  return u.alias || u.barcode
}

const COLUMNS: DataTableColumn<HardwareUnit>[] = [
  { key: 'name', header: 'Name', render: (u) => unitLabel(u) },
  { key: 'barcode', header: 'Barcode', render: (u) => u.barcode },
  { key: 'serial', header: 'Serial Number', render: (u) => u.serial_number ?? '—' },
  { key: 'make', header: 'Make', render: (u) => u.device_make ?? '—' },
  { key: 'model', header: 'Model', render: (u) => u.device_model ?? '—' },
  { key: 'part_number', header: 'Part Number', render: (u) => u.part_number ?? '—' },
  { key: 'site', header: 'Site', render: (u) => u.allocated_branch ?? '—' },
  { key: 'status', header: 'Status', render: (u) => <span className="board-card-badge">{u.status}</span> },
]

// Units declared defective/damaged/wrong item live here instead of on the
// Kanban board — they're a branch off the main lifecycle flow, not a stage
// within it, so mixing them into the board columns just adds noise.
export function ExceptionsPage() {
  const navigate = useNavigate()
  const [units, setUnits] = useState<HardwareUnit[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    listUnits()
      .then((data) => setUnits((data ?? []).filter((u) => u.is_exception)))
      .catch(() => setError('Failed to load exceptions.'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="board-page">
      <div className="page-toolbar">
        <h1 className="page-title">Exceptions</h1>
      </div>

      {error && <div className="board-error">{error}</div>}

      {loading ? (
        <p className="board-empty">Loading…</p>
      ) : (
        <DataTable
          rows={units}
          getRowKey={(u) => u.id}
          columns={COLUMNS}
          onRowClick={(u) => navigate(`/units/${u.id}`)}
          searchPlaceholder="Search alias, barcode, serial, make/model…"
          searchText={(u) => [u.alias, u.barcode, u.serial_number, u.device_make, u.device_model].filter(Boolean).join(' ')}
          emptyMessage="No units are currently declared defective, damaged, or wrong item."
        />
      )}
    </div>
  )
}
