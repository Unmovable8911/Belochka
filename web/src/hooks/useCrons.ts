import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { getCrons, updateCron, deleteCron, runCron } from "@/api/client"
import type { CronEntry, CronResult, CronRunResult } from "@/types/server"

export function useCrons(serverId: string) {
  const { t } = useTranslation()

  const [addCronOpen, setAddCronOpen] = useState(false)

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

  function fetchCrons() {
    if (!serverId) return
    setCronsLoading(true)
    setCronsError(null)
    getCrons(serverId)
      .then((result) => setCronsResult(result))
      .catch((err: Error) => setCronsError(err.message))
      .finally(() => setCronsLoading(false))
  }

  useEffect(() => {
    if (!serverId || cronsFetched) return
    setCronsFetched(true)
    fetchCrons()
  }, [serverId, cronsFetched])

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
    if (!serverId || !cronsResult) return
    const entry = cronsResult.entries[index]
    clearRowError(index)
    setRunningIndex(index)
    try {
      const result = await runCron(serverId, index)
      setRunResult({ command: entry.command, result })
    } catch (err) {
      setRowError(index, err instanceof Error ? err.message : t("cronJobs.runFailed"))
    } finally {
      setRunningIndex(null)
    }
  }

  async function handleToggle(index: number) {
    if (!serverId || !cronsResult) return
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
      const updated = await updateCron(serverId, index, {
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
    if (!serverId || deleteConfirmIndex === null || !cronsResult) return
    const index = deleteConfirmIndex
    setDeleteConfirmIndex(null)
    clearRowError(index)
    try {
      await deleteCron(serverId, index)
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

  return {
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
  }
}
