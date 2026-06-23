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
      const res = await fetch(`/api/servers/${server.id}`, {
        method: "DELETE",
      })

      if (res.status === 404) {
        // Server already deleted — treat as success
        toast.success(`Server "${server.name}" deleted`)
        onDeleted(server.id)
        onOpenChange(false)
        return
      }

      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error?.message || "Failed to delete server")
      }

      toast.success(`Server "${server.name}" deleted`)
      onDeleted(server.id)
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete server")
    } finally {
      setDeleting(false)
    }
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
