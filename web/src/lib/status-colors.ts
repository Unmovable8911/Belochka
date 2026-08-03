const ServerStatus = {
  Connected: "connected",
  Failed: "failed",
  Error: "error",
  Reconnecting: "reconnecting",
  Connecting: "connecting",
} as const

type ServerStatus = (typeof ServerStatus)[keyof typeof ServerStatus]

export function statusDotColor(status: string): string {
  switch (status) {
    case ServerStatus.Connected:
      return "bg-emerald-500"
    case ServerStatus.Failed:
    case ServerStatus.Error:
      return "bg-red-500"
    default:
      return "bg-amber-500"
  }
}
