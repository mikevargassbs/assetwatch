import { useEffect, useState, type FormEvent } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import {
  ALL_ROLES,
  createMetaDataField,
  createSiteLocation,
  createUser,
  deactivateMetaDataField,
  deactivateSiteLocation,
  getBarcodeLabelSettings,
  listBarcodeLabelFields,
  listMetaDataFields,
  listSiteLocations,
  listUsers,
  previewBarcodeLabel,
  reactivateMetaDataField,
  reactivateSiteLocation,
  setUserPassword,
  setUserRoles,
  updateBarcodeLabelSettings,
  updateMetaDataField,
  updateSiteLocation,
  updateUser,
  META_DATA_STAGES,
  type BarcodeLabelField,
  type BarcodeLabelSettings,
  type FieldDefinition,
  type SiteLocation,
  type UserSummary,
} from '../api/admin'
import { listUnits, purgeUnit, restoreUnit, type HardwareUnit } from '../api/hardware'
import { downloadTransactionDataExport, WIPE_CONFIRMATION_PHRASE, wipeAllTransactionData } from '../api/dataManagement'
import { useConfirm } from '../components/ConfirmDialogProvider'
import { useSnackbar } from '../components/SnackbarProvider'
import { DataTable } from '../components/DataTable'

const TABS = ['Users', 'Meta-Data Fields', 'Site Locations', 'Barcode Label', 'Retired Units', 'Data Management'] as const
type Tab = (typeof TABS)[number]

