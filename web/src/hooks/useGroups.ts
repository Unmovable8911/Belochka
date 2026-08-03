import { useState, useEffect, useCallback, useMemo } from "react"
import { getGroups, createGroup, updateGroup, deleteGroup, type CreateGroupPayload, type UpdateGroupPayload } from "@/api/client"
import type { Group, GroupNode, ServerInfo } from "@/types/server"

// --- Group list building ---

// buildGroupList returns a flat list of GroupNodes (a Group with its member
// servers attached, sorted by name). Groups cannot be nested.
export function buildGroupList(groups: Group[], servers: ServerInfo[]): GroupNode[] {
  const nodes: GroupNode[] = groups.map((g) => ({
    id: g.id,
    name: g.name,
    member_count: g.member_count,
    servers: [],
  }))

  // Attach servers to their group nodes
  for (const s of servers) {
    if (s.group_id) {
      const node = nodes.find((n) => n.id === s.group_id)
      if (node) {
        node.servers.push({ ...s })
      }
    }
  }

  // Sort groups by name, servers by name
  nodes.sort((a, b) => a.name.localeCompare(b.name))
  for (const node of nodes) {
    node.servers.sort((a, b) => a.name.localeCompare(b.name))
  }

  return nodes
}

// --- Hook ---

// flattenGroupsForSelect returns a flat list of groups sorted by name,
// suitable for use in a dropdown/select component.
export function flattenGroupsForSelect(groups: Group[]): { value: string; label: string }[] {
  return [...groups]
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((g) => ({ value: g.id, label: g.name }))
}

export function useGroups(servers: ServerInfo[]) {
  const [groups, setGroups] = useState<Group[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchGroups = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await getGroups()
      setGroups(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load groups")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchGroups()
  }, [fetchGroups])

  // Compute the flat group list from groups + servers (memoized, not stored in state)
  const groupList = useMemo(() => buildGroupList(groups, servers), [groups, servers])

  const create = useCallback(async (payload: CreateGroupPayload) => {
    await createGroup(payload)
  }, [])

  const update = useCallback(async (id: string, payload: UpdateGroupPayload) => {
    await updateGroup(id, payload)
  }, [])

  const remove = useCallback(async (id: string) => {
    await deleteGroup(id)
  }, [])

  return {
    groups,
    groupList,
    loading,
    error,
    refetch: fetchGroups,
    createGroup: create,
    updateGroup: update,
    deleteGroup: remove,
  }
}
