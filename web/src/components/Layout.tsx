import { useState, useCallback, type ReactNode } from "react"
import { Menu } from "lucide-react"
import { Sidebar } from "@/components/Sidebar"
import { Button } from "@/components/ui/button"
import { ContextMenuOverlay, type ContextMenuItem } from "@/components/ContextMenu"

// --- Main Layout ---

export function Layout({ children }: { children: ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)

  // Context menu overlay state (must live outside Sidebar because CSS transform
  // on the <aside> creates a new containing block, breaking position:fixed)
  const [contextMenu, setContextMenu] = useState<{
    x: number
    y: number
    items: ContextMenuItem[]
  } | null>(null)

  const closeContextMenu = useCallback(() => setContextMenu(null), [])

  const handleContextMenu = useCallback((x: number, y: number, items: ContextMenuItem[]) => {
    setContextMenu({ x, y, items })
  }, [])

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Mobile overlay (rendered before sidebar for z-index layering) */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      {/* Sidebar — single instance, responsive: fixed slide on mobile, static on desktop */}
      <Sidebar sidebarOpen={sidebarOpen} onContextMenu={handleContextMenu} />

      {/* Context menu overlay (outside sidebar DOM due to CSS transform containment) */}
      {contextMenu && (
        <ContextMenuOverlay {...contextMenu} onClose={closeContextMenu} />
      )}

      {/* Main content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Mobile top bar */}
        <div className="flex h-12 items-center gap-3 border-b bg-card px-4 lg:hidden shrink-0">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setSidebarOpen(true)}
            aria-label="Open sidebar"
          >
            <Menu className="size-5" />
          </Button>
          <span className="text-sm font-semibold">Belochka</span>
        </div>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto">
          {children}
        </main>
      </div>
    </div>
  )
}
