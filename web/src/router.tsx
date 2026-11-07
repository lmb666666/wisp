import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
} from "@tanstack/react-router"

import { RootLayout } from "@/components/root-layout"
import { Home } from "@/routes/home"
import { Login } from "@/routes/login"
import { SettingsLayout } from "@/routes/settings"

// 首屏只加载时间线与登录页；其余按路由拆分，避免把统计/设置/编辑器一次性打进主 chunk。
const PostPage = lazyRouteComponent(() => import("@/routes/post"), "PostPage")
const StatsPage = lazyRouteComponent(() => import("@/routes/stats"), "StatsPage")
const SharePage = lazyRouteComponent(() => import("@/routes/share"), "SharePage")
const PasskeysPage = lazyRouteComponent(() => import("@/routes/passkeys"), "PasskeysPage")
const TokensPage = lazyRouteComponent(() => import("@/routes/tokens"), "TokensPage")
const StoragePage = lazyRouteComponent(() => import("@/routes/storage"), "StoragePage")
const WebhooksPage = lazyRouteComponent(() => import("@/routes/webhooks"), "WebhooksPage")
const DataPage = lazyRouteComponent(() => import("@/routes/data"), "DataPage")

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
    tab: ["stats", "mood", "archive", "tags", "overview"].includes(String(search.tab))
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
