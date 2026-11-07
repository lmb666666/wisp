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
