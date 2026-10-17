import { Moon, Sun } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

const STORAGE_KEY = "wisp-theme"

export function currentTheme(): "light" | "dark" {
  return document.documentElement.classList.contains("dark") ? "dark" : "light"
}

export function applyTheme(theme: "light" | "dark") {
  const commit = () => {
    document.documentElement.classList.toggle("dark", theme === "dark")
    localStorage.setItem(STORAGE_KEY, theme)
  }
  const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown }
  const reduce =
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  if (doc.startViewTransition && !reduce) doc.startViewTransition(commit)
  else commit()
}

/** 亮 / 暗切换（跟随 index.html 内联脚本初始化，无闪烁） */
export function ThemeToggle() {
  const [theme, setTheme] = useState<"light" | "dark">(currentTheme)
  const { t } = useTranslation()

  return (
    <Button
      variant="ghost"
      size="icon"
      className="group"
      aria-label={t("nav.themeToggle")}
      title={t("nav.themeToggle")}
      onClick={() => {
        const next = theme === "dark" ? "light" : "dark"
        setTheme(next)
        applyTheme(next)
      }}
    >
      <span className="transition-transform duration-300 group-hover:rotate-12">{theme === "dark" ? <Sun /> : <Moon />}</span>
    </Button>
  )
}
