import { useQuery } from "@tanstack/react-query"
import { CalendarDays, Tag as TagIcon } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { api, splitTagFilter, type PostFilters } from "@/api/client"
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

export function activeFilterCount(f: PostFilters): number {
  let n = 0
  if (f.from || f.to) n++
  const tags = new Set(splitTagFilter(f.tags))
  if (f.tag) tags.add(f.tag)
  return n + tags.size
}

function dateInput(ts?: number): string {
  if (!ts) return ""
  const d = new Date(ts * 1000)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function rangeOf(preset: "today" | "7d" | "30d" | "year"): { from: string; to: string } {
  const now = new Date()
  const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate())
  const today = day(now)
  let from = today
  if (preset === "7d") from = new Date(today.getTime() - 6 * 86400_000)
  if (preset === "30d") from = new Date(today.getTime() - 29 * 86400_000)
  if (preset === "year") from = new Date(today.getFullYear(), 0, 1)
  return { from: dateInput(from.getTime() / 1000), to: dateInput(now.getTime() / 1000) }
}

/** 时间线筛选面板：日期预设/自定义、可见性、标签多选（ALL 语义）。 */
export function FilterPanel({
  open,
  onOpenChange,
  filters,
  isOwner,
  onApply,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  filters: PostFilters
  isOwner: boolean
  onApply: (next: Pick<PostFilters, "from" | "to" | "tags">) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <FilterForm
          filters={filters}
          isOwner={isOwner}
          onApply={onApply}
          onClose={() => onOpenChange(false)}
        />
      )}
    </Dialog>
  )
}

function FilterForm({
  filters,
  onApply,
  onClose,
}: {
  filters: PostFilters
  isOwner: boolean
  onApply: (next: Pick<PostFilters, "from" | "to" | "tags">) => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const [fromDate, setFromDate] = useState(() => dateInput(filters.from))
  const [toDate, setToDate] = useState(() => dateInput(filters.to))
  const [selected, setSelected] = useState<string[]>(() => {
    const tags = splitTagFilter(filters.tags)
    if (filters.tag && !tags.includes(filters.tag)) tags.unshift(filters.tag)
    return tags.slice(0, 20)
  })
  const [tagSearch, setTagSearch] = useState("")

  const tagsQuery = useQuery({
    queryKey: ["tags"],
    queryFn: () => api<{ items: { id: number; name: string; post_count: number }[] }>("/tags"),
  })
  const tags = useMemo(() => {
    const q = tagSearch.trim().toLowerCase()
    return (tagsQuery.data?.items ?? []).filter((tg) => !q || tg.name.toLowerCase().includes(q))
  }, [tagsQuery.data, tagSearch])

  const toggle = (name: string) => {
    if (!selected.includes(name) && selected.length >= 20) {
      toast.error(t("filter.maxTags"))
      return
    }
    setSelected((prev) => (prev.includes(name) ? prev.filter((n) => n !== name) : [...prev, name]))
  }

  const apply = () => {
    if (fromDate && toDate && fromDate > toDate) {
      toast.error(t("filter.invalidRange"))
      return
    }
    const from = fromDate
      ? Math.floor(new Date(`${fromDate}T00:00:00`).getTime() / 1000)
      : undefined
    const to = toDate ? Math.floor(new Date(`${toDate}T23:59:59`).getTime() / 1000) : undefined
    const available = new Set((tagsQuery.data?.items ?? []).map((tg) => tg.name))
    const chosen = tagsQuery.data ? selected.filter((name) => available.has(name)) : selected
    onApply({
      from,
      to,
      tags: chosen.length ? JSON.stringify(chosen) : undefined,
    })
    onClose()
  }

  const reset = () => {
    onApply({ from: undefined, to: undefined, tags: undefined })
    onClose()
  }

  return (
    <DialogContent className="max-h-[85vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{t("filter.title")}</DialogTitle>
        <DialogDescription>{t("filter.desc")}</DialogDescription>
      </DialogHeader>

      <div className="space-y-4">
        <div>
          <p className="mb-2 flex items-center gap-1.5 text-sm font-medium">
            <CalendarDays className="text-muted-foreground size-4" /> {t("filter.date")}
          </p>
          <div className="flex flex-wrap gap-1.5">
            {(["today", "7d", "30d", "year"] as const).map((p) => (
              <Button
                key={p}
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  const range = rangeOf(p)
                  setFromDate(range.from)
                  setToDate(range.to)
                }}
              >
                {t(`filter.${p}`)}
              </Button>
            ))}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setFromDate("")
                setToDate("")
              }}
            >
              {t("filter.allTime")}
            </Button>
          </div>
          <div className="mt-2 grid grid-cols-2 gap-2">
            <label className="text-muted-foreground text-xs">
              {t("filter.from")}
              <Input
                type="date"
                value={fromDate}
                max={toDate || undefined}
                onChange={(e) => setFromDate(e.target.value)}
                className="mt-1 h-8"
              />
            </label>
            <label className="text-muted-foreground text-xs">
              {t("filter.to")}
              <Input
                type="date"
                value={toDate}
                min={fromDate || undefined}
                onChange={(e) => setToDate(e.target.value)}
                className="mt-1 h-8"
              />
            </label>
          </div>
        </div>


        <div>
          <p className="mb-2 flex items-center gap-1.5 text-sm font-medium">
            <TagIcon className="text-muted-foreground size-4" /> {t("filter.tags")}
            {selected.length > 0 && (
              <span className="text-muted-foreground text-xs">
                {t("filter.tagSelected", { count: selected.length })}
              </span>
            )}
          </p>
          <Input
            value={tagSearch}
            onChange={(e) => setTagSearch(e.target.value)}
            placeholder={t("filter.tagSearch")}
            className="h-8"
            aria-label={t("filter.tagSearch")}
          />
          <div className="border-border/70 mt-2 max-h-44 overflow-y-auto rounded-xl border p-1">
            {tags.map((tg) => (
              <label
                key={tg.id}
                className="hover:bg-muted/60 flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm"
              >
                <Checkbox
                  checked={selected.includes(tg.name)}
                  onCheckedChange={() => toggle(tg.name)}
                  aria-label={tg.name}
                />
                <span className="min-w-0 flex-1 truncate">#{tg.name}</span>
                <span className="text-muted-foreground text-xs tabular-nums">{tg.post_count}</span>
              </label>
            ))}
            {tags.length === 0 && (
              <p className="text-muted-foreground px-2 py-3 text-xs">{t("filter.noTags")}</p>
            )}
          </div>
          <p className="wisp-hint mt-1">{t("filter.allHint")}</p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="ghost" onClick={reset}>
          {t("filter.reset")}
        </Button>
        <Button onClick={apply}>{t("filter.apply")}</Button>
      </DialogFooter>
    </DialogContent>
  )
}
