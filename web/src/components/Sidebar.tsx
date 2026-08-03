import { useState, useCallback, useMemo, type MouseEvent } from "react"
import { useLocation, useNavigate } from "react-router-dom"
import { Settings, LogOut, Loader2, ExternalLink, Sun, Moon, Plus, Pencil, Trash2, MoveHorizontal, TerminalSquare } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useTheme } from "@/components/theme-provider"
import { useMonitorState } from "@/hooks/useMonitorState"
import { useSidebarActions, type MoveTarget } from "@/hooks/useSidebarActions"
import { useTreeState, parseDrop, ROOT_DROP } from "@/hooks/useTreeState"
import { flattenGroupsForSelect } from "@/hooks/useGroups"
import { SettingsDialog } from "@/components/SettingsDialog"
import { GroupRow, ServerRow } from "@/components/GroupRow"
import { GroupNameDialog } from "@/components/GroupNameDialog"
import { DeleteGroupDialog } from "@/components/DeleteGroupDialog"
import { BatchCommandDialog } from "@/components/BatchCommandDialog"
import { MoveToDialog } from "@/components/MoveToDialog"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"
import * as api from "@/api/client"
import type { GroupNode, ServerInfo } from "@/types/server"
import type { ContextMenuItem } from "@/components/ContextMenu"
import appIcon from "@/assets/icon.png"

// RenameTarget opens the rename dialog, holding the Group to rename.
interface RenameTarget {
  id: string
  name: string
}

// DeleteTarget opens the delete-confirm dialog for a single Group.
interface DeleteTarget {
  id: string
  name: string
}

interface SidebarProps {
  sidebarOpen: boolean
  onContextMenu: (x: number, y: number, items: ContextMenuItem[]) => void
}

