import { useCallback } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import * as api from "@/api/client"
import { useGroups } from "@/hooks/useGroups"
import { useMonitorState } from "@/hooks/useMonitorState"

// MoveTarget identifies a Server to be moved via the "Move to" dialog or
// drag-and-drop. Groups are root-level only and cannot be moved. groupId is
// the Server's current Group ("" = no Group); the dialog pre-selects it.
export interface MoveTarget {
  id: string
  name: string
  groupId: string
}

// useSidebarActions wraps group CRUD operations with success toasts and
// side-effect orchestration (refetch). The create/rename/delete callbacks
// throw on API failure so callers (the name dialogs, the two-step delete
// flow) can render inline validation or error toasts themselves.
export function useSidebarActions() {
  const { t } = useTranslation()
  const { state: { servers }, dispatch } = useMonitorState()
  const { groups, groupList, loading, refetch, createGroup, updateGroup, deleteGroup } = useGroups(servers)

  const handleCreateGroup = useCallback(async (name: string) => {
    await createGroup({ name })
    await refetch()
    toast.success(t("groups.createdSuccess", { name }))
  }, [createGroup, refetch, t])

  const handleRenameGroup = useCallback(async (id: string, name: string) => {
    await updateGroup(id, { name })
    await refetch()
    toast.success(t("groups.renamedSuccess", { name }))
  }, [updateGroup, refetch, t])

  const handleDeleteGroup = useCallback(async (id: string, name: string) => {
    await deleteGroup(id)
    await refetch()
    toast.success(t("groups.deletedSuccess", { name }))
  }, [deleteGroup, refetch, t])

  const handleMoveItem = useCallback(async (target: MoveTarget, groupId: string) => {
    await api.updateServer(target.id, { group_id: groupId || null })
    dispatch({
      type: "update_server",
      data: { serverId: target.id, group_id: groupId || undefined },
    })
    await refetch()
  }, [dispatch, refetch])

  return {
    groups,
    groupList,
    loading,
    handleCreateGroup,
    handleRenameGroup,
    handleDeleteGroup,
    handleMoveItem,
  }
}
