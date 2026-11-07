import { Moon, Sun } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { applyTheme, currentTheme } from "@/lib/theme"

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
      <span className="transition-transform duration-300 group-hover:rotate-12">
        {theme === "dark" ? <Sun /> : <Moon />}
      </span>
    </Button>
  )
}
