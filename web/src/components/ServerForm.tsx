import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { UploadIcon, RefreshCwIcon } from "lucide-react"
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
import { uploadKeyFile } from "@/api/client"

const maxKeyFileSize = 16 * 1024 // 16 KB

export interface ServerFormProps {
  form: ServerFormData
  onFieldChange: <K extends keyof ServerFormData>(key: K, value: ServerFormData[K]) => void
  idPrefix: string
  fingerprint: string | null
  fingerprintTrusted: boolean
  onTrust: () => void
  testError: string | null
  passwordPlaceholder?: string
  existingKeyPath?: string
}

type UploadState = "idle" | "uploading" | "error"

export function ServerForm({
  form,
  onFieldChange,
  idPrefix,
  fingerprint,
  fingerprintTrusted,
  onTrust,
  testError,
  passwordPlaceholder,
  existingKeyPath,
}: ServerFormProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [uploadState, setUploadState] = useState<UploadState>("idle")
  const [uploadError, setUploadError] = useState<string | null>(null)

  const displayKeyPath = form.keyPath || existingKeyPath

  function handleUploadClick() {
    fileInputRef.current?.click()
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return

    // Reset the input so the same file can be re-selected.
    e.target.value = ""

    if (file.size > maxKeyFileSize) {
      setUploadState("error")
      setUploadError(t("addServer.keyFileTooLarge"))
      return
    }

    setUploadState("uploading")
    setUploadError(null)

    try {
      const result = await uploadKeyFile(file)
      onFieldChange("keyPath", result.path)
      setUploadState("idle")
    } catch (err) {
      setUploadState("error")
      setUploadError(
        err instanceof Error ? err.message : t("addServer.uploadKeyFailed"),
      )
    }
  }

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
            onChange={(e) => onFieldChange("port", e.target.value === "" ? 22 : parseInt(e.target.value, 10))}
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
            <SelectItem value="password">{t("serverForm.authTypePassword")}</SelectItem>
            <SelectItem value="key">{t("serverForm.authTypeKey")}</SelectItem>
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
          <Label htmlFor={`${idPrefix}key-file-path`}>{t("addServer.keyFilePath")}</Label>
          <input
            id={`${idPrefix}key-file-path`}
            ref={fileInputRef}
            type="file"
            className="hidden"
            accept=".key,.pem,.ppk"
            onChange={handleFileChange}
          />

          {displayKeyPath ? (
            <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <code
                className="truncate rounded bg-muted px-3 py-2 text-xs"
                title={displayKeyPath}
              >
                {displayKeyPath}
              </code>
              <Button
                variant="outline"
                size="sm"
                onClick={handleUploadClick}
                disabled={uploadState === "uploading"}
              >
                <RefreshCwIcon
                  className={`size-3 ${uploadState === "uploading" ? "animate-spin" : ""}`}
                />
                {existingKeyPath
                  ? t("addServer.replaceKey")
                  : t("addServer.uploadKey")}
              </Button>
            </div>
          ) : (
            <Button
              variant="outline"
              onClick={handleUploadClick}
              disabled={uploadState === "uploading"}
            >
              {uploadState === "uploading" ? (
                <>
                  <RefreshCwIcon className="size-4 animate-spin" />
                  {t("addServer.uploadingKey")}
                </>
              ) : (
                <>
                  <UploadIcon className="size-4" />
                  {t("addServer.uploadKey")}
                </>
              )}
            </Button>
          )}

          {uploadState === "error" && uploadError && (
            <p className="text-sm text-destructive">{uploadError}</p>
          )}
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
