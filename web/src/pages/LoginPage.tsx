import { useState, useEffect, type FormEvent } from "react"
import { useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import * as api from "@/api/client"
import appIcon from "@/assets/icon.png"

export default function LoginPage() {
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [lockoutSeconds, setLockoutSeconds] = useState(0)

  // Countdown timer for rate-limit lockout.
  useEffect(() => {
    if (lockoutSeconds <= 0) return
    const timer = setInterval(() => {
      setLockoutSeconds((prev) => {
        if (prev <= 1) return 0
        return prev - 1
      })
    }, 1000)
    return () => clearInterval(timer)
  }, [lockoutSeconds])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError("")

    if (!password) {
      setError(t("auth.fillAllFields"))
      return
    }

    setLoading(true)
    try {
      await api.login(password)
      const redirect = searchParams.get("redirect")
      window.location.href = redirect || "/"
    } catch (err) {
      if (err instanceof api.ApiError) {
        if (err.code === "rate_limited") {
          // Try to extract remaining seconds from the message.
          const match = err.message.match(/(\d+)s/)
          if (match) {
            setLockoutSeconds(parseInt(match[1], 10))
          } else {
            setLockoutSeconds(1800) // Default 30 minutes.
          }
          setError(err.message)
        } else {
          setError(err.message)
        }
      } else {
        setError(t("auth.unknownError"))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center">
          <img src={appIcon} alt="" className="mx-auto mb-3 max-w-14 w-full rounded-lg" aria-hidden />
          <h1 className="text-2xl font-bold">Belochka</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("auth.loginHint")}
          </p>
        </div>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid gap-1.5">
            <Label htmlFor="login-password">{t("auth.password")}</Label>
            <Input
              id="login-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoFocus
              disabled={lockoutSeconds > 0}
            />
          </div>
          {error && (
            <p className="text-sm text-red-500" role="alert">{error}</p>
          )}
          <Button
            type="submit"
            className="w-full"
            disabled={loading || lockoutSeconds > 0}
          >
            {lockoutSeconds > 0
              ? t("auth.lockedOut", { seconds: lockoutSeconds })
              : loading
                ? t("auth.loggingIn")
                : t("auth.login")}
          </Button>
        </form>
      </div>
    </div>
  )
}
