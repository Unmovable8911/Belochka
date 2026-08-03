import { useState } from "react"
import { useTranslation } from "react-i18next"
import zxcvbn from "zxcvbn"
import * as api from "@/api/client"

export interface UseChangePasswordReturn {
  oldPassword: string
  setOldPassword: (v: string) => void
  newPassword: string
  setNewPassword: (v: string) => void
  confirmPassword: string
  setConfirmPassword: (v: string) => void
  changingPassword: boolean
  showPassword: boolean
  setShowPassword: (v: boolean) => void
  passwordError: string
  passwordSuccess: string
  newStrength: number
  confirmTouched: boolean
  passwordsMatch: boolean
  canChangePassword: boolean
  clearPasswordFeedback: () => void
  handleChangePassword: () => Promise<void>
}

export function useChangePassword(): UseChangePasswordReturn {
  const { t } = useTranslation()

  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [changingPassword, setChangingPassword] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [passwordError, setPasswordError] = useState("")
  const [passwordSuccess, setPasswordSuccess] = useState("")

  const newStrength = newPassword ? zxcvbn(newPassword).score : -1
  const confirmTouched = confirmPassword.length > 0
  const passwordsMatch = newPassword === confirmPassword
  const canChangePassword =
    newPassword.length > 0 && confirmPassword.length > 0 && passwordsMatch

  function clearPasswordFeedback() {
    setPasswordError("")
    setPasswordSuccess("")
  }

  async function handleChangePassword() {
    setPasswordError("")
    setPasswordSuccess("")

    if (!oldPassword || !newPassword || !confirmPassword) {
      setPasswordError(t("auth.fillAllFields"))
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordError(t("auth.passwordsDontMatch"))
      return
    }

    setChangingPassword(true)
    try {
      await api.changePassword(oldPassword, newPassword, confirmPassword)
      setOldPassword("")
      setNewPassword("")
      setConfirmPassword("")
      setShowPassword(false)
      setPasswordSuccess(t("auth.passwordChanged"))
    } catch (err) {
      let msg: string
      if (err instanceof api.ApiError) {
        switch (err.code) {
          case "incorrect_password":
            msg = t("auth.incorrectPassword")
            break
          case "passwords_dont_match":
            msg = t("auth.passwordsDontMatch")
            break
          case "password_too_short":
            msg = t("auth.passwordTooShort")
            break
          default:
            msg = err.message
        }
      } else {
        msg = err instanceof Error ? err.message : t("auth.passwordChangeFailed")
      }
      setPasswordError(msg)
    } finally {
      setChangingPassword(false)
    }
  }

  return {
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
  }
}
