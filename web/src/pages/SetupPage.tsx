import { useState, type FormEvent } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import * as api from "@/api/client"

export default function SetupPage() {
  const { t } = useTranslation()
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError("")

    if (!password || !confirm) {
      setError(t("auth.fillAllFields"))
      return
    }
    if (password !== confirm) {
      setError(t("auth.passwordsDontMatch"))
      return
    }

    setLoading(true)
    try {
      await api.setup(password, confirm)
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
            {t("auth.setupHint")}
          </p>
        </div>
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
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="setup-confirm">{t("auth.confirmPassword")}</Label>
            <Input
              id="setup-confirm"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
            />
          </div>
          {error && (
            <p className="text-sm text-red-500" role="alert">{error}</p>
          )}
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? t("auth.settingUp") : t("auth.setPassword")}
          </Button>
        </form>
      </div>
    </div>
  )
}
