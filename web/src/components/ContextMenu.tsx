import type { ReactNode } from "react"

// ContextMenuItem defines a single entry in the right-click context menu overlay.
export interface ContextMenuItem {
  label: string
  icon?: ReactNode
  onClick: () => void
  destructive?: boolean
}

// ContextMenuOverlay renders a positioned right-click menu with a click-away backdrop.
export function ContextMenuOverlay({ x, y, items, onClose }: {
  x: number
  y: number
  items: ContextMenuItem[]
  onClose: () => void
}) {
  return (
    <>
      {/* eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions */}
      <div
        className="fixed inset-0 z-[60]"
        data-testid="context-menu-backdrop"
        onClick={onClose}
        onContextMenu={(e) => { e.preventDefault(); onClose() }}
      />
      <div
        className="fixed z-[70] min-w-[160px] rounded-md border bg-popover p-1 shadow-md"
        style={{ left: x, top: y }}
        role="menu"
      >
        {items.map((item, i) => (
          <button
            key={i}
            className={`flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground ${item.destructive ? "text-destructive" : ""}`}
            onClick={() => { item.onClick(); onClose() }}
            role="menuitem"
          >
            {item.icon}
            {item.label}
          </button>
        ))}
      </div>
    </>
  )
}
