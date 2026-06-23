import { useState } from "react"
import { ServerIcon, Trash2Icon, PencilIcon } from "lucide-react"
import { AddServerDialog } from "@/components/AddServerDialog"
import { DeleteServerDialog } from "@/components/DeleteServerDialog"
import { EditServerDialog, type ServerData } from "@/components/EditServerDialog"
import { useMonitorState, type ServerInfo } from "@/hooks/useMonitorState"
import { Button } from "@/components/ui/button"

export default function Dashboard() {
  const { state, dispatch } = useMonitorState()
  const hasServers = state.servers.length > 0
  const [serverToDelete, setServerToDelete] = useState<ServerInfo | null>(null)
  const [serverToEdit, setServerToEdit] = useState<ServerData | null>(null)
  const [editLoading, setEditLoading] = useState<string | null>(null)

  function handleDeleted(serverId: string) {
    dispatch({ type: "remove_server", data: { serverId } })
    setServerToDelete(null)
  }

  function handleServerUpdated(updated: ServerData) {
    dispatch({
      type: "update_server",
      data: { serverId: updated.id, name: updated.name, host: updated.host },
    })
    setServerToEdit(null)
  }

  async function handleEditClick(server: ServerInfo) {
    setEditLoading(server.id)
    try {
      const res = await fetch(`/api/servers/${server.id}`)
      if (!res.ok) throw new Error("Failed to fetch server details")
      const data: ServerData = await res.json()
      setServerToEdit(data)
    } catch {
      // Silently fail for now; toast could be added
    } finally {
      setEditLoading(null)
    }
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        {hasServers && <AddServerDialog />}
      </div>
      {hasServers ? (
        <div className="space-y-2">
          {state.servers.map((server) => (
            <div
              key={server.id}
              className="flex items-center justify-between rounded-lg border p-4"
            >
              <div>
                <p className="font-medium">{server.name}</p>
                <p className="text-sm text-muted-foreground">{server.host}</p>
              </div>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleEditClick(server)}
                  disabled={editLoading === server.id}
                  aria-label={`Edit ${server.name}`}
                >
                  <PencilIcon className="size-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => setServerToDelete(server)}
                  aria-label={`Delete ${server.name}`}
                >
                  <Trash2Icon className="size-4" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <ServerIcon className="size-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold mb-2">No servers configured</h2>
          <p className="text-muted-foreground mb-6">
            Add a server to start monitoring its CPU, memory, disk, and network metrics.
          </p>
          <AddServerDialog triggerLabel="Add your first server" />
        </div>
      )}

      {serverToDelete && (
        <DeleteServerDialog
          server={serverToDelete}
          open={true}
          onOpenChange={(open) => {
            if (!open) setServerToDelete(null)
          }}
          onDeleted={handleDeleted}
        />
      )}

      {serverToEdit && (
        <EditServerDialog
          server={serverToEdit}
          open={true}
          onOpenChange={(open) => {
            if (!open) setServerToEdit(null)
          }}
          onServerUpdated={handleServerUpdated}
        />
      )}
    </div>
  )
}
