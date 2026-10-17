import { fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it } from "vitest"

import { ThemeToggle, applyTheme } from "./theme-toggle"

describe("ThemeToggle", () => {
  afterEach(() => {
    localStorage.clear()
    applyTheme("light")
  })

  it("toggles dark class on html element", () => {
    render(<ThemeToggle />)
    const btn = screen.getByRole("button", { name: "切换深浅色" })

    expect(document.documentElement.classList.contains("dark")).toBe(false)
    fireEvent.click(btn)
    expect(document.documentElement.classList.contains("dark")).toBe(true)
    expect(localStorage.getItem("wisp-theme")).toBe("dark")
    fireEvent.click(btn)
    expect(document.documentElement.classList.contains("dark")).toBe(false)
  })
})
