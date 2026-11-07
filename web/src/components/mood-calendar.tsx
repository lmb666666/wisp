import { ChevronLeft, ChevronRight } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { useDeleteMood, useMoods, useSetMood } from "@/api/moods"
import { MoodIcon } from "@/components/mood-icon"
import { MoodPicker } from "@/components/mood-picker"
import { Button } from "@/components/ui/button"
import { pad2, todayKey } from "@/lib/dates"
import { countMoods, moodByDate, MOODS, type Mood } from "@/lib/moods"
import { cn } from "@/lib/utils"

/** 统计页「心情」：月度大格补记 + 年度概览 + 分布。 */
export function MoodTab({ isOwner }: { isOwner: boolean }) {
  const { t } = useTranslation()
  const today = todayKey()
  const currentYear = Number(today.slice(0, 4))
  const currentMonth = Number(today.slice(5, 7)) - 1
  const [year, setYear] = useState(currentYear)
  const [month, setMonth] = useState(currentMonth)
  const [selected, setSelected] = useState<string | null>(null)

  const { data, isLoading, isError, refetch } = useMoods(`${year}-01-01`, `${year}-12-31`)
  const entries = useMemo(() => data?.items ?? [], [data])
  const byDate = useMemo(() => moodByDate(entries), [entries])
  const counts = useMemo(() => countMoods(entries), [entries])
  const setMood = useSetMood()
  const deleteMood = useDeleteMood()

  const changeYear = (delta: number) => {
    const next = year + delta
    if (next > currentYear) return
    setYear(next)
    if (next === currentYear && month > currentMonth) setMonth(currentMonth)
    setSelected(null)
  }

  const changeMonth = (nextYear: number, nextMonth: number) => {
    setYear(nextYear)
    setMonth(nextMonth)
    setSelected(null)
  }

  const choose = (date: string, mood: Mood) => {
    setMood.mutate({ date, mood }, { onError: () => toast.error(t("mood.saveFailed")) })
  }

  const clear = (date: string) => {
    deleteMood.mutate(date, { onError: () => toast.error(t("mood.deleteFailed")) })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-baseline gap-2">
          <span className="font-display text-lg font-semibold tabular-nums">{year}</span>
          <span className="text-muted-foreground text-xs">
            {t("mood.recordedDays", { n: entries.length })}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => changeYear(-1)}
            aria-label={t("stats.prevYear")}
          >
            <ChevronLeft className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => changeYear(1)}
            disabled={year >= currentYear}
            aria-label={t("stats.nextYear")}
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>

      {isLoading && (
        <p className="text-muted-foreground py-10 text-center text-sm">{t("mood.loading")}</p>
      )}
      {isError && (
        <div className="flex flex-col items-center gap-2 py-10 text-center">
          <p className="text-muted-foreground text-sm">{t("mood.loadFailed")}</p>
          <Button variant="outline" size="sm" onClick={() => void refetch()}>
            {t("home.retry")}
          </Button>
        </div>
      )}

      {!isLoading && !isError && (
        <>
          {isOwner && <p className="text-muted-foreground text-xs">{t("mood.backfillHint")}</p>}
          <MonthEditor
            year={year}
            month={month}
            today={today}
            byDate={byDate}
            isOwner={isOwner}
            selected={selected}
            onSelect={setSelected}
            onChangeMonth={changeMonth}
          />

          {isOwner && selected && (
            <div className="wisp-card space-y-2 p-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm font-medium">
                  {t("mood.pickDate", {
                    date: t("time.date", {
                      m: Number(selected.slice(5, 7)),
                      d: Number(selected.slice(8, 10)),
                    }),
                  })}
                </p>
                {byDate.has(selected) && (
                  <button
                    type="button"
                    onClick={() => clear(selected)}
                    className="text-muted-foreground hover:text-foreground text-[11px] transition-colors"
                  >
                    {t("mood.clear")}
                  </button>
                )}
              </div>
              <MoodPicker
                value={byDate.get(selected)}
                onSelect={(mood) => choose(selected, mood)}
              />
            </div>
          )}

          <YearOverview year={year} byDate={byDate} onPickMonth={(m) => changeMonth(year, m)} />

          {entries.length === 0 ? (
            <p className="text-muted-foreground py-4 text-center text-sm">{t("mood.empty")}</p>
          ) : (
            <div className="wisp-card space-y-2 p-3">
              <p className="text-sm font-medium">{t("mood.distribution")}</p>
              {MOODS.map((mood) => {
                const n = counts[mood]
                const pct = Math.round((n / entries.length) * 100)
                return (
                  <div key={mood} className="flex items-center gap-2 text-xs" data-mood={mood}>
                    <span
                      className="mood-text flex w-20 shrink-0 items-center gap-1"
                      data-mood={mood}
                    >
                      <MoodIcon mood={mood} className="size-4" />
                      {t(`mood.${mood}`)}
                    </span>
                    <div className="bg-muted h-1.5 flex-1 overflow-hidden rounded-full">
                      <div
                        className="h-full rounded-full bg-[var(--mood-color)]"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                    <span className="w-6 text-right tabular-nums">{n}</span>
                  </div>
                )
              })}
            </div>
          )}
        </>
      )}
    </div>
  )
}

