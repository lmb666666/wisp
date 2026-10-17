import { Outlet } from "@tanstack/react-router"
import { Link } from "@tanstack/react-router"

import { ThemeToggle } from "@/components/theme-toggle"

export function RootLayout() {
  return (
    <div className="mx-auto flex min-h-dvh max-w-2xl flex-col px-4">
      <header className="sticky top-0 z-10 flex h-14 items-center justify-between bg-background/80 backdrop-blur">
        <Link to="/" className="text-lg font-semibold tracking-tight">
          Wisp
        </Link>
        <ThemeToggle />
      </header>
      <main className="flex-1 pb-16">
        <Outlet />
      </main>
    </div>
  )
}
