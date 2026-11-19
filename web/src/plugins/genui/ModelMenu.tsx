import React, { useEffect, useRef, useState } from 'react'
import { DropdownMenu } from 'radix-ui'

export interface ModelMenuProps {
  value: string
  models: readonly string[]
  disabled?: boolean
  onChange: (model: string) => void
}

const AGENTS_CHOICE = 'Agent’s choice'

export function ModelMenu({ value, models, disabled = false, onChange }: ModelMenuProps): React.ReactElement {
  const [open, setOpen] = useState(false)
  const isOpen = open && !disabled
  const menuRef = useRef<HTMLDivElement>(null)

  const updateOverflow = () => {
    const menu = menuRef.current
    if (!menu) return
    menu.dataset.moreAbove = String(menu.scrollTop > 2)
    menu.dataset.moreBelow = String(menu.scrollTop + menu.clientHeight < menu.scrollHeight - 2)
  }

  useEffect(() => {
    if (disabled) {
      setOpen(false)
    }
  }, [disabled])

  useEffect(() => {
    if (!isOpen) return
    let observer: ResizeObserver | null = null
    const frame = window.requestAnimationFrame(() => {
      updateOverflow()
      const menu = menuRef.current
      if (!menu) return
      observer = new ResizeObserver(updateOverflow)
      observer.observe(menu)
    })
    return () => {
      window.cancelAnimationFrame(frame)
      observer?.disconnect()
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

  return (
    <span className="sel-wrap">
      <DropdownMenu.Root open={isOpen} onOpenChange={setOpen} modal={false}>
        <DropdownMenu.Trigger asChild>
          <button
            type="button"
            className="sel-btn"
            data-model-menu=""
            data-testid="model-menu-trigger"
            aria-label="Model"
            aria-haspopup="listbox"
            aria-expanded={isOpen}
            disabled={disabled}
          >
            <span>{value || AGENTS_CHOICE}</span>
            <i className="ph ph-caret-down sel-caret" aria-hidden="true" />
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            className="mmenu pop"
            role="listbox"
            aria-label="Model"
            side="top"
            align="end"
            sideOffset={6}
            collisionPadding={8}
            loop
            ref={menuRef}
            onScroll={updateOverflow}
          >
            <div className="mmenu-head">Model</div>
            <DropdownMenu.RadioGroup
              value={value}
              onValueChange={(next) => {
                setOpen(false)
                onChange(next)
              }}
            >
              {options.map((option) => {
                const selected = value === option.value
                return (
                  <DropdownMenu.RadioItem
                    key={option.value || '__default__'}
                    value={option.value}
                    role="option"
                    aria-selected={selected}
                    aria-checked={undefined}
                    className={selected ? 'mi on' : 'mi'}
                    data-model={option.value}
                  >
                    <i
                      className={`ph ${option.value ? 'ph-cpu' : 'ph-sparkle'} mi-icon`}
                      aria-hidden="true"
                    />
                    <span className={option.value ? 't mono' : 't'}>{option.label}</span>
                    {selected ? <i className="ph ph-check mi-check" aria-hidden="true" /> : null}
                  </DropdownMenu.RadioItem>
                )
              })}
            </DropdownMenu.RadioGroup>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </span>
  )
}
