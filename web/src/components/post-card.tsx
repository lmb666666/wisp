import { Blurhash } from "react-blurhash"
import { Link } from "@tanstack/react-router"
import { MusicCard } from "@/components/music-card"
import { ArrowUpRight, GitBranch, MapPin, Share2 } from "lucide-react"
import { Suspense, lazy, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import type { Attachment, Post, PostBlock } from "@/api/client"
import { BlockShell } from "@/components/block-shell"
import { LocationCard } from "@/components/location-card"
import { VideoCard, WebsiteCard } from "@/components/link-blocks"
import { sharePost } from "@/lib/share"
import { cn } from "@/lib/utils"

const Lightbox = lazy(() => import("yet-another-react-lightbox"))

/** 点击 embed 卡片才加载 iframe（隐私优先；RSS/OG 始终是外链卡）。 */
function handleContentClick(e: React.MouseEvent) {
  const target = e.target as HTMLElement
  const card = target.closest?.(".link-card[data-embed-src]") as HTMLElement | null
  if (!card) {
    // 正文里的普通链接：交给浏览器/新标签处理，不要触发外层“打开详情”
    if (target.closest("a")) e.stopPropagation()
    return
  }
  e.preventDefault()
  e.stopPropagation()
  const src = card.getAttribute("data-embed-src") ?? ""
  if (!src) return
  if (card.getAttribute("data-embed-type") === "audio") {
    const audio = document.createElement("audio")
    audio.controls = true
    audio.preload = "none"
    audio.src = src
    audio.className = "mt-2 h-9 w-full"
    card.replaceWith(audio)
    return
  }
  const iframe = document.createElement("iframe")
  iframe.src = src
  iframe.className = "aspect-video w-full rounded-lg"
  iframe.allow = "autoplay; encrypted-media; picture-in-picture"
  iframe.setAttribute("allowfullscreen", "")
  iframe.loading = "lazy"
  card.replaceWith(iframe)
}

function ShareButton({ post }: { post: Post }) {
  const { t } = useTranslation()
  const share = async () => {
    const url = new URL(`/p/${post.id}`, window.location.origin).toString()
    const text = post.content_md
      .replace(/[#*_`>]/g, "")
      .slice(0, 120)
      .trim()
    const result = await sharePost({ title: t("card.shareTitle"), text, url })
    if (result === "copied") toast.success(t("card.linkCopied"))
    if (result === "failed") toast.error(t("card.copyFailed"))
  }
  return (
    <button
      type="button"
      aria-label={t("card.share")}
      title={t("card.share")}
      onClick={() => void share()}
      className="text-muted-foreground hover:bg-accent hover:text-foreground rounded-full p-1.5 transition-colors"
    >
      <Share2 className="size-4" />
    </button>
  )
}

/** 时间线卡片。乐观占位（id=optimistic-*）不渲染永久链接。 */
export function PostCard({
  post,
  variant = "timeline",
}: {
  post: Post
  variant?: "timeline" | "detail"
}) {
  const { t } = useTranslation()
  const optimistic = post.id.startsWith("optimistic-")

  const content = (
    <>
      <div className="post-content text-[15px] leading-relaxed" onClick={handleContentClick}>
        <div dangerouslySetInnerHTML={{ __html: post.content_html }} />
      </div>
      <div onClick={(e) => e.stopPropagation()}>
        <AttachmentGrid attachments={post.attachments} />
      </div>
    </>
  )

  return (
    <article className="wisp-card wisp-card-interactive overflow-hidden">
      <div className="px-4 pt-3 pb-1">
        <div className="text-muted-foreground mb-1.5 flex flex-wrap items-center gap-2 text-[11px]">
          {variant === "timeline" && (
            <time dateTime={new Date(post.published_at * 1000).toISOString()}>
              {new Date(post.published_at * 1000).toLocaleTimeString(undefined, {
                hour: "2-digit",
                minute: "2-digit",
              })}
            </time>
          )}
          {post.edited_at ? <span>{t("card.edited")}</span> : null}
        </div>

        {optimistic ? (
          content
        ) : (
          <Link to="/p/$postId" params={{ postId: post.id }} className="block">
            {content}
          </Link>
        )}

        <BlocksSection blocks={post.blocks} />

        {(post.tags.length > 0 ||
          (post.location && !(post.blocks ?? []).some((b) => b.type === "location"))) && (
          <div className="mt-2.5 flex flex-wrap gap-1.5">
            {post.location && !(post.blocks ?? []).some((b) => b.type === "location") && (
              <a
                href={osmURL(post.location)}
                target="_blank"
                rel="noopener noreferrer"
                className="wisp-chip wisp-chip-accent"
                onClick={(e) => e.stopPropagation()}
              >
                <MapPin className="size-3" /> {post.location}
              </a>
            )}
            {post.tags.map((tag) => (
              <Link
                key={tag}
                to="/"
                search={{ tag }}
                className="wisp-chip wisp-chip-accent"
                onClick={(e) => e.stopPropagation()}
              >
                #{tag}
              </Link>
            ))}
          </div>
        )}
      </div>

      {!optimistic && (
        <div className="border-border/60 bg-muted/40 mt-1 flex items-center justify-end gap-0.5 border-t px-3 py-1.5">
          <ShareButton post={post} />
          {variant === "timeline" && (
            <Link
              to="/p/$postId"
              params={{ postId: post.id }}
              aria-label={t("card.jump")}
              title={t("card.jump")}
              className="text-muted-foreground hover:bg-accent hover:text-foreground rounded-full p-1.5 transition-colors"
            >
              <ArrowUpRight className="size-4" />
            </Link>
          )}
        </div>
      )}
    </article>
  )
}

/** 图片九宫格 + 视频/音频块（朋友圈式布局）。 */
export function AttachmentGrid({ attachments }: { attachments: Attachment[] }) {
  if (attachments.length === 0) return null

  const images = attachments.filter((a) => a.kind === "image")
  const others = attachments.filter((a) => a.kind !== "image")
  return (
    <div className="mt-3 space-y-2">
      {images.length > 0 && <ImageGrid images={images} />}
      {others.map((a) =>
        a.kind === "video" ? (
          <video
            key={a.id}
            src={a.url}
            poster={a.poster_url}
            controls
            className="w-full rounded-xl"
            preload="metadata"
          />
        ) : a.kind === "audio" ? (
          <audio key={a.id} src={a.url} controls className="w-full" />
        ) : null,
      )}
    </div>
  )
}

function ImageGrid({ images }: { images: Attachment[] }) {
  const [open, setOpen] = useState(-1)
  const cols = images.length === 1 ? 1 : images.length <= 4 ? 2 : 3

  return (
    <>
      <div
        className={cn(
          "grid gap-1.5",
          cols === 1 ? "grid-cols-1" : cols === 2 ? "grid-cols-2" : "grid-cols-3",
        )}
      >
        {images.map((img, i) => (
          <button
            type="button"
            key={img.id}
            onClick={(e) => {
              e.preventDefault()
              e.stopPropagation()
              setOpen(i)
            }}
            className="bg-muted relative overflow-hidden rounded-xl p-0 leading-none"
          >
            {img.blurhash && (
              <div className="absolute inset-0">
                <Blurhash
                  hash={img.blurhash}
                  className="!h-full !w-full"
                  resolutionX={32}
                  resolutionY={32}
                  punch={1}
                />
              </div>
            )}
            <img
              src={img.thumb_url || img.url}
              alt=""
              loading="lazy"
              className={cn(
                "relative w-full object-cover",
                images.length === 1 ? "max-h-96" : "aspect-square",
              )}
            />
          </button>
        ))}
      </div>
      {open >= 0 && (
        <Suspense fallback={null}>
          <Lightbox
            open
            close={() => setOpen(-1)}
            index={Math.max(open, 0)}
            slides={images.map((img) => ({ src: img.url }))}
          />
        </Suspense>
      )}
    </>
  )
}

/** 扩展卡片：音乐（Meting 播放）/ GitHub / 位置，独立于正文。 */
function BlocksSection({ blocks }: { blocks: PostBlock[] }) {
  const { t } = useTranslation()
  if (!blocks || blocks.length === 0) return null
  return (
    <div className="mt-3 space-y-2" onClick={(e) => e.stopPropagation()}>
      {blocks.map((b, i) => {
        const key = `${b.type}-${b.url ?? b.name ?? i}-${b.audio_url ?? ""}`
        if (b.type === "music") {
          return <MusicCard key={key} block={b} />
        }
        if (b.type === "github") {
          return (
            <BlockShell
              key={key}
              icon={GitBranch}
              label={t("blocks.githubHeader")}
              href={b.url}
              action={t("blocks.open")}
            >
              <div className="min-w-0 p-3">
                <p className="truncate text-sm font-medium">
                  {b.title || t("blocks.githubFallback")}
                </p>
                <p className="text-muted-foreground mt-0.5 truncate text-xs">{b.url}</p>
              </div>
            </BlockShell>
          )
        }
        if (b.type === "website" && b.url) {
          return <WebsiteCard key={key} block={b} />
        }
        if (b.type === "video" && b.url) {
          return <VideoCard key={key} block={b} />
        }
        if (!b.name) return null
        return <LocationCard key={key} block={b} />
      })}
    </div>
  )
}

/** 位置链接：坐标直连地图，文本走 OpenStreetMap 搜索。 */
function osmURL(loc: string): string {
  const m = /^(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)$/.exec(loc.trim())
  if (m) return `https://www.openstreetmap.org/?mlat=${m[1]}&mlon=${m[2]}#map=15/${m[1]}/${m[2]}`
  return "https://www.openstreetmap.org/search?query=" + encodeURIComponent(loc)
}

/** 按日分组的标题文案。 */
function dayLabel(ts: number, t: (k: string, v?: Record<string, unknown>) => string): string {
  const d = new Date(ts * 1000)
  const now = new Date()
  const startOf = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const diffDays = Math.round((startOf(now) - startOf(d)) / 86400000)
  if (diffDays === 0) return t("time.today")
  if (diffDays === 1) return t("time.yesterday")
  return t("time.date", { m: d.getMonth() + 1, d: d.getDate() })
}

export { dayLabel }
