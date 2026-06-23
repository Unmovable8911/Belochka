import { useTranslation } from "react-i18next"
import { useMonitorState } from "../hooks/useMonitorState"

export function ConnectionBanner() {
  const { t } = useTranslation()
  const { state } = useMonitorState()

  if (state.wsConnected) {
    return null
  }

  return (
    <div
      role="alert"
      className="bg-destructive text-destructive-foreground px-4 py-2 text-center text-sm font-medium"
    >
      {t("connection.lost")}
    </div>
  )
}
