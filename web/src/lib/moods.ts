export const MOODS = ["happy", "calm", "okay", "tired", "low"] as const

export type Mood = (typeof MOODS)[number]

export interface MoodEntry {
  date: string
  mood: Mood
}

/** 日期 → 心情，供日期头/年历 O(1) 查询。 */
export function moodByDate(items: MoodEntry[]): Map<string, Mood> {
  return new Map(items.map((item) => [item.date, item.mood]))
}

/** 各心情的记录天数。 */
export function countMoods(items: MoodEntry[]): Record<Mood, number> {
  const counts = Object.fromEntries(MOODS.map((mood) => [mood, 0])) as Record<Mood, number>
  for (const item of items) counts[item.mood]++
  return counts
}
