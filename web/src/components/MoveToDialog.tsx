import { useState } from "react"
import { useTranslation } from "react-i18next"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"

// MoveToDialog moves a Server to any Group or to "no group" via a select. The
// Server's current Group is pre-selected; the dialog owns the selection and
// calls onSubmit with the chosen Group id ("" = no Group). Groups are
// root-level only and cannot be moved themselves.
export function MoveToDialog({
  open,
  onOpenChange,
  targetName,
  initialGroupId,
  groupOptions,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  targetName: string
  initialGroupId: string
  groupOptions: { value: string; label: string }[]
  onSubmit: (groupId: string) => Promise<void>
}) {
  const { t } = useTranslation()
  const [selectedGroupId, setSelectedGroupId] = useState(initialGroupId)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("groups.moveTo")} — {targetName}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-2 py-4">
          <Label id="move-to-select-label">
            {t("groups.noGroup")}
          </Label>
          <Select value={selectedGroupId} onValueChange={setSelectedGroupId}>
            <SelectTrigger aria-labelledby="move-to-select-label">
              <SelectValue placeholder={t("groups.noGroup")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("groups.noGroup")}</SelectItem>
              {groupOptions.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={() => onSubmit(selectedGroupId)}>
            {t("common.save")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
