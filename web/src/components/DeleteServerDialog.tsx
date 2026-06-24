import { useState } from "react"
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
  const [deleting, setDeleting] = useState(false)

  async function handleDelete() {
    setDeleting(true)
    try {
      await api.deleteServer(server.id)
    } catch (err) {
      if (err instanceof ApiError && err.code === "not_found") {
        // Already deleted — treat as success
      } else {
        toast.error(err instanceof Error ? err.message : "Failed to delete server")
        setDeleting(false)
        return
      }
    }
    toast.success(`Server "${server.name}" deleted`)
    onDeleted(server.id)
    onOpenChange(false)
    setDeleting(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>Delete Server</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete <strong>{server.name}</strong>? This
            action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={deleting}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={deleting}
          >
            {deleting ? "Deleting..." : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
