import { Link, useRouterState } from "@tanstack/react-router"
import { BarChart3, Languages, LogIn, LogOut, Settings2 } from "lucide-react"
import { useTranslation } from "react-i18next"

import { useLogout, useMe } from "@/api/auth"
import { AppLogo } from "@/components/app-logo"
import { ThemeToggle } from "@/components/theme-toggle"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { toggleLanguage } from "@/i18n"

type NavItem = { to: string; label: string; icon: typeof BarChart3 }

/** 顶栏：品牌 + 主导航 + 外观/语言/账号动作。 */
export function Header() {
  const { data: me } = useMe()
  const logout = useLogout()
  const { t } = useTranslation()
  const pathname = useRouterState({ select: (s) => s.location.pathname })

  const items: NavItem[] = [
    { to: "/stats", label: t("nav.stats"), icon: BarChart3 },
    { to: "/settings", label: t("nav.settings"), icon: Settings2 },
  ]

  return (
    <header className="border-border/70 bg-background/85 sticky top-0 z-20 border-b backdrop-blur-md">
      <div className="mx-auto flex h-14 w-full max-w-[720px] items-center gap-2 px-4">
        <Link to="/" className="mr-1 flex items-center gap-2 rounded-lg py-1">
          <AppLogo className="size-7" />
          <span className="font-display text-[17px] font-semibold tracking-tight">Wisp</span>
        </Link>

        <nav className="ml-auto flex items-center gap-0.5">
          {items.map(({ to, label, icon: Icon }) => {
            const active = pathname === to || pathname.startsWith(to + "/")
            return (
              <Button
                key={to}
                variant="ghost"
                size="sm"
                asChild
                className={cn("text-muted-foreground gap-1.5 px-2.5", active && "bg-accent text-accent-foreground shadow-xs")}
              >
                <Link to={to} aria-label={label} title={label}>
                  <Icon />
                  <span className="hidden sm:inline">{label}</span>
                </Link>
              </Button>
            )
          })}
        </nav>

        <div className="bg-border mx-1 hidden h-5 w-px sm:block" />

        <ThemeToggle />
        <Button
          variant="ghost"
          size="icon"
          aria-label={t("nav.langToggle")}
          title={t("nav.langToggle")}
          onClick={toggleLanguage}
        >
          <Languages />
        </Button>
        {me ? (
          <Button
            variant="ghost"
            size="icon"
            aria-label={t("nav.logout")}
            title={t("nav.logout")}
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
          >
            <LogOut />
          </Button>
        ) : (
          <Button variant="ghost" size="icon" asChild>
            <Link to="/login" aria-label={t("nav.login")} title={t("nav.login")}>
              <LogIn />
            </Link>
          </Button>
        )}
      </div>
    </header>
  )
}