export function AdminPage() {
  const { roles } = useAuth()
  const [tab, setTab] = useState<Tab>('Users')

  if (!roles.includes('admin')) {
    return <Navigate to="/board" replace />
  }

  return (
    <div className="detail-page">
      <div className="page-toolbar">
        <h1 className="page-title">Admin</h1>
      </div>

      <div className="admin-tabs">
        {TABS.map((t) => (
          <button
            key={t}
            type="button"
            className={`admin-tab${tab === t ? ' admin-tab-active' : ''}`}
            onClick={() => setTab(t)}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === 'Users' && <UsersTab />}
      {tab === 'Meta-Data Fields' && <MetaDataFieldsTab />}
      {tab === 'Site Locations' && <SiteLocationsTab />}
      {tab === 'Barcode Label' && <BarcodeLabelSettingsTab />}
      {tab === 'Retired Units' && <RetiredUnitsTab />}
      {tab === 'Data Management' && <DataManagementTab />}
    </div>
  )
}

function roleLabel(role: string) {
  return role
    .split('_')
    .map((w) => w[0].toUpperCase() + w.slice(1))
    .join(' ')
}

function UsersTab() {
  const [users, setUsers] = useState<UserSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [editingUser, setEditingUser] = useState<UserSummary | null>(null)
  const [query, setQuery] = useState('')
  const showSnackbar = useSnackbar()

  async function load(showLoading = false) {
    if (showLoading) setLoading(true)
    try {
      setUsers((await listUsers()) ?? [])
    } catch {
      showSnackbar('Failed to load users.', 'error')
    } finally {
      if (showLoading) setLoading(false)
    }
  }

  useEffect(() => {
    load(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleToggleRole(user: UserSummary, role: string) {
    const nextRoles = user.roles.includes(role)
      ? user.roles.filter((r) => r !== role)
      : [...user.roles, role]
    try {
      await setUserRoles(user.id, nextRoles)
      showSnackbar(`Updated roles for ${user.email}.`, 'success')
      load()
    } catch {
      showSnackbar('Failed to update roles.', 'error')
    }
  }

  return (
    <section className="detail-section">
      <div className="section-toolbar">
        <h2>Users</h2>
        <div className="section-toolbar-actions">
          <input
            className="data-table-search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search by email or name…"
          />
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            + New User
          </button>
        </div>
      </div>

      {loading && <p className="board-empty">Loading…</p>}
      {!loading && (
        <DataTable
          rows={users}
          getRowKey={(u) => u.id}
          rowClassName={(u) => (u.is_active ? undefined : 'admin-list-inactive')}
          searchText={(u) => `${u.email} ${u.full_name}`}
          query={query}
          onQueryChange={setQuery}
          hideSearchInput
          emptyMessage="No users yet."
          columns={[
            { key: 'email', header: 'Email', render: (u) => u.email },
            { key: 'name', header: 'Name', render: (u) => u.full_name },
            { key: 'status', header: 'Status', render: (u) => (u.is_active ? 'Active' : 'Inactive') },
            ...ALL_ROLES.map((r) => ({
              key: r,
              header: roleLabel(r),
              render: (u: UserSummary) => (
                <input type="checkbox" checked={u.roles.includes(r)} onChange={() => handleToggleRole(u, r)} />
              ),
            })),
            {
              key: 'actions',
              header: '',
              render: (u) => (
                <button type="button" className="link-button" onClick={() => setEditingUser(u)}>
                  Edit
                </button>
              ),
            },
          ]}
        />
      )}

      {showCreate && (
        <CreateUserModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false)
            load()
          }}
        />
      )}

      {editingUser && (
        <EditUserModal
          user={editingUser}
          onClose={() => setEditingUser(null)}
          onSaved={() => {
            setEditingUser(null)
            load()
          }}
        />
      )}
    </section>
  )
}

function EditUserModal({
  user,
  onClose,
  onSaved,
}: {
  user: UserSummary
  onClose: () => void
  onSaved: () => void
}) {
  const [fullName, setFullName] = useState(user.full_name)
  const [isActive, setIsActive] = useState(user.is_active)
  const [submitting, setSubmitting] = useState(false)
  const [newPassword, setNewPassword] = useState('')
  const [changingPassword, setChangingPassword] = useState(false)
  const showSnackbar = useSnackbar()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await updateUser(user.id, { full_name: fullName, is_active: isActive })
      showSnackbar(`Updated ${user.email}.`, 'success')
      onSaved()
    } catch {
      showSnackbar('Failed to update user.', 'error')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleChangePassword() {
    setChangingPassword(true)
    try {
      await setUserPassword(user.id, newPassword)
      showSnackbar(`Password changed for ${user.email}.`, 'success')
      setNewPassword('')
    } catch {
      showSnackbar('Failed to change password.', 'error')
    } finally {
      setChangingPassword(false)
    }
  }

  return (
    <div className="modal-overlay">
      <form className="modal-card" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
          &times;
        </button>
        <h2>Edit User</h2>
        <p className="modal-subtitle">{user.email}</p>
        <label htmlFor="edit-user-name">Full Name</label>
        <input id="edit-user-name" value={fullName} onChange={(e) => setFullName(e.target.value)} required />
        <label className="checkbox-row">
          <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} />
          Active (can log in)
        </label>
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting || !fullName}>
            {submitting ? 'Saving…' : 'Save'}
          </button>
        </div>

        <hr />
        <label htmlFor="edit-user-password">Change Password</label>
        <input
          id="edit-user-password"
          type="password"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          placeholder="New password (min. 8 characters)"
          autoComplete="new-password"
        />
        <div className="modal-actions">
          <button
            type="button"
            className="btn btn-primary"
            onClick={handleChangePassword}
            disabled={changingPassword || newPassword.length < 8}
          >
            {changingPassword ? 'Changing…' : 'Change Password'}
          </button>
        </div>
      </form>
    </div>
  )
}

function CreateUserModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [email, setEmail] = useState('')
  const [fullName, setFullName] = useState('')
  const [password, setPassword] = useState('')
  const [roles, setRoles] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const showSnackbar = useSnackbar()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await createUser({ email, full_name: fullName, password, roles })
      showSnackbar(`Created user ${email}.`, 'success')
      onCreated()
    } catch (err) {
      if (err instanceof Error && err.message.startsWith('409')) {
        setError('That email is already in use.')
      } else if (err instanceof Error && err.message.startsWith('400')) {
        setError('Email domain is not allowed.')
      } else {
        setError('Failed to create user.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  function toggleRole(role: string) {
    setRoles((prev) => (prev.includes(role) ? prev.filter((r) => r !== role) : [...prev, role]))
  }

  return (
    <div className="modal-overlay">
      <form className="modal-card" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
          &times;
        </button>
        <h2>New User</h2>
        <label htmlFor="new-user-email">Email</label>
        <input id="new-user-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <label htmlFor="new-user-name">Full Name</label>
        <input id="new-user-name" value={fullName} onChange={(e) => setFullName(e.target.value)} required />
        <label htmlFor="new-user-password">Password</label>
        <input
          id="new-user-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
        <label>Roles</label>
        <div className="role-checkbox-grid">
          {ALL_ROLES.map((r) => (
            <label key={r} className="checkbox-row">
              <input type="checkbox" checked={roles.includes(r)} onChange={() => toggleRole(r)} />
              {roleLabel(r)}
            </label>
          ))}
        </div>
        {error && <div className="login-error">{error}</div>}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting || !email || !fullName || !password}>
            {submitting ? 'Creating…' : 'Create'}
          </button>
        </div>
      </form>
    </div>
  )
}

