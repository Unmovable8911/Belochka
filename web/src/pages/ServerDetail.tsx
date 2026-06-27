import { useState, useEffect } from "react"
import { useParams, Link, useNavigate } from "react-router-dom"
import { ArrowLeft, Pencil, Play, Plus, Terminal, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { AlertDialog } from "radix-ui"
import { useMonitorState } from "@/hooks/useMonitorState"
import { formatBytes, formatNetworkSpeed, formatPercent, formatUptime } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { DeleteServerDialog } from "@/components/DeleteServerDialog"
import { AddCronDialog } from "@/components/AddCronDialog"
import { LanguageSwitcher } from "@/components/LanguageSwitcher"
import { RingGauge } from "@/components/RingGauge"
import { UsageBar } from "@/components/UsageBar"
import { ProcessTable } from "@/components/ProcessTable"
import { getCrons, updateCron, deleteCron, runCron } from "@/api/client"
import type { CronEntry, CronResult, CronRunResult } from "@/types/server"

type Tab = "overview" | "crons"

export default function ServerDetail() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const { state, dispatch } = useMonitorState()
  const navigate = useNavigate()

  const [deleteOpen, setDeleteOpen] = useState(false)
  const [addCronOpen, setAddCronOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<Tab>("overview")

  const [cronsLoading, setCronsLoading] = useState(false)
  const [cronsError, setCronsError] = useState<string | null>(null)
  const [cronsResult, setCronsResult] = useState<CronResult | null>(null)
  const [cronsFetched, setCronsFetched] = useState(false)

  // Edit state
  const [editEntry, setEditEntry] = useState<CronEntry | undefined>(undefined)
  const [editIndex, setEditIndex] = useState<number | undefined>(undefined)

  // Delete confirm state
  const [deleteConfirmIndex, setDeleteConfirmIndex] = useState<number | null>(null)

  // Per-row errors
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({})

  // Run cron now state
  const [runningIndex, setRunningIndex] = useState<number | null>(null)
  const [runResult, setRunResult] = useState<{ command: string; result: CronRunResult } | null>(null)

  const server = state.servers.find((s) => s.id === id)
  const metrics = id ? state.metrics[id] : undefined

  function fetchCrons() {
    if (!id) return
    setCronsLoading(true)
    setCronsError(null)
    getCrons(id)
      .then((result) => setCronsResult(result))
      .catch((err: Error) => setCronsError(err.message))
      .finally(() => setCronsLoading(false))
  }

  useEffect(() => {
    if (activeTab !== "crons" || !id || cronsFetched) return
    setCronsFetched(true)
    fetchCrons()
  }, [activeTab, id, cronsFetched])

  function setRowError(index: number, msg: string) {
    setRowErrors((prev) => ({ ...prev, [index]: msg }))
  }
  function clearRowError(index: number) {
    setRowErrors((prev) => {
      const next = { ...prev }
      delete next[index]
      return next
    })
  }

  function handleCronCreated(entry: CronEntry) {
    if (editIndex !== undefined) {
      // Replace entry at editIndex
      setCronsResult((prev) => {
        if (!prev) return prev
        const entries = prev.entries.map((e, i) => (i === editIndex ? entry : e))
        return { ...prev, entries }
      })
    } else {
      setCronsResult((prev) =>
        prev ? { ...prev, entries: [...prev.entries, entry] } : { entries: [entry], passthroughs: [] }
      )
    }
    setEditEntry(undefined)
    setEditIndex(undefined)
  }

  function handleOpenAdd() {
    setEditEntry(undefined)
    setEditIndex(undefined)
    setAddCronOpen(true)
  }

  function handleOpenEdit(entry: CronEntry, index: number) {
    setEditEntry(entry)
    setEditIndex(index)
    setAddCronOpen(true)
  }

  async function handleRunCron(index: number) {
    if (!id || !cronsResult) return
    const entry = cronsResult.entries[index]
    clearRowError(index)
    setRunningIndex(index)
    try {
      const result = await runCron(id, index)
      setRunResult({ command: entry.command, result })
    } catch (err) {
      setRowError(index, err instanceof Error ? err.message : t("cronJobs.runFailed"))
    } finally {
      setRunningIndex(null)
    }
  }

  async function handleToggle(index: number) {
    if (!id || !cronsResult) return
    const entry = cronsResult.entries[index]
    clearRowError(index)
    const toggled = { ...entry, enabled: !entry.enabled }
    // Optimistically update
    setCronsResult((prev) => {
      if (!prev) return prev
      const entries = prev.entries.map((e, i) => (i === index ? toggled : e))
      return { ...prev, entries }
    })
    try {
      const updated = await updateCron(id, index, {
        minute: entry.minute,
        hour: entry.hour,
        dayOfMonth: entry.dayOfMonth,
        month: entry.month,
        dayOfWeek: entry.dayOfWeek,
        command: entry.command,
        enabled: !entry.enabled,
      })
      setCronsResult((prev) => {
        if (!prev) return prev
        const entries = prev.entries.map((e, i) => (i === index ? updated : e))
        return { ...prev, entries }
      })
    } catch (err) {
      // Revert
      setCronsResult((prev) => {
        if (!prev) return prev
        const entries = prev.entries.map((e, i) => (i === index ? entry : e))
        return { ...prev, entries }
      })
      setRowError(index, err instanceof Error ? err.message : t("cronJobs.toggleFailed"))
    }
  }

  async function handleDeleteConfirmed() {
    if (!id || deleteConfirmIndex === null || !cronsResult) return
    const index = deleteConfirmIndex
    setDeleteConfirmIndex(null)
    clearRowError(index)
    try {
      await deleteCron(id, index)
      setCronsResult((prev) => {
        if (!prev) return prev
        const entries = prev.entries.filter((_, i) => i !== index)
        return { ...prev, entries }
      })
      // Shift row errors for indices above the deleted one
      setRowErrors((prev) => {
        const next: Record<number, string> = {}
        for (const [k, v] of Object.entries(prev)) {
          const ki = parseInt(k, 10)
          if (ki < index) next[ki] = v
          else if (ki > index) next[ki - 1] = v
        }
        return next
      })
    } catch (err) {
      setRowError(index, err instanceof Error ? err.message : t("cronJobs.deleteRowFailed"))
    }
  }

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

      {id && (
        <AddCronDialog
          serverId={id}
          open={addCronOpen}
          onOpenChange={(next) => {
            setAddCronOpen(next)
            if (!next) {
              setEditEntry(undefined)
              setEditIndex(undefined)
            }
          }}
          onCreated={handleCronCreated}
          editEntry={editEntry}
          editIndex={editIndex}
        />
      )}

      {/* Delete cron confirmation dialog */}
      <AlertDialog.Root open={deleteConfirmIndex !== null} onOpenChange={(open) => { if (!open) setDeleteConfirmIndex(null) }}>
        <AlertDialog.Portal>
          <AlertDialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
          <AlertDialog.Content className="fixed top-[50%] left-[50%] z-50 w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] rounded-lg border bg-background p-6 shadow-lg sm:max-w-lg">
            <AlertDialog.Title className="text-lg font-semibold">
              {t("cronJobs.deleteConfirmTitle")}
            </AlertDialog.Title>
            <AlertDialog.Description className="mt-2 text-sm text-muted-foreground">
              {t("cronJobs.deleteConfirmMessage")}
            </AlertDialog.Description>
            <div className="mt-4 flex justify-end gap-2">
              <AlertDialog.Cancel asChild>
                <Button variant="outline">{t("cronJobs.cancel")}</Button>
              </AlertDialog.Cancel>
              <AlertDialog.Action asChild>
                <Button variant="destructive" onClick={handleDeleteConfirmed}>
                  {t("cronJobs.deleteConfirm")}
                </Button>
              </AlertDialog.Action>
            </div>
          </AlertDialog.Content>
        </AlertDialog.Portal>
      </AlertDialog.Root>

      {/* Run cron result dialog */}
      {runResult !== null && (
        <div role="dialog" data-testid="run-output-dialog" className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="fixed inset-0 bg-black/50" onClick={() => setRunResult(null)} />
          <div className="relative z-10 w-full max-w-2xl rounded-lg border bg-background p-6 shadow-lg mx-4">
            <h2 className="text-lg font-semibold mb-4">{t("cronJobs.runDialogTitle")}</h2>
            <div className="mb-3">
              <div className="text-xs text-muted-foreground mb-1">{t("cronJobs.runDialogCommand")}</div>
              <code data-testid="run-command" className="text-sm font-mono bg-muted px-2 py-1 rounded break-all">{runResult.command}</code>
            </div>
            <div className="mb-3">
              <div className="text-xs text-muted-foreground mb-1">{t("cronJobs.runDialogExitCode")}</div>
              <span
                data-testid="run-exit-code"
                className={`text-sm font-mono font-semibold ${runResult.result.exitCode === 0 ? "text-green-600" : "text-destructive text-red-600"}`}
              >
                {runResult.result.exitCode}
              </span>
            </div>
            <div className="mb-4">
              <div className="text-xs text-muted-foreground mb-1">{t("cronJobs.runDialogOutput")}</div>
              <textarea
                readOnly
                value={runResult.result.output}
                className="w-full h-48 font-mono text-xs bg-muted rounded p-2 resize-none"
              />
            </div>
            <div className="flex justify-end">
              <Button onClick={() => setRunResult(null)}>{t("cronJobs.runDialogClose")}</Button>
            </div>
          </div>
        </div>
      )}

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
          <div className="flex justify-end mb-4">
            <Button
              size="sm"
              onClick={handleOpenAdd}
              className="cursor-pointer"
            >
              <Plus className="size-4 mr-1" />
              {t("cronJobs.addButton")}
            </Button>
          </div>

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
                  <>
                    <tr key={index} className="border-b last:border-0">
                      <td className="py-2 pr-4">
                        <button
                          role="switch"
                          aria-checked={entry.enabled}
                          data-state={entry.enabled ? "checked" : "unchecked"}
                          data-testid={`cron-status-${index}`}
                          onClick={() => handleToggle(index)}
                          className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ${
                            entry.enabled ? "bg-primary" : "bg-input"
                          }`}
                          aria-label={entry.enabled ? t("cronJobs.statusEnabled") : t("cronJobs.statusDisabled")}
                        >
                          <span
                            className={`pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform ${
                              entry.enabled ? "translate-x-4" : "translate-x-0"
                            }`}
                          />
                        </button>
                      </td>
                      <td className="py-2 pr-4 font-mono text-xs">
                        {`${entry.minute} ${entry.hour} ${entry.dayOfMonth} ${entry.month} ${entry.dayOfWeek}`}
                      </td>
                      <td className="py-2 pr-4 font-mono text-xs break-all">{entry.command}</td>
                      <td className="py-2">
                        <div className="flex items-center gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label={t("cronJobs.runButton")}
                            onClick={() => handleRunCron(index)}
                            disabled={runningIndex === index}
                            className="cursor-pointer"
                          >
                            {runningIndex === index ? (
                              <span data-testid={`cron-run-spinner-${index}`} className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                            ) : (
                              <Play className="size-4" />
                            )}
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label={t("cronJobs.editButton")}
                            onClick={() => handleOpenEdit(entry, index)}
                            className="cursor-pointer"
                          >
                            <Pencil className="size-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label={t("cronJobs.deleteButton")}
                            onClick={() => setDeleteConfirmIndex(index)}
                            className="cursor-pointer text-destructive hover:text-destructive"
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                    {rowErrors[index] && (
                      <tr key={`error-${index}`}>
                        <td colSpan={4} className="pb-2 pt-0">
                          <p
                            data-testid={`cron-row-error-${index}`}
                            className="text-xs text-destructive"
                          >
                            {rowErrors[index]}
                          </p>
                        </td>
                      </tr>
                    )}
                  </>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}
