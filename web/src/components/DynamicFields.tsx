import type { FieldDefinition } from '../api/admin'

export type MetaValues = Record<string, string | boolean>

export function defaultMetaValues(fields: FieldDefinition[], existing?: Record<string, unknown>): MetaValues {
  const values: MetaValues = {}
  for (const f of fields) {
    if (f.data_type === 'boolean') {
      values[f.field_key] = Boolean(existing?.[f.field_key])
    } else {
      values[f.field_key] = (existing?.[f.field_key] as string) ?? ''
    }
  }
  return values
}

// Renders admin-defined custom fields for a stage: a checkbox for booleans,
// a text/number input otherwise. Used wherever a stage's field definitions
// need to show up on that stage's own form.
export function DynamicFields({
  fields,
  values,
  onChange,
  heading = 'Additional Fields',
  uppercaseText = false,
}: {
  fields: FieldDefinition[]
  values: MetaValues
  onChange: (values: MetaValues) => void
  heading?: string
  uppercaseText?: boolean
}) {
  if (fields.length === 0) return null

  return (
    <>
      <h3 className="dynamic-fields-heading">{heading}</h3>
      <div className="dynamic-fields-grid">
        {fields.map((f) =>
          f.data_type === 'boolean' ? (
            <label key={f.field_key} className="checkbox-row">
              <input
                type="checkbox"
                checked={Boolean(values[f.field_key])}
                onChange={(e) => onChange({ ...values, [f.field_key]: e.target.checked })}
              />
              {f.label}
            </label>
          ) : (
            <div className="form-field" key={f.field_key}>
              <label htmlFor={`meta-${f.field_key}`}>{f.label}</label>
              <input
                id={`meta-${f.field_key}`}
                type={f.data_type === 'number' ? 'number' : 'text'}
                value={(values[f.field_key] as string) ?? ''}
                onChange={(e) => {
                  const value = uppercaseText && f.data_type !== 'number' ? e.target.value.toUpperCase() : e.target.value
                  onChange({ ...values, [f.field_key]: value })
                }}
                required={f.is_required}
              />
            </div>
          ),
        )}
      </div>
    </>
  )
}