function MetaDataFieldsTab() {
  const [stage, setStage] = useState<string>('stage1a')
  const [fields, setFields] = useState<FieldDefinition[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [editingField, setEditingField] = useState<FieldDefinition | null>(null)
  const [query, setQuery] = useState('')
  const confirm = useConfirm()
  const showSnackbar = useSnackbar()

  async function load(showLoading = false) {
    if (!stage.trim()) {
      setFields([])
      return
    }
    if (showLoading) setLoading(true)
    try {
      setFields((await listMetaDataFields(stage, true)) ?? [])
    } catch {
      showSnackbar('Failed to load field definitions.', 'error')
    } finally {
      if (showLoading) setLoading(false)
    }
  }

  useEffect(() => {
    load(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stage])

  async function handleDeactivate(field: FieldDefinition) {
    const ok = await confirm({
      title: `Remove "${field.label}"?`,
      message: 'This hides the field from the form. Existing data already saved under it is not deleted.',
      confirmLabel: 'Remove',
      danger: true,
    })
    if (!ok) return
    try {
      await deactivateMetaDataField(field.id)
      showSnackbar(`Removed field "${field.label}".`, 'success')
      load()
    } catch {
      showSnackbar('Failed to remove field.', 'error')
    }
  }

  async function handleReactivate(field: FieldDefinition) {
    try {
      await reactivateMetaDataField(field.id)
      showSnackbar(`Restored field "${field.label}".`, 'success')
      load()
    } catch {
      showSnackbar('Failed to restore field.', 'error')
    }
  }

  return (
    <section className="detail-section">
      <div className="section-toolbar">
        <h2>Meta-Data Fields</h2>
        <div className="section-toolbar-actions">
          <input
            className="data-table-search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search by label or key…"
          />
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            + New Field
          </button>
        </div>
      </div>

      <label htmlFor="stage-select">Stage</label>
      <select id="stage-select" value={stage} onChange={(e) => setStage(e.target.value)}>
        {META_DATA_STAGES.map((s) => (
          <option key={s} value={s}>
            {s}
          </option>
        ))}
      </select>
      <p className="modal-subtitle">
        "general" fields aren't tied to a lifecycle stage — they're captured on the New Unit form and shown on the
        unit's Attributes tab. Other listed stages have their own form.
      </p>

      {loading && <p className="board-empty">Loading…</p>}
      {!loading && (
        <DataTable
          rows={fields}
          getRowKey={(f) => f.id}
          rowClassName={(f) => (f.active ? undefined : 'admin-list-inactive')}
          searchText={(f) => `${f.label} ${f.field_key}`}
          query={query}
          onQueryChange={setQuery}
          hideSearchInput
          emptyMessage="No custom fields for this stage yet."
          columns={[
            { key: 'label', header: 'Label', render: (f) => f.label },
            { key: 'key', header: 'Key', render: (f) => f.field_key },
            { key: 'type', header: 'Type', render: (f) => f.data_type },
            { key: 'required', header: 'Required', render: (f) => (f.is_required ? 'Yes' : 'No') },
            { key: 'status', header: 'Status', render: (f) => (f.active ? 'Active' : 'Inactive') },
            {
              key: 'actions',
              header: '',
              render: (f) =>
                f.active ? (
                  <>
                    <button type="button" className="link-button" onClick={() => setEditingField(f)}>
                      Edit
                    </button>
                    {' · '}
                    <button type="button" className="link-button" onClick={() => handleDeactivate(f)}>
                      Remove
                    </button>
                  </>
                ) : (
                  <button type="button" className="link-button" onClick={() => handleReactivate(f)}>
                    Restore
                  </button>
                ),
            },
          ]}
        />
      )}

      {showCreate && (
        <CreateFieldModal
          stage={stage}
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false)
            load()
          }}
        />
      )}

      {editingField && (
        <EditFieldModal
          field={editingField}
          onClose={() => setEditingField(null)}
          onSaved={() => {
            setEditingField(null)
            load()
          }}
        />
      )}
    </section>
  )
}

