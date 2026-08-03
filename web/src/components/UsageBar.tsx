import type { ReactNode } from "react"
import { getUsageColor, USAGE_COLOR_HEX } from "@/lib/format"

interface UsageBarProps {
  label: ReactNode
  value: number
  rightText: ReactNode
  ariaLabel: string
}

export function UsageBar({ label, value, rightText, ariaLabel }: UsageBarProps) {
  const color = getUsageColor(value)
  const colorHex = USAGE_COLOR_HEX[color]
  return (
    <div>
      <div className="flex items-center justify-between gap-2 text-sm mb-1">
        <span>{label}</span>
        <span className="whitespace-nowrap">{rightText}</span>
      </div>
      <div
        className="h-2 w-full rounded-full bg-muted overflow-hidden"
        role="progressbar"
        aria-label={ariaLabel}
        aria-valuenow={Math.round(value)}
        aria-valuemin={0}
        aria-valuemax={100}
        data-color={color}
      >
        <div
          className="h-full rounded-full transition-all duration-300"
          style={{
            width: `${value}%`,
            background: `linear-gradient(90deg, ${colorHex}cc, ${colorHex})`,
          }}
        />
      </div>
    </div>
  )
}
