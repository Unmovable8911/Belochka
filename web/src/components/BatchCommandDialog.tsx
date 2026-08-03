import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react"
import { useTranslation } from "react-i18next"
import {
  ArrowDown,
  ArrowLeft,
  Check,
  Copy,
  CornerDownLeft,
  Folder,
  Loader2,
  Search,
  Square,
  Terminal,
} from "lucide-react"
import { useBatchRun, selectionState, type SelectionState } from "@/hooks/useBatchRun"
import { useMonitorState } from "@/hooks/useMonitorState"
import { useGroups } from "@/hooks/useGroups"
import { statusDotColor } from "@/lib/status-colors"
import { formatRunDuration } from "@/lib/format"
import type { RunResultInfo, RunResultStatus } from "@/types/batch"
import type { GroupNode } from "@/types/server"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

// Status badge styling per Run Result status. `failed` here is the Run Result
// status (execution outcome), a different layer from the failed Connection
// State shown as the selection-tree dot.
const resultBadgeClass: Record<RunResultStatus, string> = {
  pending: "bg-secondary text-secondary-foreground",
  running: "bg-amber-500/15 text-amber-600 dark:text-amber-400",
  success: "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400",
  failed: "bg-destructive/10 text-destructive",
}

function isFinished(status: RunResultStatus): boolean {
  return status === "success" || status === "failed"
}

// toCheckedState maps the tri-state selection to the Radix Checkbox contract
// (boolean | "indeterminate").
function toCheckedState(s: SelectionState): boolean | "indeterminate" {
  if (s === "indeterminate") return "indeterminate"
  return s === "checked"
}

// ServerTreeRow renders one selectable Server in the Compose selection tree
// (checkbox + Connection State dot + name). Used for Group members and for
// ungrouped Servers at the root.
function ServerTreeRow({
  serverId,
  name,
  status,
  selected,
  indent,
  onToggle,
}: {
  serverId: string
  name: string
  status: string
  selected: boolean
  indent?: boolean
  onToggle: (serverId: string) => void
}) {
  return (
    <div
      className={`flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent ${
        indent ? "pl-7" : ""
      }`}
      onClick={() => onToggle(serverId)}
    >
      <span onClick={(e) => e.stopPropagation()}>
        <Checkbox checked={selected} onCheckedChange={() => onToggle(serverId)} />
      </span>
      <span className={`size-2 shrink-0 rounded-full ${statusDotColor(status)}`} />
      <span className="truncate">{name}</span>
    </div>
  )
}

