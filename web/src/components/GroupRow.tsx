import { type MouseEvent } from "react"
import { Link, useLocation, useNavigate } from "react-router-dom"
import { ChevronRight, ChevronDown, Folder, FolderOpen, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import type { GroupNode, ServerInfo } from "@/types/server"

// --- ServerRow ---

export function ServerRow({ server, onContextMenu }: {
  server: ServerInfo
  onContextMenu?: (e: MouseEvent, server: ServerInfo) => void
}) {
  const location = useLocation()
  const href = `/server/${server.id}`
  const isActive = location.pathname === href

  return (
    <Link
      to={href}
      draggable
      onDragStart={(e) => {
        e.dataTransfer.setData("application/json", JSON.stringify({ type: "server", id: server.id }))
        e.dataTransfer.effectAllowed = "move"
        const el = e.currentTarget as HTMLElement
        el.classList.add("opacity-50")
      }}
      onDragEnd={(e) => {
        (e.currentTarget as HTMLElement).classList.remove("opacity-50")
      }}
      // A Server row is not a drop target: stop propagation so a drop on it
      // neither moves the dragged Server into a Group nor ungroups it via the
      // blank tree-area target.
      onDragOver={(e) => e.stopPropagation()}
      onDrop={(e) => e.stopPropagation()}
      onContextMenu={(e) => { if (onContextMenu) onContextMenu(e, server) }}
      className={`
        flex items-center gap-2.5 rounded-md px-3 py-1.5 text-sm transition-colors cursor-grab active:cursor-grabbing
        ${isActive
          ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
          : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"
        }
      `}
    >
      <span
        className={`size-2 shrink-0 rounded-full ${
          server.status === "connected"
            ? "bg-green-500"
            : server.status === "failed"
              ? "bg-red-500"
              : "bg-amber-500"
        }`}
      />
      <span className="truncate">{server.name}</span>
    </Link>
  )
}

// --- GroupRow ---

// GroupRow renders a single root-level Group: an expandable row that reveals
// its member servers when expanded, with a delete quick-action on hover. It is
// also a drag target so a Server can be moved into the Group. Groups are flat,
// so there is no nesting.
export function GroupRow({
  node,
  expanded,
  onToggle,
  onContextMenu,
  onServerContextMenu,
  selectedGroupId,
  onDragOverGroup,
  onDragLeaveGroup,
  onDropOnGroup,
  dragOverId,
  onDeleteClick,
}: {
  node: GroupNode
  expanded: Set<string>
  onToggle: (id: string) => void
  onContextMenu: (e: MouseEvent, node: GroupNode) => void
  onServerContextMenu: (e: MouseEvent, server: ServerInfo) => void
  selectedGroupId: string | null
  onDragOverGroup: (e: React.DragEvent, id: string) => void
  onDragLeaveGroup: (e: React.DragEvent) => void
  onDropOnGroup: (e: React.DragEvent, id: string) => void
  dragOverId: string | null
  onDeleteClick: (id: string) => void
}) {
  const { t } = useTranslation()
  const isOpen = expanded.has(node.id)
  const hasServers = node.servers.length > 0
  const isSelected = selectedGroupId === node.id

  const navigate = useNavigate()

  return (
    <div>
      <div
        className={`
          group/tree flex items-center gap-1 rounded-md px-2 py-1 text-sm transition-colors cursor-pointer
          ${isSelected
            ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
            : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"
          }
          ${dragOverId === node.id ? "ring-2 ring-primary" : ""}
        `}
        style={{ paddingLeft: "8px" }}
        onDragOver={(e) => { e.preventDefault(); e.stopPropagation(); onDragOverGroup(e, node.id) }}
        onDragLeave={(e) => { e.stopPropagation(); onDragLeaveGroup(e) }}
        onDrop={(e) => { e.preventDefault(); e.stopPropagation(); onDropOnGroup(e, node.id) }}
        onContextMenu={(e) => onContextMenu(e, node)}
        onClick={() => navigate(`/?group_id=${node.id}`)}
      >
        {/* Chevron */}
        <button
          className="p-0.5 shrink-0 hover:bg-accent rounded"
          onClick={(e) => { e.stopPropagation(); onToggle(node.id) }}
          aria-label={isOpen ? "Collapse" : "Expand"}
        >
          {hasServers ? (
            isOpen ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />
          ) : (
            <span className="w-3.5" />
          )}
        </button>

        {/* Folder icon */}
        {isOpen && hasServers ? (
          <FolderOpen className="size-3.5 shrink-0 text-sidebar-foreground/50" />
        ) : (
          <Folder className="size-3.5 shrink-0 text-sidebar-foreground/50" />
        )}

        {/* Name */}
        <span className="truncate flex-1">{node.name}</span>

        {/* Quick actions — revealed on hover. Pointer events are gated on the
            hover reveal so the invisible buttons do not intercept clicks on
            the row. The delete button opens a confirmation dialog. */}
        <div className="flex shrink-0 items-center gap-0.5 pointer-events-none opacity-0 transition-opacity group-hover/tree:pointer-events-auto group-hover/tree:opacity-100">
          <button
            className="rounded p-0.5 text-sidebar-foreground/50 hover:bg-sidebar-accent hover:text-sidebar-foreground"
            aria-label={t("groups.deleteFor", { name: node.name })}
            onClick={(e) => { e.stopPropagation(); onDeleteClick(node.id) }}
            onMouseDown={(e) => e.preventDefault()}
          >
            <Trash2 className="size-3.5" />
          </button>
        </div>

        {/* Member count — right-aligned; hidden on hover so the quick actions
            take its place */}
        {node.member_count > 0 && (
          <span className="shrink-0 text-[11px] text-sidebar-foreground/40 ml-auto group-hover/tree:hidden">
            {node.member_count}
          </span>
        )}
      </div>

      {/* Member servers — revealed when expanded */}
      {isOpen && hasServers && (
        <div>
          {node.servers.map((server) => (
            <div key={server.id} style={{ paddingLeft: "24px" }}>
              <ServerRow server={server} onContextMenu={onServerContextMenu} />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
