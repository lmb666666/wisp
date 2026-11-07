import { describe, expect, it } from "vitest"

import { countMoods, isMood, mergeMood, moodByDate, type MoodEntry } from "./moods"

describe("moods", () => {
  it("guards the five mood values", () => {
    for (const mood of ["happy", "calm", "okay", "tired", "low"]) {
      expect(isMood(mood)).toBe(true)
    }
    for (const value of ["", "great", "HAPPY", null, 1]) {
      expect(isMood(value)).toBe(false)
    }
  })

  it("merges by date and keeps ascending order", () => {
    const items: MoodEntry[] = [
      { date: "2026-11-01", mood: "calm" },
      { date: "2026-11-03", mood: "tired" },
    ]
    const merged = mergeMood(items, { date: "2026-11-01", mood: "happy" })
    expect(merged).toEqual([
      { date: "2026-11-01", mood: "happy" },
      { date: "2026-11-03", mood: "tired" },
    ])
    expect(mergeMood(merged, { date: "2026-11-02", mood: "okay" })).toHaveLength(3)
  })

  it("maps dates and counts moods", () => {
    const items: MoodEntry[] = [
      { date: "2026-11-01", mood: "calm" },
      { date: "2026-11-02", mood: "calm" },
      { date: "2026-11-03", mood: "happy" },
    ]
    expect(moodByDate(items).get("2026-11-02")).toBe("calm")
    expect(countMoods(items)).toEqual({ happy: 1, calm: 2, okay: 0, tired: 0, low: 0 })
  })
})
