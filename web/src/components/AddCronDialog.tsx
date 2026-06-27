import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import * as api from "@/api/client"
import type { CronEntry } from "@/types/server"

// Validates a cron schedule field: digits, *, /, -, ,
function isValidCronField(value: string): boolean {
  return value.trim() !== "" && /^[\d*/,\-]+$/.test(value)
}

function describeCronSchedule(
  t: ReturnType<typeof useTranslation>["t"],
  minute: string,
  hour: string,
  dom: string,
  month: string,
  dow: string,
): string {
  const allWild = dom === "*" && month === "*"

  if (minute === "*" && hour === "*" && allWild && dow === "*") {
    return t("cronJobs.scheduleEveryMinute")
  }
  if (/^\*\/(\d+)$/.test(minute) && hour === "*" && allWild && dow === "*") {
    const n = minute.slice(2)
    return t("cronJobs.scheduleEveryNMinutes", { n })
  }
  if (minute === "0" && /^\*\/(\d+)$/.test(hour) && allWild && dow === "*") {
    const n = hour.slice(2)
    return t("cronJobs.scheduleEveryNHours", { n })
  }
  if (minute === "0" && hour === "*" && allWild && dow === "*") {
    return t("cronJobs.scheduleEveryHour")
  }
  if (/^\d+$/.test(minute) && /^\d+$/.test(hour) && allWild && dow === "*") {
    const time = `${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`
    return t("cronJobs.scheduleAtTimeDaily", { time })
  }
  if (/^\d+$/.test(minute) && /^\d+$/.test(hour) && allWild && /^\d+$/.test(dow)) {
    const time = `${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`
    const day = t(`cronJobs.dayNames.${dow}`, { defaultValue: `day ${dow}` })
    return t("cronJobs.scheduleAtTimeDow", { time, day })
  }
  return t("cronJobs.scheduleCustom")
}

export interface AddCronDialogProps {
  serverId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (entry: CronEntry) => void
  /** When provided, dialog operates in edit mode. */
  editEntry?: CronEntry
  editIndex?: number
}

const defaultFields = { minute: "*", hour: "*", dayOfMonth: "*", month: "*", dayOfWeek: "*", command: "" }

export function AddCronDialog({ serverId, open, onOpenChange, onCreated, editEntry, editIndex }: AddCronDialogProps) {
  const { t } = useTranslation()
  const isEditMode = editEntry !== undefined && editIndex !== undefined
  const [fields, setFields] = useState({ ...defaultFields })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Sync fields when editEntry changes (dialog opened for edit)
  useEffect(() => {
    if (open && editEntry) {
      setFields({
        minute: editEntry.minute,
        hour: editEntry.hour,
        dayOfMonth: editEntry.dayOfMonth,
        month: editEntry.month,
        dayOfWeek: editEntry.dayOfWeek,
        command: editEntry.command,
      })
      setError(null)
    } else if (open && !editEntry) {
      setFields({ ...defaultFields })
      setError(null)
    }
  }, [open, editEntry])

  function handleOpenChange(next: boolean) {
    if (!saving) {
      onOpenChange(next)
      if (!next) {
        setFields({ ...defaultFields })
        setError(null)
      }
    }
  }

  function setField(key: keyof typeof fields, value: string) {
    setFields((prev) => ({ ...prev, [key]: value }))
  }

  const scheduleFields: Array<keyof typeof fields> = ["minute", "hour", "dayOfMonth", "month", "dayOfWeek"]

  function isFormValid(): boolean {
    return (
      scheduleFields.every((f) => isValidCronField(fields[f])) &&
      fields.command.trim() !== ""
    )
  }

  async function handleSave() {
    setSaving(true)
    setError(null)
    try {
      let entry: CronEntry
      if (isEditMode) {
        entry = await api.updateCron(serverId, editIndex!, {
          minute: fields.minute,
          hour: fields.hour,
          dayOfMonth: fields.dayOfMonth,
          month: fields.month,
          dayOfWeek: fields.dayOfWeek,
          command: fields.command,
          enabled: editEntry!.enabled,
        })
      } else {
        entry = await api.createCron(serverId, {
          minute: fields.minute,
          hour: fields.hour,
          dayOfMonth: fields.dayOfMonth,
          month: fields.month,
          dayOfWeek: fields.dayOfWeek,
          command: fields.command,
        })
      }
      onCreated(entry)
      onOpenChange(false)
      setFields({ ...defaultFields })
    } catch (err) {
      setError(err instanceof Error ? err.message : t("cronJobs.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  const schedulePreview = describeCronSchedule(
    t,
    fields.minute,
    fields.hour,
    fields.dayOfMonth,
    fields.month,
    fields.dayOfWeek,
  )

  const fieldLabels: Record<keyof typeof fields, string> = {
    minute: t("cronJobs.fieldMinute"),
    hour: t("cronJobs.fieldHour"),
    dayOfMonth: t("cronJobs.fieldDayOfMonth"),
    month: t("cronJobs.fieldMonth"),
    dayOfWeek: t("cronJobs.fieldDayOfWeek"),
    command: t("cronJobs.fieldCommand"),
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEditMode ? t("cronJobs.editDialogTitle") : t("cronJobs.addDialogTitle")}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Schedule fields row */}
          <div className="grid grid-cols-5 gap-2">
            {scheduleFields.map((key) => {
              const invalid = !isValidCronField(fields[key])
              return (
                <div key={key} className="space-y-1">
                  <Label htmlFor={`cron-field-${key}`} className="text-xs">
                    {fieldLabels[key]}
                  </Label>
                  <Input
                    id={`cron-field-${key}`}
                    aria-label={fieldLabels[key]}
                    value={fields[key]}
                    onChange={(e) => setField(key, e.target.value)}
                    className={invalid ? "border-destructive" : ""}
                  />
                </div>
              )
            })}
          </div>

          {/* Schedule preview */}
          <p
            data-testid="schedule-preview"
            className="text-xs text-muted-foreground"
          >
            {schedulePreview}
          </p>

          {/* Command field */}
          <div className="space-y-1">
            <Label htmlFor="cron-field-command">{fieldLabels.command}</Label>
            <Input
              id="cron-field-command"
              aria-label={fieldLabels.command}
              value={fields.command}
              onChange={(e) => setField("command", e.target.value)}
              placeholder="/usr/bin/script.sh"
              className={fields.command.trim() === "" && fields.command !== "" ? "border-destructive" : ""}
            />
          </div>

          {/* Inline API error */}
          {error && (
            <p
              data-testid="add-cron-error"
              className="text-sm text-destructive"
            >
              {error}
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={saving}>
            {t("cronJobs.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={!isFormValid() || saving}>
            {saving ? t("cronJobs.saving") : t("cronJobs.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
