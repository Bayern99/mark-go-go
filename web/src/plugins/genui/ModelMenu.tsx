import React, { useEffect, useState } from 'react'
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

  useEffect(() => {
    if (disabled) {
      setOpen(false)
    }
  }, [disabled])

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
            sideOffset={4}
            collisionPadding={8}
            loop
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
                    <span className="t">{option.label}</span>
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
