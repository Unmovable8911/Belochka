import { getUsageColor, formatPercent, USAGE_COLOR_HEX } from "@/lib/format"

interface RingGaugeProps {
  value: number
  testId?: string
}

export function RingGauge({ value, testId }: RingGaugeProps) {
  const color = getUsageColor(value)
  const colorHex = USAGE_COLOR_HEX[color]
  const pct = value.toFixed(1)
  return (
    <div
      data-testid={testId}
      data-color={color}
      className="relative flex items-center justify-center rounded-full size-32 shrink-0 transition-all duration-300"
      style={{
        background: `conic-gradient(${colorHex} ${pct}%, var(--gauge-track) ${pct}%)`,
        boxShadow: `0 0 20px ${colorHex}40`,
      }}
    >
      <div className="flex items-center justify-center rounded-full size-24 bg-background">
        <span className="text-lg font-bold font-mono tabular-nums">{formatPercent(value)}</span>
      </div>
    </div>
  )
}
