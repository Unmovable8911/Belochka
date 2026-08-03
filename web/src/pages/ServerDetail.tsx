import { useState } from "react"
import { useParams, Link, useNavigate } from "react-router-dom"
import { ArrowLeft, Terminal, Trash2, Pencil, Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useMonitorState } from "@/hooks/useMonitorState"
import { useGroups } from "@/hooks/useGroups"
import { formatBytes, formatUptime } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { DeleteServerDialog } from "@/components/DeleteServerDialog"
import { EditServerDialog } from "@/components/EditServerDialog"
import { toast } from "sonner"
import type { Server } from "@/types/server"
import * as api from "@/api/client"
import { CronJobsTab } from "@/components/CronJobsTab"
import { ProcessesTab } from "@/components/ProcessesTab"
import { RingGauge } from "@/components/RingGauge"
import { UsageBar } from "@/components/UsageBar"
import { CoreBar } from "@/components/CoreBar"
import { NetworkChart } from "@/components/NetworkChart"

type Tab = "overview" | "crons" | "processes"

export default function ServerDetail() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const { state, dispatch } = useMonitorState()
  const { groups } = useGroups(state.servers)
  const navigate = useNavigate()

  const [editOpen, setEditOpen] = useState(false)
  const [fullServer, setFullServer] = useState<Server | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<Tab>("overview")
  const [reconnecting, setReconnecting] = useState(false)

  const server = state.servers.find((s) => s.id === id)

  async function handleReconnect() {
    if (!id) return
    setReconnecting(true)
    try {
      await api.reconnectServer(id)
    } catch {
      toast.error(t("serverCard.reconnectFailed"))
    } finally {
      setReconnecting(false)
    }
  }

  async function handleEditClick() {
    try {
      const s = await api.getServer(id ?? "")
      setFullServer(s)
      setEditOpen(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("serverDetail.failedToLoad"))
    }
  }
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
        <div className="flex items-center gap-2">
          <Button
            variant="default"
            size="sm"
            className="cursor-pointer hover:brightness-110 hover:scale-105 transition-all"
            onClick={handleEditClick}
          >
            <Pencil className="size-4 mr-1" />
            {t("serverDetail.edit")}
          </Button>
          {server.status === "failed" && (
            <Button
              variant="outline"
              size="sm"
              disabled={reconnecting}
              onClick={handleReconnect}
              className="cursor-pointer hover:brightness-110 hover:scale-105 transition-all"
            >
              {reconnecting && <Loader2 className="size-3 animate-spin" />}
              {t("serverCard.reconnect")}
            </Button>
          )}
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
        </div>
      </div>

      {fullServer && (
        <EditServerDialog
          server={fullServer}
          open={editOpen}
          onOpenChange={setEditOpen}
          groups={groups}
          onServerUpdated={(updated) =>
            dispatch({ type: "update_server", data: { serverId: updated.id, name: updated.name, host: updated.host, group_id: updated.group_id } })
          }
        />
      )}

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
        <button
          role="tab"
          aria-selected={activeTab === "processes"}
          onClick={() => setActiveTab("processes")}
          className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
            activeTab === "processes"
              ? "border-primary text-primary"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          {t("serverDetail.tabProcesses")}
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
                <div className="text-sm font-medium">{t("serverDetail.coresCount", { count: system.coreCount })}</div>
              </div>
            </div>
          )}

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6" data-testid="metrics-grid">
            {(metrics?.aggregate || (metrics?.cores && metrics.cores.length > 0)) && (
              <div className="rounded-lg border bg-card p-4 h-72 flex flex-col overflow-hidden">
                <h2 className="text-lg font-semibold mb-4 shrink-0">{t("serverDetail.cpu")}</h2>
                <div className="flex flex-col items-center gap-6 md:flex-row md:items-stretch flex-1 min-h-0">
                  {metrics.aggregate && (
                    <div className="shrink-0 self-start">
                      <RingGauge value={metrics.aggregate.usagePercent} testId="cpu-ring-gauge" />
                    </div>
                  )}
                  <div className="flex-1 w-full overflow-y-auto min-h-0 scrollbar-hidden flex flex-wrap gap-x-4 gap-y-2 content-start">
                    {(metrics.cores ?? []).map((core, index) => (
                      <CoreBar
                        key={core.name ?? index}
                        label={`Core ${index}`}
                        value={core.usagePercent}
                        ariaLabel={`Core ${index} usage`}
                      />
                    ))}
                  </div>
                </div>
              </div>
            )}

            {metrics?.memory && (
              <div className="rounded-lg border bg-card p-4 h-72 flex flex-col overflow-hidden">
                <h2 className="text-lg font-semibold mb-4 shrink-0">{t("serverDetail.memory")}</h2>
                <div className="flex-1 overflow-y-auto min-h-0 scrollbar-hidden flex flex-col items-center gap-4">
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
              <div className="rounded-lg border bg-card p-4 h-72 flex flex-col overflow-hidden">
                <h2 className="text-lg font-semibold mb-4 shrink-0">{t("serverDetail.disk")}</h2>
                <div className="flex-1 overflow-y-auto min-h-0 scrollbar-hidden space-y-3">
                  {metrics.disk.partitions.map((partition) => {
                    const pct = partition.total > 0 ? (partition.used / partition.total) * 100 : 0
                    return (
                      <UsageBar
                        key={partition.mountPoint}
                        label={`${partition.mountPoint} (${partition.filesystem.replace("/dev/", "")})`}
                        value={pct}
                        rightText={`${formatBytes(partition.used)} / ${formatBytes(partition.total)} (${pct.toFixed(1)}%)`}
                        ariaLabel={`${partition.mountPoint} usage`}
                      />
                    )
                  })}
                </div>
              </div>
            )}

            {metrics?.network && (
              <div className="rounded-lg border bg-card p-4 h-72 flex flex-col overflow-hidden" data-testid="network-section">
                <h2 className="text-lg font-semibold shrink-0">{t("serverDetail.network")}</h2>
                <NetworkChart interfaces={metrics.network.interfaces} />
              </div>
            )}
          </div>

        </>
      )}

      {/* Cron Jobs tab panel */}
      {activeTab === "crons" && id && <CronJobsTab serverId={id} />}

      {/* Processes tab panel */}
      {activeTab === "processes" && id && <ProcessesTab serverId={id} />}
    </div>
  )
}
