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
import * as api from "@/api/client"
import { ApiError } from "@/api/client"

export interface DeleteServerDialogProps {
  server: { id: string; name: string }
  open: boolean
  onOpenChange: (open: boolean) => void
  onDeleted: (serverId: string) => void
}

export function DeleteServerDialog({
  server,
  open,
  onOpenChange,
  onDeleted,
}: DeleteServerDialogProps) {
  const { t } = useTranslation()
  const [deleting, setDeleting] = useState(false)

  async function handleDelete() {
    setDeleting(true)
    try {
      await api.deleteServer(server.id)
    } catch (err) {
      if (err instanceof ApiError && err.code === "not_found") {
        // Already deleted — treat as success
      } else {
        toast.error(err instanceof Error ? err.message : t("deleteServer.deleteFailed"))
        setDeleting(false)
        return
      }
    }
    toast.success(t("deleteServer.deletedSuccess", { name: server.name }))
    onDeleted(server.id)
    onOpenChange(false)
    setDeleting(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{t("deleteServer.title")}</DialogTitle>
          <DialogDescription>
            <Trans
              i18nKey="deleteServer.confirm"
              values={{ name: server.name }}
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
            {deleting ? t("deleteServer.deleting") : t("common.delete")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
