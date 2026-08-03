import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
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
import { ApiError } from "@/api/client"

// GroupNameDialog is the shared in-app dialog for creating and renaming a
// Group, replacing the old window.prompt usage. It validates the name inline:
// the Save button is disabled while the name is empty, and a sibling
// duplicate (409 duplicate_name from the backend) is shown as an inline error.
// When initialName is provided the input is pre-filled and its value is
// selected so a replacement can be typed directly.
export function GroupNameDialog({
  open,
  onOpenChange,
  title,
  initialName = "",
  genericErrorKey,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  initialName?: string
  genericErrorKey: string
  onSubmit: (name: string) => Promise<void>
}) {
  const { t } = useTranslation()
  const [name, setName] = useState(initialName)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  function handleOpenChange(next: boolean) {
    if (saving) return
    onOpenChange(next)
    if (!next) {
      setName("")
      setError(null)
    }
  }

  async function handleSave() {
    const trimmed = name.trim()
    if (!trimmed || saving) return
    setSaving(true)
    setError(null)
    try {
      await onSubmit(trimmed)
      onOpenChange(false)
    } catch (err) {
      if (err instanceof ApiError && err.code === "duplicate_name") {
        setError(t("groups.nameDuplicate"))
      } else {
        toast.error(t(genericErrorKey))
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        onOpenAutoFocus={(e) => {
          e.preventDefault()
          inputRef.current?.focus()
          inputRef.current?.select()
        }}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-2">
          <Label htmlFor="group-name-input">{t("groups.namePlaceholder")}</Label>
          <Input
            id="group-name-input"
            ref={inputRef}
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              if (error) setError(null)
            }}
            aria-invalid={error ? true : undefined}
            disabled={saving}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleSave()
            }}
          />
          {error && (
            <p className="text-sm text-destructive" role="alert">
              {error}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={saving}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={!name.trim() || saving}>
            {t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
