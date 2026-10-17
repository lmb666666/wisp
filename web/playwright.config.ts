import { defineConfig } from "@playwright/test"

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: process.env.WISP_URL ?? "http://localhost:18099",
    viewport: { width: 390, height: 844 }, // iPhone 14 Pro 视口
    hasTouch: true,
    locale: "zh-CN", // 断言基于中文界面
  },
  workers: 1, // 顺序执行：用例共享站点状态
})
