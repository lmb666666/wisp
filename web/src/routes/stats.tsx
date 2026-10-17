import { Link, useNavigate, useSearch } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Database,
  FileText,
  HardDrive,
  Pencil,
  Tags,
  Trash2,
} from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { useMe } from "@/api/auth"
import { api, apiError } from "@/api/client"
import { usePosts } from "@/api/posts"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

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

/** 统计 / 归档 / 标签。 */
export function StatsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: me } = useMe()
  const { tab } = useSearch({ from: "/stats" })
  const isOwner = !!me?.is_owner
  const current = tab === "overview" && !isOwner ? "stats" : (tab ?? "stats")

  return (
    <div className="wisp-page space-y-4 py-5">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold tracking-tight">{t("nav.stats")}</h1>
        <Tabs
          value={current}
          onValueChange={(v) => navigate({ to: "/stats", search: v === "stats" ? {} : { tab: v } })}
        >
          <TabsList>
            {isOwner && <TabsTrigger value="overview">{t("stats.tabOverview")}</TabsTrigger>}
            <TabsTrigger value="stats">{t("stats.tabStats")}</TabsTrigger>
            <TabsTrigger value="archive">{t("stats.tabArchive")}</TabsTrigger>
            <TabsTrigger value="tags">{t("stats.tabTags")}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      {current === "overview" && <Overview />}
      {current === "stats" && <Heatmap />}
      {current === "archive" && <ArchiveList />}
      {current === "tags" && <TagCloud />}
    </div>
  )
}

interface OverviewResp {
  posts: { total: number; today: number; public: number; private: number }
  attachments: { count: number; bytes: number; by_storage: Record<string, number> }
  tags: number
  db_bytes: number
  last_backup_at: number
  version: string
  uptime_seconds: number
}

function fmtSize(n: number): string {
  if (n < 1024) return `${n}B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)}KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)}MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(2)}GB`
}

