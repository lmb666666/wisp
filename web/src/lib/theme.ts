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
