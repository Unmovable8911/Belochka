import { Pencil, Play, Plus, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { AlertDialog } from "radix-ui"
import { Button } from "@/components/ui/button"
import { AddCronDialog } from "@/components/AddCronDialog"
import { useCrons } from "@/hooks/useCrons"

export function CronJobsTab({ serverId }: { serverId: string }) {
  const { t } = useTranslation()
  const {
    cronsLoading,
    cronsError,
    cronsResult,
    rowErrors,
    runningIndex,
    runResult,
    deleteConfirmIndex,
    editEntry,
    editIndex,
    addCronOpen,
    setAddCronOpen,
    setDeleteConfirmIndex,
    setRunResult,
    setEditEntry,
    setEditIndex,
    handleCronCreated,
    handleOpenAdd,
    handleOpenEdit,
    handleRunCron,
    handleToggle,
    handleDeleteConfirmed,
  } = useCrons(serverId)

  return (
    <>
      <AddCronDialog
        serverId={serverId}
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

        {cronsResult && !cronsLoading && !cronsError && (
          <button
            onClick={handleOpenAdd}
            className="mt-2 w-full rounded-md border border-dashed border-border bg-transparent py-2 text-sm text-muted-foreground hover:border-foreground/40 hover:text-foreground transition-colors cursor-pointer"
          >
            <Plus className="inline size-4 mr-1 align-text-bottom" />
            {t("cronJobs.addButton")}
          </button>
        )}
      </div>
    </>
  )
}
