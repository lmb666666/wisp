/** 两位数补零。 */
export function pad2(n: number): string {
  return String(n).padStart(2, "0")
}

/** 本地时区的 YYYY-MM-DD（心情按作者浏览器日期存储）。 */
export function dateKey(tsSeconds: number): string {
  const d = new Date(tsSeconds * 1000)
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}

/** 今天的本地日期键。 */
export function todayKey(): string {
  return dateKey(Date.now() / 1000)
}

/** 向前 N 天的本地日期键（用于接口范围 clamp）。 */
export function daysAgoKey(days: number): string {
  return dateKey((Date.now() - days * 86400000) / 1000)
}

const utcDay = (key: string): number => {
  const y = Number(key.slice(0, 4))
  const m = Number(key.slice(5, 7))
  const d = Number(key.slice(8, 10))
  return Date.UTC(y, m - 1, d)
}

/** 两个 YYYY-MM-DD 之间相差的天数（to - from，不受 DST 影响）。 */
export function daysBetween(from: string, to: string): number {
  return Math.round((utcDay(to) - utcDay(from)) / 86400000)
}

/** 时间线按日分组的标题文案。 */
export function dayLabel(
  ts: number,
  t: (k: string, v?: Record<string, unknown>) => string,
): string {
  const d = new Date(ts * 1000)
  const now = new Date()
  const startOf = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const diffDays = Math.round((startOf(now) - startOf(d)) / 86400000)
  if (diffDays === 0) return t("time.today")
  if (diffDays === 1) return t("time.yesterday")
  return t("time.date", { m: d.getMonth() + 1, d: d.getDate() })
}
