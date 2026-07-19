import { useEffect } from 'react'

// Input types where forcing uppercase would be wrong or meaningless.
const EXCLUDED_TYPES = new Set([
  'checkbox',
  'radio',
  'password',
  'email',
  'number',
  'date',
  'datetime-local',
  'time',
  'month',
  'week',
  'file',
  'range',
  'color',
  'hidden',
  'url',
])

// Writing straight to `target.value` goes through React's per-instance
// value-tracker override, which updates its cached "last known value" as a
// side effect. If we did that, React's own change-detection (which compares
// the tracker's cached value against the live DOM value) would see no
// difference and silently swallow the onChange event — the text would look
// uppercase (direct DOM mutation) but React state would never update. Using
// the native prototype setter bypasses that per-instance override, so the
// tracker stays "stale" and React still detects and fires the change.
const nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set
const nativeTextareaValueSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')?.set

// Forces every text input and textarea in the app to uppercase as the user
// types, app-wide, without touching each form individually. Runs in the
// capture phase so the DOM value is already uppercase by the time React's
// own onChange handlers read event.target.value.
export function useGlobalUppercaseInputs() {
  useEffect(() => {
    function handleInput(e: Event) {
      const target = e.target
      if (!(target instanceof HTMLInputElement) && !(target instanceof HTMLTextAreaElement)) return
      if (target instanceof HTMLInputElement && EXCLUDED_TYPES.has(target.type)) return
      if (target.dataset.noUppercase !== undefined) return

      const upper = target.value.toUpperCase()
      if (upper === target.value) return

      const setter = target instanceof HTMLTextAreaElement ? nativeTextareaValueSetter : nativeInputValueSetter
      const { selectionStart, selectionEnd } = target
      if (setter) {
        setter.call(target, upper)
      } else {
        target.value = upper
      }
      if (selectionStart !== null && selectionEnd !== null) {
        target.setSelectionRange(selectionStart, selectionEnd)
      }
    }

    document.addEventListener('input', handleInput, true)
    return () => document.removeEventListener('input', handleInput, true)
  }, [])
}
