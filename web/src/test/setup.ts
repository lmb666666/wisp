import "@testing-library/jest-dom/vitest"
import { cleanup } from "@testing-library/react"
import i18next from "i18next"
import { afterEach } from "vitest"

import "@/i18n"

// 测试环境固定中文（import 会被提升，须在 i18n 初始化后显式切换）
void i18next.changeLanguage("zh-CN")

afterEach(cleanup)
