import { useState } from "react"
import { PlusIcon } from "lucide-react"
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

type AuthType = "password" | "key"

interface ServerFormData {
  name: string
  host: string
  port: number
  username: string
  authType: AuthType
  password: string
  keyPath: string
}

interface TestResult {
  fingerprint: string
}

interface ServerResponse {
  id: string
  name: string
  host: string
  port: number
  auth_type: AuthType
  username: string
  key_path?: string
  host_key_fingerprint?: string
  created_at: string
  updated_at: string
}

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
  onServerAdded?: (server: ServerResponse) => void
  defaultOpen?: boolean
  defaultAuthType?: AuthType
}

export function AddServerDialog({ onServerAdded, defaultOpen = false, defaultAuthType = "password" }: AddServerDialogProps) {
  const [open, setOpen] = useState(defaultOpen)
  const [form, setForm] = useState<ServerFormData>({ ...initialFormData, authType: defaultAuthType })
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState<string | null>(null)
  const [fingerprint, setFingerprint] = useState<string | null>(null)
  const [fingerprintTrusted, setFingerprintTrusted] = useState(false)
  const [saving, setSaving] = useState(false)
  const [createdServerId, setCreatedServerId] = useState<string | null>(null)

  function resetState() {
    setForm({ ...initialFormData })
    setTesting(false)
    setTestError(null)
    setFingerprint(null)
    setFingerprintTrusted(false)
    setSaving(false)
    setCreatedServerId(null)
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
    setCreatedServerId(null)
  }

  function isFormValid(): boolean {
    return (
      form.name.trim() !== "" &&
      form.host.trim() !== "" &&
      form.username.trim() !== ""
    )
  }

  async function handleTestConnection() {
    setTesting(true)
    setTestError(null)
    setFingerprint(null)
    setFingerprintTrusted(false)

    try {
      // First, create the server (or use existing if already created)
      let serverId = createdServerId
      if (!serverId) {
        const createBody = {
          name: form.name.trim(),
          host: form.host.trim(),
          port: form.port,
          username: form.username.trim(),
          auth_type: form.authType,
          ...(form.authType === "password"
            ? { password: form.password }
            : { key_path: form.keyPath }),
        }

        const createRes = await fetch("/api/servers", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(createBody),
        })

        if (!createRes.ok) {
          const err = await createRes.json()
          throw new Error(err.error?.message || "Failed to create server")
        }

        const created: ServerResponse = await createRes.json()
        serverId = created.id
        setCreatedServerId(serverId)
      }

      // Then test the connection
      const testRes = await fetch(`/api/servers/${serverId}/test`, {
        method: "POST",
      })

      if (!testRes.ok) {
        const err = await testRes.json()
        throw new Error(err.error?.message || "Connection test failed")
      }

      const result: TestResult = await testRes.json()
      setFingerprint(result.fingerprint)
    } catch (err) {
      setTestError(err instanceof Error ? err.message : "Connection test failed")
    } finally {
      setTesting(false)
    }
  }

  async function handleSave() {
    if (!createdServerId || !fingerprint || !fingerprintTrusted) return

    setSaving(true)
    try {
      // Update the server with the confirmed fingerprint
      const updateBody = {
        name: form.name.trim(),
        host: form.host.trim(),
        port: form.port,
        username: form.username.trim(),
        auth_type: form.authType,
        host_key_fingerprint: fingerprint,
        ...(form.authType === "password"
          ? { password: form.password }
          : { key_path: form.keyPath }),
      }

      const res = await fetch(`/api/servers/${createdServerId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(updateBody),
      })

      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error?.message || "Failed to save server")
      }

      const saved: ServerResponse = await res.json()
      toast.success(`Server "${saved.name}" added successfully`)
      onServerAdded?.(saved)
      setOpen(false)
      resetState()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save server")
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
          Add Server
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Server</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="server-name">Name</Label>
            <Input
              id="server-name"
              placeholder="Production Web Server"
              value={form.name}
              onChange={(e) => updateField("name", e.target.value)}
            />
          </div>

          <div className="grid grid-cols-[1fr_auto] gap-2">
            <div className="grid gap-2">
              <Label htmlFor="server-host">Host</Label>
              <Input
                id="server-host"
                placeholder="192.168.1.100"
                value={form.host}
                onChange={(e) => updateField("host", e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="server-port">Port</Label>
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
            <Label htmlFor="server-username">Username</Label>
            <Input
              id="server-username"
              placeholder="root"
              value={form.username}
              onChange={(e) => updateField("username", e.target.value)}
            />
          </div>

          <div className="grid gap-2">
            <Label id="auth-type-label">Authentication</Label>
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
              <Label htmlFor="server-password">Password</Label>
              <Input
                id="server-password"
                type="password"
                value={form.password}
                onChange={(e) => updateField("password", e.target.value)}
              />
            </div>
          ) : (
            <div className="grid gap-2">
              <Label htmlFor="server-keypath">Key File Path</Label>
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
              <p className="text-sm font-medium">Host Key Fingerprint</p>
              <code className="block text-xs break-all bg-muted p-2 rounded">
                {fingerprint}
              </code>
              {!fingerprintTrusted ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setFingerprintTrusted(true)}
                >
                  Trust this host
                </Button>
              ) : (
                <p className="text-sm text-green-600 dark:text-green-400">
                  Host trusted
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
            {testing ? "Testing..." : "Test Connection"}
          </Button>
          <Button onClick={handleSave} disabled={!canSave}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
