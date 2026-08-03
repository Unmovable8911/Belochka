import { useState } from "react"
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Label } from "@/components/ui/label"
import { ServerForm } from "@/components/ServerForm"
import { useServerForm } from "@/hooks/useServerForm"
import { flattenGroupsForSelect } from "@/hooks/useGroups"
import { toast } from "sonner"
import type { AuthType, Group, Server, ServerFormData } from "@/types/server"
import * as api from "@/api/client"

function initFormData(defaultAuthType: AuthType, defaultGroupId?: string): ServerFormData {
  return {
    name: "",
    host: "",
    port: 22,
    username: "",
    authType: defaultAuthType,
    password: "",
    keyPath: "",
    group_id: defaultGroupId,
  }
}

export interface AddServerDialogProps {
  onServerAdded?: (server: Server) => void
  defaultOpen?: boolean
  defaultAuthType?: AuthType
  triggerLabel?: string
  groups?: Group[]
  defaultGroupId?: string
}

export function AddServerDialog({
  onServerAdded,
  defaultOpen = false,
  defaultAuthType = "password",
  triggerLabel,
  groups = [],
  defaultGroupId,
}: AddServerDialogProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(defaultOpen)
  const [form, setForm] = useState<ServerFormData>(() => initFormData(defaultAuthType, defaultGroupId))
  const {
    testing,
    testError,
    fingerprint,
    fingerprintTrusted,
    saving,
    setSaving,
    trust,
    resetTestState,
    reset,
    runTest,
  } = useServerForm()

  const groupOptions = flattenGroupsForSelect(groups)

  function resetState() {
    setForm(initFormData(defaultAuthType, defaultGroupId))
    reset()
  }

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen)
    if (!nextOpen) {
      resetState()
    }
  }

  function updateField<K extends keyof ServerFormData>(key: K, value: ServerFormData[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
    resetTestState()
  }

  function isFormValid(): boolean {
    return (
      form.name.trim() !== "" &&
      form.host.trim() !== "" &&
      form.username.trim() !== ""
    )
  }

  function buildServerBody(extras?: Record<string, unknown>): Record<string, unknown> {
    const body: Record<string, unknown> = {
      name: form.name.trim(),
      host: form.host.trim(),
      port: form.port,
      username: form.username.trim(),
      auth_type: form.authType,
      ...extras,
      ...(form.authType === "password"
        ? { password: form.password }
        : { key_path: form.keyPath }),
    }
    if (form.group_id) {
      body.group_id = form.group_id
    }
    return body
  }

  async function handleSave() {
    if (!fingerprint || !fingerprintTrusted) return

    setSaving(true)
    try {
      const saved = await api.createServer(
        buildServerBody({ host_key_fingerprint: fingerprint }),
      )

      toast.success(t("addServer.savedSuccess", { name: saved.name }))
      onServerAdded?.(saved)
      setOpen(false)
      resetState()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("addServer.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  const canTest = isFormValid() && !testing
  const canSave = fingerprint !== null && fingerprintTrusted && !saving

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon className="size-4" />
          {triggerLabel ?? t("addServer.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("addServer.title")}</DialogTitle>
        </DialogHeader>

        <ServerForm
          form={form}
          onFieldChange={updateField}
          idPrefix=""
          fingerprint={fingerprint}
          fingerprintTrusted={fingerprintTrusted}
          onTrust={trust}
          testError={testError}
        />

        {/* Group selector */}
        {groupOptions.length > 0 && (
          <div className="grid gap-2">
            <Label id="group-select-label">{t("groups.noGroup")}</Label>
            <Select
              value={form.group_id ?? ""}
              onValueChange={(value) => updateField("group_id", value || undefined)}
            >
              <SelectTrigger aria-labelledby="group-select-label">
                <SelectValue placeholder={t("groups.noGroup")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">{t("groups.noGroup")}</SelectItem>
                {groupOptions.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => runTest(buildServerBody())}
            disabled={!canTest}
          >
            {testing ? t("addServer.testing") : t("addServer.testConnection")}
          </Button>
          <Button onClick={handleSave} disabled={!canSave}>
            {saving ? t("addServer.saving") : t("addServer.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
