import { useState, type FormEvent } from "react"
import { useTranslation } from "react-i18next"
import zxcvbn from "zxcvbn"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { LANGUAGES } from "@/components/SettingsDialog"
import * as api from "@/api/client"
import i18n, { getAppLang } from "@/i18n"

type Step = "language" | "password"

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

export default function SetupPage() {
  const { t } = useTranslation()
  const [step, setStep] = useState<Step>("language")
  const [selectedLang, setSelectedLang] = useState(getAppLang)
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [fading, setFading] = useState(false)

  const strength = password ? zxcvbn(password).score : -1
  const confirmTouched = confirm.length > 0
  const passwordsMatch = password === confirm
  const canSubmit = password.length > 0 && confirm.length > 0 && passwordsMatch

  function transitionTo(next: Step) {
    setFading(true)
    setTimeout(() => {
      setStep(next)
      setError("")
      setFading(false)
    }, 150)
  }

  function selectLanguage(code: string) {
    setSelectedLang(code)
    i18n.changeLanguage(code)
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError("")

    if (!canSubmit) return

    setLoading(true)
    try {
      await api.setup(selectedLang, password, confirm)
      window.location.href = "/"
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.unknownError"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold">{t("auth.setupTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {step === "language" ? t("auth.selectLanguage") : t("auth.setupHint")}
          </p>
        </div>

        <div
          className={`transition-opacity duration-150 ${
            fading ? "opacity-0" : "opacity-100"
          }`}
        >
          {step === "language" && (
            <div className="space-y-4">
              <div className="space-y-2">
                {LANGUAGES.map(({ code, label }) => (
                  <button
                    key={code}
                    type="button"
                    onClick={() => selectLanguage(code)}
                    className={`w-full rounded-lg border px-4 py-3 text-left text-sm font-medium transition-colors hover:bg-accent ${
                      selectedLang === code
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-border"
                    }`}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <Button
                className="w-full"
                onClick={() => transitionTo("password")}
              >
                {t("auth.continue")}
              </Button>
            </div>
          )}

          {step === "password" && (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="grid gap-1.5">
                <Label htmlFor="setup-password">{t("auth.password")}</Label>
                <Input
                  id="setup-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoFocus
                />
                {strength >= 0 && <StrengthBar score={strength} />}
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="setup-confirm">{t("auth.confirmPassword")}</Label>
                <Input
                  id="setup-confirm"
                  type="password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                />
                {confirmTouched && (
                  <p
                    className={`text-xs ${
                      passwordsMatch ? "text-green-500" : "text-red-500"
                    }`}
                  >
                    {passwordsMatch
                      ? t("auth.passwordMatch")
                      : t("auth.passwordNoMatch")}
                  </p>
                )}
              </div>
              {error && (
                <p className="text-sm text-red-500" role="alert">
                  {error}
                </p>
              )}
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  className="flex-1"
                  onClick={() => transitionTo("language")}
                >
                  {t("auth.back")}
                </Button>
                <Button
                  type="submit"
                  className="flex-1"
                  disabled={loading || !canSubmit}
                >
                  {loading ? t("auth.settingUp") : t("auth.setPassword")}
                </Button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
