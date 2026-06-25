import React from "react"
import { Link } from "react-router-dom"
import { WifiOff, Loader2, ShieldAlert, KeyRound } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { UsageBar } from "@/components/UsageBar"
import { formatPercent, formatNetworkSpeed } from "@/lib/format"
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

function getDisconnectedDisplay(server: ServerInfo, t: (key: string, opts?: Record<string, unknown>) => string): {
  icon: React.ReactNode
  message: string
} | null {
  if (server.status === "connected") return null

  if (server.status === "failed") {
    const err = server.lastError ?? ""
    if (err.toLowerCase().includes("host key mismatch")) {
      return {
        icon: <ShieldAlert className="size-8 text-destructive" />,
        message: t("serverCard.hostKeyMismatch"),
      }
    }
    return {
      icon: <KeyRound className="size-8 text-destructive" />,
      message: t("serverCard.authFailed"),
    }
  }

  const attempts = server.attempts ?? 0
  if (attempts === 0) {
    return {
      icon: <Loader2 className="size-8 text-muted-foreground animate-spin" />,
      message: t("serverCard.connecting"),
    }
  }

  return {
    icon: <WifiOff className="size-8 text-muted-foreground" />,
    message: t("serverCard.reconnecting", { attempts }),
  }
}

const ServerCard = React.memo(function ServerCard({ server, metrics }: ServerCardProps) {
  const { t } = useTranslation()
  const disconnected = getDisconnectedDisplay(server, t)

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
            {cpuPercent !== undefined && (
              <UsageBar label={t("serverCard.cpu")} value={cpuPercent} rightText={formatPercent(cpuPercent)} ariaLabel="CPU usage" />
            )}

            {memPercent !== undefined && (
              <UsageBar label={t("serverCard.memory")} value={memPercent} rightText={formatPercent(memPercent)} ariaLabel="Memory usage" />
            )}

            {highestDisk && diskPercent !== undefined && (
              <UsageBar
                label={<>{t("serverCard.disk")} <span className="text-muted-foreground">({highestDisk.mountPoint})</span></>}
                value={diskPercent}
                rightText={formatPercent(diskPercent)}
                ariaLabel="Disk usage"
              />
            )}

            {hasNetwork && (
              <div className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span>{t("serverCard.network")}</span>
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
