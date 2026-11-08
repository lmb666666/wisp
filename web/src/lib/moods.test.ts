import { describe, expect, it } from "vitest"

import { countMoods, MOODS, moodByDate, type MoodEntry } from "./moods"

describe("moods", () => {
  it("keeps the five moods in display order", () => {
    expect(MOODS).toEqual(["happy", "calm", "okay", "tired", "low"])
  })

  it("maps dates to moods", () => {
    const items: MoodEntry[] = [
      { date: "2026-11-01", mood: "calm" },
      { date: "2026-11-02", mood: "calm" },
      { date: "2026-11-03", mood: "happy" },
    ]
    expect(moodByDate(items).get("2026-11-02")).toBe("calm")
    expect(moodByDate(items).get("2026-11-04")).toBeUndefined()
  })

  it("counts each mood", () => {
    const items: MoodEntry[] = [
      { date: "2026-11-01", mood: "calm" },
      { date: "2026-11-02", mood: "calm" },
      { date: "2026-11-03", mood: "happy" },
    ]
    expect(countMoods(items)).toEqual({ happy: 1, calm: 2, okay: 0, tired: 0, low: 0 })
  })
})
