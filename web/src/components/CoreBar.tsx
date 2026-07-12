import { getUsageColor, USAGE_COLOR_HEX } from "@/lib/format"
import { formatPercent } from "@/lib/format"

interface CoreBarProps {
  label: string
  value: number
  ariaLabel: string
}

export function CoreBar({ label, value, ariaLabel }: CoreBarProps) {
  const color = getUsageColor(value)
  const colorHex = USAGE_COLOR_HEX[color]

  return (
    <div
      className="flex items-start gap-1.5"
      role="progressbar"
      aria-label={ariaLabel}
      aria-valuenow={Math.round(value)}
      aria-valuemin={0}
      aria-valuemax={100}
      data-color={color}
    >
      <div className="w-6 h-9 rounded-sm bg-muted overflow-hidden flex items-end">
        <div
          className="w-full rounded-sm transition-all"
          style={{
            height: `${Math.max(0, Math.min(100, value))}%`,
            backgroundColor: colorHex,
          }}
        />
      </div>
      <div className="flex flex-col justify-between h-9">
        <span className="text-xs leading-none text-muted-foreground">{label}</span>
        <span className="text-xs leading-none font-medium tabular-nums">{formatPercent(value)}</span>
      </div>
    </div>
  )
}
