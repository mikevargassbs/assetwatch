import { useState, type InputHTMLAttributes } from 'react'

type MaskedInputProps = {
  value: string
  onChange: (value: string) => void
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type'>

export function MaskedInput({ value, onChange, ...rest }: MaskedInputProps) {
  const [revealed, setRevealed] = useState(false)

  return (
    <div className="masked-input">
      <input
        {...rest}
        type={revealed ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <button
        type="button"
        className="masked-input-toggle"
        onClick={() => setRevealed((r) => !r)}
        aria-label={revealed ? 'Hide value' : 'Show value'}
        title={revealed ? 'Hide value' : 'Show value'}
      >
        {revealed ? 'Hide' : 'Show'}
      </button>
    </div>
  )
}
