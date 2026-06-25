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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { toast } from "sonner"
import type { AuthType, Server, ServerFormData } from "@/types/server"
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
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState<string | null>(null)
  const [fingerprint, setFingerprint] = useState<string | null>(null)
  const [fingerprintTrusted, setFingerprintTrusted] = useState(false)
  const [saving, setSaving] = useState(false)

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
    setTestError(null)

    // Reset test state when connection fields change
    if (CONNECTION_FIELDS.includes(key)) {
      setFingerprint(null)
      setFingerprintTrusted(false)
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

  async function handleTestConnection() {
    setTesting(true)
    setTestError(null)
    setFingerprint(null)
    setFingerprintTrusted(false)

    try {
      await api.updateServer(server.id, buildUpdateBody())
      const result = await api.testConnection(server.id)
      setFingerprint(result.fingerprint)
    } catch (err) {
      setTestError(
        err instanceof Error ? err.message : t("addServer.connectionTestFailed"),
      )
    } finally {
      setTesting(false)
    }
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

        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="edit-server-name">{t("addServer.name")}</Label>
            <Input
              id="edit-server-name"
              placeholder="Production Web Server"
              value={form.name}
              onChange={(e) => updateField("name", e.target.value)}
            />
          </div>

          <div className="grid grid-cols-[1fr_auto] gap-2">
            <div className="grid gap-2">
              <Label htmlFor="edit-server-host">{t("addServer.host")}</Label>
              <Input
                id="edit-server-host"
                placeholder="192.168.1.100"
                value={form.host}
                onChange={(e) => updateField("host", e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit-server-port">{t("addServer.port")}</Label>
              <Input
                id="edit-server-port"
                type="number"
                className="w-20"
                value={form.port}
                onChange={(e) =>
                  updateField("port", parseInt(e.target.value) || 22)
                }
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit-server-username">{t("addServer.username")}</Label>
            <Input
              id="edit-server-username"
              placeholder="root"
              value={form.username}
              onChange={(e) => updateField("username", e.target.value)}
            />
          </div>

          <div className="grid gap-2">
            <Label id="edit-auth-type-label">{t("addServer.authentication")}</Label>
            <Select
              value={form.authType}
              onValueChange={(value: AuthType) =>
                updateField("authType", value)
              }
            >
              <SelectTrigger
                className="w-full"
                aria-labelledby="edit-auth-type-label"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="password">Password</SelectItem>
                <SelectItem value="key">SSH Key</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {form.authType === "password" ? (
            <div className="grid gap-2">
              <Label htmlFor="edit-server-password">{t("addServer.password")}</Label>
              <Input
                id="edit-server-password"
                type="password"
                placeholder="unchanged"
                value={form.password}
                onChange={(e) => updateField("password", e.target.value)}
              />
            </div>
          ) : (
            <div className="grid gap-2">
              <Label htmlFor="edit-server-keypath">{t("addServer.keyFilePath")}</Label>
              <Input
                id="edit-server-keypath"
                placeholder="/home/user/.ssh/id_rsa"
                value={form.keyPath}
                onChange={(e) => updateField("keyPath", e.target.value)}
              />
            </div>
          )}

          {testError && (
            <div
              role="alert"
              className="rounded-md border border-destructive bg-destructive/10 p-3 text-sm text-destructive"
            >
              {testError}
            </div>
          )}

          {fingerprint && (
            <div className="rounded-md border p-3 space-y-2">
              <p className="text-sm font-medium">{t("addServer.hostKeyFingerprint")}</p>
              <code className="block text-xs break-all bg-muted p-2 rounded">
                {fingerprint}
              </code>
              {!fingerprintTrusted ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setFingerprintTrusted(true)}
                >
                  {t("addServer.trustThisHost")}
                </Button>
              ) : (
                <p className="text-sm text-green-600 dark:text-green-400">
                  {t("addServer.hostTrusted")}
                </p>
              )}
            </div>
          )}

          {needsRetest && !testPassed && !testError && !testing && (
            <p className="text-sm text-yellow-600 dark:text-yellow-400">
              {t("editServer.retestRequired")}
            </p>
          )}
        </div>

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
