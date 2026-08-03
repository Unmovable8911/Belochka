import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { renderHook, waitFor, act } from "@testing-library/react"
import { buildGroupList, useGroups } from "../hooks/useGroups"
import type { Group, ServerInfo } from "../types/server"

// --- Mock API ---

vi.mock("@/api/client", () => ({
  getGroups: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  deleteGroup: vi.fn(),
}))

import { getGroups, createGroup, updateGroup, deleteGroup } from "@/api/client"

const mockGetGroups = getGroups as ReturnType<typeof vi.fn>
const mockCreateGroup = createGroup as ReturnType<typeof vi.fn>
const mockUpdateGroup = updateGroup as ReturnType<typeof vi.fn>
const mockDeleteGroup = deleteGroup as ReturnType<typeof vi.fn>

function makeGroup(overrides: Partial<Group> = {}): Group {
  return {
    id: "g1",
    name: "Production",
    member_count: 2,
    ...overrides,
  }
}

function makeServer(overrides: Partial<ServerInfo> = {}): ServerInfo {
  return {
    id: "srv-1",
    name: "web-1",
    host: "10.0.0.1",
    status: "connected",
    ...overrides,
  }
}

// --- buildGroupList tests ---

describe("buildGroupList", () => {
  it("returns empty array for empty input", () => {
    expect(buildGroupList([], [])).toEqual([])
  })

  it("builds a flat list of root groups", () => {
    const groups: Group[] = [
      makeGroup({ id: "g1", name: "Production" }),
      makeGroup({ id: "g2", name: "Staging" }),
    ]
    const list = buildGroupList(groups, [])
    expect(list).toHaveLength(2)
    expect(list[0].name).toBe("Production")
    expect(list[1].name).toBe("Staging")
  })

  it("attaches servers to their group nodes", () => {
    const groups: Group[] = [
      makeGroup({ id: "g1", name: "Production" }),
    ]
    const servers: ServerInfo[] = [
      makeServer({ id: "s1", name: "web-1", group_id: "g1" }),
    ]
    const list = buildGroupList(groups, servers)
    expect(list[0].servers).toHaveLength(1)
    expect(list[0].servers[0].name).toBe("web-1")
  })

  it("leaves servers without group_id unassigned in list", () => {
    const groups: Group[] = [
      makeGroup({ id: "g1", name: "Production" }),
    ]
    const servers: ServerInfo[] = [
      makeServer({ id: "s1", name: "web-1" }),
    ]
    const list = buildGroupList(groups, servers)
    expect(list[0].servers).toHaveLength(0)
  })

  it("sorts groups alphabetically by name", () => {
    const groups: Group[] = [
      makeGroup({ id: "g1", name: "Zulu" }),
      makeGroup({ id: "g2", name: "Alpha" }),
    ]
    const list = buildGroupList(groups, [])
    expect(list[0].name).toBe("Alpha")
    expect(list[1].name).toBe("Zulu")
  })

  it("sorts servers alphabetically within a group", () => {
    const groups: Group[] = [
      makeGroup({ id: "g1", name: "Production" }),
    ]
    const servers: ServerInfo[] = [
      makeServer({ id: "s1", name: "zulu", group_id: "g1" }),
      makeServer({ id: "s2", name: "alpha", group_id: "g1" }),
    ]
    const list = buildGroupList(groups, servers)
    expect(list[0].servers[0].name).toBe("alpha")
    expect(list[0].servers[1].name).toBe("zulu")
  })
})

// --- useGroups hook tests ---

describe("useGroups", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("returns loading=true initially, then groups and groupList on success", async () => {
    const groups: Group[] = [makeGroup({ id: "g1", name: "Production" })]
    mockGetGroups.mockResolvedValueOnce(groups)

    const { result } = renderHook(() => useGroups([]))

    expect(result.current.loading).toBe(true)
    expect(result.current.groupList).toEqual([])

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.groups).toEqual(groups)
    expect(result.current.groupList).toHaveLength(1)
    expect(result.current.groupList[0].name).toBe("Production")
    expect(result.current.error).toBeNull()
  })

  it("returns error state on API failure", async () => {
    mockGetGroups.mockRejectedValueOnce(new Error("Network error"))

    const { result } = renderHook(() => useGroups([]))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.error).toBe("Network error")
    expect(result.current.groupList).toEqual([])
  })

  it("rebuilds groupList when servers change", async () => {
    const groups: Group[] = [makeGroup({ id: "g1", name: "Production" })]
    mockGetGroups.mockResolvedValueOnce(groups)

    const { result, rerender } = renderHook(
      ({ servers }: { servers: ServerInfo[] }) => useGroups(servers),
      { initialProps: { servers: [] } }
    )

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.groupList[0].servers).toHaveLength(0)

    // Rerender with a server assigned to the group
    const newServers: ServerInfo[] = [makeServer({ id: "s1", name: "web-1", group_id: "g1" })]
    rerender({ servers: newServers })

    await waitFor(() => {
      expect(result.current.groupList[0].servers).toHaveLength(1)
    })
  })

  it("createGroup calls API and caller can refetch", async () => {
    const groups: Group[] = [makeGroup({ id: "g1", name: "Production" })]
    mockGetGroups.mockResolvedValue(groups)
    mockCreateGroup.mockResolvedValueOnce(makeGroup({ id: "g2", name: "Staging" }))

    const { result } = renderHook(() => useGroups([]))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.createGroup({ name: "Staging" })
    })

    expect(mockCreateGroup).toHaveBeenCalledWith({ name: "Staging" })

    // Refetch
    mockGetGroups.mockResolvedValueOnce([
      ...groups,
      makeGroup({ id: "g2", name: "Staging" }),
    ])
    await act(async () => {
      await result.current.refetch()
    })
    expect(result.current.groups).toHaveLength(2)
  })

  it("updateGroup calls API with id and payload", async () => {
    mockGetGroups.mockResolvedValueOnce([makeGroup({ id: "g1", name: "Production" })])
    mockUpdateGroup.mockResolvedValueOnce(makeGroup({ id: "g1", name: "Production-Updated" }))

    const { result } = renderHook(() => useGroups([]))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.updateGroup("g1", { name: "Production-Updated" })
    })

    expect(mockUpdateGroup).toHaveBeenCalledWith("g1", { name: "Production-Updated" })
  })

  it("deleteGroup calls API with id", async () => {
    mockGetGroups.mockResolvedValueOnce([makeGroup({ id: "g1", name: "Production" })])
    mockDeleteGroup.mockResolvedValueOnce(undefined)

    const { result } = renderHook(() => useGroups([]))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.deleteGroup("g1")
    })

    expect(mockDeleteGroup).toHaveBeenCalledWith("g1")
  })

  it("refetch updates groups after external mutation", async () => {
    mockGetGroups
      .mockResolvedValueOnce([makeGroup({ id: "g1", name: "Production" })])
      .mockResolvedValueOnce([
        makeGroup({ id: "g1", name: "Production" }),
        makeGroup({ id: "g2", name: "Staging" }),
      ])

    const { result } = renderHook(() => useGroups([]))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })
    expect(result.current.groups).toHaveLength(1)

    await act(async () => {
      await result.current.refetch()
    })

    expect(result.current.groups).toHaveLength(2)
  })
})
