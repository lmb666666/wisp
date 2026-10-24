import { useEffect } from "react"
import {
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  useRouterState,
} from "@tanstack/react-router"

import { useMe } from "@/api/auth"
import { useOutboxFlusher } from "@/hooks/use-outbox-flusher"
import { Header } from "@/components/header"
import { Toaster } from "@/components/ui/sonner"
import { InstallHints } from "@/components/install-hints"
import { DataPage } from "@/routes/data"
import { Home } from "@/routes/home"
import { Login } from "@/routes/login"
import { PasskeysPage } from "@/routes/passkeys"
import { PostPage } from "@/routes/post"
import { SettingsLayout } from "@/routes/settings"
import { SharePage } from "@/routes/share"
import { StatsPage } from "@/routes/stats"
import { StoragePage } from "@/routes/storage"
import { TokensPage } from "@/routes/tokens"
import { WebhooksPage } from "@/routes/webhooks"

/** 路由切换回到顶部（页面内容动画由各叶页面自己播放，避免布局重挂载）。 */
function ScrollToTop() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  useEffect(() => {
    window.scrollTo(0, 0)
  }, [pathname])
  return null
}

function RootLayout() {
  const { data: me } = useMe()
  useOutboxFlusher(!!me)
  return (
    <div className="bg-background min-h-dvh">
      <Header />
      <InstallHints />
      <ScrollToTop />
      <main className="mx-auto w-full max-w-[720px] px-4 pb-24">
        <Outlet />
      </main>
      <Toaster />
    </div>
  )
}

const rootRoute = createRootRoute({ component: RootLayout })

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Home,
  validateSearch: (
    search: Record<string, unknown>,
  ): { tag?: string; q?: string; from?: number; to?: number; tags?: string } => ({
    tag: typeof search.tag === "string" && search.tag ? search.tag : undefined,
    q: typeof search.q === "string" && search.q ? search.q : undefined,
    from: Number(search.from) > 0 ? Number(search.from) : undefined,
    to: Number(search.to) > 0 ? Number(search.to) : undefined,
    tags: typeof search.tags === "string" && search.tags ? search.tags : undefined,
  }),
})

const postRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/p/$postId",
  component: PostPage,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: Login,
})

const statsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/stats",
  component: StatsPage,
  validateSearch: (search: Record<string, unknown>): { tab?: string } => ({
    tab: ["stats", "archive", "tags", "overview"].includes(String(search.tab))
      ? String(search.tab)
      : undefined,
  }),
})

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  component: SettingsLayout,
})

const settingsIndex = createRoute({
  getParentRoute: () => settingsRoute,
  path: "/",
  component: PasskeysPage,
})
const settingsTokens = createRoute({
  getParentRoute: () => settingsRoute,
  path: "/tokens",
  component: TokensPage,
})
const settingsStorage = createRoute({
  getParentRoute: () => settingsRoute,
  path: "/storage",
  component: StoragePage,
})
const settingsIntegrations = createRoute({
  getParentRoute: () => settingsRoute,
  path: "/integrations",
  component: WebhooksPage,
})
const settingsData = createRoute({
  getParentRoute: () => settingsRoute,
  path: "/data",
  component: DataPage,
})

const shareRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/share",
  component: SharePage,
  validateSearch: (
    search: Record<string, unknown>,
  ): { title?: string; text?: string; url?: string } => ({
    title: typeof search.title === "string" ? search.title : undefined,
    text: typeof search.text === "string" ? search.text : undefined,
    url: typeof search.url === "string" ? search.url : undefined,
  }),
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([
    indexRoute,
    postRoute,
    loginRoute,
    statsRoute,
    settingsRoute.addChildren([
      settingsIndex,
      settingsTokens,
      settingsStorage,
      settingsIntegrations,
      settingsData,
    ]),
    shareRoute,
  ]),
})

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router
  }
}
