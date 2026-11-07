import { fireEvent, render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { MoodPicker } from "./mood-picker"

describe("MoodPicker", () => {
  it("renders five moods and reports the clicked one", () => {
    const onSelect = vi.fn()
    render(<MoodPicker value="calm" onSelect={onSelect} />)

    expect(screen.getAllByRole("button")).toHaveLength(5)
    expect(screen.getByRole("button", { name: "平静" })).toHaveAttribute("aria-pressed", "true")
    expect(screen.getByRole("button", { name: "开心" })).toHaveAttribute("aria-pressed", "false")

    fireEvent.click(screen.getByRole("button", { name: "开心" }))
    expect(onSelect).toHaveBeenCalledWith("happy")
  })

  it("disables all options while saving", () => {
    render(<MoodPicker disabled onSelect={() => {}} />)
    for (const button of screen.getAllByRole("button")) {
      expect(button).toBeDisabled()
    }
  })
})
