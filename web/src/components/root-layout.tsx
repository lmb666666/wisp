import { Outlet, useRouterState } from "@tanstack/react-router"
import { useEffect } from "react"

import { useMe } from "@/api/auth"
import { Header } from "@/components/header"
import { InstallHints } from "@/components/install-hints"
import { Toaster } from "@/components/ui/sonner"
import { useOutboxFlusher } from "@/hooks/use-outbox-flusher"

/** 路由切换回到顶部（页面内容动画由各叶页面自己播放，避免布局重挂载）。 */
function ScrollToTop() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  useEffect(() => {
    window.scrollTo(0, 0)
  }, [pathname])
  return null
}

export function RootLayout() {
  const { data: me } = useMe()
  useOutboxFlusher(!!me)
  return (
    <div className="bg-background min-h-dvh">
      <Header />
      <InstallHints />
      <ScrollToTop />
      <main className="wisp-main mx-auto w-full max-w-[720px] px-4">
        <Outlet />
      </main>
      <Toaster />
    </div>
  )
}
