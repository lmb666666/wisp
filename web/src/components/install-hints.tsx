import { X } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useState } from "react"

type HintKind = "wechat" | "ios"

function detectHint(): HintKind | null {
  if (typeof window === "undefined") return null
  const ua = navigator.userAgent
  if (/MicroMessenger/i.test(ua)) return "wechat"
  const isIOS =
    /iPad|iPhone|iPod/.test(ua) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1)
  const standalone =
    (window.navigator as Navigator & { standalone?: boolean }).standalone === true ||
    window.matchMedia("(display-mode: standalone)").matches
  if (isIOS && !standalone && !localStorage.getItem("wisp-hint-dismissed")) return "ios"
  return null
}

/** PWA 安装/环境提示：微信内降级提示 + iOS 添加主屏引导（可关闭）。 */
export function InstallHints() {
  const [kind, setKind] = useState<HintKind | null>(detectHint)
  const { t } = useTranslation()

  if (!kind) return null

  const dismiss = () => {
    localStorage.setItem("wisp-hint-dismissed", "1")
    setKind(null)
  }

  return (
    <div className="bg-muted/60 text-muted-foreground mx-4 my-2 flex items-start gap-2 rounded-lg border p-3 text-xs">
      {kind === "wechat" ? (
        <p>
          {t("hints.wechat")}
        </p>
      ) : (
        <p>
          {t("hints.ios")}
        </p>
      )}
      <button aria-label={t("hints.dismiss")} onClick={dismiss} className="text-muted-foreground ml-auto shrink-0">
        <X className="size-3.5" />
      </button>
    </div>
  )
}
