import type { FieldDefinition } from '../api/admin'

// Matches an admin-defined "general" text field meant to hold a link to the
// device's Axis product/support page (e.g. field_key "axis_website").
export function isAxisWebsiteField(field: FieldDefinition): boolean {
  if (field.data_type !== 'text') return false
  const key = field.field_key.toLowerCase()
  const label = field.label.toLowerCase()
  const mentionsAxis = key.includes('axis') || label.includes('axis')
  const mentionsLink = /website|url|link/.test(key) || /website|url|link/.test(label)
  return mentionsAxis && mentionsLink
}

// Axis's own site search reliably finds the product support page for a
// given model/part number, without guessing Axis's product-page URL slug
// (which doesn't always match the model number).
export function axisWebsiteUrl(deviceModel: string, partNumber: string): string {
  const query = [deviceModel, partNumber]
    .map((s) => s.trim())
    .filter(Boolean)
    .join(' ')
  if (!query) return ''
  return `https://www.axis.com/search?q=${encodeURIComponent(query)}`
}
