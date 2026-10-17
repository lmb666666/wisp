// i18n：中文为默认语言，英文随取随用。语言记忆在 localStorage("wisp-lang")。
import i18next from "i18next"
import { initReactI18next } from "react-i18next"

import en from "@/locales/en.json"
import zh from "@/locales/zh.json"

const stored = localStorage.getItem("wisp-lang")
const detected = stored ?? (navigator.language.toLowerCase().startsWith("zh") ? "zh-CN" : "en")

void i18next.use(initReactI18next).init({
  resources: {
    "zh-CN": { translation: zh },
    en: { translation: en },
  },
  lng: detected,
  fallbackLng: "zh-CN",
  interpolation: { escapeValue: false }, // React 已转义
})

export function toggleLanguage() {
  const next = i18next.language === "zh-CN" ? "en" : "zh-CN"
  void i18next.changeLanguage(next)
  localStorage.setItem("wisp-lang", next)
}
