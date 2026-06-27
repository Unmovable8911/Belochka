import { useEffect, useRef, useState, useCallback } from "react"
import { useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import "@xterm/xterm/css/xterm.css"
import type { Server } from "@/types/server"
import { getServer } from "@/api/client"

type ConnectionStatus = "connecting" | "connected" | "disconnected"

export default function Console() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const termRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const terminalRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)

  const [server, setServer] = useState<Server | null>(null)
  const [status, setStatus] = useState<ConnectionStatus>("connecting")

  useEffect(() => {
    if (!id) return
    getServer(id).then(setServer).catch(() => {})
  }, [id])

  const connect = useCallback(() => {
    if (!id || !termRef.current) return

    terminalRef.current?.dispose()

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: "monospace",
      fontSize: 14,
      theme: {
        background: "#09090b",
        foreground: "#fafafa",
      },
    })
    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(termRef.current)
    fitAddon.fit()

    terminalRef.current = term
    fitAddonRef.current = fitAddon
    setStatus("connecting")

    const protocol = location.protocol === "https:" ? "wss:" : "ws:"
    const ws = new WebSocket(`${protocol}//${location.host}/api/ws/terminal/${id}`)
    ws.binaryType = "arraybuffer"
    wsRef.current = ws

    ws.onmessage = (e) => {
      if (typeof e.data === "string") {
        const msg = JSON.parse(e.data)
        if (msg.type === "status") {
          if (msg.status === "connected") {
            setStatus("connected")
          } else if (msg.status === "disconnected") {
            setStatus("disconnected")
          }
        }
      } else {
        term.write(new Uint8Array(e.data))
      }
    }

    ws.onclose = () => {
      setStatus("disconnected")
    }

    ws.onerror = () => {
      setStatus("disconnected")
    }

    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const encoder = new TextEncoder()
        ws.send(encoder.encode(data))
      }
    })

    const sendResize = () => {
      fitAddon.fit()
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: "resize",
          cols: term.cols,
          rows: term.rows,
        }))
      }
    }

    const observer = new ResizeObserver(() => sendResize())
    observer.observe(termRef.current)

    return () => {
      observer.disconnect()
      ws.close()
      term.dispose()
    }
  }, [id])

  useEffect(() => {
    const cleanup = connect()
    return cleanup
  }, [connect])

  const handleReconnect = () => {
    wsRef.current?.close()
    connect()
  }

  return (
    <div className="flex flex-col h-screen bg-[#09090b]">
      <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-800 bg-zinc-950">
        <div className="flex items-center gap-4 text-sm text-zinc-400">
          {server && (
            <>
              <span className="text-zinc-200 font-medium">{server.name}</span>
              <span>{server.host}:{server.port}</span>
              <span className="text-xs px-1.5 py-0.5 rounded bg-zinc-800">
                {server.auth_type === "password" ? t("console.authPassword") : t("console.authKey")}
              </span>
            </>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className={`inline-block w-2 h-2 rounded-full ${
            status === "connected" ? "bg-green-500" :
            status === "connecting" ? "bg-yellow-500 animate-pulse" :
            "bg-red-500"
          }`} />
          <span className="text-xs text-zinc-400">
            {status === "connected" && t("console.statusConnected")}
            {status === "connecting" && t("console.statusConnecting")}
            {status === "disconnected" && t("console.statusDisconnected")}
          </span>
        </div>
      </div>

      <div className="flex-1 relative">
        <div ref={termRef} className="h-full w-full" />

        {status === "disconnected" && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/70">
            <div className="text-center">
              <p className="text-zinc-300 mb-4">{t("console.disconnectedMessage")}</p>
              <button
                onClick={handleReconnect}
                className="px-4 py-2 bg-zinc-200 text-zinc-900 rounded-md text-sm font-medium hover:bg-zinc-300 transition-colors cursor-pointer"
              >
                {t("console.reconnect")}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
