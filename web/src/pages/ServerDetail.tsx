import { useState, useEffect } from "react"
import { useParams, Link, useNavigate } from "react-router-dom"
import { ArrowLeft, Terminal, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useMonitorState } from "@/hooks/useMonitorState"
import { formatBytes, formatNetworkSpeed, formatPercent, formatUptime } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { DeleteServerDialog } from "@/components/DeleteServerDialog"
import { LanguageSwitcher } from "@/components/LanguageSwitcher"
import { RingGauge } from "@/components/RingGauge"
import { UsageBar } from "@/components/UsageBar"
import { ProcessTable } from "@/components/ProcessTable"
import { getCrons } from "@/api/client"
import type { CronResult } from "@/types/server"

type Tab = "overview" | "crons"

export default function ServerDetail() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const { state, dispatch } = useMonitorState()
  const navigate = useNavigate()

  const [deleteOpen, setDeleteOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<Tab>("overview")

  const [cronsLoading, setCronsLoading] = useState(false)
  const [cronsError, setCronsError] = useState<string | null>(null)
  const [cronsResult, setCronsResult] = useState<CronResult | null>(null)
  const [cronsFetched, setCronsFetched] = useState(false)

  const server = state.servers.find((s) => s.id === id)
  const metrics = id ? state.metrics[id] : undefined

  useEffect(() => {
    if (activeTab !== "crons" || !id || cronsFetched) return

    setCronsFetched(true)
    setCronsLoading(true)
    setCronsError(null)

    getCrons(id)
      .then((result) => {
        setCronsResult(result)
      })
      .catch((err: Error) => {
        setCronsError(err.message)
      })
      .finally(() => {
        setCronsLoading(false)
      })
  }, [activeTab, id, cronsFetched])

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
        <div className="flex items-center gap-2">
          <Button
            variant="default"
            size="sm"
            className="cursor-pointer hover:brightness-110 hover:scale-105 transition-all"
            onClick={() => window.open(`/server/${id}/console`, "_blank")}
          >
            <Terminal className="size-4 mr-1" />
            {t("serverDetail.console")}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            className="cursor-pointer hover:brightness-110 hover:scale-105 transition-all"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="size-4 mr-1" />
            {t("serverDetail.delete")}
          </Button>
          <LanguageSwitcher />
        </div>
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

      {/* Tab bar */}
      <div className="flex gap-1 border-b mb-6" role="tablist">
        <button
          role="tab"
          aria-selected={activeTab === "overview"}
          onClick={() => setActiveTab("overview")}
          className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
            activeTab === "overview"
              ? "border-primary text-primary"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          {t("cronJobs.tabOverview")}
        </button>
        <button
          role="tab"
          aria-selected={activeTab === "crons"}
          onClick={() => setActiveTab("crons")}
          className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
            activeTab === "crons"
              ? "border-primary text-primary"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          {t("cronJobs.tabCronJobs")}
        </button>
      </div>

      {/* Overview tab panel */}
      {activeTab === "overview" && (
        <>
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
        </>
      )}

      {/* Cron Jobs tab panel */}
      {activeTab === "crons" && (
        <div data-testid="cron-jobs-tab">
          {cronsLoading && (
            <div data-testid="cron-loading" className="flex items-center gap-2 text-muted-foreground py-8">
              <div className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
              <span>{t("cronJobs.loading")}</span>
            </div>
          )}

          {cronsError && !cronsLoading && (
            <div data-testid="cron-error" className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
              {t("cronJobs.error")}
            </div>
          )}

          {cronsResult && !cronsLoading && !cronsError && cronsResult.entries.length === 0 && (
            <div data-testid="cron-empty" className="py-8 text-center text-sm text-muted-foreground">
              {t("cronJobs.empty")}
            </div>
          )}

          {cronsResult && !cronsLoading && !cronsError && cronsResult.entries.length > 0 && (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="py-2 pr-4 text-left font-medium text-muted-foreground">{t("cronJobs.colEnabled")}</th>
                  <th className="py-2 pr-4 text-left font-medium text-muted-foreground">{t("cronJobs.colSchedule")}</th>
                  <th className="py-2 pr-4 text-left font-medium text-muted-foreground">{t("cronJobs.colCommand")}</th>
                  <th className="py-2 text-left font-medium text-muted-foreground">{t("cronJobs.colActions")}</th>
                </tr>
              </thead>
              <tbody>
                {cronsResult.entries.map((entry, index) => (
                  <tr key={index} className="border-b last:border-0">
                    <td className="py-2 pr-4">
                      <span
                        data-testid={`cron-status-${index}`}
                        className={`text-xs font-medium ${entry.enabled ? "text-green-600" : "text-muted-foreground"}`}
                      >
                        {entry.enabled ? t("cronJobs.statusEnabled") : t("cronJobs.statusDisabled")}
                      </span>
                    </td>
                    <td className="py-2 pr-4 font-mono text-xs">
                      {`${entry.minute} ${entry.hour} ${entry.dayOfMonth} ${entry.month} ${entry.dayOfWeek}`}
                    </td>
                    <td className="py-2 pr-4 font-mono text-xs break-all">{entry.command}</td>
                    <td className="py-2" />
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}
