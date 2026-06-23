import { describe, it, expect } from "vitest"
import { monitorReducer, initialMonitorState } from "../hooks/useMonitorState"
import type { MonitorState, MonitorAction, ServerMetrics } from "../hooks/useMonitorState"

function makeMetrics(overrides: Partial<ServerMetrics> = {}): ServerMetrics {
  return {
    cpu: { aggregate: { usagePercent: 0 }, cores: [] },
    memory: { total: 0, used: 0, available: 0, swapTotal: 0, swapUsed: 0 },
    disk: { partitions: [] },
    network: { interfaces: [] },
    process: { processes: [] },
    system: { hostname: "", kernel: "", uptimeSec: 0, osName: "", coreCount: 0 },
    ...overrides,
  }
}

describe("monitorReducer", () => {
  it("initializes full state from a snapshot message", () => {
    const snapshotData = {
      servers: [
        { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
        { id: "srv-2", name: "db-1", host: "10.0.0.2", status: "disconnected" },
      ],
      metrics: {
        "srv-1": {
          cpu: { aggregate: { usagePercent: 45.2 }, cores: [] },
          memory: { total: 8589934592, used: 4294967296, available: 4294967296, swapTotal: 0, swapUsed: 0 },
          disk: { partitions: [] },
          network: { interfaces: [] },
          process: { processes: [] },
          system: { hostname: "web-1", kernel: "5.15.0", uptimeSec: 86400, osName: "Ubuntu 22.04", coreCount: 4 },
        },
      },
    }

    const action: MonitorAction = { type: "snapshot", data: snapshotData }
    const state = monitorReducer(initialMonitorState, action)

    expect(state.servers).toHaveLength(2)
    expect(state.servers[0]).toEqual({ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" })
    expect(state.servers[1]).toEqual({ id: "srv-2", name: "db-1", host: "10.0.0.2", status: "disconnected" })
    expect(state.metrics["srv-1"].cpu.aggregate.usagePercent).toBe(45.2)
    expect(state.metrics["srv-1"].system.hostname).toBe("web-1")
  })

  it("updates metrics for a specific server", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [
        { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
        { id: "srv-2", name: "db-1", host: "10.0.0.2", status: "connected" },
      ],
      metrics: {
        "srv-1": makeMetrics({ cpu: { aggregate: { usagePercent: 10 }, cores: [] } }),
        "srv-2": makeMetrics({ cpu: { aggregate: { usagePercent: 20 }, cores: [] } }),
      },
    }

    const updatedMetrics = makeMetrics({ cpu: { aggregate: { usagePercent: 75.5 }, cores: [] } })
    const action: MonitorAction = {
      type: "metrics",
      data: { serverId: "srv-1", metrics: updatedMetrics },
    }

    const state = monitorReducer(stateWithServers, action)

    expect(state.metrics["srv-1"].cpu.aggregate.usagePercent).toBe(75.5)
    // srv-2 unchanged
    expect(state.metrics["srv-2"].cpu.aggregate.usagePercent).toBe(20)
  })

  it("updates server connection status", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [
        { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
        { id: "srv-2", name: "db-1", host: "10.0.0.2", status: "connected" },
      ],
    }

    const action: MonitorAction = {
      type: "status",
      data: { serverId: "srv-1", status: "disconnected" },
    }

    const state = monitorReducer(stateWithServers, action)

    expect(state.servers[0].status).toBe("disconnected")
    // srv-2 unchanged
    expect(state.servers[1].status).toBe("connected")
  })

  it("returns state unchanged for unknown action type", () => {
    const state = monitorReducer(initialMonitorState, { type: "unknown" } as unknown as MonitorAction)
    expect(state).toBe(initialMonitorState)
  })

  it("sets WebSocket connection state", () => {
    const action: MonitorAction = { type: "ws_connected", data: true }
    const state = monitorReducer(initialMonitorState, action)
    expect(state.wsConnected).toBe(true)

    const state2 = monitorReducer(state, { type: "ws_connected", data: false })
    expect(state2.wsConnected).toBe(false)
  })

  it("removes server and its metrics on remove_server action", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [
        { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
        { id: "srv-2", name: "db-1", host: "10.0.0.2", status: "connected" },
      ],
      metrics: {
        "srv-1": makeMetrics(),
        "srv-2": makeMetrics(),
      },
    }

    const action: MonitorAction = {
      type: "remove_server",
      data: { serverId: "srv-1" },
    }

    const state = monitorReducer(stateWithServers, action)

    expect(state.servers).toHaveLength(1)
    expect(state.servers[0].id).toBe("srv-2")
    expect(state.metrics["srv-1"]).toBeUndefined()
    expect(state.metrics["srv-2"]).toBeDefined()
  })
})
