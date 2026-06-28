import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { ServerForm } from "@/components/ServerForm"
import { useServerForm } from "@/hooks/useServerForm"
import { toast } from "sonner"
import type { Server, ServerFormData } from "@/types/server"
import * as api from "@/api/client"

export interface EditServerDialogProps {
  server: Server
  open: boolean
  onOpenChange: (open: boolean) => void
  onServerUpdated?: (server: Server) => void
}

/** Fields that require re-testing when changed */
const CONNECTION_FIELDS: (keyof ServerFormData)[] = [
  "host",
  "port",
  "username",
  "authType",
  "password",
  "keyPath",
]

function serverToForm(server: Server): ServerFormData {
  return {
    name: server.name,
    host: server.host,
    port: server.port,
    username: server.username,
    authType: server.auth_type,
    password: "",
    keyPath: server.key_path ?? "",
  }
}

function hasConnectionFieldChanged(
  current: ServerFormData,
  original: ServerFormData,
): boolean {
  for (const field of CONNECTION_FIELDS) {
    if (field === "password" || field === "keyPath") {
      // Non-empty password/key means user changed it
      if (current[field] !== "") return true
    } else if (current[field] !== original[field]) {
      return true
    }
  }
  return false
}

export function EditServerDialog({
  server,
  open,
  onOpenChange,
  onServerUpdated,
}: EditServerDialogProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<ServerFormData>(() => serverToForm(server))
  const [originalForm] = useState<ServerFormData>(() => serverToForm(server))
  const {
    testing,
    testError,
    fingerprint,
    fingerprintTrusted,
    saving,
    setTestError,
    setSaving,
    trust,
    resetTestState,
    runTest,
  } = useServerForm()

  // Re-sync form when server prop changes
  useEffect(() => {
    setForm(serverToForm(server))
  }, [server])

  const connectionChanged = hasConnectionFieldChanged(form, originalForm)

  // When only name changed (no connection fields), save is allowed directly
  const nameOnlyChanged =
    form.name !== originalForm.name && !connectionChanged

  function updateField<K extends keyof ServerFormData>(
    key: K,
    value: ServerFormData[K],
  ) {
    setForm((prev) => ({ ...prev, [key]: value }))

    // Reset test state when connection fields change; otherwise just clear errors.
    if (CONNECTION_FIELDS.includes(key)) {
      resetTestState()
    } else {
      setTestError(null)
    }
  }

  function isFormValid(): boolean {
    return (
      form.name.trim() !== "" &&
      form.host.trim() !== "" &&
      form.username.trim() !== ""
    )
  }

  const hasAnyChange = connectionChanged || form.name !== originalForm.name
  const needsRetest = connectionChanged
  const testPassed = fingerprint !== null && fingerprintTrusted
  const canTest = isFormValid() && !testing && needsRetest
  const canSave =
    isFormValid() &&
    !saving &&
    hasAnyChange &&
    (nameOnlyChanged || (connectionChanged && testPassed))

  function buildUpdateBody(extras?: Record<string, unknown>): Record<string, unknown> {
    const body: Record<string, unknown> = {
      name: form.name.trim(),
      host: form.host.trim(),
      port: form.port,
      username: form.username.trim(),
      auth_type: form.authType,
      ...extras,
    }

    if (form.authType === "password") {
      if (form.password !== "") {
        body.password = form.password
      }
    } else {
      body.key_path = form.keyPath
    }

    return body
  }

  function handleTestConnection() {
    // Stateless test: pass the id so the backend can reuse the stored
    // password when it was not re-entered. Nothing is persisted here.
    return runTest({ ...buildUpdateBody(), id: server.id })
  }

  async function handleSave() {
    if (!canSave) return

    setSaving(true)
    try {
      const extras: Record<string, unknown> = {}
      if (fingerprint) {
        extras.host_key_fingerprint = fingerprint
      }

      const saved = await api.updateServer(server.id, buildUpdateBody(extras))
      toast.success(t("editServer.savedSuccess", { name: saved.name }))
      onServerUpdated?.(saved)
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("addServer.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("editServer.title")}</DialogTitle>
        </DialogHeader>

        <ServerForm
          form={form}
          onFieldChange={updateField}
          idPrefix="edit-"
          fingerprint={fingerprint}
          fingerprintTrusted={fingerprintTrusted}
          onTrust={trust}
          testError={testError}
          passwordPlaceholder="unchanged"
        />

        {needsRetest && !testPassed && !testError && !testing && (
          <p className="text-sm text-yellow-600 dark:text-yellow-400">
            {t("editServer.retestRequired")}
          </p>
        )}

        <DialogFooter>
          {needsRetest && (
            <Button
              variant="outline"
              onClick={handleTestConnection}
              disabled={!canTest}
            >
              {testing ? t("addServer.testing") : t("addServer.testConnection")}
            </Button>
          )}
          <Button onClick={handleSave} disabled={!canSave}>
            {saving ? t("addServer.saving") : t("addServer.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
