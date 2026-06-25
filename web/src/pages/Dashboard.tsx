import { ServerIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { AddServerDialog } from "@/components/AddServerDialog"
import { LanguageSwitcher } from "@/components/LanguageSwitcher"
import { ServerCard } from "@/components/ServerCard"
import { useMonitorState } from "@/hooks/useMonitorState"

export default function Dashboard() {
  const { t } = useTranslation()
  const { state } = useMonitorState()
  const hasServers = state.servers.length > 0

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">{t("dashboard.title")}</h1>
        <div className="flex items-center gap-2">
          {hasServers && <AddServerDialog />}
          <LanguageSwitcher />
        </div>
      </div>
      {hasServers ? (
        <div
          data-testid="server-grid"
          className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4"
        >
          {state.servers.map((server) => (
            <ServerCard
              key={server.id}
              server={server}
              metrics={state.metrics[server.id]}
            />
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <ServerIcon className="size-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold mb-2">{t("dashboard.noServers")}</h2>
          <p className="text-muted-foreground mb-6">
            {t("dashboard.noServersHint")}
          </p>
          <AddServerDialog triggerLabel={t("dashboard.addFirstServer")} />
        </div>
      )}
    </div>
  )
}
