import { useQueryClient } from "@tanstack/react-query"
import { useEffect } from "react"
import { toast } from "sonner"

import { flushOutbox } from "@/api/outbox"

/** 离线队列冲刷器：页面加载 / 网络恢复 / 每 30s 重试一次。 */
export function useOutboxFlusher(enabled: boolean) {
  const qc = useQueryClient()

  useEffect(() => {
    if (!enabled) return

    const run = async () => {
      const { sent, needsAuth, dropped } = await flushOutbox()
      if (sent > 0) {
        qc.invalidateQueries({ queryKey: ["posts"] })
        qc.invalidateQueries({ queryKey: ["tags"] })
      }
      if (needsAuth) {
        // 只在真的有内容被挡住时提示（下次冲刷有结果仍会提醒）
        toast.warning("有离线内容等待发布，请先重新登录")
      }
      if (dropped > 0) {
        toast.error("有离线内容发布失败（请求被拒绝），已从队列移除")
      }
    }

    run()
    window.addEventListener("online", run)
    const timer = setInterval(run, 30_000)
    return () => {
      window.removeEventListener("online", run)
      clearInterval(timer)
    }
  }, [enabled, qc])
}
