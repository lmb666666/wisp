import { ChevronDown, MapPin } from "lucide-react"
import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import type { PostBlock } from "@/api/client"
import { BlockShell } from "@/components/block-shell"
import { useClickOutside } from "@/hooks/use-click-outside"
import { formatCoords, mapEmbedURL, osmURL, parseCoords, type Coords } from "@/lib/geo"

/** 位置卡：显示坐标，点击后才加载 OSM 地图（隐私优先）。 */
export function LocationCard({ block }: { block: PostBlock }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  useClickOutside(rootRef, () => setOpen(false), open, triggerRef)

  const coords: Coords | null =
    typeof block.lat === "number" && typeof block.lng === "number"
      ? { lat: block.lat, lng: block.lng }
      : parseCoords(block.name ?? "")
  const href = osmURL(block.name ?? "", coords)

  return (
    <div ref={rootRef}>
      <BlockShell
        icon={MapPin}
        label={t("blocks.locationHeader")}
        href={href}
        action={t("blocks.locationLabel")}
      >
        {coords ? (
          <>
            <button
              ref={triggerRef}
              type="button"
              aria-expanded={open}
              aria-label={open ? t("blocks.mapCollapse") : t("blocks.mapExpand")}
              className="hover:bg-accent/60 flex w-full items-center gap-2 p-3 text-left transition-colors"
              onClick={() => setOpen((v) => !v)}
            >
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm">{block.name}</span>
                <span className="text-muted-foreground mt-0.5 block text-[11px] tabular-nums">
                  {formatCoords(coords)}
                </span>
              </span>
              <ChevronDown
                className={`text-muted-foreground size-4 shrink-0 transition-transform ${open ? "rotate-180" : ""}`}
              />
            </button>
            {open && (
              <div className="border-border/60 border-t">
                <iframe
                  title={block.name}
                  src={mapEmbedURL(coords)}
                  className="aspect-video w-full"
                  loading="lazy"
                  referrerPolicy="no-referrer"
                />
              </div>
            )}
          </>
        ) : (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="hover:bg-accent/60 block truncate p-3 text-sm transition-colors"
          >
            {block.name}
          </a>
        )}
      </BlockShell>
    </div>
  )
}
