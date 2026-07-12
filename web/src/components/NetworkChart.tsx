import { useState, useEffect, useRef } from "react"
import { useTranslation } from "react-i18next"
import type { NetworkInterface } from "@/types/server"
import { formatNetworkSpeed } from "@/lib/format"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

// --- Constants ---

const VIRTUAL_INTERFACE_PATTERNS = [
  /^lo$/,
  /^docker/,
  /^veth/,
  /^br-/,
  /^virbr/,
]

const MAX_HISTORY_MS = 20_000
const SVG_W = 600
const SVG_H = 220
const PAD = { top: 8, right: 24, bottom: 24, left: 60 }
const PLOT_W = SVG_W - PAD.left - PAD.right
const PLOT_H = SVG_H - PAD.top - PAD.bottom

// --- Helpers ---

interface DataPoint {
  timestamp: number
  rx: number
  tx: number
}

function isPhysical(iface: NetworkInterface): boolean {
  return !VIRTUAL_INTERFACE_PATTERNS.some((p) => p.test(iface.name))
}

function niceMax(value: number): number {
  if (value <= 0) return 100
  const exp = Math.floor(Math.log10(value))
  const mag = Math.pow(10, exp)
  const norm = value / mag
  if (norm <= 1) return mag
  if (norm <= 2) return 2 * mag
  if (norm <= 5) return 5 * mag
  return 10 * mag
}

function getYTicks(rawMax: number): number[] {
  const ceil = niceMax(rawMax)
  const step = ceil / 4
  return [0, step, step * 2, step * 3, ceil]
}

// --- Component ---

export function NetworkChart({ interfaces }: { interfaces: NetworkInterface[] }) {
  const { t } = useTranslation()
  const physical = interfaces.filter(isPhysical)

  const [selectedIface, setSelectedIface] = useState<string>("")
  const [history, setHistory] = useState<DataPoint[]>([])
  const lastRecordedSec = useRef(0)
  const prevIfaceRef = useRef<string>("")

  // Ensure a valid selection exists.
  useEffect(() => {
    if (physical.length === 0) return
    if (!physical.find((i) => i.name === selectedIface)) {
      setSelectedIface(physical[0].name)
    }
  }, [physical, selectedIface])

  // Reset history when the selected interface changes.
  useEffect(() => {
    if (prevIfaceRef.current !== selectedIface) {
      prevIfaceRef.current = selectedIface
      setHistory([])
      lastRecordedSec.current = 0
    }
  }, [selectedIface])

  // Record new data points (one per second at most).
  useEffect(() => {
    const iface = physical.find((i) => i.name === selectedIface)
    if (!iface) return

    const sec = Math.floor(Date.now() / 1000)
    if (sec === lastRecordedSec.current) return
    lastRecordedSec.current = sec

    const now = Date.now()
    setHistory((prev) => {
      const cutoff = now - MAX_HISTORY_MS
      const next = [
        ...prev,
        {
          timestamp: now,
          rx: Math.max(0, iface.rxBytesPerSec),
          tx: Math.max(0, iface.txBytesPerSec),
        },
      ]
      return next.filter((p) => p.timestamp >= cutoff)
    })
  }, [physical, selectedIface])

  // --- Chart calculations ---

  const now = Date.now()
  const windowStart = now - MAX_HISTORY_MS
  const allValues = history.flatMap((p) => [p.rx, p.tx])
  const rawMax = Math.max(...allValues, 1)
  const yTicks = getYTicks(rawMax)
  const yCeil = niceMax(rawMax)

  const xToSvg = (ts: number) =>
    PAD.left + ((ts - windowStart) / MAX_HISTORY_MS) * PLOT_W
  const yToSvg = (v: number) =>
    PAD.top + PLOT_H - (v / yCeil) * PLOT_H

  const rxPoints = history
    .map((p) => `${xToSvg(p.timestamp)},${yToSvg(p.rx)}`)
    .join(" ")
  const txPoints = history
    .map((p) => `${xToSvg(p.timestamp)},${yToSvg(p.tx)}`)
    .join(" ")

  const xLabels = [
    { label: "20s", ratio: 0 },
    { label: "15s", ratio: 0.25 },
    { label: "10s", ratio: 0.5 },
    { label: "5s", ratio: 0.75 },
    { label: "0s", ratio: 1 },
  ]

  const current = physical.find((i) => i.name === selectedIface)

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      {/* Current rate indicators + interface selector in same row */}
      <div className="flex items-center justify-between text-xs mb-1 shrink-0">
        <div className="flex gap-4">
          {current && (
            <>
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full bg-[#3b82f6]" />
                RX {formatNetworkSpeed(current.rxBytesPerSec)}
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full bg-[#f97316]" />
                TX {formatNetworkSpeed(current.txBytesPerSec)}
              </span>
            </>
          )}
        </div>
        {physical.length > 0 && (
          <Select value={selectedIface} onValueChange={setSelectedIface}>
            <SelectTrigger size="sm" className="h-6 text-xs px-2 gap-1">
              <SelectValue />
            </SelectTrigger>
            <SelectContent align="end">
              {physical.map((iface) => (
                <SelectItem key={iface.name} value={iface.name}>
                  {iface.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      {/* Chart area */}
      <div className="flex-1 min-h-0 relative">
        <svg
          viewBox={`0 0 ${SVG_W} ${SVG_H}`}
          className="w-full h-full"
          preserveAspectRatio="xMidYMid meet"
          role="img"
          aria-label={`Network traffic chart`}
        >
          {/* Horizontal grid lines + Y-axis labels */}
          {yTicks.map((tick) => {
            const y = yToSvg(tick)
            return (
              <g key={tick}>
                <line
                  x1={PAD.left}
                  y1={y}
                  x2={SVG_W - PAD.right}
                  y2={y}
                  stroke="currentColor"
                  strokeOpacity={0.1}
                />
                <text
                  x={PAD.left - 6}
                  y={y + 4}
                  textAnchor="end"
                  className="fill-current text-[11px]"
                  style={{ fillOpacity: 0.5 }}
                >
                  {formatNetworkSpeed(tick)}
                </text>
              </g>
            )
          })}

          {/* X-axis labels */}
          {xLabels.map((xl) => (
            <text
              key={xl.label}
              x={PAD.left + xl.ratio * PLOT_W}
              y={SVG_H - 6}
              textAnchor="middle"
              className="fill-current text-[11px]"
              style={{ fillOpacity: 0.5 }}
            >
              {xl.label}
            </text>
          ))}

          {/* Baseliine */}
          <line
            x1={PAD.left}
            y1={PAD.top + PLOT_H}
            x2={SVG_W - PAD.right}
            y2={PAD.top + PLOT_H}
            stroke="currentColor"
            strokeOpacity={0.2}
          />

          {/* RX polyline */}
          {rxPoints && (
            <polyline
              points={rxPoints}
              fill="none"
              stroke="#3b82f6"
              strokeWidth="1.5"
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          )}

          {/* TX polyline */}
          {txPoints && (
            <polyline
              points={txPoints}
              fill="none"
              stroke="#f97316"
              strokeWidth="1.5"
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          )}

          {/* Empty state */}
          {history.length === 0 && (
            <text
              x={SVG_W / 2}
              y={SVG_H / 2}
              textAnchor="middle"
              className="fill-current text-xs"
              style={{ fillOpacity: 0.4 }}
            >
              {physical.length === 0 ? t("networkChart.noInterfaces") : t("networkChart.waitingForData")}
            </text>
          )}
        </svg>

      </div>
    </div>
  )
}
