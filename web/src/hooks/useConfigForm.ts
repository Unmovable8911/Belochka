import { useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import * as api from "@/api/client"
import type { AppConfig } from "@/types/server"

// Fields that require a restart when changed.
const RESTART_FIELDS: (keyof AppConfig)[] = ["port", "data_dir"]

export interface UseConfigFormReturn {
  initial: AppConfig | null
  form: AppConfig | null
  saving: boolean
  restartFields: string[]
  showRestartDialog: boolean
  restarting: boolean
  /** Fetches config from the API. Returns true on success. */
  loadConfig: () => Promise<boolean>
  updateField: <K extends keyof AppConfig>(key: K, value: AppConfig[K]) => void
  /** Saves the diff. Returns true if no restart is required (caller may close the dialog). */
  handleSave: () => Promise<boolean>
  handleRestart: () => Promise<void>
  dismissRestartDialog: () => void
}

export function useConfigForm(
  i18n: { changeLanguage: (lang: string) => Promise<unknown> },
): UseConfigFormReturn {
  const { t } = useTranslation()
  const [initial, setInitial] = useState<AppConfig | null>(null)
  const [form, setForm] = useState<AppConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [restartFields, setRestartFields] = useState<string[]>([])
  const [showRestartDialog, setShowRestartDialog] = useState(false)
  const [restarting, setRestarting] = useState(false)

  const loadConfig = useCallback(async (): Promise<boolean> => {
    setRestartFields([])
    setShowRestartDialog(false)
    try {
      const cfg = await api.getConfig()
      setInitial(cfg)
      setForm(cfg)
      return true
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.saveFailed"))
      return false
    }
  }, [t])

  function updateField<K extends keyof AppConfig>(key: K, value: AppConfig[K]) {
    setForm((prev) => (prev ? { ...prev, [key]: value } : prev))
  }

  const handleSave = useCallback(async (): Promise<boolean> => {
    if (!form || !initial) return false

    // Build diff: only changed fields
    const patch: Partial<AppConfig> = {}
    for (const k of Object.keys(form) as (keyof AppConfig)[]) {
      if (form[k] !== initial[k]) {
        // @ts-expect-error assigning heterogeneous values
        patch[k] = form[k]
      }
    }

    if (Object.keys(patch).length === 0) {
      return true // nothing to save, treat as "can close"
    }

    setSaving(true)
    try {
      const result = await api.patchConfig(patch)

      if (result.restart_required) {
        const changed = RESTART_FIELDS.filter((f) => f in patch).map((f) =>
          f === "port" ? t("settings.port") : t("settings.dataDir"),
        )
        setRestartFields(changed)
        setShowRestartDialog(true)
        toast.success(t("settings.savedSuccess"))
      } else {
        setRestartFields([])
        setShowRestartDialog(false)
        toast.success(t("settings.savedSuccess"))
      }

      if (patch.language && patch.language !== initial.language) {
        i18n.changeLanguage(patch.language as string)
      }

      setInitial(result)
      return !result.restart_required
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.saveFailed"))
      return false
    } finally {
      setSaving(false)
    }
  }, [form, initial, i18n, t])

  const handleRestart = useCallback(async () => {
    setRestarting(true)
    try {
      await api.restartServer()
      // Brief delay to let the response reach the client, then close.
      setTimeout(() => window.close(), 500)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.restartDialog.restartFailed"))
      setRestarting(false)
    }
  }, [t])

  const dismissRestartDialog = useCallback(() => {
    setShowRestartDialog(false)
    setRestartFields([])
  }, [])

  return {
    initial,
    form,
    saving,
    restartFields,
    showRestartDialog,
    restarting,
    loadConfig,
    updateField,
    handleSave,
    handleRestart,
    dismissRestartDialog,
  }
}
