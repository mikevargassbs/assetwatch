import { useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'

export interface TabItem {
  key: string
  label: string
  // Optional per-tab status dot — 'done' (stage complete) or 'attention'
  // (needs a look, e.g. an open defect). Omit for tabs with no such concept
  // (Attributes, Audit History).
  status?: 'done' | 'attention'
}

interface TabsProps {
  tabs: TabItem[]
  active: string
  onChange: (key: string) => void
}

// A sliding-indicator tab bar (underline style), inspired by
// https://smoothui.dev/docs/components/animated-tabs — reimplemented with
// plain CSS transitions instead of a motion library, since none is used
// elsewhere in this app. Supports click and roving-tabindex arrow/Home/End
// keyboard navigation, and respects prefers-reduced-motion.
export function Tabs({ tabs, active, onChange }: TabsProps) {
  const listRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({})
  const [indicator, setIndicator] = useState<{ left: number; width: number }>({ left: 0, width: 0 })

  useLayoutEffect(() => {
    function measure() {
      const el = tabRefs.current[active]
      const list = listRef.current
      if (!el || !list) return
      const listRect = list.getBoundingClientRect()
      const elRect = el.getBoundingClientRect()
      setIndicator({ left: elRect.left - listRect.left, width: elRect.width })
    }
    measure()
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [active, tabs])

  function handleKeyDown(e: KeyboardEvent<HTMLButtonElement>, index: number) {
    let nextIndex: number
    if (e.key === 'ArrowRight') nextIndex = (index + 1) % tabs.length
    else if (e.key === 'ArrowLeft') nextIndex = (index - 1 + tabs.length) % tabs.length
    else if (e.key === 'Home') nextIndex = 0
    else if (e.key === 'End') nextIndex = tabs.length - 1
    else return
    e.preventDefault()
    const next = tabs[nextIndex]
    onChange(next.key)
    tabRefs.current[next.key]?.focus()
  }

  return (
    <div className="tabs" role="tablist" ref={listRef}>
      {tabs.map((tab, index) => (
        <button
          key={tab.key}
          ref={(el) => {
            tabRefs.current[tab.key] = el
          }}
          type="button"
          role="tab"
          aria-selected={active === tab.key}
          tabIndex={active === tab.key ? 0 : -1}
          className={`tabs-item${active === tab.key ? ' tabs-item-active' : ''}`}
          onClick={() => onChange(tab.key)}
          onKeyDown={(e) => handleKeyDown(e, index)}
        >
          {tab.status && <span className={`tabs-item-dot tabs-item-dot-${tab.status}`} aria-hidden="true" />}
          {tab.label}
        </button>
      ))}
      <span className="tabs-indicator" style={{ left: indicator.left, width: indicator.width }} />
    </div>
  )
}