function Overview() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ["overview"],
    queryFn: () => api<OverviewResp>("/admin/overview"),
  })
  if (isLoading) return <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
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
      hint: t("overview.postsHint", {
        public: data.posts.public,
        private: data.posts.private,
      }),
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
      value: fmtSize(data.attachments.bytes),
      hint: t("overview.mediaHint", { count: data.attachments.count, storage: storage || "—" }),
    },
    {
      icon: Tags,
      label: t("overview.tagCount"),
      value: String(data.tags),
      hint: t("overview.tagsHint", { size: fmtSize(data.db_bytes) }),
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
      <div className="bg-muted/40 text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 rounded-xl px-3 py-2 text-xs">
        <span className="inline-flex items-center gap-1">
          <Database className="size-3.5" />
          {t("overview.dbSize", { size: fmtSize(data.db_bytes) })}
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

function Heatmap() {
  const { t } = useTranslation()
  const [year, setYear] = useState(new Date().getFullYear())
  const { data, isLoading } = useQuery({
    queryKey: ["stats", year],
    queryFn: () => api<DailyResp>(`/stats/daily?year=${year}`),
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

function ArchiveList() {
  const { t } = useTranslation()
  const posts = usePosts()
  const groups = useMemo(() => {
    const items = posts.data?.pages.flatMap((p) => p.items) ?? []
    const out: { label: string; posts: typeof items }[] = []
    for (const post of items) {
      const d = new Date(post.published_at * 1000)
      const label = t("time.month", { y: d.getFullYear(), m: d.getMonth() + 1 })
      const last = out[out.length - 1]
      if (last && last.label === label) last.posts.push(post)
      else out.push({ label, posts: [post] })
    }
    return out
  }, [posts.data, t])

  return (
    <div className="space-y-3">
      {groups.map((g) => (
        <section key={g.label} className="wisp-card p-4">
          <h3 className="mb-1 text-sm font-medium">{g.label}</h3>
          <ul className="divide-border divide-y">
            {g.posts.map((p) => {
              const d = new Date(p.published_at * 1000)
              return (
                <li key={p.id}>
                  <Link
                    to="/p/$postId"
                    params={{ postId: p.id }}
                    className="hover:text-primary flex items-baseline gap-2 py-2 text-sm"
                  >
                    <span className="text-muted-foreground w-10 shrink-0 text-xs tabular-nums">
                      {pad(d.getMonth() + 1)}-{pad(d.getDate())}
                    </span>
                    <span className="truncate">
                      {(p.content_md.split("\n")[0] || t("card.imageOnly")).slice(0, 60)}
                    </span>
                  </Link>
                </li>
              )
            })}
          </ul>
        </section>
      ))}
      <div className="text-center">
        {posts.isFetchingNextPage && (
          <p className="text-muted-foreground text-sm">{t("common.loadMore")}</p>
        )}
        {posts.hasNextPage && !posts.isFetchingNextPage && (
          <Button
            variant="outline"
            size="sm"
            className="rounded-lg"
            onClick={() => posts.fetchNextPage()}
          >
            {t("common.loadMore")}
          </Button>
        )}
      </div>
    </div>
  )
}

interface TagRow {
  id: number
  name: string
  post_count: number
}

function TagCloud() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const { data: me } = useMe()
  const [editing, setEditing] = useState(false)
  const [selected, setSelected] = useState<number[]>([])
  const [mergeInto, setMergeInto] = useState("")
  const [mergeOpen, setMergeOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<TagRow | null>(null)
  const [renameValue, setRenameValue] = useState("")
  const [deleteTarget, setDeleteTarget] = useState<TagRow | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["tags"],
    queryFn: () => api<{ items: TagRow[] }>("/tags"),
  })
  const tags = data?.items ?? []

  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ["tags"] })
    void qc.invalidateQueries({ queryKey: ["posts"] })
    void qc.invalidateQueries({ queryKey: ["post"] })
  }

  const rename = useMutation({
    mutationFn: (input: { id: number; name: string }) =>
      api(`/tags/${input.id}`, { method: "PATCH", body: { name: input.name } }),
    onSuccess: () => {
      toast.success(t("stats.renamed"))
      setRenameTarget(null)
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })
  const merge = useMutation({
    mutationFn: (input: { from_ids: number[]; into: string }) =>
      api("/tags/merge", { method: "POST", body: input }),
    onSuccess: () => {
      toast.success(t("stats.merged"))
      setMergeOpen(false)
      setSelected([])
      setMergeInto("")
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })
  const remove = useMutation({
    mutationFn: (id: number) => api(`/tags/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success(t("stats.tagDeleted"))
      setDeleteTarget(null)
      setSelected((prev) => prev.filter((id) => id !== deleteTarget?.id))
      invalidate()
    },
    onError: (err) => toast.error(apiError(err).message),
  })

  const toggle = (id: number) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))

  if (isLoading) return <p className="text-muted-foreground text-sm">{t("common.loading")}</p>

  return (
    <section className="wisp-card p-4">
      {me?.is_owner && (
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <p className="wisp-hint">{editing ? t("stats.tagsEditHint") : t("stats.tagsHint")}</p>
          <Button
            variant={editing ? "secondary" : "outline"}
            size="sm"
            onClick={() => {
              setEditing((v) => !v)
              setSelected([])
            }}
          >
            {editing ? (
              t("common.done")
            ) : (
              <>
                <Pencil /> {t("stats.manageTags")}
              </>
            )}
          </Button>
        </div>
      )}

      {editing && selected.length > 0 && (
        <div className="bg-muted/50 mb-3 flex flex-wrap items-center gap-2 rounded-xl p-2">
          <span className="text-muted-foreground text-xs">
            {t("stats.selectedCount", { count: selected.length })}
          </span>
          <Input
            value={mergeInto}
            onChange={(e) => setMergeInto(e.target.value)}
            placeholder={t("stats.mergePlaceholder")}
            aria-label={t("stats.mergeInto")}
            className="h-8 max-w-[220px] flex-1"
          />
          <Button size="sm" disabled={!mergeInto.trim()} onClick={() => setMergeOpen(true)}>
            {t("stats.mergeAction")}
          </Button>
        </div>
      )}

      {editing ? (
        <ul className="divide-border/60 divide-y">
          {tags.map((tg) => (
            <li key={tg.id} className="flex items-center gap-2 py-1.5">
              <Checkbox
                checked={selected.includes(tg.id)}
                onCheckedChange={() => toggle(tg.id)}
                aria-label={tg.name}
              />
              <span className="min-w-0 flex-1 truncate text-sm">#{tg.name}</span>
              <span className="text-muted-foreground text-xs tabular-nums">{tg.post_count}</span>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                aria-label={t("stats.renameTag", { name: tg.name })}
                onClick={() => {
                  setRenameTarget(tg)
                  setRenameValue(tg.name)
                }}
              >
                <Pencil />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                aria-label={t("stats.deleteTag", { name: tg.name })}
                onClick={() => setDeleteTarget(tg)}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <div className="flex flex-wrap gap-2">
          {tags.map((tg) => (
            <Link key={tg.name} to="/" search={{ tag: tg.name }} className="wisp-chip">
              #{tg.name}
              <span className="text-muted-foreground/70 ml-0.5 tabular-nums">{tg.post_count}</span>
            </Link>
          ))}
        </div>
      )}

      {tags.length === 0 && <p className="text-muted-foreground text-sm">{t("stats.noTags")}</p>}

      <Dialog open={renameTarget !== null} onOpenChange={(v) => !v && setRenameTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("stats.renameTitle")}</DialogTitle>
            <DialogDescription>{t("stats.renameDesc")}</DialogDescription>
          </DialogHeader>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              if (renameTarget && renameValue.trim()) {
                rename.mutate({ id: renameTarget.id, name: renameValue })
              }
            }}
          >
            <Input
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              aria-label={t("stats.tagName")}
              autoFocus
            />
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setRenameTarget(null)}>
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={!renameValue.trim() || rename.isPending}>
                {t("stats.saveName")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={mergeOpen} onOpenChange={setMergeOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("stats.mergeTitle")}</DialogTitle>
            <DialogDescription>
              {t("stats.mergeDesc", { count: selected.length, name: mergeInto.trim() })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setMergeOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              disabled={merge.isPending}
              onClick={() =>
                merge.mutate({
                  from_ids: selected,
                  into: mergeInto.trim(),
                })
              }
            >
              {t("stats.mergeAction")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteTarget !== null} onOpenChange={(v) => !v && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("stats.deleteTagTitle")}</DialogTitle>
            <DialogDescription>
              {t("stats.deleteTagDesc", { name: deleteTarget?.name ?? "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteTarget(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => deleteTarget && remove.mutate(deleteTarget.id)}
            >
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
