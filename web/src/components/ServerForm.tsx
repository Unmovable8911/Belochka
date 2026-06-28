import { useTranslation } from "react-i18next"
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
import type { AuthType, ServerFormData } from "@/types/server"

export interface ServerFormProps {
  form: ServerFormData
  onFieldChange: <K extends keyof ServerFormData>(key: K, value: ServerFormData[K]) => void
  idPrefix: string
  fingerprint: string | null
  fingerprintTrusted: boolean
  onTrust: () => void
  testError: string | null
  passwordPlaceholder?: string
}

export function ServerForm({
  form,
  onFieldChange,
  idPrefix,
  fingerprint,
  fingerprintTrusted,
  onTrust,
  testError,
  passwordPlaceholder,
}: ServerFormProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-4 py-4">
      <div className="grid gap-2">
        <Label htmlFor={`${idPrefix}server-name`}>{t("addServer.name")}</Label>
        <Input
          id={`${idPrefix}server-name`}
          placeholder="Production Web Server"
          value={form.name}
          onChange={(e) => onFieldChange("name", e.target.value)}
        />
      </div>

      <div className="grid grid-cols-[1fr_auto] gap-2">
        <div className="grid gap-2">
          <Label htmlFor={`${idPrefix}server-host`}>{t("addServer.host")}</Label>
          <Input
            id={`${idPrefix}server-host`}
            placeholder="192.168.1.100"
            value={form.host}
            onChange={(e) => onFieldChange("host", e.target.value)}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor={`${idPrefix}server-port`}>{t("addServer.port")}</Label>
          <Input
            id={`${idPrefix}server-port`}
            type="number"
            className="w-20"
            value={form.port}
            onChange={(e) => onFieldChange("port", parseInt(e.target.value) || 22)}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor={`${idPrefix}server-username`}>{t("addServer.username")}</Label>
        <Input
          id={`${idPrefix}server-username`}
          placeholder="root"
          value={form.username}
          onChange={(e) => onFieldChange("username", e.target.value)}
        />
      </div>

      <div className="grid gap-2">
        <Label id={`${idPrefix}auth-type-label`}>{t("addServer.authentication")}</Label>
        <Select
          value={form.authType}
          onValueChange={(value: AuthType) => onFieldChange("authType", value)}
        >
          <SelectTrigger className="w-full" aria-labelledby={`${idPrefix}auth-type-label`}>
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
          <Label htmlFor={`${idPrefix}server-password`}>{t("addServer.password")}</Label>
          <Input
            id={`${idPrefix}server-password`}
            type="password"
            placeholder={passwordPlaceholder}
            value={form.password}
            onChange={(e) => onFieldChange("password", e.target.value)}
          />
        </div>
      ) : (
        <div className="grid gap-2">
          <Label htmlFor={`${idPrefix}server-keypath`}>{t("addServer.keyFilePath")}</Label>
          <Input
            id={`${idPrefix}server-keypath`}
            placeholder="/home/user/.ssh/id_rsa"
            value={form.keyPath}
            onChange={(e) => onFieldChange("keyPath", e.target.value)}
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
            <Button variant="outline" size="sm" onClick={onTrust}>
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
  )
}
