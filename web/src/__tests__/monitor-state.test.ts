import { describe, it, expect } from "vitest"
import { monitorReducer, initialMonitorState } from "../hooks/useMonitorState"
import type { MonitorState, MonitorAction } from "../hooks/useMonitorState"
import type { ServerMetrics } from "../types/server"

function makeMetrics(overrides: Partial<ServerMetrics> = {}): ServerMetrics {
  return {
    aggregate: { usagePercent: 0 },
    cores: [],
    memory: { total: 0, used: 0, swapTotal: 0, swapUsed: 0 },
    disk: { partitions: [] },
    network: { interfaces: [] },
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
          aggregate: { usagePercent: 45.2 },
          cores: [],
          memory: { total: 8589934592, used: 4294967296, swapTotal: 0, swapUsed: 0 },
          disk: { partitions: [] },
          network: { interfaces: [] },
          system: { hostname: "web-1", kernel: "5.15.0", uptimeSec: 86400, osName: "Ubuntu 22.04", coreCount: 4 },
        },
      },
    }

    const action: MonitorAction = { type: "snapshot", data: snapshotData }
    const state = monitorReducer(initialMonitorState, action)

    expect(state.servers).toHaveLength(2)
    expect(state.servers[0]).toEqual({ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" })
    expect(state.servers[1]).toEqual({ id: "srv-2", name: "db-1", host: "10.0.0.2", status: "disconnected" })
    expect(state.metrics["srv-1"].aggregate.usagePercent).toBe(45.2)
    expect(state.metrics["srv-1"].system.hostname).toBe("web-1")
  })

  // Metrics and status actions removed — backend sends full state via "snapshot" only

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

  it("adds server on add_server action (empty list)", () => {
    const action: MonitorAction = {
      type: "add_server",
      data: { id: "srv-new", name: "new-server", host: "10.0.0.3" },
    }

    const state = monitorReducer(initialMonitorState, action)

    expect(state.servers).toHaveLength(1)
    expect(state.servers[0]).toEqual({
      id: "srv-new",
      name: "new-server",
      host: "10.0.0.3",
      status: "connecting",
    })
  })

  it("appends server on add_server action (non-empty list)", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [
        { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
      ],
    }

    const action: MonitorAction = {
      type: "add_server",
      data: { id: "srv-2", name: "web-2", host: "10.0.0.2" },
    }

    const state = monitorReducer(stateWithServers, action)

    expect(state.servers).toHaveLength(2)
    expect(state.servers[0].id).toBe("srv-1")
    expect(state.servers[1].id).toBe("srv-2")
  })

  it("skips duplicate on add_server when server already exists", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [
        { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
      ],
    }

    const action: MonitorAction = {
      type: "add_server",
      data: { id: "srv-1", name: "web-1", host: "10.0.0.1" },
    }

    const state = monitorReducer(stateWithServers, action)

    // Should be the same reference (no change)
    expect(state).toBe(stateWithServers)
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
