import { describe, expect, it } from "vitest"

import { dateKey, dayLabel, daysBetween, todayKey } from "./dates"

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

describe("date keys", () => {
  it("uses the local calendar date", () => {
    const local = new Date(2026, 10, 7, 12, 30)
    expect(dateKey(local.getTime() / 1000)).toBe("2026-11-07")
    expect(todayKey()).toBe(dateKey(Date.now() / 1000))
  })

  it("counts days across month/year/leap boundaries", () => {
    expect(daysBetween("2026-11-01", "2026-11-07")).toBe(6)
    expect(daysBetween("2026-12-31", "2027-01-01")).toBe(1)
    expect(daysBetween("2024-02-28", "2024-03-01")).toBe(2)
    expect(daysBetween("2026-11-07", "2026-11-01")).toBe(-6)
  })
})