function EditFieldModal({
  field,
  onClose,
  onSaved,
}: {
  field: FieldDefinition
  onClose: () => void
  onSaved: () => void
}) {
  const [label, setLabel] = useState(field.label)
  const [isRequired, setIsRequired] = useState(field.is_required)
  const [sortOrder, setSortOrder] = useState(String(field.sort_order))
  const [submitting, setSubmitting] = useState(false)
  const showSnackbar = useSnackbar()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await updateMetaDataField(field.id, {
        label,
        is_required: isRequired,
        sort_order: parseInt(sortOrder, 10) || 0,
      })
      showSnackbar(`Updated field "${label}".`, 'success')
      onSaved()
    } catch {
      showSnackbar('Failed to update field.', 'error')
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
        <h2>Edit Field</h2>
        <p className="modal-subtitle">
          {field.stage} / {field.field_key} ({field.data_type}) — key and type can't be changed once created.
        </p>
        <label htmlFor="edit-field-label">Label</label>
        <input id="edit-field-label" value={label} onChange={(e) => setLabel(e.target.value)} required />
        <label htmlFor="edit-field-sort">Sort Order</label>
        <input
          id="edit-field-sort"
          type="number"
          value={sortOrder}
          onChange={(e) => setSortOrder(e.target.value)}
        />
        <label className="checkbox-row">
          <input type="checkbox" checked={isRequired} onChange={(e) => setIsRequired(e.target.checked)} />
          Required
        </label>
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting || !label}>
            {submitting ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </div>
  )
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

export function CreateFieldModal({
  stage,
  onClose,
  onCreated,
}: {
  stage: string
  onClose: () => void
  onCreated: () => void
}) {
  const [label, setLabel] = useState('')
  const [fieldKey, setFieldKey] = useState('')
  const [fieldKeyTouched, setFieldKeyTouched] = useState(false)
  const [dataType, setDataType] = useState('boolean')
  const [isRequired, setIsRequired] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const showSnackbar = useSnackbar()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await createMetaDataField({
        stage,
        field_key: fieldKey,
        label,
        data_type: dataType,
        is_required: isRequired,
        sort_order: 0,
      })
      showSnackbar(`Added field "${label}".`, 'success')
      onCreated()
    } catch {
      setError('Failed to create field. The key may already be in use for this stage.')
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
        <h2>New Field ({stage})</h2>
        <label htmlFor="field-label">Label</label>
        <input
          id="field-label"
          value={label}
          onChange={(e) => {
            setLabel(e.target.value)
            if (!fieldKeyTouched) {
              setFieldKey(slugify(e.target.value))
            }
          }}
          placeholder="e.g. Axis Website"
          required
        />
        <label htmlFor="field-key">Field Key</label>
        <input
          id="field-key"
          value={fieldKey}
          onChange={(e) => {
            setFieldKey(e.target.value)
            setFieldKeyTouched(e.target.value !== '')
          }}
          placeholder="e.g. axis_website"
          required
        />
        <label htmlFor="field-type">Data Type</label>
        <select id="field-type" value={dataType} onChange={(e) => setDataType(e.target.value)}>
          <option value="boolean">Boolean (checkbox)</option>
          <option value="text">Text</option>
          <option value="number">Number</option>
        </select>
        <label className="checkbox-row">
          <input type="checkbox" checked={isRequired} onChange={(e) => setIsRequired(e.target.checked)} />
          Required
        </label>
        {error && <div className="login-error">{error}</div>}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting || !label || !fieldKey}>
            {submitting ? 'Creating…' : 'Create'}
          </button>
        </div>
      </form>
    </div>
  )
}

