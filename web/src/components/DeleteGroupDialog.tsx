import { useState } from "react"
import { useTranslation, Trans } from "react-i18next"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"

// DeleteGroupDialog confirms deleting a Group before the delete runs,
// mirroring the server-delete flow. The confirm message states the orphan-up
// consequence: the Group's content moves to its parent. The deletion itself
// is delegated to onDelete (which throws on API failure); the dialog surfaces
// a toast and stays open so the user can retry or cancel.
export function DeleteGroupDialog({
  open,
  group,
  onOpenChange,
  onDelete,
}: {
  open: boolean
  group: { id: string; name: string }
  onOpenChange: (open: boolean) => void
  onDelete: (id: string, name: string) => Promise<void>
}) {
  const { t } = useTranslation()
  const [deleting, setDeleting] = useState(false)

  async function handleDelete() {
    setDeleting(true)
    try {
      await onDelete(group.id, group.name)
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("groups.deleteFailed"))
    } finally {
      setDeleting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{t("groups.deleteGroupTitle")}</DialogTitle>
          <DialogDescription>
            <Trans
              i18nKey="groups.deleteGroupConfirm"
              values={{ name: group.name }}
              components={{ strong: <strong /> }}
            />
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={deleting}
          >
            {t("common.cancel")}
          </Button>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={deleting}
          >
            {deleting ? t("groups.deleting") : t("common.delete")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
