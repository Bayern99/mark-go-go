import type { ReactElement, ReactNode } from 'react'
import { Tooltip } from 'radix-ui'

export interface AppTooltipProps {
  label: ReactNode
  children: ReactElement
  disabled?: boolean
}

export function AppTooltip({ label, children, disabled = false }: AppTooltipProps): ReactElement {
  if (disabled || label === null || label === undefined || label === '') {
    return children
  }

  return (
    <Tooltip.Root>
      <Tooltip.Trigger asChild>{children}</Tooltip.Trigger>
      <Tooltip.Portal>
        <Tooltip.Content className="mgo-tooltip" sideOffset={6} collisionPadding={8}>
          {label}
          <Tooltip.Arrow className="mgo-tooltip-arrow" width={10} height={5} />
        </Tooltip.Content>
      </Tooltip.Portal>
    </Tooltip.Root>
  )
}
