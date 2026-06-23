import type { ReactNode } from "react"
import { useMonitorState } from "../hooks/useMonitorState"

export function StaleDataOverlay({ children }: { children: ReactNode }) {
  const { state } = useMonitorState()

  return (
    <div
      data-testid="stale-data-overlay"
      className={`transition-opacity duration-300 ${state.wsConnected ? "" : "opacity-50"}`}
    >
      {children}
    </div>
  )
}