function SiteLocationsTab() {
  const [locations, setLocations] = useState<SiteLocation[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [editingLocation, setEditingLocation] = useState<SiteLocation | null>(null)
  const [query, setQuery] = useState('')
  const confirm = useConfirm()
  const showSnackbar = useSnackbar()

  async function load(showLoading = false) {
    if (showLoading) setLoading(true)
    try {
      setLocations((await listSiteLocations(true)) ?? [])
    } catch {
      showSnackbar('Failed to load site locations.', 'error')
    } finally {
      if (showLoading) setLoading(false)
    }
  }

  useEffect(() => {
    load(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleRemove(loc: SiteLocation) {
    const ok = await confirm({
      title: `Remove "${loc.name}"?`,
      message: 'It will no longer appear in the Site Name suggestions for new shipments.',
      confirmLabel: 'Remove',
      danger: true,
    })
    if (!ok) return
    try {
      await deactivateSiteLocation(loc.id)
      showSnackbar(`Removed site "${loc.name}".`, 'success')
      load()
    } catch {
      showSnackbar('Failed to remove site.', 'error')
    }
  }

  async function handleReactivate(loc: SiteLocation) {
    try {
      await reactivateSiteLocation(loc.id)
      showSnackbar(`Restored site "${loc.name}".`, 'success')
      load()
    } catch {
      showSnackbar('Failed to restore site.', 'error')
    }
  }

  return (
    <section className="detail-section">
      <div className="section-toolbar">
        <h2>Site Locations</h2>
        <div className="section-toolbar-actions">
          <input
            className="data-table-search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search by name…"
          />
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            + Add Site
          </button>
        </div>
      </div>

      {loading && <p className="board-empty">Loading…</p>}
      {!loading && (
        <DataTable
          rows={locations}
          getRowKey={(loc) => loc.id}
          rowClassName={(loc) => (loc.active ? undefined : 'admin-list-inactive')}
          searchText={(loc) => loc.name}
          query={query}
          onQueryChange={setQuery}
          hideSearchInput
          emptyMessage="No site locations yet."
          columns={[
            { key: 'name', header: 'Name', render: (loc) => loc.name },
            { key: 'region', header: 'Region', render: (loc) => loc.region ?? '—' },
            { key: 'ip_gateway', header: 'IP Gateway', render: (loc) => loc.ip_gateway ?? '—' },
            { key: 'subnet_mask', header: 'Subnet Mask', render: (loc) => loc.subnet_mask ?? '—' },
            { key: 'status', header: 'Status', render: (loc) => (loc.active ? 'Active' : 'Inactive') },
            {
              key: 'actions',
              header: '',
              render: (loc) =>
                loc.active ? (
                  <>
                    <button type="button" className="link-button" onClick={() => setEditingLocation(loc)}>
                      Edit
                    </button>
                    {' · '}
                    <button type="button" className="link-button" onClick={() => handleRemove(loc)}>
                      Remove
                    </button>
                  </>
                ) : (
                  <button type="button" className="link-button" onClick={() => handleReactivate(loc)}>
                    Restore
                  </button>
                ),
            },
          ]}
        />
      )}

      {showCreate && (
        <CreateSiteLocationModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false)
            load()
          }}
        />
      )}

      {editingLocation && (
        <EditSiteLocationModal
          location={editingLocation}
          onClose={() => setEditingLocation(null)}
          onSaved={() => {
            setEditingLocation(null)
            load()
          }}
        />
      )}
    </section>
  )
}

function CreateSiteLocationModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState('')
  const [region, setRegion] = useState('')
  const [ipGateway, setIPGateway] = useState('')
  const [subnetMask, setSubnetMask] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const showSnackbar = useSnackbar()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!name) return
    setSubmitting(true)
    setError(null)
    try {
      await createSiteLocation({
        name,
        region: region || undefined,
        ip_gateway: ipGateway || undefined,
        subnet_mask: subnetMask || undefined,
      })
      showSnackbar(`Added site "${name}".`, 'success')
      onCreated()
    } catch (err) {
      setError(err instanceof Error && err.message.startsWith('409') ? 'That site name already exists.' : 'Failed to add site.')
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
        <h2>Add Site Location</h2>
        <label htmlFor="new-site-name">Name</label>
        <input id="new-site-name" value={name} onChange={(e) => setName(e.target.value)} required />
        <label htmlFor="new-site-region">Region</label>
        <input id="new-site-region" value={region} onChange={(e) => setRegion(e.target.value)} />
        <label htmlFor="new-site-ip-gateway">IP Gateway</label>
        <input id="new-site-ip-gateway" value={ipGateway} onChange={(e) => setIPGateway(e.target.value)} />
        <label htmlFor="new-site-subnet-mask">Subnet Mask</label>
        <input id="new-site-subnet-mask" value={subnetMask} onChange={(e) => setSubnetMask(e.target.value)} />
        {error && <div className="login-error">{error}</div>}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting || !name}>
            {submitting ? 'Adding…' : 'Add Site'}
          </button>
        </div>
      </form>
    </div>
  )
}

