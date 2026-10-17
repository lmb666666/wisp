import { Link, useSearch } from "@tanstack/react-router"

import { useTranslation } from "react-i18next"

import { useMe } from "@/api/auth"
import { PublishBox } from "@/components/publish-box"

/**
 * 系统分享入口（PWA share_target）。
 * Android 安装后，任意 App 的「分享到 Wisp」会带 title/text/url 到这里。
 */
export function SharePage() {
  const { title, text, url } = useSearch({ from: "/share" })
  const { data: me, isLoading } = useMe()
  const { t } = useTranslation()

  const shared = [text || title, url].filter(Boolean).join("\n\n")

  if (isLoading) return <p className="text-muted-foreground py-6 text-sm">{t("common.loading")}</p>
  if (!me)
    return (
      <div className="py-16 text-center text-sm">
        <p className="text-muted-foreground">{t("share.needLogin")}</p>
        <Link to="/login" className="text-primary mt-2 inline-block underline">
          {t("common.goLogin")}
        </Link>
      </div>
    )

  return (
    <div>
      <p className="text-muted-foreground py-2 text-xs">{t("home.fromShare")}</p>
      <PublishBox initialShare={shared} />
    </div>
  )
}
