import { useEffect, useMemo, useState, type ReactNode } from 'react'

export interface DataTableColumn<T> {
  key: string
  header: ReactNode
  render: (row: T) => ReactNode
}

interface DataTableProps<T> {
  columns: DataTableColumn<T>[]
  rows: T[]
  getRowKey: (row: T) => string | number
  rowClassName?: (row: T) => string | undefined
  searchPlaceholder?: string
  /** Extracts the text to match a row against the search query. */
  searchText?: (row: T) => string
  /**
   * Controls the search query from outside (e.g. to render the search box
   * inline with a toolbar button instead of DataTable's own layout).
   * Omit to let DataTable manage the query itself and render its own input.
   */
  query?: string
  onQueryChange?: (query: string) => void
  /** Suppresses DataTable's own search input — use with `query`/`onQueryChange`. */
  hideSearchInput?: boolean
  pageSize?: number
  emptyMessage?: string
  onRowClick?: (row: T) => void
}

const DEFAULT_PAGE_SIZE = 10

// A reusable table with client-side search and pagination, so each admin
// list doesn't have to re-implement filtering/paging by hand.
export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  rowClassName,
  searchPlaceholder = 'Search…',
  searchText,
  query: controlledQuery,
  onQueryChange,
  hideSearchInput = false,
  pageSize = DEFAULT_PAGE_SIZE,
  emptyMessage = 'Nothing to show.',
  onRowClick,
}: DataTableProps<T>) {
  const [internalQuery, setInternalQuery] = useState('')
  const query = controlledQuery ?? internalQuery
  const setQuery = onQueryChange ?? setInternalQuery
  const [page, setPage] = useState(1)

  const filtered = useMemo(() => {
    if (!searchText || !query.trim()) return rows
    const q = query.trim().toLowerCase()
    return rows.filter((row) => searchText(row).toLowerCase().includes(q))
  }, [rows, query, searchText])

  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize))

  useEffect(() => {
    setPage(1)
  }, [query, rows.length])

  const clampedPage = Math.min(page, pageCount)
  const pageRows = filtered.slice((clampedPage - 1) * pageSize, clampedPage * pageSize)

  return (
    <div className="data-table">
      {searchText && !hideSearchInput && (
        <input
          className="data-table-search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={searchPlaceholder}
        />
      )}

      {filtered.length === 0 ? (
        <p className="board-empty">{emptyMessage}</p>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="report-table">
            <thead>
              <tr>
                {columns.map((c) => (
                  <th key={c.key}>{c.header}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {pageRows.map((row) => (
                <tr
                  key={getRowKey(row)}
                  className={`${rowClassName?.(row) ?? ''}${onRowClick ? ' data-table-row-clickable' : ''}`}
                  onClick={onRowClick ? () => onRowClick(row) : undefined}
                >
                  {columns.map((c) => (
                    <td key={c.key}>{c.render(row)}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {pageCount > 1 && (
        <div className="data-table-pagination">
          <button type="button" className="btn" disabled={clampedPage <= 1} onClick={() => setPage(clampedPage - 1)}>
            Previous
          </button>
          <span>
            Page {clampedPage} of {pageCount}
          </span>
          <button
            type="button"
            className="btn"
            disabled={clampedPage >= pageCount}
            onClick={() => setPage(clampedPage + 1)}
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}
