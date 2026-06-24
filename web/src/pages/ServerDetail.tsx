import { useState, useMemo } from "react"
import { useParams, Link, useNavigate } from "react-router-dom"
import { ArrowLeft, ArrowUp, ArrowDown, Trash2 } from "lucide-react"
import { useMonitorState } from "@/hooks/useMonitorState"
import { formatBytes, formatNetworkSpeed, formatPercent, formatUptime, getUsageColor, type UsageColor } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { DeleteServerDialog } from "@/components/DeleteServerDialog"
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table"

const COLOR_MAP: Record<UsageColor, string> = {
  green: "#22c55e",
  yellow: "#eab308",
  red: "#ef4444",
}

type SortColumn = "pid" | "user" | "cpuPct" | "memPct" | "command"
type SortDirection = "asc" | "desc"

const COLUMN_HEADERS: { key: SortColumn; label: string }[] = [
  { key: "pid", label: "PID" },
  { key: "user", label: "User" },
  { key: "cpuPct", label: "CPU%" },
  { key: "memPct", label: "Memory%" },
  { key: "command", label: "Command" },
]

export default function ServerDetail() {
  const { id } = useParams<{ id: string }>()
  const { state, dispatch } = useMonitorState()
  const navigate = useNavigate()

  const [sortColumn, setSortColumn] = useState<SortColumn>("cpuPct")
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc")
  const [deleteOpen, setDeleteOpen] = useState(false)

  const server = state.servers.find((s) => s.id === id)
  const metrics = id ? state.metrics[id] : undefined

  const sortedProcesses = useMemo(() => {
    const procs = metrics?.process?.processes ?? []
    const top20 = procs.slice(0, 20)
    return [...top20].sort((a, b) => {
      const aVal = a[sortColumn]
      const bVal = b[sortColumn]
      if (typeof aVal === "string" && typeof bVal === "string") {
        return sortDirection === "asc"
          ? aVal.localeCompare(bVal)
          : bVal.localeCompare(aVal)
      }
      const aNum = aVal as number
      const bNum = bVal as number
      return sortDirection === "asc" ? aNum - bNum : bNum - aNum
    })
  }, [metrics?.process?.processes, sortColumn, sortDirection])

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

      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">{server.name}</h1>
        <Button
          variant="destructive"
          size="sm"
          className="cursor-pointer hover:brightness-110 hover:scale-105 transition-all"
          onClick={() => setDeleteOpen(true)}
        >
          <Trash2 className="size-4 mr-1" />
          Delete
        </Button>
      </div>

      <DeleteServerDialog
        server={server}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={(serverId) => {
          dispatch({ type: "remove_server", data: { serverId } })
          navigate("/")
        }}
      />

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

      {/* 2x2 Grid: CPU | Memory over Disk | Network */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6" data-testid="metrics-grid">
        {/* CPU Section */}
        {metrics?.cpu && (
          <div className="rounded-lg border bg-card p-4">
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

        {/* Memory Section */}
        {metrics?.memory && (
          <div className="rounded-lg border bg-card p-4">
            <h2 className="text-lg font-semibold mb-4">Memory</h2>
            <div className="flex flex-col items-center gap-4">
              {(() => {
                const mem = metrics.memory
                const pct = mem.total > 0 ? (mem.used / mem.total) * 100 : 0
                const color = getUsageColor(pct)
                const colorHex = COLOR_MAP[color]
                return (
                  <>
                    <div
                      data-testid="memory-ring-gauge"
                      data-color={color}
                      className="relative flex items-center justify-center rounded-full size-32 shrink-0"
                      style={{
                        background: `conic-gradient(${colorHex} ${pct.toFixed(1)}%, #e5e7eb ${pct.toFixed(1)}%)`,
                      }}
                    >
                      <div className="flex items-center justify-center rounded-full size-24 bg-background">
                        <span className="text-lg font-bold">{formatPercent(pct)}</span>
                      </div>
                    </div>
                    <div className="text-sm text-center">
                      <span>{formatBytes(mem.used)} / {formatBytes(mem.total)}</span>
                    </div>
                    {mem.swapTotal > 0 && (
                      <div className="text-sm text-muted-foreground text-center" data-testid="swap-info">
                        Swap: {formatBytes(mem.swapUsed)} / {formatBytes(mem.swapTotal)}
                      </div>
                    )}
                  </>
                )
              })()}
            </div>
          </div>
        )}

        {/* Disk Section */}
        {metrics?.disk && (
          <div className="rounded-lg border bg-card p-4">
            <h2 className="text-lg font-semibold mb-4">Disk</h2>
            <div className="space-y-3">
              {metrics.disk.partitions.map((partition) => {
                const pct = partition.total > 0 ? (partition.used / partition.total) * 100 : 0
                const color = getUsageColor(pct)
                const colorHex = COLOR_MAP[color]
                return (
                  <div key={partition.mountPoint}>
                    <div className="flex justify-between text-sm mb-1">
                      <span>{partition.mountPoint}</span>
                      <span>{formatBytes(partition.used)} / {formatBytes(partition.total)}</span>
                    </div>
                    <div
                      className="h-2 w-full rounded-full bg-muted overflow-hidden"
                      role="progressbar"
                      aria-label={`${partition.mountPoint} usage`}
                      aria-valuenow={Math.round(pct)}
                      aria-valuemin={0}
                      aria-valuemax={100}
                      data-color={color}
                    >
                      <div
                        className="h-full rounded-full transition-all"
                        style={{
                          width: `${pct}%`,
                          backgroundColor: colorHex,
                        }}
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {/* Network Section */}
        {metrics?.network && (
          <div className="rounded-lg border bg-card p-4" data-testid="network-section">
            <h2 className="text-lg font-semibold mb-4">Network</h2>
            <div className="space-y-3">
              {metrics.network.interfaces.map((iface) => (
                <div key={iface.name} className="flex items-center justify-between text-sm">
                  <span className="font-medium">{iface.name}</span>
                  <div className="flex gap-4">
                    <span>RX: {formatNetworkSpeed(iface.rxBytesPerSec)}</span>
                    <span>TX: {formatNetworkSpeed(iface.txBytesPerSec)}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Process Table */}
      {metrics?.process && (
        <div className="mt-6" data-testid="process-section">
          <h2 className="text-lg font-semibold mb-4">Processes</h2>
          {sortedProcesses.length === 0 ? (
            <p className="text-sm text-muted-foreground">No process data available.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  {COLUMN_HEADERS.map(({ key, label }) => (
                    <TableHead
                      key={key}
                      className="cursor-pointer select-none"
                      onClick={() => {
                        if (sortColumn === key) {
                          setSortDirection((d) => (d === "asc" ? "desc" : "asc"))
                        } else {
                          setSortColumn(key)
                          setSortDirection(key === "user" || key === "command" ? "asc" : "desc")
                        }
                      }}
                    >
                      <span className="inline-flex items-center gap-1">
                        {label}
                        {sortColumn === key && (
                          sortDirection === "asc"
                            ? <ArrowUp className="size-3" />
                            : <ArrowDown className="size-3" />
                        )}
                      </span>
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedProcesses.map((proc) => (
                  <TableRow key={proc.pid}>
                    <TableCell>{proc.pid}</TableCell>
                    <TableCell>{proc.user}</TableCell>
                    <TableCell>{formatPercent(proc.cpuPct)}</TableCell>
                    <TableCell>{formatPercent(proc.memPct)}</TableCell>
                    <TableCell>{proc.command}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      )}
    </div>
  )
}
