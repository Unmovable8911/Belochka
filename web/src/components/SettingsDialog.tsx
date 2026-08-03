import { useState } from "react"
import { Settings, RefreshCw, Globe, Server, ScrollText, Lock } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
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
import { useChangePassword } from "@/hooks/useChangePassword"
import { useConfigForm } from "@/hooks/useConfigForm"

// Sorted by native language name (Latin scripts first, then Cyrillic, CJK).
export const LANGUAGES = [
  { code: "de", label: "Deutsch" },
  { code: "en", label: "English" },
  { code: "es", label: "Español" },
  { code: "fr", label: "Français" },
  { code: "it", label: "Italiano" },
  { code: "pt", label: "Português" },
  { code: "ru", label: "Русский" },
  { code: "zh", label: "中文" },
  { code: "zh-TW", label: "繁體中文" },
] as const

function StrengthBar({ score }: { score: number }) {
  const { t } = useTranslation()
  const labels = ["0", "1", "2", "3", "4"] as const
  const colors = [
    "bg-red-500",
    "bg-orange-500",
    "bg-yellow-500",
    "bg-lime-500",
    "bg-green-500",
  ]

  return (
    <div className="space-y-1">
      <div className="flex gap-1">
        {[0, 1, 2, 3, 4].map((i) => (
          <div
            key={i}
            className="h-1.5 flex-1 rounded-full bg-muted transition-colors"
          >
            <div
              className={`h-full rounded-full transition-all ${
                i <= score ? colors[score] : "bg-transparent"
              }`}
            />
          </div>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">
        {t(`auth.passwordStrength.${labels[score]}`)}
      </p>
    </div>
  )
}

function SectionHeader({
  icon: Icon,
  label,
}: {
  icon: React.ComponentType<{ className?: string }>
  label: string
}) {
  return (
    <h3 className="flex items-center gap-2 text-sm font-semibold border-b pb-2">
      <Icon className="size-4 text-muted-foreground" />
      {label}
    </h3>
  )
}

function ToggleSwitch({
  id,
  checked,
  onChange,
}: {
  id: string
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  return (
    <label
      htmlFor={id}
      className="relative inline-flex items-center cursor-pointer select-none"
    >
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="peer sr-only"
      />
      <div className="h-5 w-9 rounded-full bg-muted peer-checked:bg-primary transition-colors peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/50" />
      <div className="absolute start-0.5 size-4 rounded-full bg-background border border-border transition-transform peer-checked:translate-x-4" />
    </label>
  )
}

export function SettingsDialog({ children }: { children?: React.ReactNode }) {
  const { t, i18n } = useTranslation()
  const [open, setOpen] = useState(false)

  const {
    form,
    saving,
    restartFields,
    showRestartDialog,
    restarting,
    loadConfig,
    updateField,
    handleSave,
    handleRestart,
    dismissRestartDialog,
  } = useConfigForm(i18n)

  const {
    oldPassword,
    setOldPassword,
    newPassword,
    setNewPassword,
    confirmPassword,
    setConfirmPassword,
    changingPassword,
    showPassword,
    setShowPassword,
    passwordError,
    passwordSuccess,
    newStrength,
    confirmTouched,
    passwordsMatch,
    canChangePassword,
    clearPasswordFeedback,
    handleChangePassword,
  } = useChangePassword()

  async function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      const ok = await loadConfig()
      if (!ok) setOpen(false)
    }
  }

  async function onSave() {
    const canClose = await handleSave()
    if (canClose) setOpen(false)
  }

  function onDismissRestart() {
    dismissRestartDialog()
    setOpen(false)
  }

  const fieldsList = restartFields.join("、")

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogTrigger asChild>
          {children ? (
            children
          ) : (
            <Button variant="ghost" size="icon" aria-label={t("settings.title")}>
              <Settings className="size-4" />
            </Button>
          )}
        </DialogTrigger>
        <DialogContent className="sm:max-w-lg max-h-[85vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>{t("settings.title")}</DialogTitle>
            <DialogDescription>
              {t("settings.description")}
            </DialogDescription>
          </DialogHeader>

          {form && (
            <div className="flex-1 overflow-y-auto scrollbar-hidden min-h-0 -mx-6 px-6">
              <div className="grid gap-6 py-2">

                {/* General */}
                <section className="grid gap-3">
                  <SectionHeader icon={Globe} label={t("settings.sections.general")} />
                  <div className="grid gap-1.5">
                    <Label htmlFor="settings-language">{t("settings.language")}</Label>
                    <Select
                      value={form.language}
                      onValueChange={(val) => updateField("language", val)}
                    >
                      <SelectTrigger id="settings-language" aria-label={t("settings.language")}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {LANGUAGES.map(({ code, label }) => (
                          <SelectItem key={code} value={code}>
                            {label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">
                      {t("settings.languageDesc")}
                    </p>
                  </div>
                </section>

                {/* Server */}
                <section className="grid gap-3">
                  <SectionHeader icon={Server} label={t("settings.sections.server")} />
                  <div className="grid gap-1.5">
                    <Label htmlFor="settings-port">{t("settings.port")}</Label>
                    <Input
                      id="settings-port"
                      type="number"
                      value={form.port}
                      onChange={(e) => updateField("port", Number(e.target.value))}
                    />
                    <p className="text-xs text-muted-foreground">
                      {t("settings.portDesc")}
                    </p>
                  </div>
                  <div className="grid gap-1.5">
                    <Label htmlFor="settings-data-dir">{t("settings.dataDir")}</Label>
                    <Input
                      id="settings-data-dir"
                      value={form.data_dir}
                      onChange={(e) => updateField("data_dir", e.target.value)}
                    />
                    <p className="text-xs text-muted-foreground">
                      {t("settings.dataDirDesc")}
                    </p>
                  </div>
                </section>

                {/* Logging */}
                <section className="grid gap-3">
                  <SectionHeader icon={ScrollText} label={t("settings.sections.logging")} />
                  <div className="grid gap-1.5">
                    <Label htmlFor="settings-log-path">{t("settings.logPath")}</Label>
                    <Input
                      id="settings-log-path"
                      value={form.log_path}
                      placeholder={t("settings.logPathPlaceholder")}
                      onChange={(e) => updateField("log_path", e.target.value)}
                    />
                    <p className="text-xs text-muted-foreground">
                      {t("settings.logPathDesc")}
                    </p>
                  </div>
                  <div className="grid gap-1.5">
                    <Label htmlFor="settings-log-retention">{t("settings.logRetentionDays")}</Label>
                    <Input
                      id="settings-log-retention"
                      type="number"
                      value={form.log_retention_days}
                      onChange={(e) => updateField("log_retention_days", Number(e.target.value))}
                    />
                    <p className="text-xs text-muted-foreground">
                      {t("settings.logRetentionDesc")}
                    </p>
                  </div>
                </section>

                {/* Security — Change password */}
                <section className="grid gap-3">
                  <SectionHeader icon={Lock} label={t("settings.sections.security")} />
                  <div className="space-y-4">
                    <div className="grid gap-1.5">
                      <Label htmlFor="settings-old-password">{t("auth.oldPassword")}</Label>
                      <Input
                        id="settings-old-password"
                        type={showPassword ? "text" : "password"}
                        value={oldPassword}
                        onChange={(e) => { setOldPassword(e.target.value); clearPasswordFeedback() }}
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor="settings-new-password">{t("auth.newPassword")}</Label>
                      <Input
                        id="settings-new-password"
                        type={showPassword ? "text" : "password"}
                        value={newPassword}
                        onChange={(e) => { setNewPassword(e.target.value); clearPasswordFeedback() }}
                      />
                      {newStrength >= 0 && <StrengthBar score={newStrength} />}
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor="settings-confirm-password">{t("auth.confirmNewPassword")}</Label>
                      <Input
                        id="settings-confirm-password"
                        type={showPassword ? "text" : "password"}
                        value={confirmPassword}
                        onChange={(e) => { setConfirmPassword(e.target.value); clearPasswordFeedback() }}
                      />
                      {confirmTouched && !passwordError && (
                        <p
                          className={`text-xs ${
                            passwordsMatch ? "text-green-500" : "text-destructive"
                          }`}
                        >
                          {passwordsMatch
                            ? t("auth.passwordMatch")
                            : t("auth.passwordNoMatch")}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <ToggleSwitch
                        id="settings-show-password"
                        checked={showPassword}
                        onChange={setShowPassword}
                      />
                      <Label htmlFor="settings-show-password" className="text-sm font-normal text-muted-foreground cursor-pointer">
                        {t("auth.showPassword")}
                      </Label>
                    </div>
                    {passwordError && (
                      <p className="text-sm text-destructive" role="alert">{passwordError}</p>
                    )}
                    {passwordSuccess && (
                      <p className="text-sm text-green-500" role="status">{passwordSuccess}</p>
                    )}
                    <Button
                      variant="secondary"
                      onClick={handleChangePassword}
                      disabled={changingPassword || !oldPassword || !canChangePassword}
                    >
                      {changingPassword ? t("auth.changingPassword") : t("auth.changePassword")}
                    </Button>
                  </div>
                </section>

              </div>
            </div>
          )}

          <DialogFooter>
            <Button onClick={onSave} disabled={saving || !form}>
              {saving ? t("settings.saving") : t("settings.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Restart dialog */}
      <Dialog open={showRestartDialog} onOpenChange={(next) => { if (!next) onDismissRestart() }}>
        <DialogContent className="sm:max-w-md" showCloseButton={!restarting}>
          <DialogHeader>
            <DialogTitle>{t("settings.restartDialog.title")}</DialogTitle>
            <DialogDescription>
              {t("settings.restartDialog.message")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {restartFields.length > 0 && (
              <p className="text-sm text-muted-foreground">
                {t("settings.restartDialog.fields", { fields: fieldsList })}
              </p>
            )}
            {restarting && (
              <div className="flex items-center justify-center gap-2 py-2 text-sm text-muted-foreground">
                <RefreshCw className="size-4 animate-spin" />
                <span>{t("settings.restartDialog.restarting")}</span>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={onDismissRestart} disabled={restarting}>
              {t("settings.restartDialog.later")}
            </Button>
            <Button onClick={handleRestart} disabled={restarting}>
              {restarting ? t("settings.restartDialog.restarting") : t("settings.restartDialog.restartNow")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
