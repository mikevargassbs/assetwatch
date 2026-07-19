import { useEffect, useRef, useState } from 'react'

interface ComboboxProps {
  id?: string
  value: string
  onChange: (value: string) => void
  options: string[]
  placeholder?: string
  uppercase?: boolean
}

// A text input with a filtered suggestion dropdown drawn from `options`.
// Free typing is always accepted — selecting a suggestion just fills the
// value, it doesn't restrict what can be submitted.
export function Combobox({ id, value, onChange, options, placeholder, uppercase = false }: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const filtered = options.filter((o) => o.toLowerCase().includes(value.toLowerCase()))

  return (
    <div className="combobox" ref={containerRef}>
      <input
        id={id}
        value={value}
        placeholder={placeholder}
        autoComplete="off"
        onChange={(e) => {
          onChange(uppercase ? e.target.value.toUpperCase() : e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
      />
      {open && filtered.length > 0 && (
        <ul className="combobox-menu">
          {filtered.map((option) => (
            <li key={option}>
              <button
                type="button"
                onClick={() => {
                  onChange(option)
                  setOpen(false)
                }}
              >
                {option}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
