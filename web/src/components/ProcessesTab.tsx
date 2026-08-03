import { useState, useMemo, useCallback, useRef, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { RefreshCw, Loader2, XCircle } from "lucide-react"
import { toast } from "sonner"
import { useProcesses } from "@/hooks/useProcesses"
import { killProcess } from "@/api/client"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatBytes } from "@/lib/format"
import type { Process } from "@/types/server"

type SortField = "pid" | "user" | "rss" | "cpuPct" | "memPct" | "etime" | "command"
type SortDir = "asc" | "desc"

// Column keys used for width tracking
type ColKey = "pid" | "user" | "rss" | "cpuPct" | "memPct" | "etime" | "command" | "actions"

const DEFAULT_COL_WIDTHS: Record<ColKey, number> = {
  pid: 60,
  user: 80,
  rss: 80,
  cpuPct: 60,
  memPct: 60,
  etime: 90,
  command: 200,
  actions: 50,
}

const MIN_COL_WIDTH = 40

interface Props {
  serverId: string
}

// Kill dialog state
interface KillDialogState {
  open: boolean
  process: Process | null
}

export function ProcessesTab({ serverId }: Props) {
  const { t } = useTranslation()
  const { state, fetchProcesses, setAutoRefresh } = useProcesses(serverId)
  const [search, setSearch] = useState("")
  const [sortField, setSortField] = useState<SortField>("cpuPct")
  const [sortDir, setSortDir] = useState<SortDir>("desc")
  const [killDialog, setKillDialog] = useState<KillDialogState>({ open: false, process: null })
  const [killSignal, setKillSignal] = useState<string>("SIGTERM")
  const [killing, setKilling] = useState(false)
  const [colWidths, setColWidths] = useState<Record<ColKey, number>>(DEFAULT_COL_WIDTHS)

  // --- Column resize ---
  const resizing = useRef<{ key: ColKey; startX: number; startW: number } | null>(null)

  const onResizeMouseDown = useCallback((e: React.MouseEvent, key: ColKey) => {
    e.preventDefault()
    e.stopPropagation()
    resizing.current = { key, startX: e.clientX, startW: colWidths[key] }
  }, [colWidths])

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!resizing.current) return
      const { key, startX, startW } = resizing.current
      const delta = e.clientX - startX
      setColWidths((prev) => ({
        ...prev,
        [key]: Math.max(MIN_COL_WIDTH, startW + delta),
      }))
    }
    const onUp = () => {
      resizing.current = null
    }
    document.addEventListener("mousemove", onMove)
    document.addEventListener("mouseup", onUp)
    return () => {
      document.removeEventListener("mousemove", onMove)
      document.removeEventListener("mouseup", onUp)
    }
  }, [])

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"))
    } else {
      setSortField(field)
      setSortDir("desc")
    }
  }

  // Flat list: filter → sort
  const { rows, totalCount } = useMemo(() => {
    const keywords = search.trim().toLowerCase().split(/\s+/).filter(Boolean)
    let list = state.processes ?? []
    if (keywords.length > 0) {
      list = list.filter((p) =>
        keywords.every((kw) =>
          p.command.toLowerCase().includes(kw) || p.user.toLowerCase().includes(kw),
        ),
      )
    }

    const cmp = makeComparator(sortField, sortDir)
    const sorted = [...list].sort(cmp)
    return { rows: sorted, totalCount: list.length }
  }, [state.processes, search, sortField, sortDir])

  // --- Kill ---
  const handleKillClick = useCallback((p: Process) => {
    setKillSignal("SIGTERM")
    setKillDialog({ open: true, process: p })
  }, [])

  const handleKillConfirm = useCallback(async () => {
    if (!killDialog.process) return
    setKilling(true)
    try {
      await killProcess(serverId, killDialog.process.pid, killSignal)
      toast.success(t("processes.killSuccess", { pid: killDialog.process.pid }))
      setKillDialog({ open: false, process: null })
      fetchProcesses()
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("processes.killFailed")
      toast.error(msg)
    } finally {
      setKilling(false)
    }
  }, [killDialog.process, killSignal, serverId, t, fetchProcesses])

  function renderSortIcon(field: SortField) {
    if (sortField !== field) return <span className="ml-1 text-muted-foreground opacity-30">↕</span>
    return <span className="ml-1">{sortDir === "asc" ? "↑" : "↓"}</span>
  }

  // Render a table head cell with resize handle
  function renderHead(key: ColKey, label: string, sortFieldKey: SortField, alignRight = false) {
    return (
      <TableHead
        style={{ width: colWidths[key] }}
        className={`cursor-pointer select-none relative ${alignRight ? "text-right" : ""}`}
        onClick={() => handleSort(sortFieldKey)}
      >
        <span className="inline-flex items-center">
          {label}{renderSortIcon(sortFieldKey)}
        </span>
        {/* Resize handle */}
        <div
          className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize hover:bg-primary/50 transition-colors"
          onMouseDown={(e) => onResizeMouseDown(e, key)}
          onClick={(e) => e.stopPropagation()}
        />
      </TableHead>
    )
  }

  return (
    <div className="space-y-4" data-testid="processes-tab">
      {/* Toolbar */}
      <div className="flex items-center gap-3 flex-wrap">
        <Input
          placeholder={t("processes.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-xs"
          data-testid="process-search"
        />
        {/* Toggle switch for auto-refresh */}
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <button
            type="button"
            role="switch"
            aria-checked={state.autoRefresh}
            onClick={() => setAutoRefresh(!state.autoRefresh)}
            data-testid="auto-refresh-toggle"
            className={`
              relative inline-flex h-5 w-9 shrink-0 items-center rounded-full
              transition-colors duration-200 ease-in-out
              focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring
              cursor-pointer
              ${state.autoRefresh ? "bg-primary" : "bg-input"}
            `}
          >
            <span
              className={`
                pointer-events-none inline-block size-3.5 rounded-full bg-background shadow-sm
                transition-transform duration-200 ease-in-out
                ${state.autoRefresh ? "translate-x-4" : "translate-x-1"}
              `}
            />
          </button>
          <span className="text-xs text-muted-foreground">{t("processes.autoRefresh")}</span>
        </label>
        <Badge variant="secondary" data-testid="process-count">
          {t("processes.processCount", { count: totalCount })}
        </Badge>
        {/* Manual refresh — only visible when auto-refresh is off, positioned at right */}
        {!state.autoRefresh && (
          <Button
            variant="outline"
            size="sm"
            onClick={fetchProcesses}
            disabled={state.loading}
            className="cursor-pointer ml-auto"
            data-testid="process-refresh-btn"
          >
            <RefreshCw className={`size-4 mr-1 ${state.loading ? "animate-spin" : ""}`} />
            {t("processes.refresh")}
          </Button>
        )}
      </div>

      {/* Loading */}
      {state.loading && !state.fetched && (
        <div className="flex items-center gap-2 text-muted-foreground py-8" data-testid="processes-loading">
          <Loader2 className="size-4 animate-spin" />
          {t("processes.loading")}
        </div>
      )}

      {/* Error */}
      {state.error && (
        <div className="rounded-md bg-destructive/10 border border-destructive/30 p-4 text-sm text-destructive" data-testid="processes-error">
          {state.error}
        </div>
      )}

      {/* Empty */}
      {state.fetched && !state.loading && !state.error && rows.length === 0 && (
        <div className="text-center text-muted-foreground py-8" data-testid="processes-empty">
          {t("processes.empty")}
        </div>
      )}

      {/* Table */}
      {rows.length > 0 && (
        <div className="rounded-md border">
          <Table data-testid="process-table">
            <TableHeader>
              <TableRow>
                {renderHead("pid", t("processes.colPID"), "pid")}
                {renderHead("user", t("processes.colUser"), "user")}
                {renderHead("rss", t("processes.colRSS"), "rss", true)}
                {renderHead("cpuPct", t("processes.colCPU"), "cpuPct", true)}
                {renderHead("memPct", t("processes.colMEM"), "memPct", true)}
                {renderHead("etime", t("processes.colETime"), "etime")}
                {renderHead("command", t("processes.colCommand"), "command")}
                <TableHead style={{ width: colWidths.actions }} className="text-right">
                  {/* Actions column — no sort, no resize */}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((p) => (
                <TableRow key={p.pid} data-testid={`process-row-${p.pid}`}>
                  <TableCell className="font-mono text-xs" title={t("processes.ppid", { ppid: p.ppid })}>
                    {p.pid}
                  </TableCell>
                  <TableCell className="text-xs">{p.user}</TableCell>
                  <TableCell className="text-xs text-right font-mono">{formatBytes(p.rss * 1024)}</TableCell>
                  <TableCell className="text-xs text-right font-mono">{p.cpuPct.toFixed(1)}</TableCell>
                  <TableCell className="text-xs text-right font-mono">{p.memPct.toFixed(1)}</TableCell>
                  <TableCell className="text-xs font-mono">{p.etime}</TableCell>
                  <TableCell className="text-xs font-mono truncate max-w-[300px]" title={p.command}>
                    {p.command_name}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="size-7 cursor-pointer text-destructive hover:text-destructive"
                      disabled={p.protected}
                      title={p.protected ? t("processes.protectedProcess") : t("processes.kill")}
                      onClick={() => handleKillClick(p)}
                      data-testid={`kill-btn-${p.pid}`}
                    >
                      <XCircle className="size-3.5" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Kill confirmation dialog */}
      <Dialog open={killDialog.open} onOpenChange={(open) => !open && setKillDialog({ open: false, process: null })}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("processes.confirmKillTitle")}</DialogTitle>
            <DialogDescription asChild>
              <div className="mt-2 space-y-1 text-sm">
                {killDialog.process && (
                  <>
                    <p>{t("processes.confirmKillDesc")}</p>
                    <p><strong>PID:</strong> {killDialog.process.pid}</p>
                    <p><strong>User:</strong> {killDialog.process.user}</p>
                    <p><strong>Command:</strong> {killDialog.process.command}</p>
                  </>
                )}
              </div>
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2 py-2">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="kill-signal"
                value="SIGTERM"
                checked={killSignal === "SIGTERM"}
                onChange={() => setKillSignal("SIGTERM")}
                data-testid="signal-sigterm"
              />
              <span className="text-sm">{t("processes.signalSIGTERM")}</span>
            </label>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="kill-signal"
                value="SIGKILL"
                checked={killSignal === "SIGKILL"}
                onChange={() => setKillSignal("SIGKILL")}
                data-testid="signal-sigkill"
              />
              <span className="text-sm text-destructive">{t("processes.signalSIGKILL")}</span>
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setKillDialog({ open: false, process: null })} className="cursor-pointer">
              {t("processes.cancel")}
            </Button>
            <Button variant="destructive" size="sm" onClick={handleKillConfirm} disabled={killing} className="cursor-pointer" data-testid="kill-confirm-btn">
              {killing ? <Loader2 className="size-3 animate-spin mr-1" /> : null}
              {t("processes.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function makeComparator(field: SortField, dir: SortDir): (a: Process, b: Process) => number {
  const sign = dir === "asc" ? 1 : -1
  return (a, b) => {
    let cmp = 0
    switch (field) {
      case "pid":
        cmp = a.pid - b.pid
        break
      case "user":
        cmp = a.user.localeCompare(b.user)
        break
      case "rss":
        cmp = a.rss - b.rss
        break
      case "cpuPct":
        cmp = a.cpuPct - b.cpuPct
        break
      case "memPct":
        cmp = a.memPct - b.memPct
        break
      case "etime":
        cmp = a.etime.localeCompare(b.etime)
        break
      case "command":
        cmp = a.command.localeCompare(b.command)
        break
    }
    return cmp * sign
  }
}