function MonthEditor({
  year,
  month,
  today,
  byDate,
  isOwner,
  selected,
  onSelect,
  onChangeMonth,
}: {
  year: number
  month: number
  today: string
  byDate: Map<string, Mood>
  isOwner: boolean
  selected: string | null
  onSelect: (date: string) => void
  onChangeMonth: (year: number, month: number) => void
}) {
  const { t, i18n } = useTranslation()
  const weekdays = useMemo(() => {
    // 2024-01-01 是周一；narrow 在中文是「一」…「日」。
    const fmt = new Intl.DateTimeFormat(i18n.language, { weekday: "narrow" })
    return Array.from({ length: 7 }, (_, i) => fmt.format(new Date(2024, 0, 1 + i)))
  }, [i18n.language])

  const todayYear = Number(today.slice(0, 4))
  const todayMonth = Number(today.slice(5, 7)) - 1
  const leading = (new Date(year, month, 1).getDay() + 6) % 7
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const atCurrentMonth = year > todayYear || (year === todayYear && month >= todayMonth)

  const shift = (delta: number) => {
    const next = new Date(year, month + delta, 1)
    if (next > new Date(todayYear, todayMonth, 1)) return
    onChangeMonth(next.getFullYear(), next.getMonth())
  }

  return (
    <div className="wisp-card p-3">
      <div className="mb-2 flex items-center justify-between">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => shift(-1)}
          aria-label={t("stats.prevYear")}
        >
          <ChevronLeft className="size-4" />
        </Button>
        <p className="text-sm font-medium">{t("time.month", { y: year, m: month + 1 })}</p>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => shift(1)}
          disabled={atCurrentMonth}
          aria-label={t("stats.nextYear")}
        >
          <ChevronRight className="size-4" />
        </Button>
      </div>

      <div className="text-muted-foreground mb-1 grid grid-cols-7 gap-1 text-center text-[10px]">
        {weekdays.map((label, i) => (
          <span key={i}>{label}</span>
        ))}
      </div>

      <div className="grid grid-cols-7 gap-1">
        {Array.from({ length: leading }, (_, i) => (
          <span key={`pad-${i}`} />
        ))}
        {Array.from({ length: daysInMonth }, (_, i) => {
          const day = i + 1
          const key = `${year}-${pad2(month + 1)}-${pad2(day)}`
          const mood = byDate.get(key)
          const future = key > today
          const disabled = !isOwner || future
          return (
            <button
              key={key}
              type="button"
              data-mood={mood}
              data-date={key}
              disabled={disabled}
              aria-pressed={selected === key}
              aria-label={`${key}${mood ? ` · ${t(`mood.${mood}`)}` : ""}`}
              onClick={() => onSelect(key)}
              className={cn(
                "mood-day flex aspect-square flex-col items-center justify-center gap-0.5 rounded-lg border text-[11px] transition-colors",
                future
                  ? "cursor-default opacity-40"
                  : disabled
                    ? "cursor-default"
                    : "cursor-pointer",
              )}
            >
              <span>{day}</span>
              {mood && <span className="mood-dot size-1.5 rounded-full bg-[var(--mood-color)]" />}
            </button>
          )
        })}
      </div>
    </div>
  )
}

function YearOverview({
  year,
  byDate,
  onPickMonth,
}: {
  year: number
  byDate: Map<string, Mood>
  onPickMonth: (month: number) => void
}) {
  const { t } = useTranslation()
  return (
    <div className="grid grid-cols-3 gap-x-3 gap-y-4">
      {Array.from({ length: 12 }, (_, month) => {
        const leading = (new Date(year, month, 1).getDay() + 6) % 7
        const daysInMonth = new Date(year, month + 1, 0).getDate()
        return (
          <div key={month}>
            <button
              type="button"
              onClick={() => onPickMonth(month)}
              className="text-muted-foreground hover:text-foreground mb-1 text-[10px] transition-colors"
            >
              {t("stats.monthShort", { m: month + 1 })}
            </button>
            <div className="grid grid-cols-7 gap-[3px]">
              {Array.from({ length: leading }, (_, i) => (
                <span key={`pad-${i}`} />
              ))}
              {Array.from({ length: daysInMonth }, (_, i) => {
                const key = `${year}-${pad2(month + 1)}-${pad2(i + 1)}`
                return <span key={key} data-mood={byDate.get(key)} className="mood-cell" />
              })}
            </div>
          </div>
        )
      })}
    </div>
  )
}
