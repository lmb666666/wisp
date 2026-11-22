import { useQueryClient } from "@tanstack/react-query"
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { flushOutbox } from "@/api/outbox"

/** 离线队列冲刷器：页面加载 / 网络恢复 / 每 30s 重试一次。 */
export function useOutboxFlusher(enabled: boolean) {
  const qc = useQueryClient()
  const { t } = useTranslation()

  useEffect(() => {
    if (!enabled) return

    let lastWarn = 0
    const run = async () => {
      const { sent, needsAuth, dropped } = await flushOutbox()
      if (sent > 0) {
        qc.invalidateQueries({ queryKey: ["posts"] })
        qc.invalidateQueries({ queryKey: ["tags"] })
      }
      if (needsAuth) {
        // 同一会话 60s 内只提醒一次：30s 的重试节奏不该变成 toast 轰炸
        if (Date.now() - lastWarn > 60_000) {
          lastWarn = Date.now()
          toast.warning(t("outbox.needsAuth"))
        }
      }
      if (dropped > 0) {
        toast.error(t("outbox.dropped"))
      }
    }

    run()
    window.addEventListener("online", run)
    const timer = setInterval(run, 30_000)
    return () => {
      window.removeEventListener("online", run)
      clearInterval(timer)
    }
  }, [enabled, qc, t])
}
