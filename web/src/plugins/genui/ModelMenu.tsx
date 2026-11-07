import React, { useEffect, useRef, useState } from 'react'

export interface ModelMenuProps {
  value: string
  models: readonly string[]
  disabled?: boolean
  onChange: (model: string) => void
}

const AGENTS_CHOICE = 'Agent’s choice'

export function ModelMenu({ value, models, disabled = false, onChange }: ModelMenuProps): React.ReactElement {
  const [open, setOpen] = useState(false)
  const wrapRef = useRef<HTMLSpanElement | null>(null)
  const isOpen = open && !disabled

  useEffect(() => {
    if (disabled) {
      setOpen(false)
    }
  }, [disabled])

  useEffect(() => {
    if (!isOpen) return

    const onOutsidePointer = (event: MouseEvent) => {
      const target = event.target as Node | null
      if (wrapRef.current && target && !wrapRef.current.contains(target)) {
        setOpen(false)
      }
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        setOpen(false)
      }
    }

    document.addEventListener('pointerdown', onOutsidePointer)
    document.addEventListener('mousedown', onOutsidePointer)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onOutsidePointer)
      document.removeEventListener('mousedown', onOutsidePointer)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [isOpen])

  const uniqueModels = Array.from(new Set(models.map((m) => m.trim()).filter(Boolean)))
  if (value && !uniqueModels.includes(value)) {
    uniqueModels.push(value)
  }

  const options = [
    { value: '', label: AGENTS_CHOICE },
    ...uniqueModels.map((m) => ({ value: m, label: m })),
  ]

  const select = (next: string) => {
    setOpen(false)
    onChange(next)
  }

  return (
    <span className="sel-wrap" ref={wrapRef}>
      <button
        type="button"
        className="sel-btn"
        data-model-menu=""
        data-testid="model-menu-trigger"
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        disabled={disabled}
        onClick={() => setOpen((prev) => !prev)}
      >
        <span>{value || AGENTS_CHOICE}</span>
        <svg
          className="sel-caret"
          aria-hidden="true"
          width="12"
          height="12"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M4 6l4 4 4-4" />
        </svg>
      </button>
      {isOpen ? (
        <div className="mmenu pop" role="listbox" aria-label="Model">
          {options.map((option) => {
            const selected = value === option.value
            return (
              <button
                key={option.value || '__default__'}
                type="button"
                role="option"
                className={selected ? 'mi on' : 'mi'}
                aria-selected={selected}
                data-model={option.value}
                onClick={() => select(option.value)}
              >
                <span className="t">{option.label}</span>
                {selected ? (
                  <svg
                    className="chk"
                    aria-hidden="true"
                    width="14"
                    height="14"
                    viewBox="0 0 16 16"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M3.5 8.5l3 3 6-6.5" />
                  </svg>
                ) : null}
              </button>
            )
          })}
        </div>
      ) : null}
    </span>
  )
}
