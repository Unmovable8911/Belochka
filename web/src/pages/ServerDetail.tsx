import { useParams, Link } from "react-router-dom"
import { ArrowLeft } from "lucide-react"
import { useMonitorState } from "@/hooks/useMonitorState"
import { formatPercent, formatUptime, getUsageColor, type UsageColor } from "@/lib/format"

const COLOR_MAP: Record<UsageColor, string> = {
  green: "#22c55e",
  yellow: "#eab308",
  red: "#ef4444",
}

export default function ServerDetail() {
  const { id } = useParams<{ id: string }>()
  const { state } = useMonitorState()

  const server = state.servers.find((s) => s.id === id)
  const metrics = id ? state.metrics[id] : undefined

  if (!server) {
    return (
      <div className="p-6">
        <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
          <ArrowLeft className="size-4" />
          Back to Dashboard
        </Link>
        <h1 className="text-2xl font-bold">Server not found</h1>
        <p className="text-muted-foreground">The server you are looking for does not exist or has been removed.</p>
      </div>
    )
  }

  const system = metrics?.system

  return (
    <div className="p-6">
      <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
        <ArrowLeft className="size-4" />
        Back to Dashboard
      </Link>

      <h1 className="text-2xl font-bold mb-6">{server.name}</h1>

      {/* System Info Bar */}
      {system && (
        <div className="flex flex-wrap gap-6 mb-8 rounded-lg border bg-card p-4" data-testid="system-info-bar">
          <div>
            <div className="text-xs text-muted-foreground">Hostname</div>
            <div className="text-sm font-medium">{system.hostname}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Kernel</div>
            <div className="text-sm font-medium">{system.kernel}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Uptime</div>
            <div className="text-sm font-medium">{formatUptime(system.uptimeSec)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">OS</div>
            <div className="text-sm font-medium">{system.osName}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Cores</div>
            <div className="text-sm font-medium">{system.coreCount} cores</div>
          </div>
        </div>
      )}

      {/* CPU Section */}
      {metrics?.cpu && (
        <div className="mb-8">
          <h2 className="text-lg font-semibold mb-4">CPU</h2>
          <div className="flex flex-col items-center gap-6 md:flex-row md:items-start">
            {/* Ring Gauge */}
            {(() => {
              const pct = metrics.cpu.aggregate.usagePercent
              const color = getUsageColor(pct)
              const colorHex = COLOR_MAP[color]
              return (
                <div
                  data-testid="cpu-ring-gauge"
                  data-color={color}
                  className="relative flex items-center justify-center rounded-full size-32 shrink-0"
                  style={{
                    background: `conic-gradient(${colorHex} ${pct}%, #e5e7eb ${pct}%)`,
                  }}
                >
                  <div className="flex items-center justify-center rounded-full size-24 bg-background">
                    <span className="text-lg font-bold">{formatPercent(pct)}</span>
                  </div>
                </div>
              )
            })()}

            {/* Per-core bars */}
            <div className="flex-1 w-full space-y-2">
              {metrics.cpu.cores.map((core, index) => {
                const coreColor = getUsageColor(core.usagePercent)
                const coreColorHex = COLOR_MAP[coreColor]
                return (
                  <div key={core.name ?? index}>
                    <div className="flex justify-between text-sm mb-1">
                      <span>Core {index}</span>
                      <span>{formatPercent(core.usagePercent)}</span>
                    </div>
                    <div
                      className="h-2 w-full rounded-full bg-muted overflow-hidden"
                      role="progressbar"
                      aria-label={`Core ${index} usage`}
                      aria-valuenow={Math.round(core.usagePercent)}
                      aria-valuemin={0}
                      aria-valuemax={100}
                      data-color={coreColor}
                    >
                      <div
                        className="h-full rounded-full transition-all"
                        style={{
                          width: `${core.usagePercent}%`,
                          backgroundColor: coreColorHex,
                        }}
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