function EditSiteLocationModal({
  location,
  onClose,
  onSaved,
}: {
  location: SiteLocation
  onClose: () => void
  onSaved: () => void
}) {
  const [name, setName] = useState(location.name)
  const [region, setRegion] = useState(location.region ?? '')
  const [ipGateway, setIPGateway] = useState(location.ip_gateway ?? '')
  const [subnetMask, setSubnetMask] = useState(location.subnet_mask ?? '')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const showSnackbar = useSnackbar()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await updateSiteLocation(location.id, {
        name,
        region: region || undefined,
        ip_gateway: ipGateway || undefined,
        subnet_mask: subnetMask || undefined,
      })
      showSnackbar(`Updated "${name}".`, 'success')
      onSaved()
    } catch (err) {
      setError(err instanceof Error && err.message.startsWith('409') ? 'That site name already exists.' : 'Failed to update site.')
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
        <h2>Edit Site Location</h2>
        <label htmlFor="edit-site-name">Name</label>
        <input id="edit-site-name" value={name} onChange={(e) => setName(e.target.value)} required />
        <label htmlFor="edit-site-region">Region</label>
        <input id="edit-site-region" value={region} onChange={(e) => setRegion(e.target.value)} />
        <label htmlFor="edit-site-ip-gateway">IP Gateway</label>
        <input id="edit-site-ip-gateway" value={ipGateway} onChange={(e) => setIPGateway(e.target.value)} />
        <label htmlFor="edit-site-subnet-mask">Subnet Mask</label>
        <input id="edit-site-subnet-mask" value={subnetMask} onChange={(e) => setSubnetMask(e.target.value)} />
        {error && <div className="login-error">{error}</div>}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={submitting || !name}>
            {submitting ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </div>
  )
}

