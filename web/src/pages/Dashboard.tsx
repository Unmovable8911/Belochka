import { useCallback, useMemo, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { ServerIcon, Home, ChevronRight, Pencil, Trash2, Terminal } from "lucide-react"
import { useTranslation } from "react-i18next"
import { AddServerDialog } from "@/components/AddServerDialog"
import { ServerCard } from "@/components/ServerCard"
import { ContextMenuOverlay, type ContextMenuItem } from "@/components/ContextMenu"
import { EditServerDialog } from "@/components/EditServerDialog"
import { DeleteServerDialog } from "@/components/DeleteServerDialog"
import { useMonitorState } from "@/hooks/useMonitorState"
import { useGroups } from "@/hooks/useGroups"
import { toast } from "sonner"
import type { Server, ServerInfo } from "@/types/server"
import * as api from "@/api/client"

// --- Component ---

export default function Dashboard() {
  const { t } = useTranslation()
  const { state, dispatch } = useMonitorState()
  const [searchParams] = useSearchParams()
  const { groups } = useGroups(state.servers)

  // Right-click menu on the server cards. State is local to the Dashboard
  // because its content area has no CSS transform creating a containing
  // block (unlike the Sidebar's <aside>, which is why that menu state lives
  // in the Layout shell).
  const [contextMenu, setContextMenu] = useState<{
    x: number
    y: number
    items: ContextMenuItem[]
  } | null>(null)

  // editTarget holds the full Server fetched from the API (the dashboard
  // state only carries id/name/host/group_id); null means the dialog is closed.
  const [editTarget, setEditTarget] = useState<Server | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null)

  // Edit loads the full record so SSH credentials and config are shown,
  // mirroring the detail page's Edit flow. On failure, surface an error toast.
  const handleEdit = useCallback(async (serverId: string) => {
    try {
      const server = await api.getServer(serverId)
      setEditTarget(server)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("serverDetail.failedToLoad"))
    }
  }, [t])

  const handleDelete = useCallback((server: { id: string; name: string }) => {
    setDeleteTarget({ id: server.id, name: server.name })
  }, [])

  // Console opens in a new browser tab, matching the detail page's Console
  // button, so the user keeps their Dashboard filter and scroll position.
  const handleConsole = useCallback((serverId: string) => {
    window.open(`/server/${serverId}/console`, "_blank")
  }, [])

  // Right-clicking a card builds the Edit/Delete/Console menu. All callbacks
  // are stable (useCallback), so ServerCard's React.memo keeps working.
  // preventDefault/stopPropagation happen in ServerCard itself; the event is
  // used here only for its coordinates.
  const handleCardContextMenu = useCallback((e: React.MouseEvent, server: ServerInfo) => {
    setContextMenu({
      x: e.clientX,
      y: e.clientY,
      items: [
        {
          label: t("serverDetail.edit"),
          icon: <Pencil className="size-4" />,
          onClick: () => handleEdit(server.id),
        },
        {
          label: t("serverDetail.delete"),
          icon: <Trash2 className="size-4" />,
          destructive: true,
          onClick: () => handleDelete({ id: server.id, name: server.name }),
        },
        {
          label: t("serverDetail.console"),
          icon: <Terminal className="size-4" />,
          onClick: () => handleConsole(server.id),
        },
      ],
    })
  }, [t, handleEdit, handleDelete, handleConsole])

  const groupIdFilter = searchParams.get("group_id")
  const isFiltering = !!groupIdFilter

  // Filter servers based on URL params. Groups are flat, so filtering by a
  // group matches its direct members only.
  const filteredServers = useMemo(() => {
    if (groupIdFilter) {
      return state.servers.filter((s) => s.group_id === groupIdFilter)
    }
    return state.servers
  }, [state.servers, groupIdFilter])

  // The currently-filtered group, for the breadcrumb label.
  const filteredGroup = useMemo(
    () => (groupIdFilter ? groups.find((g) => g.id === groupIdFilter) : undefined),
    [groupIdFilter, groups]
  )

  const hasServers = filteredServers.length > 0
  const totalHasServers = state.servers.length > 0

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold">{t("dashboard.title")}</h1>
        <div className="flex items-center gap-2">
          {totalHasServers && (
            <AddServerDialog
              groups={groups}
              defaultGroupId={groupIdFilter ?? undefined}
              onServerAdded={(s) =>
                dispatch({ type: "add_server", data: { id: s.id, name: s.name, host: s.host, group_id: s.group_id } })
              }
            />
          )}
        </div>
      </div>

      {/* Breadcrumb */}
      {isFiltering && (
        <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4" aria-label="Breadcrumb">
          <Link
            to="/"
            className="flex items-center gap-1 hover:text-foreground transition-colors"
          >
            <Home className="size-3.5" />
            <span>{t("breadcrumb.allServers")}</span>
          </Link>
          <span className="flex items-center gap-1.5">
            <ChevronRight className="size-3.5" />
            <span className="text-foreground font-medium">
              {filteredGroup?.name ?? groupIdFilter}
            </span>
          </span>
        </nav>
      )}

      {/* Server grid or empty state */}
      {hasServers ? (
        <div
          data-testid="server-grid"
          className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,20rem),1fr))] gap-4"
        >
          {filteredServers.map((server) => (
            <ServerCard
              key={server.id}
              server={server}
              metrics={state.metrics[server.id]}
              onContextMenu={handleCardContextMenu}
            />
          ))}
        </div>
      ) : isFiltering ? (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <ServerIcon className="size-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold mb-2">{t("dashboard.noServers")}</h2>
          <p className="text-muted-foreground">This group has no servers.</p>
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <ServerIcon className="size-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold mb-2">{t("dashboard.noServers")}</h2>
          <p className="text-muted-foreground mb-6">
            {t("dashboard.noServersHint")}
          </p>
          <AddServerDialog
            triggerLabel={t("dashboard.addFirstServer")}
            groups={groups}
            onServerAdded={(s) =>
              dispatch({ type: "add_server", data: { id: s.id, name: s.name, host: s.host, group_id: s.group_id } })
            }
          />
        </div>
      )}

      {/* Card context menu */}
      {contextMenu && (
        <ContextMenuOverlay {...contextMenu} onClose={() => setContextMenu(null)} />
      )}

      {/* Edit / delete dialogs reached from the context menu */}
      {editTarget && (
        <EditServerDialog
          server={editTarget}
          open
          onOpenChange={(open) => { if (!open) setEditTarget(null) }}
          groups={groups}
          onServerUpdated={(updated) =>
            dispatch({ type: "update_server", data: { serverId: updated.id, name: updated.name, host: updated.host, group_id: updated.group_id } })
          }
        />
      )}
      {deleteTarget && (
        <DeleteServerDialog
          server={deleteTarget}
          open
          onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}
          onDeleted={(serverId) => {
            dispatch({ type: "remove_server", data: { serverId } })
            setDeleteTarget(null)
          }}
        />
      )}
    </div>
  )
}
