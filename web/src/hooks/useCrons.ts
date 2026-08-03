import { useReducer, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { getCrons, updateCron, deleteCron, runCron } from "@/api/client"
import type { CronEntry, CronResult, CronRunResult } from "@/types/server"

// --- State ---

export interface CronState {
  addCronOpen: boolean
  cronsLoading: boolean
  cronsError: string | null
  cronsResult: CronResult | null
  cronsFetched: boolean
  editEntry: CronEntry | undefined
  editIndex: number | undefined
  deleteConfirmIndex: number | null
  rowErrors: Record<number, string>
  runningIndex: number | null
  runResult: { command: string; result: CronRunResult } | null
}

export const initialCronState: CronState = {
  addCronOpen: false,
  cronsLoading: false,
  cronsError: null,
  cronsResult: null,
  cronsFetched: false,
  editEntry: undefined,
  editIndex: undefined,
  deleteConfirmIndex: null,
  rowErrors: {},
  runningIndex: null,
  runResult: null,
}

// --- Actions ---

export type CronAction =
  | { type: "SET_ADD_CRON_OPEN"; open: boolean }
  | { type: "SET_EDIT_ENTRY"; entry: CronEntry | undefined }
  | { type: "SET_EDIT_INDEX"; index: number | undefined }
  | { type: "SET_DELETE_CONFIRM_INDEX"; index: number | null }
  | { type: "SET_RUN_RESULT"; result: { command: string; result: CronRunResult } | null }
  | { type: "SET_RUNNING_INDEX"; index: number | null }
  | { type: "SET_ROW_ERROR"; index: number; msg: string }
  | { type: "CLEAR_ROW_ERROR"; index: number }
  | { type: "FETCH_START" }
  | { type: "FETCH_SUCCESS"; data: CronResult }
  | { type: "FETCH_ERROR"; error: string }
  | { type: "SET_CRONS_FETCHED" }
  | { type: "CRON_CREATED"; entry: CronEntry }
  | { type: "TOGGLE_OPTIMISTIC"; index: number }
  | { type: "TOGGLE_CONFIRM"; index: number; updated: CronEntry }
  | { type: "TOGGLE_REVERT"; index: number; original: CronEntry }
  | { type: "DELETE_CRON"; index: number }

// --- Reducer ---

export function cronReducer(state: CronState, action: CronAction): CronState {
  switch (action.type) {
    case "SET_ADD_CRON_OPEN":
      return { ...state, addCronOpen: action.open }

    case "SET_EDIT_ENTRY":
      return { ...state, editEntry: action.entry }

    case "SET_EDIT_INDEX":
      return { ...state, editIndex: action.index }

    case "SET_DELETE_CONFIRM_INDEX":
      return { ...state, deleteConfirmIndex: action.index }

    case "SET_RUN_RESULT":
      return { ...state, runResult: action.result }

    case "SET_RUNNING_INDEX":
      return { ...state, runningIndex: action.index }

    case "SET_ROW_ERROR":
      return {
        ...state,
        rowErrors: { ...state.rowErrors, [action.index]: action.msg },
      }

    case "CLEAR_ROW_ERROR": {
      const { [action.index]: _, ...rest } = state.rowErrors
      return { ...state, rowErrors: rest }
    }

    case "FETCH_START":
      return { ...state, cronsLoading: true, cronsError: null }

    case "FETCH_SUCCESS":
      return { ...state, cronsResult: action.data, cronsLoading: false }

    case "FETCH_ERROR":
      return { ...state, cronsError: action.error, cronsLoading: false }

    case "SET_CRONS_FETCHED":
      return { ...state, cronsFetched: true }

    case "CRON_CREATED": {
      if (state.editIndex !== undefined && state.cronsResult) {
        return {
          ...state,
          cronsResult: {
            ...state.cronsResult,
            entries: state.cronsResult.entries.map((e, i) =>
              i === state.editIndex ? action.entry : e
            ),
          },
          editEntry: undefined,
          editIndex: undefined,
        }
      }
      return {
        ...state,
        cronsResult: state.cronsResult
          ? { ...state.cronsResult, entries: [...state.cronsResult.entries, action.entry] }
          : { entries: [action.entry], passthroughs: [] },
        editEntry: undefined,
        editIndex: undefined,
      }
    }

    case "TOGGLE_OPTIMISTIC": {
      if (!state.cronsResult) return state
      return {
        ...state,
        cronsResult: {
          ...state.cronsResult,
          entries: state.cronsResult.entries.map((e, i) =>
            i === action.index ? { ...e, enabled: !e.enabled } : e
          ),
        },
      }
    }

    case "TOGGLE_CONFIRM": {
      if (!state.cronsResult) return state
      return {
        ...state,
        cronsResult: {
          ...state.cronsResult,
          entries: state.cronsResult.entries.map((e, i) =>
            i === action.index ? action.updated : e
          ),
        },
      }
    }

    case "TOGGLE_REVERT": {
      if (!state.cronsResult) return state
      return {
        ...state,
        cronsResult: {
          ...state.cronsResult,
          entries: state.cronsResult.entries.map((e, i) =>
            i === action.index ? action.original : e
          ),
        },
      }
    }

    case "DELETE_CRON": {
      if (!state.cronsResult) return state
      const entries = state.cronsResult.entries.filter((_, i) => i !== action.index)
      const rowErrors: Record<number, string> = {}
      for (const [k, v] of Object.entries(state.rowErrors)) {
        const ki = parseInt(k, 10)
        if (ki < action.index) rowErrors[ki] = v
        else if (ki > action.index) rowErrors[ki - 1] = v
      }
      return {
        ...state,
        cronsResult: { ...state.cronsResult, entries },
        rowErrors,
      }
    }

    default:
      return state
  }
}

// --- Hook ---

export function useCrons(serverId: string) {
  const { t } = useTranslation()

  const [state, dispatch] = useReducer(cronReducer, initialCronState)

  function fetchCrons() {
    if (!serverId) return
    dispatch({ type: "FETCH_START" })
    getCrons(serverId)
      .then((result) => dispatch({ type: "FETCH_SUCCESS", data: result }))
      .catch((err: Error) => dispatch({ type: "FETCH_ERROR", error: err.message }))
  }

  useEffect(() => {
    if (!serverId || state.cronsFetched) return
    dispatch({ type: "SET_CRONS_FETCHED" })
    fetchCrons()
  }, [serverId, state.cronsFetched])

  function handleCronCreated(entry: CronEntry) {
    dispatch({ type: "CRON_CREATED", entry })
  }

  function handleOpenAdd() {
    dispatch({ type: "SET_EDIT_ENTRY", entry: undefined })
    dispatch({ type: "SET_EDIT_INDEX", index: undefined })
    dispatch({ type: "SET_ADD_CRON_OPEN", open: true })
  }

  function handleOpenEdit(entry: CronEntry, index: number) {
    dispatch({ type: "SET_EDIT_ENTRY", entry })
    dispatch({ type: "SET_EDIT_INDEX", index })
    dispatch({ type: "SET_ADD_CRON_OPEN", open: true })
  }

  async function handleRunCron(index: number) {
    if (!serverId || !state.cronsResult) return
    const entry = state.cronsResult.entries[index]
    dispatch({ type: "CLEAR_ROW_ERROR", index })
    dispatch({ type: "SET_RUNNING_INDEX", index })
    try {
      const result = await runCron(serverId, index)
      dispatch({ type: "SET_RUN_RESULT", result: { command: entry.command, result } })
    } catch (err) {
      dispatch({
        type: "SET_ROW_ERROR",
        index,
        msg: err instanceof Error ? err.message : t("cronJobs.runFailed"),
      })
    } finally {
      dispatch({ type: "SET_RUNNING_INDEX", index: null })
    }
  }

  async function handleToggle(index: number) {
    if (!serverId || !state.cronsResult) return
    const entry = state.cronsResult.entries[index]
    dispatch({ type: "CLEAR_ROW_ERROR", index })
    dispatch({ type: "TOGGLE_OPTIMISTIC", index })
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
      dispatch({ type: "TOGGLE_CONFIRM", index, updated })
    } catch (err) {
      dispatch({ type: "TOGGLE_REVERT", index, original: entry })
      dispatch({
        type: "SET_ROW_ERROR",
        index,
        msg: err instanceof Error ? err.message : t("cronJobs.toggleFailed"),
      })
    }
  }

  async function handleDeleteConfirmed() {
    if (!serverId || state.deleteConfirmIndex === null || !state.cronsResult) return
    const index = state.deleteConfirmIndex
    dispatch({ type: "SET_DELETE_CONFIRM_INDEX", index: null })
    dispatch({ type: "CLEAR_ROW_ERROR", index })
    try {
      await deleteCron(serverId, index)
      dispatch({ type: "DELETE_CRON", index })
    } catch (err) {
      dispatch({
        type: "SET_ROW_ERROR",
        index,
        msg: err instanceof Error ? err.message : t("cronJobs.deleteRowFailed"),
      })
    }
  }

  // Wrapper functions preserving the original return signature
  const setAddCronOpen = (open: boolean) => dispatch({ type: "SET_ADD_CRON_OPEN", open })
  const setDeleteConfirmIndex = (index: number | null) =>
    dispatch({ type: "SET_DELETE_CONFIRM_INDEX", index })
  const setRunResult = (result: { command: string; result: CronRunResult } | null) =>
    dispatch({ type: "SET_RUN_RESULT", result })
  const setEditEntry = (entry: CronEntry | undefined) =>
    dispatch({ type: "SET_EDIT_ENTRY", entry })
  const setEditIndex = (index: number | undefined) =>
    dispatch({ type: "SET_EDIT_INDEX", index })

  return {
    cronsLoading: state.cronsLoading,
    cronsError: state.cronsError,
    cronsResult: state.cronsResult,
    rowErrors: state.rowErrors,
    runningIndex: state.runningIndex,
    runResult: state.runResult,
    deleteConfirmIndex: state.deleteConfirmIndex,
    editEntry: state.editEntry,
    editIndex: state.editIndex,
    addCronOpen: state.addCronOpen,
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
