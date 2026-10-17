import { Globe2, Play, Video } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import type { PostBlock } from "@/api/client"
import { BlockShell } from "@/components/block-shell"
import { videoEmbed } from "@/lib/video"

function hostOf(raw?: string): string {
  try {
    return new URL(raw ?? "").hostname.replace(/^www\./, "")
  } catch {
    return raw ?? ""
  }
}

/** 显式网站卡片：标题/封面来自 link_previews，缓存未命中时回退域名。 */
export function WebsiteCard({ block }: { block: PostBlock }) {
  const { t } = useTranslation()
  const [coverOK, setCoverOK] = useState(true)
  const host = hostOf(block.url)

  return (
    <BlockShell
      icon={Globe2}
      label={t("blocks.websiteHeader")}
      href={block.url}
      action={t("blocks.open")}
    >
      <a
        href={block.url}
        target="_blank"
        rel="noopener noreferrer"
        className="hover:bg-accent/60 flex items-center gap-3 p-3 transition-colors"
      >
        {block.cover && coverOK && (
          <img
            src={block.cover}
            alt=""
            loading="lazy"
            referrerPolicy="no-referrer"
            onError={() => setCoverOK(false)}
            className="bg-muted h-14 w-20 shrink-0 rounded-lg object-cover"
          />
        )}
        <span className="min-w-0">
          <span className="block truncate text-sm font-medium">{block.title || host}</span>
          {block.title && (
            <span className="text-muted-foreground mt-0.5 block truncate text-xs">{host}</span>
          )}
        </span>
      </a>
    </BlockShell>
  )
}

/** 显式视频卡片：点击封面后才挂载 16:9 iframe。 */
export function VideoCard({ block }: { block: PostBlock }) {
  const { t } = useTranslation()
  const [loaded, setLoaded] = useState(false)
  const [coverOK, setCoverOK] = useState(true)
  const local = videoEmbed(block.url ?? "")
  const embed = block.embed_url?.startsWith("http") ? block.embed_url : local?.embed
  const provider =
    block.provider === "youtube"
      ? "YouTube"
      : block.provider === "bilibili"
        ? t("blocks.providerBilibili")
        : t("blocks.videoHeader")

  return (
    <BlockShell icon={Video} label={t("blocks.videoHeader")} href={block.url} action={provider}>
      {loaded && embed ? (
        <iframe
          title={block.title || t("blocks.videoHeader")}
          src={embed}
          className="aspect-video w-full"
          allow="autoplay; encrypted-media; picture-in-picture; fullscreen"
          allowFullScreen
          loading="lazy"
          referrerPolicy="no-referrer"
        />
      ) : (
        <button
          type="button"
          aria-label={t("blocks.playVideo")}
          className="group relative block aspect-video w-full overflow-hidden bg-black/80 text-left"
          onClick={() => {
            if (embed) setLoaded(true)
            else window.open(block.url, "_blank", "noopener,noreferrer")
          }}
        >
          {block.cover && coverOK ? (
            <img
              src={block.cover}
              alt=""
              loading="lazy"
              referrerPolicy="no-referrer"
              onError={() => setCoverOK(false)}
              className="size-full object-cover opacity-90 transition-transform duration-300 group-hover:scale-[1.02]"
            />
          ) : (
            <span className="from-primary/30 flex size-full items-center justify-center bg-gradient-to-br to-black/60" />
          )}
          <span className="absolute inset-0 flex items-center justify-center">
            <span className="bg-background/90 text-foreground grid size-12 place-items-center rounded-full shadow-lg transition-transform group-hover:scale-105">
              <Play className="size-5 translate-x-px" />
            </span>
          </span>
          {block.title && (
            <span className="absolute inset-x-0 bottom-0 line-clamp-2 bg-gradient-to-t from-black/80 to-transparent px-3 pt-8 pb-2 text-xs font-medium text-white">
              {block.title}
            </span>
          )}
        </button>
      )}
    </BlockShell>
  )
}
