import React from "react"
import { Link } from "react-router-dom"
import { WifiOff, Loader2, ShieldAlert, KeyRound } from "lucide-react"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import { formatPercent, formatNetworkSpeed, getUsageColor } from "@/lib/format"
import type { ServerInfo, ServerMetrics, NetworkInterface, DiskPartition } from "@/hooks/useMonitorState"

interface ServerCardProps {
  server: ServerInfo
  metrics?: ServerMetrics
}

const VIRTUAL_INTERFACE_PATTERNS = [
  /^lo$/,
  /^docker/,
  /^veth/,
  /^br-/,
  /^virbr/,
]

function isPhysicalInterface(iface: NetworkInterface): boolean {
  return !VIRTUAL_INTERFACE_PATTERNS.some((pattern) => pattern.test(iface.name))
}

function getHighestUsagePartition(partitions: DiskPartition[]): DiskPartition | null {
  if (partitions.length === 0) return null
  return partitions.reduce((highest, current) => {
    const highestPct = highest.total > 0 ? (highest.used / highest.total) * 100 : 0
    const currentPct = current.total > 0 ? (current.used / current.total) * 100 : 0
    return currentPct > highestPct ? current : highest
  })
}

function statusVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "connected":
      return "default"
    case "error":
    case "failed":
      return "destructive"
    default:
      return "secondary"
  }
}

function ColoredProgress({
  value,
  label,
}: {
  value: number
  label: string
}) {
  const color = getUsageColor(value)
  return (
    <Progress
      value={value}
      aria-label={label}
      data-color={color}
    />
  )
}

function getDisconnectedDisplay(server: ServerInfo): {
  icon: React.ReactNode
  message: string
} | null {
  if (server.status === "connected") return null

  if (server.status === "failed") {
    const err = server.lastError ?? ""
    if (err.toLowerCase().includes("host key mismatch")) {
      return {
        icon: <ShieldAlert className="size-8 text-destructive" />,
        message: "Host key mismatch — check configuration",
      }
    }
    return {
      icon: <KeyRound className="size-8 text-destructive" />,
      message: "Auth failed — check configuration",
    }
  }

  // reconnecting
  const attempts = server.attempts ?? 0
  if (attempts === 0) {
    return {
      icon: <Loader2 className="size-8 text-muted-foreground animate-spin" />,
      message: "Connecting...",
    }
  }

  return {
    icon: <WifiOff className="size-8 text-muted-foreground" />,
    message: `Reconnecting (${attempts}/∞)`,
  }
}

const ServerCard = React.memo(function ServerCard({ server, metrics }: ServerCardProps) {
  const disconnected = getDisconnectedDisplay(server)

  const cpuPercent = metrics?.cpu.aggregate.usagePercent
  const memPercent = metrics?.memory
    ? (metrics.memory.used / metrics.memory.total) * 100
    : undefined

  const highestDisk = metrics?.disk ? getHighestUsagePartition(metrics.disk.partitions) : null
  const diskPercent = highestDisk && highestDisk.total > 0
    ? (highestDisk.used / highestDisk.total) * 100
    : undefined

  const physicalInterfaces = metrics?.network
    ? metrics.network.interfaces.filter(isPhysicalInterface)
    : []
  const aggregatedRx = physicalInterfaces.reduce((sum, i) => sum + i.rxBytesPerSec, 0)
  const aggregatedTx = physicalInterfaces.reduce((sum, i) => sum + i.txBytesPerSec, 0)
  const hasNetwork = metrics?.network !== undefined

  return (
    <Link
      to={`/server/${server.id}`}
      className="block focus:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-xl"
      aria-label={server.name}
    >
      <Card data-testid="server-card" className="hover:shadow-md transition-shadow cursor-pointer">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>{server.name}</CardTitle>
            <Badge variant={statusVariant(server.status)}>{server.status}</Badge>
          </div>
          <CardDescription>{server.host}</CardDescription>
        </CardHeader>

        {disconnected ? (
          <CardContent className="min-h-[140px] flex flex-col items-center justify-center text-center gap-2">
            {disconnected.icon}
            <p className="text-sm text-muted-foreground">{disconnected.message}</p>
          </CardContent>
        ) : metrics ? (
          <CardContent className="space-y-3">
            {/* CPU */}
            {cpuPercent !== undefined && (
              <div className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span>CPU</span>
                  <span>{formatPercent(cpuPercent)}</span>
                </div>
                <ColoredProgress value={cpuPercent} label="CPU usage" />
              </div>
            )}

            {/* Memory */}
            {memPercent !== undefined && (
              <div className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span>Memory</span>
                  <span>{formatPercent(memPercent)}</span>
                </div>
                <ColoredProgress value={Math.round(memPercent)} label="Memory usage" />
              </div>
            )}

            {/* Disk */}
            {highestDisk && diskPercent !== undefined && (
              <div className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span>Disk <span className="text-muted-foreground">({highestDisk.mountPoint})</span></span>
                  <span>{formatPercent(diskPercent)}</span>
                </div>
                <ColoredProgress value={Math.round(diskPercent)} label="Disk usage" />
              </div>
            )}

            {/* Network */}
            {hasNetwork && (
              <div className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span>Network</span>
                  <span>
                    <span aria-label="receive">↓ {formatNetworkSpeed(aggregatedRx)}</span>
                    {" "}
                    <span aria-label="transmit">↑ {formatNetworkSpeed(aggregatedTx)}</span>
                  </span>
                </div>
              </div>
            )}
          </CardContent>
        ) : null}
      </Card>
    </Link>
  )
})

export { ServerCard }
export type { ServerCardProps }
