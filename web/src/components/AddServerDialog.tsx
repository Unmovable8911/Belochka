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

const initialFormData: ServerFormData = {
  name: "",
  host: "",
  port: 22,
  username: "",
  authType: "password",
  password: "",
  keyPath: "",
}

export interface AddServerDialogProps {
  onServerAdded?: (server: Server) => void
  defaultOpen?: boolean
  defaultAuthType?: AuthType
  triggerLabel?: string
}

export function AddServerDialog({ onServerAdded, defaultOpen = false, defaultAuthType = "password", triggerLabel }: AddServerDialogProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(defaultOpen)
  const [form, setForm] = useState<ServerFormData>({ ...initialFormData, authType: defaultAuthType })
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState<string | null>(null)
  const [fingerprint, setFingerprint] = useState<string | null>(null)
  const [fingerprintTrusted, setFingerprintTrusted] = useState(false)
  const [saving, setSaving] = useState(false)

  function resetState() {
    setForm({ ...initialFormData })
    setTesting(false)
    setTestError(null)
    setFingerprint(null)
    setFingerprintTrusted(false)
    setSaving(false)
  }

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen)
    if (!nextOpen) {
      resetState()
    }
  }

  function updateField<K extends keyof ServerFormData>(key: K, value: ServerFormData[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
    // Reset test state when form changes
    setTestError(null)
    setFingerprint(null)
    setFingerprintTrusted(false)
  }

  function isFormValid(): boolean {
    return (
      form.name.trim() !== "" &&
      form.host.trim() !== "" &&
      form.username.trim() !== ""
    )
  }

  function buildServerBody(extras?: Record<string, unknown>): Record<string, unknown> {
    return {
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
  }

  async function handleTestConnection() {
    setTesting(true)
    setTestError(null)
    setFingerprint(null)
    setFingerprintTrusted(false)

    try {
      const result = await api.testConnection(buildServerBody())
      setFingerprint(result.fingerprint)
    } catch (err) {
      setTestError(err instanceof Error ? err.message : t("addServer.connectionTestFailed"))
    } finally {
      setTesting(false)
    }
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

        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="server-name">{t("addServer.name")}</Label>
            <Input
              id="server-name"
              placeholder="Production Web Server"
              value={form.name}
              onChange={(e) => updateField("name", e.target.value)}
            />
          </div>

          <div className="grid grid-cols-[1fr_auto] gap-2">
            <div className="grid gap-2">
              <Label htmlFor="server-host">{t("addServer.host")}</Label>
              <Input
                id="server-host"
                placeholder="192.168.1.100"
                value={form.host}
                onChange={(e) => updateField("host", e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="server-port">{t("addServer.port")}</Label>
              <Input
                id="server-port"
                type="number"
                className="w-20"
                value={form.port}
                onChange={(e) => updateField("port", parseInt(e.target.value) || 22)}
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="server-username">{t("addServer.username")}</Label>
            <Input
              id="server-username"
              placeholder="root"
              value={form.username}
              onChange={(e) => updateField("username", e.target.value)}
            />
          </div>

          <div className="grid gap-2">
            <Label id="auth-type-label">{t("addServer.authentication")}</Label>
            <Select
              value={form.authType}
              onValueChange={(value: AuthType) => updateField("authType", value)}
            >
              <SelectTrigger className="w-full" aria-labelledby="auth-type-label">
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
              <Label htmlFor="server-password">{t("addServer.password")}</Label>
              <Input
                id="server-password"
                type="password"
                value={form.password}
                onChange={(e) => updateField("password", e.target.value)}
              />
            </div>
          ) : (
            <div className="grid gap-2">
              <Label htmlFor="server-keypath">{t("addServer.keyFilePath")}</Label>
              <Input
                id="server-keypath"
                placeholder="/home/user/.ssh/id_rsa"
                value={form.keyPath}
                onChange={(e) => updateField("keyPath", e.target.value)}
              />
            </div>
          )}

          {testError && (
            <div role="alert" className="rounded-md border border-destructive bg-destructive/10 p-3 text-sm text-destructive">
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
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={handleTestConnection}
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
