import { useQueryClient } from "@tanstack/react-query"
import { useEffect } from "react"

import { flushOutbox } from "@/api/outbox"

/** 离线队列冲刷器：页面加载 / 网络恢复 / 每 30s 重试一次。 */
export function useOutboxFlusher(enabled: boolean) {
  const qc = useQueryClient()

  useEffect(() => {
    if (!enabled) return

    const run = async () => {
      const sent = await flushOutbox()
      if (sent > 0) {
        qc.invalidateQueries({ queryKey: ["posts"] })
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
