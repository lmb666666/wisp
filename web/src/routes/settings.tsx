import { Link, Outlet, useRouterState } from "@tanstack/react-router"
import { Database, Download, KeyRound, ShieldCheck, Webhook } from "lucide-react"
import { useTranslation } from "react-i18next"

import { useMe } from "@/api/auth"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const TABS = [
  { to: "/settings", key: "account", icon: ShieldCheck },
  { to: "/settings/tokens", key: "tokens", icon: KeyRound },
  { to: "/settings/storage", key: "storage", icon: Database },
  { to: "/settings/integrations", key: "integrations", icon: Webhook },
  { to: "/settings/data", key: "data", icon: Download },
] as const

/** 设置中心：左侧导航（移动端横向滚动）。 */
export function SettingsLayout() {
  const { t } = useTranslation()
  const { data: me, isLoading } = useMe()
  const pathname = useRouterState({ select: (s) => s.location.pathname })

  if (isLoading) {
    return <div className="bg-muted mx-auto my-8 h-40 w-full max-w-md animate-pulse rounded-2xl" />
  }
  if (!me) {
    return (
      <div className="flex flex-col items-center gap-3 py-20 text-center text-sm">
        <p className="text-muted-foreground">{t("common.needLogin")}</p>
        <Button asChild variant="outline" size="sm">
          <Link to="/login">{t("common.goLogin")}</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="py-6">
      <h1 className="font-display mb-5 text-2xl font-semibold tracking-tight">{t("settings.title")}</h1>
      <div className="grid gap-4 md:grid-cols-[190px_minmax(0,1fr)]">
        <nav data-settings-nav className="wisp-card h-max overflow-x-auto p-1.5 md:sticky md:top-20">
          <div className="grid grid-cols-2 gap-1 md:grid-cols-1">
            {TABS.map(({ to, key, icon: Icon }) => {
              const active = to === "/settings" ? pathname === "/settings" : pathname.startsWith(to)
              return (
                <Link
                  key={to}
                  to={to}
                  className={cn(
                    "flex items-center gap-2 rounded-[10px] px-3 py-2 text-sm transition-colors",
                    active
                      ? "bg-accent text-accent-foreground font-medium shadow-xs"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  <Icon className="size-4" />
                  {t(`settings.tabs.${key}`)}
                </Link>
              )
            })}
          </div>
        </nav>
        <div key={pathname} className="wisp-page min-w-0">
          <Outlet />
        </div>
      </div>
    </div>
  )
}
