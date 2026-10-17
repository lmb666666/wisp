import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FileAudio, FileText, FileVideo, ImageIcon, Loader2, Trash2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { api, apiError } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"

interface MediaItem {
  id: string
  kind: "image" | "video" | "audio" | "file"
  mime: string
  size: number
  url: string
  thumb_url: string
  storage: string
  created_at: number
  post_id: string | null
  post_excerpt: string
}

interface MediaStats {
  count: number
  bytes: number
  orphan: number
  by_kind: Record<string, { count: number; bytes: number }>
}

function fmtBytes(n: number): string {
  if (n < 1024) return `${n}B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)}KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)}MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(2)}GB`
}

/** 设置 · 数据：媒体库（筛选、占用统计、无主文件清理）。 */
export function MediaSection() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [kind, setKind] = useState("all")
  const [orphanOnly, setOrphanOnly] = useState(false)
  const [pending, setPending] = useState<MediaItem | null>(null)

  const stats = useQuery({
    queryKey: ["media-stats"],
    queryFn: () => api<MediaStats>("/admin/media/stats"),
  })
  const media = useInfiniteQuery({
    queryKey: ["media", kind, orphanOnly],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      api<{ items: MediaItem[]; total: number }>("/admin/media", {
        params: {
          kind: kind === "all" ? undefined : kind,
          orphan: orphanOnly ? 1 : undefined,
          limit: 24,
          offset: pageParam,
        },
      }),
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.items.length, 0)
      return loaded < last.total ? loaded : undefined
    },
  })
  const del = useMutation({
    mutationFn: (item: MediaItem) =>
      api(`/admin/media/${item.id}${item.post_id ? "?force=1" : ""}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success(t("media.deleted"))
      qc.invalidateQueries({ queryKey: ["media"] })
      qc.invalidateQueries({ queryKey: ["media-stats"] })
      qc.invalidateQueries({ queryKey: ["posts"] })
      qc.invalidateQueries({ queryKey: ["post"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
    },
    onError: (err) => toast.error(apiError(err).message),
  })

  const items = media.data?.pages.flatMap((p) => p.items) ?? []

  return (
    <section className="wisp-card p-5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="wisp-section-title">{t("media.title")}</h2>
          <p className="wisp-hint mt-1">{t("media.hint")}</p>
        </div>
        {stats.data && (
          <p className="text-muted-foreground text-xs tabular-nums">
            {t("media.stats", {
              count: stats.data.count,
              size: fmtBytes(stats.data.bytes),
              orphan: stats.data.orphan,
            })}
          </p>
        )}
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-3">
        <Select value={kind} onValueChange={setKind}>
          <SelectTrigger className="w-[120px]" aria-label={t("media.kind")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("media.all")}</SelectItem>
            <SelectItem value="image">{t("media.image")}</SelectItem>
            <SelectItem value="video">{t("media.video")}</SelectItem>
            <SelectItem value="audio">{t("media.audio")}</SelectItem>
            <SelectItem value="file">{t("media.file")}</SelectItem>
          </SelectContent>
        </Select>
        <label className="text-muted-foreground flex items-center gap-2 text-sm">
          <Switch checked={orphanOnly} onCheckedChange={setOrphanOnly} />
          {t("media.orphanOnly")}
        </label>
      </div>

      {media.isLoading && (
        <p className="text-muted-foreground py-6 text-sm">{t("common.loading")}</p>
      )}
      {!media.isLoading && items.length === 0 && (
        <p className="text-muted-foreground py-6 text-sm">{t("media.empty")}</p>
      )}

      <div className="mt-3 grid grid-cols-3 gap-2 sm:grid-cols-4">
        {items.map((item) => (
          <div
            key={item.id}
            className="bg-muted/50 group relative overflow-hidden rounded-xl border"
          >
            <div className="flex aspect-square items-center justify-center">
              {item.kind === "image" && item.thumb_url ? (
                <img
                  src={item.thumb_url}
                  alt=""
                  loading="lazy"
                  className="size-full object-cover"
                />
              ) : item.kind === "video" ? (
                <FileVideo className="text-muted-foreground size-6" />
              ) : item.kind === "audio" ? (
                <FileAudio className="text-muted-foreground size-6" />
              ) : item.kind === "file" ? (
                <FileText className="text-muted-foreground size-6" />
              ) : (
                <ImageIcon className="text-muted-foreground size-6" />
              )}
            </div>
            <div className="px-2 py-1.5">
              <p className="text-muted-foreground flex items-center justify-between text-[10px] tabular-nums">
                {fmtBytes(item.size)}
                {item.storage !== "local" && <Badge variant="outline">{item.storage}</Badge>}
              </p>
              <p className="text-muted-foreground mt-0.5 truncate text-[10px]">
                {item.post_id ? item.post_excerpt || t("media.attached") : t("media.orphan")}
              </p>
            </div>
            <button
              type="button"
              aria-label={t("common.delete")}
              className="bg-background/90 text-muted-foreground hover:text-destructive absolute top-1 right-1 rounded-full p-1 opacity-100 shadow-sm transition-opacity focus-visible:opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
              onClick={() => setPending(item)}
            >
              <Trash2 className="size-3.5" />
            </button>
          </div>
        ))}
      </div>

      {media.hasNextPage && (
        <div className="mt-3 text-center">
          <Button
            variant="outline"
            size="sm"
            disabled={media.isFetchingNextPage}
            onClick={() => void media.fetchNextPage()}
          >
            {media.isFetchingNextPage && <Loader2 className="animate-spin" />}
            {t("common.loadMore")}
          </Button>
        </div>
      )}

      <Dialog open={pending !== null} onOpenChange={(v) => !v && setPending(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("media.deleteTitle")}</DialogTitle>
            <DialogDescription>
              {pending?.post_id ? t("media.deleteAttachedDesc") : t("media.deleteOrphanDesc")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setPending(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={del.isPending}
              onClick={() => {
                if (pending) del.mutate(pending, { onSuccess: () => setPending(null) })
              }}
            >
              {del.isPending ? <Loader2 className="animate-spin" /> : <Trash2 />}{" "}
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
