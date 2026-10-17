/// <reference types="vitest/config" />
import path from "node:path"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import { defineConfig } from "vite"
import { VitePWA } from "vite-plugin-pwa"

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: "autoUpdate",
      includeAssets: [
        "icon.svg",
        "icon-192.png",
        "icon-512.png",
        "icon-maskable-512.png",
        "apple-touch-icon.png",
        "favicon.ico",
      ],
      manifest: {
        name: "Wisp",
        short_name: "Wisp",
        description: "自托管轻量说说：手机上 3 秒发出一条带图的想法",
        lang: "zh-CN",
        display: "standalone",
        start_url: "/",
        theme_color: "#b85a2f",
        background_color: "#f8f1e7",
        icons: [
          { src: "/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
          { src: "/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
          {
            src: "/icon-maskable-512.png",
            sizes: "512x512",
            type: "image/png",
            purpose: "maskable",
          },
          { src: "/icon.svg", sizes: "any", type: "image/svg+xml", purpose: "any" },
        ],
        // 系统分享菜单直达发布（Android 安装后生效）
        share_target: {
          action: "/share",
          method: "GET",
          params: {
            title: "title",
            text: "text",
            url: "url",
          },
        },
      },
      workbox: {
        // SPA 离线壳：导航请求回退到缓存的 index.html
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/api\//, /^\/media\//, /^\/feed\.xml$/],
        runtimeCaching: [
          {
            urlPattern: ({ url, request }: { url: URL; request: Request }) =>
              url.pathname.startsWith("/media/") && request.method === "GET",
            handler: "CacheFirst",
            options: {
              cacheName: "media",
              expiration: { maxEntries: 300, maxAgeSeconds: 60 * 60 * 24 * 30 },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  build: {
    rollupOptions: {
      output: {
        // 编辑器与看图组件按需加载，控制首屏体积（函数形式：Rolldown 兼容）
        manualChunks(id: string) {
          if (!id.includes("node_modules")) return undefined
          if (id.includes("@tiptap") || id.includes("tiptap-markdown")) return "editor"
          if (id.includes("yet-another-react-lightbox")) return "lightbox"
          return undefined
        },
      },
    },
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["src/test/setup.ts"],
    exclude: ["**/node_modules/**", "**/dist/**", "e2e/**"],
  },
})