function BarcodeLabelSettingsTab() {
  const [settings, setSettings] = useState<BarcodeLabelSettings | null>(null)
  const [availableFields, setAvailableFields] = useState<BarcodeLabelField[]>([])
  const [width, setWidth] = useState('')
  const [height, setHeight] = useState('')
  const [fields, setFields] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [previewError, setPreviewError] = useState(false)
  const showSnackbar = useSnackbar()

  useEffect(() => {
    ;(async () => {
      try {
        const [s, f] = await Promise.all([getBarcodeLabelSettings(), listBarcodeLabelFields()])
        setSettings(s)
        setWidth(String(s.barcode_label_width_mm))
        setHeight(String(s.barcode_label_height_mm))
        setFields(s.barcode_label_fields)
        setAvailableFields(f)
      } catch {
        showSnackbar('Failed to load barcode label settings.', 'error')
      } finally {
        setLoading(false)
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function toggleField(key: string) {
    setFields((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]))
  }

  // Live preview: re-render the sample sticker PDF a moment after the user
  // stops changing width/height/fields, so they can see the effect before
  // saving.
  useEffect(() => {
    const w = parseFloat(width)
    const h = parseFloat(height)
    if (!(w >= 10 && w <= 300 && h >= 10 && h <= 300)) {
      setPreviewError(false)
      return
    }

    let cancelled = false
    const timer = setTimeout(async () => {
      try {
        const blob = await previewBarcodeLabel({
          barcode_label_width_mm: w,
          barcode_label_height_mm: h,
          barcode_label_fields: fields,
        })
        if (cancelled) return
        setPreviewUrl((prev) => {
          if (prev) URL.revokeObjectURL(prev)
          return URL.createObjectURL(blob)
        })
        setPreviewError(false)
      } catch {
        if (!cancelled) setPreviewError(true)
      }
    }, 400)

    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [width, height, fields])

  useEffect(() => {
    return () => {
      if (previewUrl) URL.revokeObjectURL(previewUrl)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      const s = await updateBarcodeLabelSettings({
        barcode_label_width_mm: parseFloat(width),
        barcode_label_height_mm: parseFloat(height),
        barcode_label_fields: fields,
      })
      setSettings(s)
      showSnackbar('Updated barcode label settings.', 'success')
    } catch {
      showSnackbar('Failed to update barcode label settings.', 'error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="detail-section">
      <div className="section-toolbar">
        <h2>Barcode Label</h2>
      </div>
      <p className="modal-subtitle">
        The physical size and printed fields of the barcode sticker (e.g. for a TSC TTP-244 Pro
        label printer). Match the size to the label stock loaded in the printer. The barcode
        graphic itself always prints — the checkboxes below control which text lines appear
        underneath it. The preview updates as you change settings and shows sample data — save to
        apply to real unit stickers.
      </p>

      {loading && <p className="board-empty">Loading…</p>}
      {!loading && settings && (
        <div className="barcode-label-settings">
          <form onSubmit={handleSubmit} className="barcode-label-form">
            <div className="site-location-add-form">
              <label htmlFor="label-width">Width (mm)</label>
              <input
                id="label-width"
                type="number"
                min="10"
                max="300"
                step="0.1"
                value={width}
                onChange={(e) => setWidth(e.target.value)}
                required
              />
              <label htmlFor="label-height">Height (mm)</label>
              <input
                id="label-height"
                type="number"
                min="10"
                max="300"
                step="0.1"
                value={height}
                onChange={(e) => setHeight(e.target.value)}
                required
              />
            </div>

            <label>Fields</label>
            <div className="role-checkbox-grid">
              {availableFields.map((f) => (
                <label key={f.key} className="checkbox-row">
                  <input type="checkbox" checked={fields.includes(f.key)} onChange={() => toggleField(f.key)} />
                  {f.label}
                </label>
              ))}
            </div>

            <button type="submit" className="btn btn-primary" disabled={submitting || !width || !height}>
              {submitting ? 'Saving…' : 'Save'}
            </button>
          </form>

          <div className="barcode-label-preview">
            <h3>Preview</h3>
            {previewError && <p className="board-empty">Enter a width and height between 10mm and 300mm.</p>}
            {!previewError && previewUrl && (
              <iframe src={previewUrl} title="Barcode label preview" className="barcode-label-preview-frame" />
            )}
          </div>
        </div>
      )}
    </section>
  )
}

function RetiredUnitsTab() {
  const [units, setUnits] = useState<HardwareUnit[]>([])
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  const confirm = useConfirm()
  const showSnackbar = useSnackbar()
  const navigate = useNavigate()

  async function load(showLoading = false) {
    if (showLoading) setLoading(true)
    try {
      const all = await listUnits(true)
      setUnits(all.filter((u) => u.status === 'retired'))
    } catch {
      showSnackbar('Failed to load retired units.', 'error')
    } finally {
      if (showLoading) setLoading(false)
    }
  }

  useEffect(() => {
    load(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleRestore(unit: HardwareUnit) {
    try {
      await restoreUnit(unit.id)
      showSnackbar(`Restored unit "${unit.barcode}".`, 'success')
      load()
    } catch {
      showSnackbar('Failed to restore unit.', 'error')
    }
  }

  async function handlePurge(unit: HardwareUnit) {
    const ok = await confirm({
      title: `Permanently delete "${unit.barcode}"?`,
      message:
        'This cannot be undone. The unit and all its related records (configuration, shipment, installation, acceptance, defects) will be deleted forever.',
      confirmLabel: 'Permanently Delete',
      danger: true,
    })
    if (!ok) return
    try {
      await purgeUnit(unit.id)
      showSnackbar(`Permanently deleted unit "${unit.barcode}".`, 'success')
      load()
    } catch {
      showSnackbar('Failed to permanently delete unit.', 'error')
    }
  }

  return (
    <section className="detail-section">
      <div className="section-toolbar">
        <h2>Retired Units</h2>
        <p className="modal-subtitle">
          Units that have been deleted are retired, not removed, so their history stays intact. Restore them back to
          active, or permanently delete them if they're no longer needed.
        </p>
      </div>

      {loading && <p className="board-empty">Loading…</p>}
      {!loading && (
        <DataTable
          rows={units}
          getRowKey={(u) => u.id}
          searchText={(u) => `${u.barcode} ${u.alias ?? ''} ${u.serial_number ?? ''}`}
          query={query}
          onQueryChange={setQuery}
          emptyMessage="No retired units."
          columns={[
            {
              key: 'barcode',
              header: 'Barcode',
              render: (u) => (
                <button type="button" className="link-button" onClick={() => navigate(`/units/${u.id}`)}>
                  {u.barcode}
                </button>
              ),
            },
            { key: 'alias', header: 'Alias', render: (u) => u.alias ?? '—' },
            { key: 'serial_number', header: 'Serial Number', render: (u) => u.serial_number ?? '—' },
            { key: 'device_make', header: 'Device Make', render: (u) => u.device_make ?? '—' },
            { key: 'device_model', header: 'Device Model', render: (u) => u.device_model ?? '—' },
            {
              key: 'actions',
              header: '',
              render: (u) => (
                <>
                  <button type="button" className="link-button" onClick={() => handleRestore(u)}>
                    Restore
                  </button>
                  {' · '}
                  <button type="button" className="link-button" onClick={() => handlePurge(u)}>
                    Permanently Delete
                  </button>
                </>
              ),
            },
          ]}
        />
      )}
    </section>
  )
}

function DataManagementTab() {
  const confirm = useConfirm()
  const showSnackbar = useSnackbar()
  const [exporting, setExporting] = useState(false)
  const [wiping, setWiping] = useState(false)
  const [hasExported, setHasExported] = useState(false)
  const [phrase, setPhrase] = useState('')

  async function handleExport() {
    setExporting(true)
    try {
      await downloadTransactionDataExport()
      setHasExported(true)
      showSnackbar('Transaction data exported.', 'success')
    } catch {
      showSnackbar('Failed to export transaction data.', 'error')
    } finally {
      setExporting(false)
    }
  }

  async function handleWipe() {
    if (phrase !== WIPE_CONFIRMATION_PHRASE) return

    const ok = await confirm({
      title: 'Permanently delete ALL transaction data?',
      message:
        'This deletes every hardware unit, receiving, configuration, shipment, installation, acceptance, and defect record, plus the entire audit trail. This cannot be undone. Reference data (users, roles, site locations) is not affected.',
      confirmLabel: 'Permanently Delete Everything',
      danger: true,
    })
    if (!ok) return

    setWiping(true)
    try {
      await wipeAllTransactionData()
      showSnackbar('All transaction data has been permanently deleted.', 'success')
      setPhrase('')
      setHasExported(false)
    } catch {
      showSnackbar('Failed to wipe transaction data.', 'error')
    } finally {
      setWiping(false)
    }
  }

  return (
    <section className="detail-section">
      <div className="section-toolbar">
        <h2>Data Management</h2>
        <p className="modal-subtitle">
          Permanently clear all transaction data — hardware units and every record tied to them (receiving,
          configuration, shipments, installations, acceptances, defects, delivery dockets) — along with the full
          audit trail. This is irreversible. Export a JSON backup first if you want to keep a copy.
        </p>
      </div>

      <div className="form-grid">
        <button type="button" className="btn" onClick={handleExport} disabled={exporting}>
          {exporting ? 'Exporting…' : 'Export All Transaction Data (JSON)'}
        </button>
      </div>

      <div className="danger-zone">
        <h3>Danger Zone</h3>

        {!hasExported && <p className="board-empty">Recommended: export a backup above before deleting.</p>}

        <div className="danger-zone-field">
          <label>
            Type <strong>{WIPE_CONFIRMATION_PHRASE}</strong> to enable permanent deletion
          </label>
          <input
            type="text"
            value={phrase}
            onChange={(e) => setPhrase(e.target.value)}
            placeholder={WIPE_CONFIRMATION_PHRASE}
          />
        </div>

        <button
          type="button"
          className="btn btn-danger"
          onClick={handleWipe}
          disabled={phrase !== WIPE_CONFIRMATION_PHRASE || wiping}
        >
          {wiping ? 'Deleting…' : 'Permanently Delete All Transaction Data'}
        </button>
      </div>
    </section>
  )
}
