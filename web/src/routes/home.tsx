import { useNavigate, useSearch } from "@tanstack/react-router"
import { PenLine, Search, SlidersHorizontal, X } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { useMe } from "@/api/auth"
import { splitTagFilter, type PostFilters } from "@/api/client"
import { usePosts, useSearch as useSearchQuery } from "@/api/posts"
import { activeFilterCount, FilterPanel } from "@/components/filter-panel"
import { dayLabel, PostCard } from "@/components/post-card"
import { PublishBox } from "@/components/publish-box"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

/** 时间线入口：有搜索词走结果页，否则时间线。 */
export function Home() {
  const { tag, q, from, to, vis, tags } = useSearch({ from: "/" })
  const { data: me } = useMe()

  if (q) return <SearchResults q={q} />
  return <Timeline filters={{ tag, from, to, vis, tags }} authed={!!me} isOwner={!!me?.is_owner} />
}

/** 时间线：发布框（登录后）+ 无限滚动 + 按日分组 + 组合筛选。 */
function Timeline({
  filters,
  authed,
  isOwner,
}: {
  filters: PostFilters
  authed: boolean
  isOwner: boolean
}) {
  const { t } = useTranslation()
  const posts = usePosts(filters)
  const sentinelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const io = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting && posts.hasNextPage && !posts.isFetchingNextPage) {
        posts.fetchNextPage()
      }
    })
    io.observe(el)
    return () => io.disconnect()
  }, [posts.hasNextPage, posts.isFetchingNextPage, posts.fetchNextPage, posts])

  const groups = useMemo(() => {
    const items = posts.data?.pages.flatMap((p) => p.items) ?? []
    const out: { label: string; posts: typeof items }[] = []
    for (const post of items) {
      const label = dayLabel(post.published_at, t)
      const last = out[out.length - 1]
      if (last && last.label === label) last.posts.push(post)
      else out.push({ label, posts: [post] })
    }
    return out
  }, [posts.data, t])

  return (
    <div className="wisp-page">
      {authed && <PublishBox />}
      <SearchBar filters={filters} isOwner={isOwner} />
      <FilterChips filters={filters} />

      {posts.isLoading && (
        <p className="text-muted-foreground py-10 text-center text-sm">{t("common.loading")}</p>
      )}

      {posts.isError && (
        <div className="flex flex-col items-center gap-2 py-16 text-center">
          <p className="text-muted-foreground text-sm">{t("home.loadFailed")}</p>
          <Button variant="outline" size="sm" onClick={() => void posts.refetch()}>
            {t("home.retry")}
          </Button>
        </div>
      )}

      {groups.map((group) => (
        <section key={group.label} className="mb-1">
          <h2 className="bg-background/85 text-muted-foreground sticky top-14 z-10 -mx-1 mb-2 px-1 py-2 text-xs font-medium backdrop-blur">
            {group.label}
          </h2>
          <div className="wisp-stagger space-y-3">
            {group.posts.map((post) => (
              <PostCard key={post.id} post={post} />
            ))}
          </div>
        </section>
      ))}

      {!posts.isLoading && !posts.isError && groups.length === 0 && (
        <div className="flex flex-col items-center gap-2 py-20 text-center">
          <div className="bg-muted text-muted-foreground rounded-2xl p-3">
            <PenLine className="size-5" />
          </div>
          <p className="text-muted-foreground text-sm">
            {authed ? t("home.emptyAuthed") : t("home.emptyPublic")}
          </p>
          {authed && (
            <p className="text-muted-foreground/70 max-w-xs text-xs">{t("home.emptyHint")}</p>
          )}
        </div>
      )}

      <div ref={sentinelRef} className="h-4" />
      {posts.isFetchingNextPage && (
        <p className="text-muted-foreground py-4 text-center text-sm">{t("common.loadMore")}</p>
      )}
    </div>
  )
}

const EMPTY_FILTERS: PostFilters = {
  tag: undefined,
  from: undefined,
  to: undefined,
  vis: undefined,
  tags: undefined,
}