export function BatchCommandDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const {
    state,
    open: openDialog,
    setScript,
    toggleServer,
    toggleGroup,
    toggleAll,
    setMode,
    runScript,
    cancel,
    sendInput,
  } = useBatchRun()
  const { state: monitorState } = useMonitorState()
  const { groupList } = useGroups(monitorState.servers)

  // Focused Server in the results view; defaults to the first target.
  const [focusedServerId, setFocusedServerId] = useState<string | null>(null)
  const [inputText, setInputText] = useState("")
  const [filterText, setFilterText] = useState("")
  // Follow-bottom for the live output: auto-scroll only while the user stays
  // at the bottom; scrolling up pauses following until "jump to bottom".
  const [stickToBottom, setStickToBottom] = useState(true)
  const [copied, setCopied] = useState(false)
  const outputRef = useRef<HTMLPreElement>(null)
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (open) void openDialog()
  }, [open, openDialog])

  // Reset the focused Server whenever the results list changes identity (a
  // new dispatch or a fresh load).
  useEffect(() => {
    if (state.results.length > 0) {
      setFocusedServerId((prev) =>
        prev && state.results.some((r) => r.server_id === prev) ? prev : state.results[0].server_id
      )
    }
  }, [state.results])

  // A new focused Server starts pinned to the bottom of its own output.
  useEffect(() => {
    setStickToBottom(true)
  }, [focusedServerId])

  const serversByName = useMemo(() => {
    const map = new Map(monitorState.servers.map((s) => [s.id, s]))
    return map
  }, [monitorState.servers])

  const allServerIds = useMemo(() => monitorState.servers.map((s) => s.id), [monitorState.servers])
  const selectedSet = useMemo(() => new Set(state.selected), [state.selected])
  const running = state.run?.status === "running"

  // Compose view helpers
  const allSelection = selectionState(allServerIds, selectedSet)
  const canDispatch =
    state.script.trim() !== "" && state.selected.length > 0 && !state.dispatching
  const lastRunAvailable = state.run !== null && state.run.status === "done"
  const scriptLines = state.script === "" ? 0 : state.script.split("\n").length

  // Server filter for the selection tree; a Group stays visible when its name
  // or any member matches (a name match shows the whole Group).
  const filterLower = filterText.trim().toLowerCase()
  const visibleGroups = useMemo(() => {
    const q = filterLower
    return groupList
      .map((node) => {
        const groupMatches = q === "" || node.name.toLowerCase().includes(q)
        return {
          node,
          members: groupMatches
            ? node.servers
            : node.servers.filter((s) => s.name.toLowerCase().includes(q)),
        }
      })
      .filter((g) => g.members.length > 0)
  }, [groupList, filterLower])
  const visibleUngrouped = useMemo(
    () =>
      monitorState.servers.filter(
        (s) => !s.group_id && (filterLower === "" || s.name.toLowerCase().includes(filterLower))
      ),
    [monitorState.servers, filterLower]
  )
  const filterNoMatches =
    filterLower !== "" && visibleGroups.length === 0 && visibleUngrouped.length === 0

  // Results view helpers
  const finishedCount = state.results.filter((r) => isFinished(r.status)).length
  const finishedPercent =
    state.results.length === 0 ? 0 : Math.round((finishedCount / state.results.length) * 100)
  const runDone = state.run?.status === "done"
  // Total run duration = earliest start to latest finish across Results.
  const runDuration = useMemo(() => {
    let start: string | undefined
    let end: string | undefined
    for (const r of state.results) {
      if (r.started_at && (!start || r.started_at < start)) start = r.started_at
      if (r.finished_at && (!end || r.finished_at > end)) end = r.finished_at
    }
    return formatRunDuration(start, end)
  }, [state.results])

  function handleDispatch() {
    void runScript(state.script, state.selected)
  }

  function handleScriptKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && canDispatch) {
      e.preventDefault()
      handleDispatch()
    }
  }

  function handleGroupToggle(node: GroupNode) {
    toggleGroup(node.servers.map((s) => s.id))
  }

  function handleInputSubmit(e: FormEvent) {
    e.preventDefault()
    if (!focusedServerId || !inputText) return
    sendInput(focusedServerId, `${inputText}\n`)
    setInputText("")
  }

  function handleOutputScroll() {
    const el = outputRef.current
    if (!el) return
    setStickToBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 32)
  }

  function handleJumpToBottom() {
    setStickToBottom(true)
    const el = outputRef.current
    if (el) el.scrollTop = el.scrollHeight
  }

  async function handleCopyOutput() {
    const output = focusedResult?.output
    if (!output || !navigator.clipboard) return
    try {
      await navigator.clipboard.writeText(output)
      setCopied(true)
      if (copyTimer.current) clearTimeout(copyTimer.current)
      copyTimer.current = setTimeout(() => setCopied(false), 1500)
    } catch {
      // Clipboard unavailable; leave the button unchanged.
    }
  }

  // Auto-scroll the focused output pane to the bottom as new output arrives,
  // unless the user scrolled up to inspect history.
  useEffect(() => {
    if (!stickToBottom) return
    const el = outputRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [focusedServerId, state.results, stickToBottom])

  const focusedResult = state.results.find((r) => r.server_id === focusedServerId)
  const focusedRunning = focusedResult?.status === "running"

  function renderStatusBadge(result: RunResultInfo) {
    return (
      <Badge variant="outline" className={resultBadgeClass[result.status]}>
        {t(`batch.status${result.status.charAt(0).toUpperCase()}${result.status.slice(1)}`)}
      </Badge>
    )
  }

  function renderMeta(result: RunResultInfo) {
    const bits: string[] = []
    if (isFinished(result.status) && result.exit_code !== undefined) {
      bits.push(t("batch.exitCode", { code: result.exit_code }))
    }
    const duration = formatRunDuration(result.started_at, result.finished_at)
    if (duration) bits.push(duration)
    return bits.join(" · ")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-5xl! h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Terminal className="size-4" />
            {t("batch.title")}
          </DialogTitle>
          <DialogDescription>{t("batch.description")}</DialogDescription>
        </DialogHeader>

        {state.mode === "compose" ? (
          <div className="grid flex-1 min-h-0 grid-rows-[minmax(0,1fr)_auto] gap-4 overflow-y-auto py-2">
            <div className="grid min-h-0 gap-4 sm:grid-cols-2">
              {/* Script editor */}
              <div className="grid min-h-0 gap-1.5 sm:grid-rows-[auto_minmax(0,1fr)]">
                <div className="flex items-baseline justify-between gap-2">
                  <Label htmlFor="batch-script">{t("batch.script")}</Label>
                  {scriptLines > 0 && (
                    <span className="text-xs text-muted-foreground">
                      {t("batch.lines", { count: scriptLines })}
                    </span>
                  )}
                </div>
                <textarea
                  id="batch-script"
                  value={state.script}
                  onChange={(e) => setScript(e.target.value)}
                  onKeyDown={handleScriptKeyDown}
                  placeholder={t("batch.scriptPlaceholder")}
                  spellCheck={false}
                  className="min-h-32 w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                />
              </div>

              {/* Server selection tree */}
              <div className="grid min-h-0 gap-1.5 sm:grid-rows-[auto_auto_minmax(0,1fr)]">
                <div className="flex items-baseline justify-between gap-2">
                  <Label>{t("batch.targets")}</Label>
                  <span className="text-xs text-muted-foreground">
                    {state.selected.length}/{monitorState.servers.length}
                  </span>
                </div>
                <div className="relative">
                  <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={filterText}
                    onChange={(e) => setFilterText(e.target.value)}
                    placeholder={t("batch.filterPlaceholder")}
                    aria-label={t("batch.filterPlaceholder")}
                    className="h-8 pl-8"
                  />
                </div>
                <div className="max-h-64 min-h-0 overflow-y-auto rounded-md border border-border p-2 sm:max-h-none">
                  {monitorState.servers.length === 0 ? (
                    <p className="px-2 py-3 text-xs text-muted-foreground italic">
                      {t("batch.noServers")}
                    </p>
                  ) : (
                    <div className="space-y-0.5">
                      {/* Global select-all convenience. Rows are clickable divs;
                          the Checkbox stops propagation so a direct checkbox
                          click toggles exactly once. */}
                      <div
                        className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                        onClick={() => toggleAll(allServerIds)}
                      >
                        <span onClick={(e) => e.stopPropagation()}>
                          <Checkbox
                            checked={toCheckedState(allSelection)}
                            onCheckedChange={() => toggleAll(allServerIds)}
                          />
                        </span>
                        <span className="font-medium">{t("batch.selectAll")}</span>
                        <span className="ml-auto text-xs text-muted-foreground">
                          {state.selected.length}/{monitorState.servers.length}
                        </span>
                      </div>

                      <div className="my-1 border-t border-border" />

                      {/* Groups, then ungrouped Servers at the root */}
                      {visibleGroups.map(({ node, members }) => {
                        const groupState = selectionState(
                          node.servers.map((s) => s.id),
                          selectedSet
                        )
                        return (
                          <div key={node.id}>
                            <div
                              className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                              onClick={() => handleGroupToggle(node)}
                            >
                              <span onClick={(e) => e.stopPropagation()}>
                                <Checkbox
                                  checked={toCheckedState(groupState)}
                                  disabled={node.servers.length === 0}
                                  onCheckedChange={() => handleGroupToggle(node)}
                                />
                              </span>
                              <Folder className="size-3.5 shrink-0 text-muted-foreground" />
                              <span className="truncate">{node.name}</span>
                              <span className="ml-auto shrink-0 text-xs text-muted-foreground">
                                {node.member_count}
                              </span>
                            </div>
                            {members.map((server) => (
                              <ServerTreeRow
                                key={server.id}
                                serverId={server.id}
                                name={server.name}
                                status={server.status}
                                selected={selectedSet.has(server.id)}
                                indent
                                onToggle={toggleServer}
                              />
                            ))}
                          </div>
                        )
                      })}
                      {visibleUngrouped.map((server) => (
                        <ServerTreeRow
                          key={server.id}
                          serverId={server.id}
                          name={server.name}
                          status={server.status}
                          selected={selectedSet.has(server.id)}
                          onToggle={toggleServer}
                        />
                      ))}

                      {filterNoMatches && (
                        <p className="px-2 py-3 text-xs text-muted-foreground italic">
                          {t("batch.noMatches")}
                        </p>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Dispatch actions + inline hints */}
            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0 text-xs text-muted-foreground">
                {state.dispatchError ? (
                  <span className="text-destructive">
                    {state.dispatchError.code === "batch_run_in_progress"
                      ? t("batch.runInProgress")
                      : state.dispatchError.message || t("batch.dispatchFailed")}
                  </span>
                ) : state.script.trim() === "" ? (
                  <span>{t("batch.needScript")}</span>
                ) : state.selected.length === 0 ? (
                  <span>{t("batch.needSelection")}</span>
                ) : (
                  <span className="hidden sm:inline">{t("batch.shortcutRun")}</span>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {lastRunAvailable && (
                  <Button variant="outline" onClick={() => setMode("results")}>
                    {t("batch.viewLastResult")}
                  </Button>
                )}
                <Button onClick={handleDispatch} disabled={!canDispatch}>
                  {state.dispatching && <Loader2 className="size-4 animate-spin" />}
                  {state.dispatching
                    ? t("batch.dispatching")
                    : t("batch.runOn", { count: state.selected.length })}
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <div className="grid flex-1 min-h-0 grid-rows-[auto_auto_minmax(0,1fr)] gap-3 overflow-y-auto py-2">
            {/* Results header */}
            <div className="flex items-center justify-between gap-2">
              <Button variant="outline" size="sm" onClick={() => setMode("compose")}>
                <ArrowLeft className="size-3.5" />
                {t("batch.newCommand")}
              </Button>
              <div className="flex items-center gap-2">
                {running && (
                  <Button variant="destructive" size="sm" onClick={() => void cancel()}>
                    <Square className="size-3.5" />
                    {t("batch.cancelRun")}
                  </Button>
                )}
                {!state.wsConnected && running && (
                  <span className="text-xs text-muted-foreground">{t("batch.reconnecting")}</span>
                )}
              </div>
            </div>

            {/* Aggregate progress across all target Servers */}
            {state.results.length > 0 && (
              <div className="flex items-center gap-3">
                <div className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-primary transition-[width] duration-300"
                    style={{ width: `${finishedPercent}%` }}
                  />
                </div>
                <span className="shrink-0 text-xs text-muted-foreground">
                  {runDone && runDuration
                    ? t("batch.overviewDone", {
                        done: finishedCount,
                        total: state.results.length,
                        duration: runDuration,
                      })
                    : t("batch.overview", {
                        done: finishedCount,
                        total: state.results.length,
                      })}
                </span>
              </div>
            )}

            <div className="grid min-h-0 gap-4 sm:grid-cols-[200px_minmax(0,1fr)]">
              {/* Per-Server status list */}
              <div className="max-h-64 min-w-0 overflow-y-auto rounded-md border border-border sm:max-h-none">
                {state.results.length === 0 ? (
                  <p className="p-3 text-xs text-muted-foreground italic">
                    {t("batch.emptyResults")}
                  </p>
                ) : (
                  <ul className="divide-y divide-border">
                    {state.results.map((result) => {
                      const server = serversByName.get(result.server_id)
                      const focused = result.server_id === focusedServerId
                      return (
                        <li key={result.server_id}>
                          <button
                            type="button"
                            onClick={() => setFocusedServerId(result.server_id)}
                            className={`w-full px-3 py-2 text-left text-sm transition-colors hover:bg-accent ${
                              focused ? "bg-accent/60" : ""
                            }`}
                          >
                            <div className="flex items-center justify-between gap-2">
                              <span className="truncate font-medium">
                                {server?.name ?? result.server_id}
                              </span>
                              {renderStatusBadge(result)}
                            </div>
                            {result.error && result.status === "failed" && (
                              <p
                                className="mt-0.5 truncate text-xs text-destructive"
                                title={result.error}
                              >
                                {t("batch.errorPrefix", { message: result.error })}
                              </p>
                            )}
                          </button>
                        </li>
                      )
                    })}
                  </ul>
                )}
              </div>

              {/* Focused Server terminal pane */}
              <div className="grid min-w-0 gap-2 sm:grid-rows-[minmax(0,1fr)_auto]">
                <div className="relative flex h-80 min-h-0 flex-col rounded-md border border-border bg-black/90 p-3 sm:h-auto">
                  <div className="mb-2 flex items-center justify-between gap-2">
                    <span className="truncate text-xs font-medium text-zinc-300">
                      {focusedResult
                        ? serversByName.get(focusedResult.server_id)?.name ??
                          focusedResult.server_id
                        : ""}
                    </span>
                    {focusedResult && (
                      <span className="flex shrink-0 items-center gap-2">
                        <span className="text-xs text-zinc-500">
                          {renderMeta(focusedResult)}
                          {focusedResult.truncated && ` · ${t("batch.truncated")}`}
                        </span>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-6 text-zinc-500 hover:text-zinc-200"
                          onClick={() => void handleCopyOutput()}
                          aria-label={copied ? t("batch.copied") : t("batch.copyOutput")}
                        >
                          {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                        </Button>
                      </span>
                    )}
                  </div>
                  <pre
                    ref={outputRef}
                    onScroll={handleOutputScroll}
                    data-testid="batch-output"
                    className="min-h-0 flex-1 overflow-y-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-zinc-100"
                  >
                    {focusedResult?.output || ""}
                  </pre>
                  {/* Jump-to-bottom affordance while following is paused */}
                  {!stickToBottom && focusedResult && focusedResult.output !== "" && (
                    <button
                      type="button"
                      onClick={handleJumpToBottom}
                      aria-label={t("batch.jumpToBottom")}
                      className="absolute right-2.5 bottom-2.5 rounded-full border border-border bg-background/90 p-1.5 text-muted-foreground shadow-sm transition-colors hover:text-foreground"
                    >
                      <ArrowDown className="size-3.5" />
                    </button>
                  )}
                </div>

                {/* Interactive stdin — enabled only while the focused Server's
                    process is still running */}
                <form onSubmit={handleInputSubmit} className="flex gap-2">
                  <Input
                    value={inputText}
                    onChange={(e) => setInputText(e.target.value)}
                    disabled={!focusedRunning}
                    placeholder={
                      focusedRunning
                        ? t("batch.inputPlaceholder", {
                            name: focusedResult
                              ? serversByName.get(focusedResult.server_id)?.name ??
                                focusedResult.server_id
                              : "",
                          })
                        : t("batch.inputDisabled")
                    }
                    className="font-mono"
                  />
                  <Button
                    type="submit"
                    size="icon"
                    disabled={!focusedRunning || inputText === ""}
                    aria-label={t("batch.send")}
                  >
                    <CornerDownLeft className="size-4" />
                  </Button>
                </form>
              </div>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
