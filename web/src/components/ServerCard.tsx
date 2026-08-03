import React, { useState } from "react"
import { Link } from "react-router-dom"
import { WifiOff, Loader2, ShieldAlert, KeyRound } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { UsageBar } from "@/components/UsageBar"
import { formatPercent, formatNetworkSpeed } from "@/lib/format"
import * as api from "@/api/client"
import { toast } from "sonner"
import type { ServerInfo, ServerMetrics, DiskPartition } from "@/types/server"

interface ServerCardProps {
  server: ServerInfo
  metrics?: ServerMetrics
  /** Optional right-click handler for the card surface. The card stays
      presentational: it forwards its own event and server, and menu
      construction lives in the parent. */
  onContextMenu?: (e: React.MouseEvent, server: ServerInfo) => void
}
function getRootPartition(partitions: DiskPartition[]): DiskPartition | null {
  return partitions.find((p) => p.mountPoint === "/") ?? null
}

function statusVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "connected":
      return "default"
    case "error":
    case "failed":
      return "destructive"
    case "reconnecting":
      return "secondary"
    default:
      return "secondary"
  }
}

function statusDotColor(status: string): string {
  switch (status) {
    case "connected":
      return "bg-emerald-500"
    case "failed":
    case "error":
      return "bg-red-500"
    default:
      return "bg-amber-500"
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

const ServerCard = React.memo(function ServerCard({ server, metrics, onContextMenu }: ServerCardProps) {
  const { t } = useTranslation()
  const [reconnecting, setReconnecting] = useState(false)
  const disconnected = getDisconnectedDisplay(server, t)

  async function handleReconnect(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    setReconnecting(true)
    try {
      await api.reconnectServer(server.id)
    } catch {
      toast.error(t("serverCard.reconnectFailed"))
    } finally {
      setReconnecting(false)
    }
  }

  const cpuPercent = metrics?.aggregate?.usagePercent
  const memPercent = metrics?.memory
    ? (metrics.memory.used / metrics.memory.total) * 100
    : undefined

  const rootDisk = metrics?.disk ? getRootPartition(metrics.disk.partitions) : null
  const diskPercent = rootDisk && rootDisk.total > 0
    ? (rootDisk.used / rootDisk.total) * 100
    : undefined

  const interfaces = metrics?.network?.interfaces ?? []
  const aggregatedRx = interfaces.reduce((sum, i) => sum + i.rxBytesPerSec, 0)
  const aggregatedTx = interfaces.reduce((sum, i) => sum + i.txBytesPerSec, 0)
  const hasNetwork = metrics?.network !== undefined

  return (
    <Link
      to={`/server/${server.id}`}
      className="block focus:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-xl"
      aria-label={server.name}
      onContextMenu={(e) => {
        if (onContextMenu) {
          e.preventDefault()
          e.stopPropagation()
          onContextMenu(e, server)
        }
      }}
    >
      <Card data-testid="server-card" className="h-[300px] hover:shadow-lg hover:shadow-primary/5 hover:border-primary/30 transition-all duration-200 cursor-pointer group">
        <CardHeader>
          <div className="flex items-center justify-between gap-2 min-w-0">
            <CardTitle className="truncate">{server.name}</CardTitle>
            <Badge variant={statusVariant(server.status)} className="gap-1.5">
              <span className={`size-1.5 rounded-full ${statusDotColor(server.status)}`} />
              {server.status}
            </Badge>
          </div>
          <CardDescription className="truncate">{server.host}</CardDescription>
        </CardHeader>

        {disconnected ? (
          <CardContent className={`flex flex-1 flex-col items-center justify-center text-center gap-2 border-l-4 ${
            server.status === "failed" ? "border-l-destructive" : "border-l-amber-500"
          }`}>
            {disconnected.icon}
            <p className="text-sm text-muted-foreground">{disconnected.message}</p>
            {server.status === "failed" && (
              <Button
                variant="outline"
                size="sm"
                disabled={reconnecting}
                onClick={handleReconnect}
                className="mt-1"
              >
                {reconnecting && <Loader2 className="size-3 animate-spin" />}
                {t("serverCard.reconnect")}
              </Button>
            )}
          </CardContent>
        ) : metrics ? (
          <CardContent className="flex flex-1 flex-col justify-center space-y-3">
            {cpuPercent !== undefined && (
              <UsageBar label={t("serverCard.cpu")} value={cpuPercent} rightText={formatPercent(cpuPercent)} ariaLabel="CPU usage" />
            )}

            {memPercent !== undefined && (
              <UsageBar label={t("serverCard.memory")} value={memPercent} rightText={formatPercent(memPercent)} ariaLabel="Memory usage" />
            )}

            {rootDisk && diskPercent !== undefined && (
              <UsageBar
                label={t("serverCard.disk")}
                value={diskPercent}
                rightText={formatPercent(diskPercent)}
                ariaLabel="Disk usage"
              />
            )}

            {hasNetwork && (
              <div className="flex items-center justify-between gap-2 text-sm">
                <span>{t("serverCard.network")}</span>
                <span className="font-mono tabular-nums whitespace-nowrap">
                  <span aria-label="receive">↓ {formatNetworkSpeed(aggregatedRx)}</span>
                  {" "}
                  <span aria-label="transmit">↑ {formatNetworkSpeed(aggregatedTx)}</span>
                </span>
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
