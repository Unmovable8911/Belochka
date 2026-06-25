import { useState } from "react"
import { useParams, Link, useNavigate } from "react-router-dom"
import { ArrowLeft, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useMonitorState } from "@/hooks/useMonitorState"
import { formatBytes, formatNetworkSpeed, formatPercent, formatUptime } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { DeleteServerDialog } from "@/components/DeleteServerDialog"
import { RingGauge } from "@/components/RingGauge"
import { UsageBar } from "@/components/UsageBar"
import { ProcessTable } from "@/components/ProcessTable"

export default function ServerDetail() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const { state, dispatch } = useMonitorState()
  const navigate = useNavigate()

  const [deleteOpen, setDeleteOpen] = useState(false)

  const server = state.servers.find((s) => s.id === id)
  const metrics = id ? state.metrics[id] : undefined

  if (!server) {
    return (
      <div className="p-6">
        <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
          <ArrowLeft className="size-4" />
          {t("serverDetail.backToDashboard")}
        </Link>
        <h1 className="text-2xl font-bold">{t("serverDetail.notFound")}</h1>
        <p className="text-muted-foreground">{t("serverDetail.notFoundHint")}</p>
      </div>
    )
  }

  const system = metrics?.system

  return (
    <div className="p-6">
      <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
        <ArrowLeft className="size-4" />
        {t("serverDetail.backToDashboard")}
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
          {t("serverDetail.delete")}
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

      {system && (
        <div className="flex flex-wrap gap-6 mb-8 rounded-lg border bg-card p-4" data-testid="system-info-bar">
          <div>
            <div className="text-xs text-muted-foreground">{t("serverDetail.hostname")}</div>
            <div className="text-sm font-medium">{system.hostname}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{t("serverDetail.kernel")}</div>
            <div className="text-sm font-medium">{system.kernel}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{t("serverDetail.uptime")}</div>
            <div className="text-sm font-medium">{formatUptime(system.uptimeSec)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{t("serverDetail.os")}</div>
            <div className="text-sm font-medium">{system.osName}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{t("serverDetail.cores")}</div>
            <div className="text-sm font-medium">{system.coreCount} cores</div>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6" data-testid="metrics-grid">
        {metrics?.cpu && (
          <div className="rounded-lg border bg-card p-4">
            <h2 className="text-lg font-semibold mb-4">{t("serverDetail.cpu")}</h2>
            <div className="flex flex-col items-center gap-6 md:flex-row md:items-start">
              <RingGauge value={metrics.cpu.aggregate.usagePercent} testId="cpu-ring-gauge" />
              <div className="flex-1 w-full space-y-2">
                {metrics.cpu.cores.map((core, index) => (
                  <UsageBar
                    key={core.name ?? index}
                    label={`Core ${index}`}
                    value={core.usagePercent}
                    rightText={formatPercent(core.usagePercent)}
                    ariaLabel={`Core ${index} usage`}
                  />
                ))}
              </div>
            </div>
          </div>
        )}

        {metrics?.memory && (
          <div className="rounded-lg border bg-card p-4">
            <h2 className="text-lg font-semibold mb-4">{t("serverDetail.memory")}</h2>
            <div className="flex flex-col items-center gap-4">
              <RingGauge
                value={metrics.memory.total > 0 ? (metrics.memory.used / metrics.memory.total) * 100 : 0}
                testId="memory-ring-gauge"
              />
              <div className="text-sm text-center">
                <span>{formatBytes(metrics.memory.used)} / {formatBytes(metrics.memory.total)}</span>
              </div>
              {metrics.memory.swapTotal > 0 && (
                <div className="text-sm text-muted-foreground text-center" data-testid="swap-info">
                  {t("serverDetail.swap")}: {formatBytes(metrics.memory.swapUsed)} / {formatBytes(metrics.memory.swapTotal)}
                </div>
              )}
            </div>
          </div>
        )}

        {metrics?.disk && (
          <div className="rounded-lg border bg-card p-4">
            <h2 className="text-lg font-semibold mb-4">{t("serverDetail.disk")}</h2>
            <div className="space-y-3">
              {metrics.disk.partitions.map((partition) => {
                const pct = partition.total > 0 ? (partition.used / partition.total) * 100 : 0
                return (
                  <UsageBar
                    key={partition.mountPoint}
                    label={partition.mountPoint}
                    value={pct}
                    rightText={`${formatBytes(partition.used)} / ${formatBytes(partition.total)}`}
                    ariaLabel={`${partition.mountPoint} usage`}
                  />
                )
              })}
            </div>
          </div>
        )}

        {metrics?.network && (
          <div className="rounded-lg border bg-card p-4" data-testid="network-section">
            <h2 className="text-lg font-semibold mb-4">{t("serverDetail.network")}</h2>
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

      {metrics?.process && (
        <div className="mt-6" data-testid="process-section">
          <h2 className="text-lg font-semibold mb-4">{t("serverDetail.processes")}</h2>
          <ProcessTable processes={metrics.process.processes} />
        </div>
      )}
    </div>
  )
}
