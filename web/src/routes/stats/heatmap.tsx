import { useQuery } from "@tanstack/react-query"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { api } from "@/api/client"
import { Button } from "@/components/ui/button"
import { clientTZOffset } from "@/lib/dates"
import { cn } from "@/lib/utils"

interface DailyResp {
  year: number
  items: { day: string; count: number }[]
}

const LEVELS = [
  "bg-muted",
  "bg-primary/25",
  "bg-primary/45",
  "bg-primary/70",
  "bg-primary",
] as const

function levelOf(n: number) {
  if (n <= 0) return 0
  if (n === 1) return 1
  if (n <= 3) return 2
  if (n <= 6) return 3
  return 4
}

function pad(n: number) {
  return String(n).padStart(2, "0")
}

function key(d: Date) {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 发布热力图（GitHub 风格年视图）。 */
export function Heatmap() {
  const { t } = useTranslation()
  const [year, setYear] = useState(new Date().getFullYear())
  const { data, isLoading } = useQuery({
    queryKey: ["stats", year, clientTZOffset],
    queryFn: () => api<DailyResp>(`/stats/daily?year=${year}&tz_offset=${clientTZOffset}`),
  })

  const { weeks, months, total, activeDays, streak } = useMemo(() => {
    const byDay = new Map<string, number>()
    for (const it of data?.items ?? []) byDay.set(it.day, it.count)
    const total = [...byDay.values()].reduce((a, b) => a + b, 0)
    const activeDays = [...byDay.values()].filter((n) => n > 0).length

    // 从 1 月 1 日所在周（周一为起点）开始
    const jan1 = new Date(year, 0, 1)
    const start = new Date(jan1)
    start.setDate(start.getDate() - ((jan1.getDay() + 6) % 7))

    const weeks: { date: string; day: Date; count: number; inYear: boolean }[][] = []
    const months: { label: string; start: number; end: number }[] = []
    const cursor = new Date(start)
    while (cursor.getFullYear() <= year && weeks.length < 54) {
      const week: { date: string; day: Date; count: number; inYear: boolean }[] = []
      for (let i = 0; i < 7; i++) {
        const k = key(cursor)
        week.push({
          date: k,
          day: new Date(cursor),
          count: byDay.get(k) ?? 0,
          inYear: cursor.getFullYear() === year,
        })
        cursor.setDate(cursor.getDate() + 1)
      }
      // 该周包含某月 1 号时开一个新的月份段
      const first = week.find((d) => d.inYear && d.day.getDate() === 1)
      if (first) {
        const prev = months.at(-1)
        if (prev) prev.end = weeks.length
        months.push({
          label: t("stats.monthShort", { m: first.day.getMonth() + 1 }),
          start: weeks.length,
          end: weeks.length + 1,
        })
      } else {
        const prev = months.at(-1)
        if (prev) prev.end = weeks.length + 1
      }
      weeks.push(week)
      if (cursor.getFullYear() > year) break
    }

    // 最长连续记录
    let best = 0
    let run = 0
    const startIter = new Date(year, 0, 1)
    for (let d = new Date(startIter); d.getFullYear() === year; d.setDate(d.getDate() + 1)) {
      if ((byDay.get(key(d)) ?? 0) > 0) {
        run++
        best = Math.max(best, run)
      } else {
        run = 0
      }
    }
    const lastMonth = months.at(-1)
    if (lastMonth) lastMonth.end = weeks.length
    return { weeks, months, total, activeDays, streak: best }
  }, [data, year, t])

  const CELL = 16
  const cell = "size-[13px] rounded-[3px]"
  const scrollRef = useRef<HTMLDivElement | null>(null)

  // 默认滚到最近（右侧），让当前月份可见
  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollLeft = el.scrollWidth
  }, [data, year])

  return (
    <section className="wisp-card p-4">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <div className="text-muted-foreground flex items-center gap-3 text-xs">
          <span>
            <b className="text-foreground text-base tabular-nums">{total}</b> {t("stats.posts")}
          </span>
          <span className="bg-border h-3 w-px" />
          <span>
            {t("stats.activeDays")} <b className="text-foreground tabular-nums">{activeDays}</b>
          </span>
          <span className="bg-border h-3 w-px" />
          <span>
            {t("stats.longestStreak")} <b className="text-foreground tabular-nums">{streak}</b>
          </span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            onClick={() => setYear((y) => y - 1)}
            aria-label={t("stats.prevYear")}
          >
            <ChevronLeft className="size-4" />
          </Button>
          <span className="w-10 text-center text-sm font-medium tabular-nums">{year}</span>
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            onClick={() => setYear((y) => y + 1)}
            disabled={year >= new Date().getFullYear()}
            aria-label={t("stats.nextYear")}
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="bg-muted h-28 animate-pulse rounded-lg" />
      ) : (
        <div ref={scrollRef} className="overflow-x-auto pb-1">
          <div className="min-w-max">
            {/* 月份标签：按周跨度对齐 */}
            <div className="text-muted-foreground mb-1 flex pl-[27px] text-[10px]">
              {months.map((m) => (
                <div
                  key={m.start}
                  style={{ width: (m.end - m.start) * CELL }}
                  className="whitespace-nowrap"
                >
                  {m.label}
                </div>
              ))}
            </div>
            <div className="flex gap-1">
              <div className="bg-card text-muted-foreground sticky left-0 z-10 flex w-5 flex-col gap-[3px] pt-0 text-[10px] leading-[13px]">
                <span className="h-[13px]" />
                <span className="h-[13px]">{t("stats.weekMon")}</span>
                <span className="h-[13px]" />
                <span className="h-[13px]">{t("stats.weekWed")}</span>
                <span className="h-[13px]" />
                <span className="h-[13px]">{t("stats.weekFri")}</span>
                <span className="h-[13px]" />
              </div>
              <div className="flex gap-[3px]">
                {weeks.map((week, i) => (
                  <div key={i} className="flex flex-col gap-[3px]">
                    {week.map((d) => (
                      <div
                        key={d.date}
                        title={`${d.date} · ${d.count} ${t("stats.posts")}`}
                        className={cn(cell, d.inYear ? LEVELS[levelOf(d.count)] : "bg-transparent")}
                      />
                    ))}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
      <div className="text-muted-foreground mt-3 flex items-center justify-end gap-1 text-[10px]">
        {t("stats.less")}
        {LEVELS.map((c) => (
          <div key={c} className={cn(cell, c)} />
        ))}
        {t("stats.more")}
      </div>
    </section>
  )
}