function SearchBar({ filters, isOwner }: { filters: PostFilters; isOwner: boolean }) {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [input, setInput] = useState("")
  const [open, setOpen] = useState(false)
  const active = activeFilterCount(filters)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault()
        setOpen(true)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  const go = (next: PostFilters & { q?: string }) =>
    navigate({
      to: "/",
      search: {
        tag: next.tag,
        from: next.from,
        to: next.to,
        vis: next.vis,
        tags: next.tags,
        q: next.q,
      },
    })

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    const query = input.trim()
    if (!query) return
    // 搜索接口不带组合筛选；进入搜索时丢弃筛选，避免 URL 声称已过滤却未生效。
    go({ q: query })
  }

  return (
    <>
      <form onSubmit={submit} className="relative my-3 flex items-center gap-2">
        <div className="relative min-w-0 flex-1">
          <Search className="text-muted-foreground absolute top-1/2 left-3.5 size-4 -translate-y-1/2" />
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={t("home.searchPlaceholder")}
            className="bg-card h-10 rounded-xl pl-10 shadow-sm"
            aria-label={t("home.searchAria")}
          />
        </div>
        <Button
          type="button"
          variant={active > 0 ? "secondary" : "outline"}
          className="h-10 shrink-0 rounded-xl"
          aria-label={t("filter.open")}
          onClick={() => setOpen(true)}
          title={t("filter.shortcut")}
        >
          <SlidersHorizontal />
          <span className="hidden sm:inline">{t("filter.open")}</span>
          {active > 0 && (
            <span className="bg-primary text-primary-foreground ml-0.5 rounded-full px-1.5 text-[10px] tabular-nums">
              {active}
            </span>
          )}
        </Button>
      </form>

      <FilterPanel
        open={open}
        onOpenChange={setOpen}
        filters={filters}
        isOwner={isOwner}
        onApply={(next) => go({ ...filters, ...next, tag: undefined })}
      />
    </>
  )
}

function FilterChips({ filters }: { filters: PostFilters }) {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const tags = Array.from(
    new Set([...(filters.tag ? [filters.tag] : []), ...splitTagFilter(filters.tags)]),
  )
  if (activeFilterCount(filters) === 0) return null

  const go = (next: PostFilters) =>
    navigate({
      to: "/",
      search: { tag: next.tag, from: next.from, to: next.to, vis: next.vis, tags: next.tags },
    })
  const fmt = (ts: number) =>
    new Date(ts * 1000).toLocaleDateString(undefined, { month: "short", day: "numeric" })
  const dateLabel =
    filters.from && filters.to
      ? `${fmt(filters.from)} – ${fmt(filters.to)}`
      : filters.from
        ? `≥ ${fmt(filters.from)}`
        : filters.to
          ? `≤ ${fmt(filters.to)}`
          : ""
  const removeTag = (name: string) => {
    const next = tags.filter((x) => x !== name)
    go({
      ...filters,
      tag: filters.tag === name ? undefined : filters.tag,
      tags: next.length ? JSON.stringify(next) : undefined,
    })
  }

  return (
    <div className="mb-3 flex flex-wrap items-center gap-1.5">
      {dateLabel && (
        <FilterChip
          label={dateLabel}
          removeLabel={t("filter.remove", { label: dateLabel })}
          onRemove={() => go({ ...filters, from: undefined, to: undefined })}
        />
      )}
      {filters.vis && (
        <FilterChip
          label={t(`filter.vis.${filters.vis}`)}
          removeLabel={t("filter.remove", { label: t(`filter.vis.${filters.vis}`) })}
          onRemove={() => go({ ...filters, vis: undefined })}
        />
      )}
      {tags.map((tag) => (
        <FilterChip
          key={tag}
          label={`#${tag}`}
          removeLabel={t("filter.remove", { label: `#${tag}` })}
          onRemove={() => removeTag(tag)}
        />
      ))}
      <button
        type="button"
        className="text-muted-foreground hover:text-foreground px-1 text-xs"
        onClick={() => go({ ...EMPTY_FILTERS })}
      >
        {t("filter.clearAll")}
      </button>
    </div>
  )
}

function FilterChip({
  label,
  removeLabel,
  onRemove,
}: {
  label: string
  removeLabel: string
  onRemove: () => void
}) {
  return (
    <span className="bg-accent text-accent-foreground inline-flex items-center gap-0.5 rounded-full py-0.5 pr-1 pl-2 text-xs">
      {label}
      <button
        type="button"
        aria-label={removeLabel}
        className="hover:bg-background/60 rounded-full p-0.5"
        onClick={onRemove}
      >
        <X className="size-3" />
      </button>
    </span>
  )
}

function SearchResults({ q }: { q: string }) {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { data, isLoading } = useSearchQuery(q)

  return (
    <div className="wisp-page py-4">
      <div className="mb-3 flex items-center justify-between">
        <p className="text-muted-foreground text-sm">
          {isLoading
            ? t("home.searching")
            : t("home.searchResult", { q, n: data?.items.length ?? 0 })}
        </p>
        <Button variant="ghost" size="sm" onClick={() => navigate({ to: "/" })}>
          <X /> {t("home.clearSearch")}
        </Button>
      </div>
      <div className="wisp-stagger space-y-3">
        {data?.items.map((post) => (
          <PostCard key={post.id} post={post} />
        ))}
      </div>
      {!isLoading && data?.items.length === 0 && (
        <p className="text-muted-foreground py-16 text-center text-sm">{t("home.noMatch")}</p>
      )}
    </div>
  )
}
