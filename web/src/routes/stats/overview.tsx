import { Link } from "@tanstack/react-router"
import { useQuery } from "@tanstack/react-query"
import { CalendarDays, Database, FileText, HardDrive, Smile, Tags } from "lucide-react"
import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import { useMoods } from "@/api/moods"
import { api } from "@/api/client"
import { MoodIcon } from "@/components/mood-icon"
import { clientTZOffset, todayKey } from "@/lib/dates"
import { fmtBytes } from "@/lib/format"
import { countMoods, MOODS } from "@/lib/moods"

interface OverviewResp {
  posts: { total: number; today: number }
  attachments: { count: number; bytes: number; by_storage: Record<string, number> }
  tags: number
  db_bytes: number
  last_backup_at: number
  version: string
  uptime_seconds: number
}

/** 站点概览（owner）：指标卡 + 年度心情 + 运行信息。 */
export function Overview() {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useQuery({
    queryKey: ["overview", clientTZOffset],
    queryFn: () => api<OverviewResp>(`/admin/overview?tz_offset=${clientTZOffset}`),
  })
  const year = Number(todayKey().slice(0, 4))
  const moods = useMoods(`${year}-01-01`, `${year}-12-31`)
  const entries = useMemo(() => moods.data?.items ?? [], [moods.data])
  const counts = useMemo(() => countMoods(entries), [entries])

  if (isLoading) return <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
  if (isError) return <p className="text-muted-foreground text-sm">{t("common.loadFailed")}</p>
  if (!data) return null

  const storage = Object.entries(data.attachments.by_storage)
    .map(([k, v]) => `${k} ${v}`)
    .join(" · ")
  const days = Math.floor(data.uptime_seconds / 86400)
  const hours = Math.floor((data.uptime_seconds % 86400) / 3600)
  const minutes = Math.floor((data.uptime_seconds % 3600) / 60)
  const uptime =
    days > 0
      ? t("overview.daysHours", { d: days, h: hours })
      : hours > 0
        ? t("overview.hoursMinutes", { h: hours, m: minutes })
        : t("overview.minutes", { m: minutes })

  const cards = [
    {
      icon: FileText,
      label: t("overview.posts"),
      value: String(data.posts.total),
    },
    {
      icon: CalendarDays,
      label: t("overview.today"),
      value: String(data.posts.today),
      hint: t("overview.todayHint"),
    },
    {
      icon: HardDrive,
      label: t("overview.media"),
      value: fmtBytes(data.attachments.bytes),
      hint: t("overview.mediaHint", { count: data.attachments.count, storage: storage || "—" }),
    },
    {
      icon: Tags,
      label: t("overview.tagCount"),
      value: String(data.tags),
      hint: t("overview.tagsHint", { size: fmtBytes(data.db_bytes) }),
    },
  ]

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {cards.map(({ icon: Icon, label, value, hint }) => (
          <section key={label} className="wisp-card p-4">
            <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
              <Icon className="size-3.5" /> {label}
            </p>
            <p className="font-display mt-2 text-2xl font-semibold tabular-nums">{value}</p>
            <p className="text-muted-foreground mt-1 text-[11px] leading-snug">{hint}</p>
          </section>
        ))}
      </div>
      <section className="wisp-card p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
              <Smile className="size-3.5" /> {t("overview.moodTitle", { year })}
            </p>
            {moods.isLoading ? (
              <p className="text-muted-foreground mt-2 text-sm">{t("mood.loading")}</p>
            ) : entries.length === 0 ? (
              <p className="text-muted-foreground mt-2 text-sm">{t("overview.moodEmpty")}</p>
            ) : (
              <>
                <p className="font-display mt-2 text-2xl font-semibold tabular-nums">
                  {t("overview.moodDays", { count: entries.length })}
                </p>
                <div className="bg-muted mt-2 flex h-2 overflow-hidden rounded-full">
                  {MOODS.map((mood) =>
                    counts[mood] > 0 ? (
                      <div
                        key={mood}
                        data-mood={mood}
                        className="bg-[var(--mood-color)]"
                        style={{ width: `${(counts[mood] / entries.length) * 100}%` }}
                      />
                    ) : null,
                  )}
                </div>
                <div className="text-muted-foreground mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px]">
                  {MOODS.map((mood) =>
                    counts[mood] > 0 ? (
                      <span key={mood} className="inline-flex items-center gap-1" data-mood={mood}>
                        <MoodIcon mood={mood} className="mood-text size-3.5" />
                        {t(`mood.${mood}`)} {counts[mood]}
                      </span>
                    ) : null,
                  )}
                </div>
              </>
            )}
          </div>
          <Link
            to="/stats"
            search={{ tab: "mood" }}
            className="text-muted-foreground hover:text-foreground shrink-0 text-xs transition-colors"
          >
            {t("overview.moodView")}
          </Link>
        </div>
      </section>
      <div className="bg-muted/40 text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 rounded-xl px-3 py-2 text-xs">
        <span className="inline-flex items-center gap-1">
          <Database className="size-3.5" />
          {t("overview.dbSize", { size: fmtBytes(data.db_bytes) })}
        </span>
        <span>
          {data.last_backup_at > 0
            ? t("overview.lastBackupAt", {
                time: new Date(data.last_backup_at * 1000).toLocaleString(),
              })
            : `${t("overview.lastBackup")}: ${t("overview.never")}`}
        </span>
        <span>v{data.version}</span>
        <span>
          {t("overview.uptime")} {uptime}
        </span>
      </div>
    </div>
  )
}
