import { describe, expect, it } from "vitest"

import { dayLabel } from "./dates"

const t = (key: string, vars?: Record<string, unknown>) =>
  vars ? `${key}:${String(vars.m)}/${String(vars.d)}` : key

describe("dayLabel", () => {
  it("labels today and yesterday", () => {
    const now = new Date()
    expect(dayLabel(Math.floor(now.getTime() / 1000), t)).toBe("time.today")

    const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 12)
    expect(dayLabel(Math.floor(yesterday.getTime() / 1000), t)).toBe("time.yesterday")
  })

  it("formats older dates as month/day", () => {
    const older = new Date(2024, 2, 5, 12)
    expect(dayLabel(Math.floor(older.getTime() / 1000), t)).toBe("time.date:3/5")
  })
})
