import type { ErrorComponentProps } from "@tanstack/react-router"

import { Button } from "@/components/ui/button"
import { useTranslation } from "react-i18next"

// 路由级错误兜底：渲染异常或 chunk 加载失败时给出降级 UI，
// 而不是整棵树白屏（TanStack Router defaultErrorComponent）。
export function RouteError({ error }: ErrorComponentProps) {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3 px-6 text-center">
      <p className="text-lg font-medium">{t("error.title")}</p>
      <p className="max-w-md text-sm text-muted-foreground">
        {error instanceof Error ? error.message : t("error.unknown")}
      </p>
      <Button variant="outline" size="sm" onClick={() => window.location.reload()}>
        {t("error.retry")}
      </Button>
    </div>
  )
}