export function Sidebar({ sidebarOpen, onContextMenu }: SidebarProps) {
  const { t } = useTranslation()
  const { theme, setTheme, resolvedTheme } = useTheme()
  const location = useLocation()
  const navigate = useNavigate()
  const { state } = useMonitorState()
  const { groups, groupList, loading, handleCreateGroup, handleRenameGroup, handleDeleteGroup, handleMoveItem } = useSidebarActions()
  const servers = state.servers

  // Tree expansion and drag-target state live in useTreeState (a pure reducer
  // with the seed-new-groups invariant); only dialog and orchestration state
  // stays here.
  const { expanded, dragOverId, handleToggle, handleDragOver, clearDragTarget } =
    useTreeState(groupList.map((node) => node.id))

  // Sidebar state
  const [moveTarget, setMoveTarget] = useState<MoveTarget | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)
  const [loggingOut, setLoggingOut] = useState(false)

  // URL-derived state
  const searchParams = new URLSearchParams(location.search)
  const selectedGroupId = searchParams.get("group_id")

  // Servers with no Group render directly at the root of the tree, as siblings
  // of the root-level Groups.
  const ungroupedRootServers = useMemo(
    () => servers.filter((s) => !s.group_id).sort((a, b) => a.name.localeCompare(b.name)),
    [servers]
  )

  // --- Context menu handlers ---

  const handleGroupContextMenu = useCallback((e: MouseEvent, node: GroupNode) => {
    e.preventDefault()
    e.stopPropagation()
    onContextMenu(e.clientX, e.clientY, [
      {
        label: t("groups.rename"),
        icon: <Pencil className="size-4" />,
        onClick: () => setRenameTarget({ id: node.id, name: node.name }),
      },
      {
        label: t("groups.delete"),
        icon: <Trash2 className="size-4" />,
        destructive: true,
        onClick: () => setDeleteTarget({ id: node.id, name: node.name }),
      },
    ])
  }, [t, onContextMenu])

  const handleServerContextMenu = useCallback((e: MouseEvent, server: ServerInfo) => {
    e.preventDefault()
    e.stopPropagation()
    onContextMenu(e.clientX, e.clientY, [
      {
        label: t("groups.moveTo"),
        icon: <MoveHorizontal className="size-4" />,
        onClick: () => {
          setMoveTarget({ id: server.id, name: server.name, groupId: server.group_id || "" })
        },
      },
    ])
  }, [t, onContextMenu])

  const handleRootContextMenu = useCallback((e: MouseEvent) => {
    e.preventDefault()
    onContextMenu(e.clientX, e.clientY, [
      {
        label: t("groups.newGroup"),
        icon: <Plus className="size-4" />,
        onClick: () => setCreateOpen(true),
      },
    ])
  }, [t, onContextMenu])

  // --- Group mutation handlers ---

  const handleCreateSubmit = useCallback(async (name: string) => {
    await handleCreateGroup(name)
  }, [handleCreateGroup])

  const handleRenameSubmit = useCallback(async (name: string) => {
    if (!renameTarget) return
    await handleRenameGroup(renameTarget.id, name)
  }, [renameTarget, handleRenameGroup])

  // Runs when the delete-confirm dialog is confirmed. The dialog owns the
  // failure toast; here we only perform the delete and, if the deleted Group
  // was filtering the dashboard, clear the filter so the user is not stranded
  // on a dangling ?group_id.
  const handleDeleteConfirm = useCallback(async (id: string, name: string) => {
    await handleDeleteGroup(id, name)
    if (selectedGroupId === id) {
      navigate("/")
    }
  }, [handleDeleteGroup, selectedGroupId, navigate])

  // --- Drag-and-drop handlers ---

  const handleDragOverGroup = useCallback((e: React.DragEvent, id: string) => {
    e.preventDefault()
    handleDragOver(id)
  }, [handleDragOver])

  const handleDragLeaveGroup = useCallback(() => {
    clearDragTarget()
  }, [clearDragTarget])

  // resolveDrop validates the application/json drag payload; only server
  // payloads move anything, and a malformed payload is surfaced as a toast.
  const handleDropOnGroup = useCallback(async (e: React.DragEvent, targetGroupId: string) => {
    clearDragTarget()
    const parsed = parseDrop(e.dataTransfer.getData("application/json"))
    if (!parsed.ok) {
      if (parsed.reason === "malformed") toast.error(t("groups.moveFailed"))
      return
    }
    try {
      // groupId is only used by the Move-to dialog's pre-selection; a dropped
      // Server goes straight to its target, so it is not set here.
      await handleMoveItem({ id: parsed.serverId, name: "", groupId: "" }, targetGroupId)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("groups.moveFailed"))
    }
  }, [handleMoveItem, t, clearDragTarget])

  // The whole tree area is a drop target: dropping a Server on blank space
  // moves it out of its Group (ungroups it). Group and Server rows stop
  // propagation so they stay their own targets.
  const handleDragOverRoot = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    handleDragOver(ROOT_DROP)
  }, [handleDragOver])

  const handleDragLeaveRoot = useCallback(() => {
    clearDragTarget()
  }, [clearDragTarget])

  const handleDropOnRoot = useCallback(async (e: React.DragEvent) => {
    clearDragTarget()
    const parsed = parseDrop(e.dataTransfer.getData("application/json"))
    if (!parsed.ok) {
      if (parsed.reason === "malformed") toast.error(t("groups.moveFailed"))
      return
    }
    try {
      await handleMoveItem({ id: parsed.serverId, name: "", groupId: "" }, "")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("groups.moveFailed"))
    }
  }, [handleMoveItem, t, clearDragTarget])

  // --- Move-to handler ---

  // Called by MoveToDialog with the chosen Group id ("" = no Group). The
  // dialog owns the selection; this handler performs the move and its toasts.
  const handleMoveSubmit = useCallback(async (groupId: string) => {
    if (!moveTarget) return
    try {
      await handleMoveItem(moveTarget, groupId)
      toast.success(t("groups.movedSuccess", { name: moveTarget.name }))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("groups.moveFailed"))
    } finally {
      setMoveTarget(null)
    }
  }, [moveTarget, handleMoveItem, t])

  // --- Logout ---

  async function handleLogout() {
    setLoggingOut(true)
    try {
      await api.logout()
      window.location.href = "/login"
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("auth.logoutFailed"))
      setLoggingOut(false)
    }
  }

  function cycleTheme() {
    setTheme(theme === "dark" ? "light" : "dark")
  }

  // Derived
  const ThemeIcon = resolvedTheme === "dark" ? Moon : Sun
  const hasGroups = groupList.length > 0 || ungroupedRootServers.length > 0
  const groupSelectOptions = flattenGroupsForSelect(groups)

  return (
    <>
      <aside
        className={`
          fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground
          transition-transform duration-200 ease-in-out
          lg:static lg:translate-x-0
          ${sidebarOpen ? "translate-x-0" : "-translate-x-full"}
        `}
      >
        {/* Brand */}
        <div className="flex h-14 items-center justify-between border-b border-sidebar-border px-4 shrink-0">
          <div className="flex items-center gap-2.5">
            <img src={appIcon} alt="Belochka" className="size-7 rounded-md" />
            <span className="text-sm font-semibold tracking-tight">Belochka</span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="size-7 text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent"
            onClick={cycleTheme}
            aria-label={t("settings.theme")}
          >
            <ThemeIcon className="size-3.5" />
          </Button>
        </div>

        {/* Server list */}
        {/* The root context menu covers the whole tree area (title, blank
            space above/below the nodes) so the browser default menu never
            appears there. Group and Server rows stop propagation and show
            their own menus. */}
        <nav
          className={`
            flex-1 overflow-y-auto scrollbar-hidden py-3
            ${dragOverId === ROOT_DROP ? "ring-2 ring-primary ring-inset rounded-md" : ""}
          `}
          onContextMenu={handleRootContextMenu}
          onDragOver={handleDragOverRoot}
          onDragLeave={handleDragLeaveRoot}
          onDrop={handleDropOnRoot}
        >
          <div className="flex items-center justify-between px-3 mb-1.5">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-sidebar-foreground/50 px-1">
              {t("sidebar.servers")}
            </h2>
            <Button
              variant="ghost"
              size="icon-xs"
              className="text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-sidebar-accent"
              onClick={() => setCreateOpen(true)}
              aria-label={t("groups.newGroup")}
            >
              <Plus className="size-3.5" />
            </Button>
          </div>

          {loading && servers.length === 0 ? (
            <p className="px-4 py-3 text-xs text-sidebar-foreground/40 italic">
              {t("dashboard.noServers")}
            </p>
          ) : (
            <div className="space-y-0.5 px-2">
              {/* Root-level Groups */}
              {groupList.map((node) => (
                <GroupRow
                  key={node.id}
                  node={node}
                  expanded={expanded}
                  onToggle={handleToggle}
                  onContextMenu={handleGroupContextMenu}
                  onServerContextMenu={handleServerContextMenu}
                  selectedGroupId={selectedGroupId}
                  onDragOverGroup={handleDragOverGroup}
                  onDragLeaveGroup={handleDragLeaveGroup}
                  onDropOnGroup={handleDropOnGroup}
                  dragOverId={dragOverId}
                  onDeleteClick={(id) => {
                    const node = groupList.find((n) => n.id === id)
                    if (node) setDeleteTarget({ id: node.id, name: node.name })
                  }}
                />
              ))}

              {/* Ungrouped Servers — rendered at the root, as siblings of the
                  Groups, so no virtual "Ungrouped" section is needed */}
              {ungroupedRootServers.map((server) => (
                <ServerRow key={server.id} server={server} onContextMenu={handleServerContextMenu} />
              ))}

              {/* Empty state */}
              {!hasGroups && servers.length === 0 && (
                <p className="px-2 py-3 text-xs text-sidebar-foreground/40 italic">
                  {t("dashboard.noServers")}
                </p>
              )}
            </div>
          )}
        </nav>

        {/* Bottom actions */}
        <div className="border-t border-sidebar-border p-2 space-y-1 shrink-0">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setBatchOpen(true)}
            className="w-full justify-start gap-2.5 text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground"
          >
            <TerminalSquare className="size-4" />
            {t("batch.trigger")}
          </Button>
          <SettingsDialog>
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-start gap-2.5 text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground"
            >
              <Settings className="size-4" />
              {t("settings.title")}
            </Button>
          </SettingsDialog>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleLogout}
            disabled={loggingOut}
            className="w-full justify-start gap-2.5 text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground"
          >
            {loggingOut ? <Loader2 className="size-4 animate-spin" /> : <LogOut className="size-4" />}
            {t("auth.logout")}
          </Button>
          <a
            href="https://github.com/Unmovable8911/Belochka"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1.5 px-3 py-1 text-[11px] text-sidebar-foreground/35 hover:text-sidebar-foreground/60 transition-colors"
          >
            <ExternalLink className="size-3" />
            GitHub
          </a>
        </div>
      </aside>

      {/* Batch Command dialog — the only entry point for dispatching a batch */}
      {batchOpen && <BatchCommandDialog open onOpenChange={setBatchOpen} />}

      {/* Create / rename group dialogs */}
      {createOpen && (
        <GroupNameDialog
          open
          title={t("groups.newGroup")}
          genericErrorKey="groups.createFailed"
          onOpenChange={() => setCreateOpen(false)}
          onSubmit={handleCreateSubmit}
        />
      )}
      {renameTarget && (
        <GroupNameDialog
          open
          title={t("groups.renameGroupTitle")}
          initialName={renameTarget.name}
          genericErrorKey="groups.renameFailed"
          onOpenChange={() => setRenameTarget(null)}
          onSubmit={handleRenameSubmit}
        />
      )}
      {deleteTarget && (
        <DeleteGroupDialog
          open
          group={deleteTarget}
          onOpenChange={() => setDeleteTarget(null)}
          onDelete={handleDeleteConfirm}
        />
      )}

      {/* Move-to dialog */}
      {moveTarget && (
        <MoveToDialog
          open
          onOpenChange={() => setMoveTarget(null)}
          targetName={moveTarget.name}
          initialGroupId={moveTarget.groupId}
          groupOptions={groupSelectOptions}
          onSubmit={handleMoveSubmit}
        />
      )}
    </>
  )
}
