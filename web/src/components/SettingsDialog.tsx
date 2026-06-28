import { useState } from "react"
import { Settings } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import * as api from "@/api/client"
import type { AppConfig } from "@/types/server"

const LANGUAGES = [
  { code: "en", label: "English" },
  { code: "zh", label: "中文" },
  { code: "fr", label: "Français" },
  { code: "ru", label: "Русский" },
] as const

// Fields that require a restart when changed.
const RESTART_FIELDS: (keyof AppConfig)[] = ["port", "data_dir"]

export function SettingsDialog() {
  const { t, i18n } = useTranslation()
  const [open, setOpen] = useState(false)
  const [initial, setInitial] = useState<AppConfig | null>(null)
  const [form, setForm] = useState<AppConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [restartFields, setRestartFields] = useState<string[]>([])

  async function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setRestartFields([])
      try {
        const cfg = await api.getConfig()
        setInitial(cfg)
        setForm(cfg)
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("settings.saveFailed"))
        setOpen(false)
      }
    }
  }

  function updateField<K extends keyof AppConfig>(key: K, value: AppConfig[K]) {
    setForm((prev) => prev ? { ...prev, [key]: value } : prev)
  }

  async function handleSave() {
    if (!form || !initial) return

    // Build diff: only changed fields
    const patch: Partial<AppConfig> = {}
    for (const k of Object.keys(form) as (keyof AppConfig)[]) {
      if (form[k] !== initial[k]) {
        // @ts-expect-error assigning heterogeneous values
        patch[k] = form[k]
      }
    }

    if (Object.keys(patch).length === 0) {
      setOpen(false)
      return
    }

    setSaving(true)
    try {
      const result = await api.patchConfig(patch)

      if (result.restart_required) {
        // Collect which restart-required fields were in the patch
        const changed = RESTART_FIELDS.filter((f) => f in patch).map((f) =>
          f === "port" ? t("settings.port") : t("settings.dataDir"),
        )
        setRestartFields(changed)
      } else {
        setRestartFields([])
      }

      if (patch.language && patch.language !== initial.language) {
        i18n.changeLanguage(patch.language as string)
      }

      setInitial(result)
      toast.success(t("settings.savedSuccess"))

      if (!result.restart_required) {
        setOpen(false)
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("settings.title")}>
          <Settings className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("settings.title")}</DialogTitle>
        </DialogHeader>

        {form && (
          <div className="grid gap-4 py-2">
            {/* Language */}
            <div className="grid gap-1.5">
              <Label htmlFor="settings-language">{t("settings.language")}</Label>
              <Select
                value={form.language}
                onValueChange={(val) => updateField("language", val)}
              >
                <SelectTrigger id="settings-language" aria-label={t("settings.language")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {LANGUAGES.map(({ code, label }) => (
                    <SelectItem key={code} value={code}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Log path */}
            <div className="grid gap-1.5">
              <Label htmlFor="settings-log-path">{t("settings.logPath")}</Label>
              <Input
                id="settings-log-path"
                value={form.log_path}
                placeholder={t("settings.logPathPlaceholder")}
                onChange={(e) => updateField("log_path", e.target.value)}
              />
            </div>

            {/* Log retention */}
            <div className="grid gap-1.5">
              <Label htmlFor="settings-log-retention">{t("settings.logRetentionDays")}</Label>
              <Input
                id="settings-log-retention"
                type="number"
                value={form.log_retention_days}
                onChange={(e) => updateField("log_retention_days", Number(e.target.value))}
              />
            </div>

            {/* Port */}
            <div className="grid gap-1.5">
              <Label htmlFor="settings-port">{t("settings.port")}</Label>
              <Input
                id="settings-port"
                type="number"
                value={form.port}
                onChange={(e) => updateField("port", Number(e.target.value))}
              />
            </div>

            {/* Data directory */}
            <div className="grid gap-1.5">
              <Label htmlFor="settings-data-dir">{t("settings.dataDir")}</Label>
              <Input
                id="settings-data-dir"
                value={form.data_dir}
                onChange={(e) => updateField("data_dir", e.target.value)}
              />
            </div>

            {/* Restart notice */}
            {restartFields.length > 0 && (
              <div
                data-testid="restart-notice"
                className="rounded-md border border-yellow-500 bg-yellow-50 p-3 text-sm text-yellow-800 dark:border-yellow-600 dark:bg-yellow-950 dark:text-yellow-200"
              >
                <p className="font-medium">{t("settings.restartRequired")}</p>
                <ul className="mt-1 list-disc pl-4">
                  {restartFields.map((f) => (
                    <li key={f}>{f}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button onClick={handleSave} disabled={saving || !form}>
            {saving ? t("settings.saving") : t("settings.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
