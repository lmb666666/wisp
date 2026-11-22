import { Link } from "@tanstack/react-router"
import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import { usePosts } from "@/api/posts"
import { Button } from "@/components/ui/button"
import { pad2 } from "@/lib/dates"

/** 归档列表：按月分组的全部帖子。 */
export function ArchiveList() {
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
      {posts.isLoading && (
        <p className="text-muted-foreground py-6 text-center text-sm">{t("common.loading")}</p>
      )}
      {posts.isError && (
        <p className="text-muted-foreground py-6 text-center text-sm">{t("common.loadFailed")}</p>
      )}
      {groups.length === 0 && !posts.isLoading && !posts.isError && (
        <p className="text-muted-foreground py-6 text-center text-sm">{t("stats.archiveEmpty")}</p>
      )}
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
                      {pad2(d.getMonth() + 1)}-{pad2(d.getDate())}
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
